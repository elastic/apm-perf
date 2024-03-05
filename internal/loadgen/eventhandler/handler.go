// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package eventhandler contains code for replaying batch of agent data to APM
// server.
package eventhandler

import (
	"bufio"
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"math/bits"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/klauspost/compress/zlib"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"go.elastic.co/fastjson"
	"go.uber.org/zap"
	"golang.org/x/time/rate"
)

var (
	newlineBytes = []byte("\n")

	// supportedTSFormats lists variations of RFC3339 for supporting
	// different formats for the timezone offset. Copied from apm-data.
	supportedTSFormats = []string{
		"2006-01-02T15:04:05Z07:00", // RFC3339
		"2006-01-02T15:04:05Z0700",
		"2006-01-02T15:04:05Z07",
	}
)

type batch struct {
	metadata []byte
	events   []event
}

type event struct {
	payload    []byte
	objectType string
	timestamp  time.Time
}

// Handler replays stored events to an APM Server.
//
// It is safe to make concurrent calls to Handler methods.
type Handler struct {
	mu         sync.Mutex // guards config.Rand
	config     Config
	writerPool sync.Pool
	logger     *zap.Logger

	batches      []batch
	minTimestamp time.Time // across all batches
}

// Config holds configuration for Handler.
type Config struct {
	// Path holds the path to a file within Storage, holding recorded
	// event batches for replaying. Path may be a glob, in which case
	// it may match more than one file.
	//
	// The file contents are expected to hold ND-JSON event streams in
	// plain text. These may be transformed, and will be compressed
	// (with zlib.BestSpeed to minimise overhead), during replay.
	Path string

	// Storage holds the fs.FS from which the file specified by Path
	// will read, for extracting batches.
	Storage fs.FS

	// Limiter holds a rate.Limiter for controlling the event rate.
	//
	// If Limiter is nil, an infinite rate limiter will be used.
	Limiter *rate.Limiter

	// Transport holds the Transport that will be used for replaying
	// event batches.
	Transport Transport

	// Rand, if non-nil, will be used for field randomization.
	//
	// If Rand is nil, then a new Rand will be created, seeded with
	// a value read from crypto/rand. If Rand is supplied (non-nil),
	// it must not be invoked concurrently by any other goroutines.
	Rand *rand.Rand

	// IgnoreErrors when set to true ignores HTTP errors while sending
	// events using the event handler.
	IgnoreErrors bool

	// RunForever when set to true, will keep the handler running
	// until a signal is received to stop it.
	RunForever bool

	// Writer writes replayable events to buffer.
	Writer EventWriter

	// RewriteTimestamps controls whether event timestamps are rewritten
	// during replay.
	//
	// If this is false, then the original timestamps are unmodified.
	//
	// If this is true, then for each call to SendBatches the timestamps
	// be adjusted as follows: first we calculate the smallest timestamp
	// across all of the batches, and then compute the delta between an
	// event's timestamp and the smallest timestamp; this is then added
	// to the current time as recorded when SendBatches is invoked.
	RewriteTimestamps bool

	// RewriteIDs controls whether trace, transaction, span, and error
	// event IDs are rewritten.
	//
	// If this is false, then the original IDs are unmodified.
	//
	// If this is true, then for each call to SendBatches the event IDs
	// will be adjusted as follows: first, we generate a random 64-bit
	// integer. Next, for each event ID whose value is entirely hex
	// digits, we will decode each hex digit to a 4-bit value, XOR it
	// with the next 4 bits in the randon number (rotating left), and
	// then re-encode as a hex digit. Note that we intentionally use a
	// single random value to ensure IDs are randomised consistently,
	// such that event relationships are maintained.
	RewriteIDs bool

	// RewriteServiceNames controls the rewriting of `service.name`
	// in events.
	RewriteServiceNames bool

	// RewriteServiceNodeNames controls the rewriting of
	// `service.node.name` in events.
	RewriteServiceNodeNames bool

	// RewriteServiceTargetNames controls the rewriting of
	// `service.target.name` in events.
	RewriteServiceTargetNames bool

	// RewriteSpanNames controls the rewriting of `span.name` in events.
	RewriteSpanNames bool

	// RewriteTransactionNames controls the rewriting of `transaction.name`
	// in events.
	RewriteTransactionNames bool

	// RewriteTransactionTypes controls the rewriting of `transaction.type`
	// in events.
	RewriteTransactionTypes bool
}

