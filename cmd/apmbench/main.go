// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"testing"
	"time"

	"go.uber.org/zap"
)

func main() {
	flag.Parse()

	logger, err := setupLogger()
	if err != nil {
		log.Fatalf("failed to setup logger: %v", err)
	}

	extraMetrics := func(b *testing.B) {}
	resetStoreFunc := func() {}
	if cfg.BenchmarkTelemetryEndpoint != "" {
		telemetry := telemetry{endpoint: cfg.BenchmarkTelemetryEndpoint}
		extraMetrics = func(b *testing.B) {
			// TODO (lahsivjar): get a context with timeout based on test timeout
			if err := assertCleanupState(context.Background(), telemetry, logger); err != nil {
				logger.Warn(
					"failed to get cleanup metric, continuing without cleanup",
					zap.Error(err),
				)
			}
			m, err := telemetry.GetAll()
			if err != nil {
				logger.Warn(
					"failed to retrive benchmark metrics, extra metrics will not be reported",
					zap.Error(err),
				)
				return
			}
			// extra metrics may be aggregated by a grouping key. We can sum all the
			// values for the grouping key to get the final benchmark results.
			for unit, grp := range m {
				var total float64
				for _, val := range grp {
					total += val
				}
				b.ReportMetric(total, unit)
			}
		}
		resetStoreFunc = func() {
			if err := telemetry.Reset(); err != nil {
				logger.Warn(
					"failed to reset store, benchmark report may be corrupted",
					zap.Error(err),
				)
			}
		}
	}
	// Run benchmarks
	if err := Run(
		extraMetrics,
		resetStoreFunc,
		Benchmark1000Transactions,
		BenchmarkAgentAll,
		BenchmarkAgentGo,
		BenchmarkAgentNodeJS,
		BenchmarkAgentPython,
		BenchmarkAgentRuby,
		Benchmark10000AggregationGroups,
		BenchmarkOTLPTraces,
		BenchmarkOTLPLogs,
		BenchmarkOTLPMetrics,
	); err != nil {
		logger.Fatal("failed to run benchmarks", zap.Error(err))
	}
	logger.Info("finished running benchmarks")
}

func setupLogger() (*zap.Logger, error) {
	loggerCfg := zap.NewProductionConfig()
	if cfg.Debug {
		loggerCfg.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}
	return loggerCfg.Build()
}

func assertCleanupState(ctx context.Context, telemetry telemetry, logger *zap.Logger) error {
	if len(cfg.CleanupKeys) == 0 {
		// If no cleanup keys are specified then, as a best effort, sleep
		// for a specific duration to ensure the pipelines have a chance to
		// be fully consumed and metrics to be reported.
		logger.Info("no cleanup keys specified, sleep for 1 minute before proceeding")
		time.Sleep(time.Minute)
		return nil
	}

	t := time.NewTicker(5 * time.Second)
	defer t.Stop()

	var idx int
	for {
		// cleanup is successful if we have asserted all the cleanup keys.
		if idx == len(cfg.CleanupKeys) {
			logger.Debug("cleanup condition satisfied")
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("cleanup condition not satisfied: %w", ctx.Err())
		case <-t.C:
			for idx < len(cfg.CleanupKeys) {
				key := cfg.CleanupKeys[idx]
				m, err := telemetry.Get(key)
				if err != nil {
					logger.Warn(
						"failed to get cleanup metric, will be retried",
						zap.Error(err),
						zap.String("key", key),
					)
					break
				}
				ok := true
				for gid, v := range m {
					if v != 0 {
						logger.Debug(
							"cleanup condition not satisfied, will be retried",
							zap.String("key", key),
							zap.String("group", gid),
							zap.Float64("val", v),
						)
						ok = false
						break
					}
				}
				// If a cleanup key fails then there is no need to try anything else.
				if !ok {
					break
				}
				// If the cleanup metric is successful for the current key then
				// move onto the next key.
				idx++
			}
		}
	}
}

func init() {
	testing.Init()
}
