// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/elastic/apm-perf/internal/loadgen"
	loadgencfg "github.com/elastic/apm-perf/internal/loadgen/config"
	"github.com/elastic/apm-perf/internal/loadgen/eventhandler"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"go.elastic.co/apm/v2"
	"go.elastic.co/apm/v2/transport"
)

func Benchmark1000Transactions(b *testing.B, l *rate.Limiter) {
	b.RunParallel(func(pb *testing.PB) {
		tracer := newTracer(b)
		for pb.Next() {
			for i := 0; i < 1000; i++ {
				if err := l.Wait(context.Background()); err != nil {
					// panicing ensures that the error is reported
					// see: https://github.com/golang/go/issues/32066
					panic(err)
				}
				tracer.StartTransaction("name", "type").End()
			}
			// TODO(axw) implement a transport that enables streaming
			// events in a way that we can block when the queue is full,
			// without flushing. Alternatively, make this an option in
			// TracerOptions?
			tracer.Flush(nil)
		}
	})
}

func BenchmarkAgentAll(b *testing.B, l *rate.Limiter) {
	benchmarkAgent(b, l, `apm-*.ndjson`)
}

func BenchmarkAgentGo(b *testing.B, l *rate.Limiter) {
	benchmarkAgent(b, l, `apm-go*.ndjson`)
}

func BenchmarkAgentNodeJS(b *testing.B, l *rate.Limiter) {
	benchmarkAgent(b, l, `apm-nodejs*.ndjson`)
}

func BenchmarkAgentPython(b *testing.B, l *rate.Limiter) {
	benchmarkAgent(b, l, `apm-python*.ndjson`)
}

func BenchmarkAgentRuby(b *testing.B, l *rate.Limiter) {
	benchmarkAgent(b, l, `apm-ruby*.ndjson`)
}

func Benchmark10000AggregationGroups(b *testing.B, l *rate.Limiter) {
	// Benchmark memory usage on aggregating high cardinality data.
	// This should generate a lot of groups for service transaction metrics,
	// transaction metrics, and service destination metrics.
	//
	// Using b.N instead of b.RunParallel since this benchmark is about memory
	// usage.
	//
	// If rate limiter is used, it is possible that part of the 10k
	// transactions will not fit into the same 1m aggregation period, and this
	// will cause a lower observed memory usage.
	for n := 0; n < b.N; n++ {
		tracer := newTracer(b)
		for i := 0; i < 10000; i++ {
			if err := l.Wait(context.Background()); err != nil {
				// panicing ensures that the error is reported
				// see: https://github.com/golang/go/issues/32066
				panic(err)
			}
			tx := tracer.StartTransaction(fmt.Sprintf("name%d", i), fmt.Sprintf("type%d", i))
			span := tx.StartSpanOptions(fmt.Sprintf("name%d", i), fmt.Sprintf("type%d", i), apm.SpanOptions{})
			span.Context.SetServiceTarget(apm.ServiceTargetSpanContext{
				Name: fmt.Sprintf("name%d", i),
				Type: fmt.Sprintf("resource%d", i),
			})
			span.Duration = time.Second
			span.End()
			tx.End()
		}
		tracer.Flush(nil)
	}
}

func newTracer(tb testing.TB) *apm.Tracer {
	httpTransport, err := transport.NewHTTPTransport(transport.HTTPTransportOptions{
		ServerURLs:  []*url.URL{loadgencfg.Config.ServerURL},
		APIKey:      loadgencfg.Config.APIKey,
		SecretToken: loadgencfg.Config.SecretToken,
	})
	if err != nil {
		// panicing ensures that the error is reported
		// see: https://github.com/golang/go/issues/32066
		panic(err)
	}
	tracer, err := apm.NewTracerOptions(apm.TracerOptions{
		Transport: httpTransport,
	})
	if err != nil {
		// panicing ensures that the error is reported
		// see: https://github.com/golang/go/issues/32066
		panic(err)
	}
	tb.Cleanup(tracer.Close)
	return tracer
}

func newEventHandler(tb testing.TB, p string, l *rate.Limiter) *eventhandler.Handler {
	protocol := "apm/http"
	if strings.HasPrefix(p, "otlp-") {
		protocol = "otlp/http"
	}
	h, err := loadgen.NewEventHandler(loadgen.EventHandlerParams{
		Logger:            zap.NewNop(),
		Path:              p,
		Limiter:           l,
		URL:               loadgencfg.Config.ServerURL.String(),
		Token:             loadgencfg.Config.SecretToken,
		APIKey:            loadgencfg.Config.APIKey,
		IgnoreErrors:      loadgencfg.Config.IgnoreErrors,
		RewriteIDs:        loadgencfg.Config.RewriteIDs,
		RewriteTimestamps: loadgencfg.Config.RewriteTimestamps,
		Headers:           loadgencfg.Config.Headers,
		Protocol:          protocol,
	})
	if err != nil {
		// panicing ensures that the error is reported
		// see: https://github.com/golang/go/issues/32066
		panic(err)
	}
	return h
}

func benchmarkAgent(b *testing.B, l *rate.Limiter, expr string) {
	h := newEventHandler(b, expr, l)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := h.SendBatches(context.Background())
			if err != nil {
				// panicing ensures that the error is reported
				// see: https://github.com/golang/go/issues/32066
				panic(fmt.Sprintf("failed to send batches: %+v", err))
			}
		}
	})
}