// New creates a new Handler with config.
func New(logger *zap.Logger, config Config, ec EventCollector) (*Handler, error) {
	if config.Transport == nil {
		return nil, errors.New("empty transport received")
	}
	if config.Limiter == nil {
		config.Limiter = rate.NewLimiter(rate.Inf, 0)
	}
	if config.Rand == nil {
		var rngseed int64
		err := binary.Read(cryptorand.Reader, binary.LittleEndian, &rngseed)
		if err != nil {
			return nil, fmt.Errorf("failed to generate seed for math/rand: %w", err)
		}
		config.Rand = rand.New(rand.NewSource(rngseed))
	}
	if config.Writer == nil {
		return nil, errors.New("empty writer received")
	}

	h := Handler{
		logger: logger.Named("handler"),
		config: config,
		writerPool: sync.Pool{
			New: func() any {
				pw := &pooledWriter{}
				zw, err := zlib.NewWriterLevel(&pw.buf, zlib.BestSpeed)
				if err != nil {
					logger.Panic(err.Error())
				}
				pw.Writer = zw
				return pw
			},
		},
	}

	matches, err := fs.Glob(config.Storage, config.Path)
	if err != nil {
		return nil, err
	}

	logger.Debug("file matching pattern found for batch extraction", zap.String("matches", strings.Join(matches, ",")))
	for _, path := range matches {
		f, err := config.Storage.Open(path)
		if err != nil {
			return nil, err
		}

		s := bufio.NewScanner(f)
		var current *batch
		for s.Scan() {
			line := s.Bytes()
			if len(line) == 0 {
				continue
			}

			// TODO(marclop): Suppport RUM headers and handle them differently.
			if err := ec.Filter(line); err != nil {
				return nil, fmt.Errorf("line filter failed: %w", err)
			}

			// Copy the line, as it will be overwritten by the next scan.
			linecopy := make([]byte, len(line))
			copy(linecopy, line)
			// if the line is meta, create a new batch
			if ec.IsMeta(line) {
				h.batches = append(h.batches, batch{
					metadata: linecopy,
				})
				current = &h.batches[len(h.batches)-1]
			} else {
				event := ec.Process(linecopy)
				if !event.timestamp.IsZero() {
					if h.minTimestamp.IsZero() || event.timestamp.Before(h.minTimestamp) {
						h.minTimestamp = event.timestamp
					}
				}
				current.events = append(current.events, event)
			}
		}
	}
	if len(h.batches) == 0 {
		return nil, errors.New("eventhandler: glob matched no files, please specify a valid glob pattern")
	}

	logger.Debug("collected batches", zap.Int("size", len(h.batches)))
	return &h, nil
}

// SendBatchesInLoop will send events in loop, such that it can be used to warm up the remote APM Server,
// by sending events until the context is canceled.
func (h *Handler) SendBatchesInLoop(ctx context.Context) error {
	// state is created here because the total number of events in h.batches can be smaller than the burst
	// and it can lead the sendBatches to finish its cycle without sending the desired burst number of events.
	// If we keep the state within sendBatches, it will wait for the interval t whenever starting the function as
	// `s.sent` is set to 0 so it needs to know the previous run's state

	s := state{
		burst: h.config.Limiter.Burst(),
	}

	// Send events in batches and when getting an error restart from where
	// we left.
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if _, err := h.sendBatches(ctx, &s); err != nil {
				if h.config.RunForever {
					h.logger.Error("failed to send batch of events", zap.Error(err))
					continue
				}
				return err
			}
			// safeguard `s.sent` so that it doesn't exceed math.MaxInt
			// but keep the remainder so the next batches know where to start
			if s.burst > 0 {
				s.sent = s.sent % s.burst
			}
		}
	}
}

// SendBatches sends the loaded trace data to the configured transport,
// returning the total number of documents sent and any transport errors.
func (h *Handler) SendBatches(ctx context.Context) (int, error) {
	return h.sendBatches(ctx, &state{
		burst: h.config.Limiter.Burst(),
	})
}

func (h *Handler) sendBatches(ctx context.Context, s *state) (int, error) {
	h.mu.Lock()
	randomBits := h.config.Rand.Uint64()
	h.mu.Unlock()
	h.logger.Debug("got random bits for batch")

	s.w = h.writerPool.Get().(*pooledWriter)
	defer h.writerPool.Put(s.w)
	defer s.w.Reset()
	h.logger.Debug("got writer from pool")

	baseTimestamp := time.Now().UTC()
	h.logger.Debug("calculated base timestamp", zap.String("timestamp", baseTimestamp.String()))
	for _, batch := range h.batches {
		if err := h.sendBatch(ctx, s, batch, baseTimestamp, randomBits); err != nil {
			return s.sent, err
		}
	}
	return s.sent, nil
}

