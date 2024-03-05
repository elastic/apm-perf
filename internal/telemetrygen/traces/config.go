// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// This file is forked from https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/cmd/telemetrygen/internal/traces/config.go,
// which is licensed under Apache-2 and Copyright The OpenTelemetry Authors.
//
// It modifies the original implementation to remove unnecessary files,
// and to accept a configurable logger.

package traces

import (
	"time"

	"github.com/spf13/pflag"

	"github.com/elastic/apm-perf/internal/telemetrygen/common"
)

// Config describes the test scenario.
type Config struct {
	common.Config
	NumTraces        int
	NumChildSpans    int
	PropagateContext bool
	ServiceName      string
	StatusCode       string
	Batch            bool
	LoadSize         int

	SpanDuration time.Duration
}

// Flags registers config flags.
func (c *Config) Flags(fs *pflag.FlagSet) {
	c.CommonFlags(fs)

	fs.StringVar(&c.HTTPPath, "otlp-http-url-path", "/v1/traces", "Which URL path to write to")

	fs.IntVar(&c.NumTraces, "traces", 1, "Number of traces to generate in each worker (ignored if duration is provided)")
	fs.IntVar(&c.NumChildSpans, "child-spans", 1, "Number of child spans to generate for each trace")
	fs.BoolVar(&c.PropagateContext, "marshal", false, "Whether to marshal trace context via HTTP headers")
	fs.StringVar(&c.ServiceName, "service", "telemetrygen", "Service name to use")
	fs.StringVar(&c.StatusCode, "status-code", "0", "Status code to use for the spans, one of (Unset, Error, Ok) or the equivalent integer (0,1,2)")
	fs.BoolVar(&c.Batch, "batch", true, "Whether to batch traces")
	fs.IntVar(&c.LoadSize, "size", 0, "Desired minimum size in MB of string data for each trace generated. This can be used to test traces with large payloads, i.e. when testing the OTLP receiver endpoint max receive size.")
	fs.DurationVar(&c.SpanDuration, "span-duration", 123*time.Microsecond, "The duration of each generated span.")
}
