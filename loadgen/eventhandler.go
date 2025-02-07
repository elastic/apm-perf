// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package loadgen contains code for generating load based on real agent data.
package loadgen

import (
	"embed"
	"fmt"
	"math/rand"
	"path/filepath"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"

	"github.com/elastic/apm-perf/loadgen/eventhandler"

	"go.elastic.co/apm/v2/transport"
)

// events holds the current stored events.
//
//go:embed events/*.ndjson
var events embed.FS

type EventHandlerParams struct {
	Logger *zap.Logger

	Path                      string
	URL                       string
	Token                     string
	APIKey                    string
	Limiter                   *rate.Limiter
	Rand                      *rand.Rand
	IgnoreErrors              bool
	RunForever                bool
	RewriteIDs                bool
	RewriteServiceNames       bool
	RewriteServiceNodeNames   bool
	RewriteServiceTargetNames bool
	RewriteSpanNames          bool
	RewriteTransactionNames   bool
	RewriteTransactionTypes   bool
	RewriteTimestamps         bool
	// Headers contains HTTP headers shipped with all requests.
	// NOTE: these headers are not sanitized in logs.
	Headers map[string]string

	// One of: apm/http, otlp/http
	// NOTE: otlp/grpc is not supported
	Protocol string
	// One of: any, logs, metrics, traces
	// NOTE: for Protocol apm/http there is no difference
	// between each value. When using Protocol otlp/http
	// each data type requires a separate EventHandler.
	Datatype string
}

func (e EventHandlerParams) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	// NOTE: Logger is ignored.
	enc.AddString("path", e.Path)
	enc.AddString("url", e.URL)
	enc.AddString("token", "REDACTED")
	enc.AddString("api_key", "REDACTED")
	// FIXME: add Limiter.
	// FIXME: add Rand.
	enc.AddBool("ignore_errors", e.IgnoreErrors)
	enc.AddBool("rewrite_ids", e.RewriteIDs)
	enc.AddBool("rewrite_service_names", e.RewriteServiceNames)
	enc.AddBool("rewrite_service_node_names", e.RewriteServiceNodeNames)
	enc.AddBool("rewrite_service_target_names", e.RewriteServiceTargetNames)
	enc.AddBool("rewrite_span_names", e.RewriteSpanNames)
	enc.AddBool("rewrite_transaction_names", e.RewriteTransactionNames)
	enc.AddBool("rewrite_transaction_tpes", e.RewriteTransactionTypes)
	enc.AddBool("rewrite_timestamps", e.RewriteTimestamps)

	for k, v := range e.Headers {
		enc.AddString(k, v)
	}

	return nil
}

// NewEventHandler creates a eventhandler which loads the files matching the
// passed regex.
func NewEventHandler(p EventHandlerParams) (*eventhandler.Handler, error) {
	if p.Logger == nil {
		return nil, fmt.Errorf("nil logger in params")
	}
	switch p.Protocol {
	case "apm/http":
		return newAPMEventHandler(p)
	case "otlp/http":
		// TODO: support OTLP event handling
		// switch p.Datatype {
		// case "logs":
		// case "metrics":
		// case "traces":
		// }

		return nil, fmt.Errorf("invalid datatype (%s) for protocol (%s)", p.Datatype, p.Protocol)
	}

	return nil, fmt.Errorf("invalid or unsupported protocol (%s)", p.Protocol)
}

func newAPMEventHandler(p EventHandlerParams) (*eventhandler.Handler, error) {
	// We call the HTTPTransport constructor to avoid copying all the config
	// parsing that creates the `*http.Client`.
	t, err := transport.NewHTTPTransport(transport.HTTPTransportOptions{})
	if err != nil {
		return nil, fmt.Errorf("cannot create HTTP transport: %w", err)
	}

	c := eventhandler.Config{
		Path:                      filepath.Join("events", p.Path),
		Transport:                 eventhandler.NewAPMTransport(p.Logger, t.Client, p.URL, p.Token, p.APIKey, p.Headers),
		Storage:                   events,
		Limiter:                   p.Limiter,
		Rand:                      p.Rand,
		IgnoreErrors:              p.IgnoreErrors,
		RunForever:                p.RunForever,
		RewriteIDs:                p.RewriteIDs,
		RewriteServiceNames:       p.RewriteServiceNames,
		RewriteServiceNodeNames:   p.RewriteServiceNodeNames,
		RewriteServiceTargetNames: p.RewriteServiceTargetNames,
		RewriteSpanNames:          p.RewriteSpanNames,
		RewriteTransactionNames:   p.RewriteTransactionNames,
		RewriteTransactionTypes:   p.RewriteTransactionTypes,
		RewriteTimestamps:         p.RewriteTimestamps,
	}

	return eventhandler.NewAPM(p.Logger, c)
}
