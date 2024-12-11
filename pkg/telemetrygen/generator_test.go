package telemetrygen_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync/atomic"
	"testing"

	"github.com/elastic/apm-perf/pkg/telemetrygen"
	"github.com/stretchr/testify/require"
)

func TestGeneration(t *testing.T) {
	srv, m := newTestServer(t, testServerConfig{http.StatusServiceUnavailable})

	cfg := telemetrygen.DefaultConfig()
	cfg.Secure = false

	u, err := url.Parse(srv.URL)
	cfg.ServerURL = u

	cfg.EventRate.Set("1000/s")
	g, err := telemetrygen.New(cfg)
	// g.Logger = zap.Must(zap.NewDevelopment())
	require.NoError(t, err)

	err = g.RunBlocking(context.Background())
	require.NoError(t, err)
	require.Greater(t, m.Load(), int32(0))
}

type testServerConfig struct {
	responseStatus int
}
type metrics struct {
	Received *atomic.Int32
}

func newTestServer(t *testing.T, cfg testServerConfig) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	mux := http.NewServeMux()
	requestsReceived := &atomic.Int32{}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestsReceived.Add(1)
		w.Write([]byte("ok"))
		return
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv, requestsReceived
}
