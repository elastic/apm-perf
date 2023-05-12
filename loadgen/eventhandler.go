// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package loadgen

import (
	"embed"
	"math/rand"
	"path/filepath"

	"golang.org/x/time/rate"

	"github.com/elastic/apm-perf/loadgen/eventhandler"

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
	RewriteIDs                bool
	RewriteServiceNames       bool
	RewriteServiceNodeNames   bool
	RewriteServiceTargetNames bool
	RewriteSpanNames          bool
	RewriteTransactionNames   bool
	RewriteTransactionTypes   bool
	RewriteTimestamps         bool
	Headers                   map[string]string
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
