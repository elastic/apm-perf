// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"

	"github.com/elastic/apm-perf/internal/otelcollector"
)

func main() {
	flag.Parse()

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to setup logger: %v", err)
	}

	extraMetrics := func(*testing.B) error { return nil }
	resetFunc := func() {}
	if cfg.CollectorConfigYaml != "" {
		logger.Info("starting otel collector...")

		collectorCfg := otelcollector.DefaultConfig()
		err := collectorCfg.LoadConfigFromYamlFile(cfg.CollectorConfigYaml)
		if err != nil {
			logger.Fatal("failed to load collector config", zap.Error(err))
		}

		collector, err := otelcollector.New(collectorCfg, logger)
		if err != nil {
			logger.Fatal("failed to create a new collector", zap.Error(err))
		}
		logger.Info("loaded collector configuration", zap.Object("config", &collectorCfg))

		var wg sync.WaitGroup
		defer wg.Wait()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		wg.Add(1)
		go func(ctx context.Context) {
			defer wg.Done()

			if err := collector.Run(ctx); err != nil {
				logger.Fatal("failed to run collector", zap.Error(err))
			}
		}(ctx)

		extraMetrics = func(b *testing.B) error {
			var errs []error
			for _, cfg := range collectorCfg.InMemoryStoreConfig {
				m, err := collector.GetAggregatedMetric(cfg)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				b.ReportMetric(m, cfg.Alias)
			}
			if len(errs) > 0 {
				return errors.Join(errs...)
			}
			return nil
		}
		resetFunc = collector.Reset

		// Wait for otel collector to be ready
		ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := collector.Wait(ctx); err != nil {
			logger.Fatal("failed to start collector", zap.Error(err))
		}
	}

	if err := Run(
		extraMetrics,
		resetFunc,
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
}

func init() {
	testing.Init()
}
