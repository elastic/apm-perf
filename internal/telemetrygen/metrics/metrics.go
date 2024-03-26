// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// This file is forked from https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/790e18f1733e71debc7608aed98ace654ac76a60/cmd/telemetrygen/internal/metrics/metrics.go,
// which is licensed under Apache-2 and Copyright The OpenTelemetry Authors.
//
// This original implementation is modified:
// - Start function now only creates a logger when it is not already configured in cfg

package metrics

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	semconv "go.opentelemetry.io/collector/semconv/v1.13.0"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/elastic/apm-perf/internal/telemetrygen/common"
)

// Start starts the metric telemetry generator
func Start(cfg *Config) error {
	logger := cfg.Logger
	if logger == nil {
		newLogger, err := common.CreateLogger(cfg.SkipSettingGRPCLogger)
		if err != nil {
			return err
		}
		logger = newLogger
	}
	logger.Info("starting the metrics generator with configuration", zap.Any("config", cfg))

	expFunc := func() (sdkmetric.Exporter, error) {
		var exp sdkmetric.Exporter
		if cfg.UseHTTP {
			var exporterOpts []otlpmetrichttp.Option

			logger.Info("starting HTTP exporter")
			exporterOpts, err := httpExporterOptions(cfg)
			if err != nil {
				return nil, err
			}
			exp, err = otlpmetrichttp.New(context.Background(), exporterOpts...)
			if err != nil {
				return nil, fmt.Errorf("failed to obtain OTLP HTTP exporter: %w", err)
			}
		} else {
			var exporterOpts []otlpmetricgrpc.Option

			logger.Info("starting gRPC exporter")
			exporterOpts, err := grpcExporterOptions(cfg)
			if err != nil {
				return nil, err
			}
			exp, err = otlpmetricgrpc.New(context.Background(), exporterOpts...)
			if err != nil {
				return nil, fmt.Errorf("failed to obtain OTLP gRPC exporter: %w", err)
			}
		}
		return exp, nil
	}

	if err := Run(cfg, expFunc, logger); err != nil {
		logger.Error("failed to stop the exporter", zap.Error(err))
		return err
	}

	return nil
}

// Run executes the test scenario.
func Run(c *Config, exp func() (sdkmetric.Exporter, error), logger *zap.Logger) error {
	if c.TotalDuration > 0 {
		c.NumMetrics = 0
	} else if c.NumMetrics <= 0 {
		return fmt.Errorf("either `metrics` or `duration` must be greater than 0")
	}

	limit := rate.Limit(c.Rate)
	if c.Rate == 0 {
		limit = rate.Inf
		logger.Info("generation of metrics isn't being throttled")
	} else {
		logger.Info("generation of metrics is limited", zap.Float64("per-second", float64(limit)))
	}

	wg := sync.WaitGroup{}
	res := resource.NewWithAttributes(semconv.SchemaURL, c.GetAttributes()...)

	running := &atomic.Bool{}
	running.Store(true)

	for i := 0; i < c.WorkerCount; i++ {
		wg.Add(1)
		w := worker{
			numMetrics:     c.NumMetrics,
			metricType:     c.MetricType,
			limitPerSecond: limit,
			totalDuration:  c.TotalDuration,
			running:        running,
			wg:             &wg,
			logger:         logger.With(zap.Int("worker", i)),
			index:          i,
		}

		go w.simulateMetrics(res, exp, c.GetTelemetryAttributes())
	}
	if c.TotalDuration > 0 {
		time.Sleep(c.TotalDuration)
		running.Store(false)
	}
	wg.Wait()
	return nil
}
