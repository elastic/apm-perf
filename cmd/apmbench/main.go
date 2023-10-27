// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"flag"
	"log"
	"testing"

	"go.uber.org/zap"
)

func main() {
	flag.Parse()

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to setup logger: %v", err)
	}

	// Run benchmarks
	telemetry := telemetry{endpoint: cfg.BenchmarkTelemetryEndpoint}
	extraMetrics := func(b *testing.B) {
		m, err := telemetry.GetAll()
		if err != nil {
			logger.Warn("failed to retrive benchmark metrics", zap.Error(err))
			return
		}
		for unit, val := range m {
			b.ReportMetric(val, unit)
		}
	}
	resetStoreFunc := func() {
		if err := telemetry.Reset(); err != nil {
			logger.Warn("failed to reset store, benchmark report may be corrupted", zap.Error(err))
		}
	}
	if err := Run(
		extraMetrics,
		resetStoreFunc,
		Benchmark1000Transactions,
		BenchmarkOTLPTraces,
		BenchmarkAgentAll,
		BenchmarkAgentGo,
		BenchmarkAgentNodeJS,
		BenchmarkAgentPython,
		BenchmarkAgentRuby,
		Benchmark10000AggregationGroups,
	); err != nil {
		logger.Fatal("failed to run benchmarks", zap.Error(err))
	}
	logger.Info("finished running benchmarks")
}

func init() {
	testing.Init()
}
