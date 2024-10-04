// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package proxy contains the code to build http proxy for apm.
package proxy

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"
	"go.uber.org/zap"
)

const (
	defaultLicense = `{"license":{"uid":"cc49813b-6b8e-2138-fbb8-243ae2b3deed","type":"enterprise","status":"active"}}`
	defaultInfo    = `{
"name": "instance-0000000001",
"cluster_name": "eca3b3c3bbee4816bb92f82184e328dd",
"cluster_uuid": "cc49813b-6b8e-2138-fbb8-243ae2b3deed",
"version": {
	"number": "8.15.1",
	"build_flavor": "default",
	"build_type": "docker",
	"build_hash": "253e8544a65ad44581194068936f2a5d57c2c051",
	"build_date": "2024-09-02T22:04:47.310170297Z",
	"build_snapshot": false,
	"lucene_version": "9.11.1",
	"minimum_wire_compatibility_version": "7.17.0",
	"minimum_index_compatibility_version": "7.0.0"
},
"tagline": "You Know, for Search"
}`
)

var memPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

type stubES struct {
	logger              *zap.Logger
	auth                string
	info, license       []byte
	unknownPathCallback http.HandlerFunc
}

func NewHandlerStubES(options ...StubESOption) http.Handler {
	h := new(stubES)
	options = append([]StubESOption{
		StubESWithLogger(zap.NewNop()),
		StubESWithInfo(defaultInfo),
		StubESWithLicense(defaultLicense),
		StubESWithUnknownPathCallback(func(w http.ResponseWriter, req *http.Request) {
			h.logger.Error("unknown path", zap.String("path", req.URL.Path))
		}),
	}, options...)
	for _, opt := range options {
		opt(h)
	}
	return h
}

func (h stubES) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("X-Elastic-Product", "Elasticsearch")
	auth, _ := strings.CutPrefix(req.Header.Get("Authorization"), "Basic ")
	if len(h.auth) > 0 && string(auth) != h.auth {
		h.logger.Error(
			"authentication failed",
			zap.String("actual", auth),
			zap.String("expected", h.auth),
		)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch req.URL.Path {
	case "/":
		_, _ = w.Write(h.info)
		return
	case "/_license":
		_, _ = w.Write(h.license)
		return
	case "/_bulk":
		first := true
		var body io.Reader
		switch req.Header.Get("Content-Encoding") {
		case "gzip":
			r, err := gzip.NewReader(req.Body)
			if err != nil {
				h.logger.Error("gzip reader err", zap.Error(err))
				http.Error(w, fmt.Sprintf("reader error: %v", err), http.StatusInternalServerError)
				return
			}
			defer r.Close()
			body = r
		case "zstd":
			r, err := zstd.NewReader(req.Body)
			if err != nil {
				h.logger.Error("zstd reader err", zap.Error(err))
				http.Error(w, fmt.Sprintf("reader error: %v", err), http.StatusInternalServerError)
				return
			}
			defer r.Close()
			body = r
		default:
			body = req.Body
		}

		jsonw := memPool.Get().(*bytes.Buffer)
		defer func() {
			jsonw.Reset()
			memPool.Put(jsonw)
		}()

		_, _ = jsonw.Write([]byte(`{"items":[`))
		scanner := bufio.NewScanner(body)
		for scanner.Scan() {
			// Action is always "create", skip decoding.
			if !scanner.Scan() {
				h.logger.Error("unexpected payload")
				http.Error(w, "expected source", http.StatusBadRequest)
				return
			}
			if first {
				first = false
			} else {
				_ = jsonw.WriteByte(',')
			}
			jsonw.Write([]byte(`{"create":{"status":201}}`))
		}
		if err := scanner.Err(); err != nil {
			h.logger.Error("scanner error", zap.Error(err))
			http.Error(w, fmt.Sprintf("scanner error: %v", err), http.StatusBadRequest)
		} else {
			jsonw.Write([]byte(`]}`))
			_, _ = w.Write(jsonw.Bytes())
		}
	default:
		h.unknownPathCallback(w, req)
	}
}

type StubESOption func(*stubES)

func StubESWithLogger(logger *zap.Logger) StubESOption {
	return func(h *stubES) {
		h.logger = logger
	}
}

func StubESWithAuth(username, password string) StubESOption {
	return func(h *stubES) {
		h.auth = base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
	}
}

func StubESWithInfo(info string) StubESOption {
	return func(h *stubES) {
		h.info = []byte(info)
	}
}

func StubESWithLicense(license string) StubESOption {
	return func(h *stubES) {
		h.license = []byte(license)
	}
}

func StubESWithUnknownPathCallback(callback http.HandlerFunc) StubESOption {
	return func(h *stubES) {
		h.unknownPathCallback = callback
	}
}
