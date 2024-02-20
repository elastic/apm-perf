// Licensed to The OpenTelemetry Authors under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. The OpenTelemetry Authors licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

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