func (h *Handler) sendBatch(
	ctx context.Context,
	s *state,
	b batch,
	baseTimestamp time.Time,
	randomBits uint64,
) error {
	events := b.events
	h.logger.Debug("sending events in burst", zap.Int("size", len(events)))

	for len(events) > 0 {
		n := len(events)
		if s.burst > 0 {
			mod := s.sent % s.burst
			if mod == 0 {
				// We're starting a new iteration, so wait to send a burst.
				if err := h.config.Limiter.WaitN(ctx, s.burst); err != nil {
					return err
				}
			}
			// Send as many events of the batch as we can, up to the
			// burst size minus however many events have been sent for
			// this iteration.
			capacity := s.burst - mod
			h.logger.Debug("calculated capacity", zap.Int("value", capacity))

			if n > capacity {
				n = capacity
			}
		}
		if err := h.config.Writer(h.config, h.minTimestamp,
			s.w, batch{
				metadata: b.metadata,
				events:   events[:n],
			}, baseTimestamp, randomBits); err != nil {
			return err
		}
		h.logger.Debug("wrote events to buffer")

		if err := s.w.Close(); err != nil {
			return err
		}
		h.logger.Debug("closed writer")

		if err := h.config.Transport.SendEvents(ctx, &s.w.buf, h.config.IgnoreErrors); err != nil {
			return err
		}
		h.logger.Debug("sent events through transport")

		s.w.Reset()
		s.sent += n
		events = events[n:]
	}
	return nil
}

func rewriteJSONObject(w *pooledWriter, object gjson.Result, f func(key, value gjson.Result) bool) {
	w.rewriteBuf.RawByte('{')
	first := true
	object.ForEach(func(key, value gjson.Result) bool {
		if first {
			first = false
		} else {
			w.rewriteBuf.RawByte(',')
		}
		w.rewriteBuf.RawString(key.Raw)
		w.rewriteBuf.RawByte(':')
		return f(key, value)
	})
	w.rewriteBuf.RawByte('}')
}

func randomizeTraceID(out *bytes.Buffer, in string, randomBits uint64) bool {
	n := len(in)
	for i := 0; i < n; i++ {
		b := in[i]
		if !((b >= '0' && b <= '9') ||
			(b >= 'a' && b <= 'f') ||
			(b >= 'A' && b <= 'F')) {
			// Not all hex.
			return false
		}
	}

	out.Grow(n)
	for i := 0; i < n; i++ {
		b := in[i]

		var h uint8
		switch {
		case b >= '0' && b <= '9':
			h = b - '0'
		case b >= 'a' && b <= 'f':
			h = 10 + (b - 'a')
		case b >= 'A' && b <= 'F':
			h = 10 + (b - 'A')
		}
		h = (h ^ uint8(randomBits)) & 0x0f
		randomBits = bits.RotateLeft64(randomBits, 4)

		if h < 10 {
			b = '0' + h
		} else {
			b = 'a' + (h - 10)
		}
		out.WriteByte(b)
	}
	return true
}

func randomizeASCIIField(data []byte, path string, randomBits uint64, scratch *bytes.Buffer) ([]byte, error) {
	result := gjson.GetBytes(data, path)
	if !result.Exists() {
		return data, nil
	}
	randomizeASCII(scratch, result.Str, randomBits)
	defer scratch.Reset()

	data, err := sjson.SetBytes(data, path, scratch.String())
	if err != nil {
		return nil, fmt.Errorf("failed to rewrite %q: %w", path, err)
	}
	return data, nil
}

// randomizeASCII replaces ASCII letter and digit runes in the input
// with random ASCII runes in the same category.
func randomizeASCII(out *bytes.Buffer, in string, randomBits uint64) {
	for _, r := range in {
		// '0' > 'A' > 'a'
		if r < '0' || r > 'z' {
			out.WriteRune(r)
			continue
		}
		// Use 5 bits, which is enough to cover either
		// 26 ASCII letters or 10 ASCII digits.
		i := (uint8(randomBits) & 0x1f)
		randomBits = bits.RotateLeft64(randomBits, 5)
		switch {
		case r >= 'a':
			r = rune('a' + i%26)
		case r >= 'A' && r <= 'Z':
			r = rune('A' + i%26)
		case r <= '9':
			r = rune('0' + i%10)
		}
		out.WriteRune(r)
	}
}

type state struct {
	w     *pooledWriter
	burst int
	sent  int
}

type pooledWriter struct {
	rewriteBuf fastjson.Writer
	idBuf      bytes.Buffer
	buf        bytes.Buffer
	*zlib.Writer
}

func (pw *pooledWriter) Reset() {
	pw.rewriteBuf.Reset()
	pw.idBuf.Reset()
	pw.buf.Reset()
	pw.Writer.Reset(&pw.buf)
}
