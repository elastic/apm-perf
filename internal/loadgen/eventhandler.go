// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package loadgen contains code for generating load based on real agent data.
package loadgen

import (
	"embed"
	"math/rand"
	"path/filepath"

	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"

	"github.com/elastic/apm-perf/internal/loadgen/eventhandler"

	"go.elastic.co/apm/v2/transport"
)

// events holds the current stored events.
//
//go:embed events/*.ndjson
var events embed.FS

type EventHandlerParams struct {
	Path                      string
	URL                       string
	Token                     string
	APIKey                    string
	Limiter                   *rate.Limiter
	Rand                      *rand.Rand
	IgnoreErrors              bool
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
	enc.AddBool("rewrite_ids", e.RewriteServiceNodeNames)
	enc.AddBool("rewrite_ids", e.RewriteServiceTargetNames)
	enc.AddBool("rewrite_ids", e.RewriteSpanNames)
	enc.AddBool("rewrite_ids", e.RewriteTransactionNames)
	enc.AddBool("rewrite_ids", e.RewriteTransactionTypes)
	enc.AddBool("rewrite_ids", e.RewriteTimestamps)

	for k, v := range e.Headers {
		enc.AddString(k, v)
	}

	return nil
}

// NewEventHandler creates a eventhandler which loads the files matching the
// passed regex.
func NewEventHandler(p EventHandlerParams) (*eventhandler.Handler, error) {
	// We call the HTTPTransport constructor to avoid copying all the config
	// parsing that creates the `*http.Client`.
	t, err := transport.NewHTTPTransport(transport.HTTPTransportOptions{})
	if err != nil {
		return nil, err
	}
	transp := eventhandler.NewTransport(t.Client, p.URL, p.Token, p.APIKey, p.Headers)
	return eventhandler.New(eventhandler.Config{
		Path:                      filepath.Join("events", p.Path),
		Transport:                 transp,
		Storage:                   events,
		Limiter:                   p.Limiter,
		Rand:                      p.Rand,
		IgnoreErrors:              p.IgnoreErrors,
		RewriteIDs:                p.RewriteIDs,
		RewriteServiceNames:       p.RewriteServiceNames,
		RewriteServiceNodeNames:   p.RewriteServiceNodeNames,
		RewriteServiceTargetNames: p.RewriteServiceTargetNames,
		RewriteSpanNames:          p.RewriteSpanNames,
		RewriteTransactionNames:   p.RewriteTransactionNames,
		RewriteTransactionTypes:   p.RewriteTransactionTypes,
		RewriteTimestamps:         p.RewriteTimestamps,
	})
}
