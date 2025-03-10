// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package telemetrygen_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-perf/pkg/telemetrygen"
)

func TestGeneration(t *testing.T) {
	srv, m := newTestServer(t)

	cfg := telemetrygen.DefaultConfig()
	cfg.Secure = false

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	cfg.ServerURL = u

	cfg.EventRate.Set("1000/s")
	g, err := telemetrygen.New(cfg)
	// g.Logger = zap.Must(zap.NewDevelopment())
	require.NoError(t, err)

	err = g.RunBlocking(context.Background())
	require.NoError(t, err)
	require.Greater(t, m.Load(), int32(0))
}

func newTestServer(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	mux := http.NewServeMux()
	requestsReceived := &atomic.Int32{}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestsReceived.Add(1)
		w.Write([]byte("ok"))
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv, requestsReceived
}

func TestGenerationV7(t *testing.T) {
	srv, m := newTestServer(t)

	cfg := telemetrygen.DefaultConfig()
	cfg.Secure = false
	cfg.TargetV7APMServer = true

	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	cfg.ServerURL = u

	cfg.EventRate.Set("10/s")
	g, err := telemetrygen.New(cfg)
	// g.Logger = zap.Must(zap.NewDevelopment())
	require.NoError(t, err)

	err = g.RunBlocking(context.Background())
	require.NoError(t, err)
	require.Greater(t, m.Load(), int32(0))
}
