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

package main // import "github.com/open-telemetry/opentelemetry-collector-contrib/telemetrygen/internal/telemetrygen"

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/elastic/apm-perf/internal/telemetrygen/logs"
	"github.com/elastic/apm-perf/internal/telemetrygen/metadata"
	"github.com/elastic/apm-perf/internal/telemetrygen/metrics"
	"github.com/elastic/apm-perf/internal/telemetrygen/traces"
)

var (
	tracesCfg  *traces.Config
	metricsCfg *metrics.Config
	logsCfg    *logs.Config
)

// rootCmd is the root command on which will be run children commands
var rootCmd = &cobra.Command{
	Use:     "telemetrygen",
	Short:   "Telemetrygen simulates a client generating traces, metrics, and logs",
	Example: "telemetrygen traces\ntelemetrygen metrics\ntelemetrygen logs",
}

// tracesCmd is the command responsible for sending traces
var tracesCmd = &cobra.Command{
	Use:     "traces",
	Short:   fmt.Sprintf("Simulates a client generating traces. (Stability level: %s)", metadata.TracesStability),
	Example: "telemetrygen traces",
	RunE: func(cmd *cobra.Command, args []string) error {
		return traces.Start(tracesCfg)
	},
}

// metricsCmd is the command responsible for sending metrics
var metricsCmd = &cobra.Command{
	Use:     "metrics",
	Short:   fmt.Sprintf("Simulates a client generating metrics. (Stability level: %s)", metadata.MetricsStability),
	Example: "telemetrygen metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		return metrics.Start(metricsCfg)
	},
}

// logsCmd is the command responsible for sending logs
var logsCmd = &cobra.Command{
	Use:     "logs",
	Short:   fmt.Sprintf("Simulates a client generating logs. (Stability level: %s)", metadata.LogsStability),
	Example: "telemetrygen logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		return logs.Start(logsCfg)
	},
}

func init() {
	rootCmd.AddCommand(tracesCmd, metricsCmd, logsCmd)

	tracesCfg = new(traces.Config)
	tracesCfg.Flags(tracesCmd.Flags())

	metricsCfg = new(metrics.Config)
	metricsCfg.Flags(metricsCmd.Flags())

	logsCfg = new(logs.Config)
	logsCfg.Flags(logsCmd.Flags())

	// Disabling completion command for end user
	// https://github.com/spf13/cobra/blob/master/shell_completions.md
	rootCmd.CompletionOptions.DisableDefaultCmd = true

}

// Execute tries to run the input command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		// TODO: Uncomment the line below when using Run instead of RunE in the xxxCmd functions
		// fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
