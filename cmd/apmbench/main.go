// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/url"
	"os"
	"os/signal"
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

	// Create otel collector
	collectorCfg := otelcollector.DefaultConfig()
	if cfg.APMServerURL != "" {
		apmURL, err := url.Parse(cfg.APMServerURL)
		if err != nil {
			logger.Fatal("invalid apm-server-url specified")
		}
		collectorCfg.OTLPExporterEndpoint = apmURL.Host
		if apmURL.Port() == "" {
			collectorCfg.OTLPExporterEndpoint += ":443"
		}
		if cfg.APMSecretToken != "" {
			collectorCfg.OTLPExporterHeaders = map[string]string{
				"Authorization": "Bearer " + cfg.APMSecretToken,
			}
		}
	}
	if cfg.CollectorConfigYaml != "" {
		err := collectorCfg.LoadConfigFromYamlFile(cfg.CollectorConfigYaml)
		if err != nil {
			logger.Fatal("failed to load collector config", zap.Error(err))
		}
	}
	collector, err := otelcollector.New(collectorCfg, logger)
	if err != nil {
		logger.Fatal("failed to create a new collector", zap.Error(err))
	}
	logger.Info("loaded collector configuration", zap.Object("config", &collectorCfg))

	// Start otel collector
	var wg sync.WaitGroup
	defer wg.Wait()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wg.Add(1)
	go func(ctx context.Context) {
		defer wg.Done()

		logger.Info("starting otel collector...")
		if err := collector.Run(ctx); err != nil {
			logger.Fatal("failed to run collector", zap.Error(err))
		}
	}(ctx)

	// Wait for otel collector to be ready
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := collector.Wait(ctx); err != nil {
		logger.Fatal("failed to start collector", zap.Error(err))
	}

	// Run benchmarks
	extraMetrics := func(b *testing.B) error {
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
	if err := Run(
		extraMetrics,
		collector.Reset,
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

	// If server-mode is enabled then keep the otel collector running
	if cfg.ServerMode {
		logger.Info("continuing to serve OTEL collector endpoints")
		c := make(chan os.Signal, 1)
		signal.Notify(c, os.Interrupt)
		<-c
	}
}

func init() {
	testing.Init()
}
