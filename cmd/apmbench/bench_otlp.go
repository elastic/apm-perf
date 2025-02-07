// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"runtime"
	"testing"
	"time"

	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/elastic/apm-perf/internal/telemetrygen/common"
	"github.com/elastic/apm-perf/internal/telemetrygen/logs"
	"github.com/elastic/apm-perf/internal/telemetrygen/metrics"
	"github.com/elastic/apm-perf/internal/telemetrygen/traces"
	loadgencfg "github.com/elastic/apm-perf/loadgen/config"
)

func commonConfigWithHTTPPath(httpPath string) common.Config {
	insecure := !loadgencfg.Config.Secure

	serverURL := loadgencfg.Config.ServerURL
	endpoint := serverURL.Host
	if serverURL.Port() == "" {
		switch serverURL.Scheme {
		case "http":
			endpoint += ":80"
			insecure = true
		case "https":
			endpoint += ":443"
		}
	}

	secretToken := loadgencfg.Config.SecretToken
	apiKey := loadgencfg.Config.APIKey
	headers := make(map[string]string)
	for k, v := range loadgencfg.Config.Headers {
		headers[k] = v
	}
	if secretToken != "" || apiKey != "" {
		if apiKey != "" {
			// higher priority to APIKey auth
			headers["Authorization"] = "ApiKey " + apiKey
		} else {
			headers["Authorization"] = "Bearer " + secretToken
		}
	}

	return common.Config{
		WorkerCount:           runtime.GOMAXPROCS(0),
		Rate:                  0,
		TotalDuration:         0,
		ReportingInterval:     0,
		SkipSettingGRPCLogger: true,
		CustomEndpoint:        endpoint,
		Insecure:              insecure,
		UseHTTP:               false,
		HTTPPath:              httpPath,
		Headers:               headers,
		ResourceAttributes:    nil,
		TelemetryAttributes:   nil,
		CaFile:                "",
		ClientAuth:            common.ClientAuth{},
		Logger:                zap.NewNop(),
	}
}

func BenchmarkOTLPLogs(b *testing.B, l *rate.Limiter) {
	config := logs.Config{
		Config:  commonConfigWithHTTPPath("/v1/logs"),
		NumLogs: b.N,
		Body:    "test",
	}
	if err := logs.Start(&config); err != nil {
		b.Fatal(err)
	}
}

func BenchmarkOTLPTraces(b *testing.B, l *rate.Limiter) {
	config := traces.Config{
		Config:           commonConfigWithHTTPPath("/v1/traces"),
		NumTraces:        b.N,
		NumChildSpans:    1,
		PropagateContext: false,
		ServiceName:      "foo",
		StatusCode:       "0",
		Batch:            true,
		LoadSize:         0,
		SpanDuration:     123 * time.Microsecond,
	}
	if err := traces.Start(&config); err != nil {
		b.Fatal(err)
	}
}

func BenchmarkOTLPMetrics(b *testing.B, l *rate.Limiter) {
	config := metrics.Config{
		Config:     commonConfigWithHTTPPath("/v1/metrics"),
		NumMetrics: b.N,
		MetricType: "Sum",
	}
	if err := metrics.Start(&config); err != nil {
		b.Fatal(err)
	}
}
