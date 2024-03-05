// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// This file is forked from https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/790e18f1733e71debc7608aed98ace654ac76a60/cmd/telemetrygen/internal/metrics/config.go,
// which is licensed under Apache-2 and Copyright The OpenTelemetry Authors.
//
// This file does not contain significant modifications.

package metrics

import (
	"github.com/spf13/pflag"

	"github.com/elastic/apm-perf/internal/telemetrygen/common"
)

// Config describes the test scenario.
type Config struct {
	common.Config
	NumMetrics int
	MetricType metricType
}

// Flags registers config flags.
func (c *Config) Flags(fs *pflag.FlagSet) {
	// Use Gauge as default metric type.
	c.MetricType = metricTypeGauge

	c.CommonFlags(fs)

	fs.StringVar(&c.HTTPPath, "otlp-http-url-path", "/v1/metrics", "Which URL path to write to")

	fs.Var(&c.MetricType, "metric-type", "Metric type enum. must be one of 'Gauge' or 'Sum'")
	fs.IntVar(&c.NumMetrics, "metrics", 1, "Number of metrics to generate in each worker (ignored if duration is provided)")
}
