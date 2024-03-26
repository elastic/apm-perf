// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// This file is forked from https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/790e18f1733e71debc7608aed98ace654ac76a60/cmd/telemetrygen/internal/logs/logs.go,
// which is licensed under Apache-2 and Copyright The OpenTelemetry Authors.
//
// This original implementation is modified:
// - Start function now only creates a logger when it is not already configured in cfg

package logs

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	"go.uber.org/zap"
	"golang.org/x/time/rate"

	"github.com/elastic/apm-perf/internal/telemetrygen/common"
)

// Start starts the log telemetry generator
func Start(cfg *Config) error {
	logger := cfg.Logger
	if logger == nil {
		newLogger, err := common.CreateLogger(cfg.SkipSettingGRPCLogger)
		if err != nil {
			return err
		}
		logger = newLogger
	}

	e, err := newExporter(context.Background(), cfg)
	if err != nil {
		return err
	}

	if err = Run(cfg, e, logger); err != nil {
		logger.Error("failed to stop the exporter", zap.Error(err))
		return err
	}

	return nil
}

// Run executes the test scenario.
func Run(c *Config, exp exporter, logger *zap.Logger) error {
	if c.TotalDuration > 0 {
		c.NumLogs = 0
	} else if c.NumLogs <= 0 {
		return fmt.Errorf("either `logs` or `duration` must be greater than 0")
	}

	limit := rate.Limit(c.Rate)
	if c.Rate == 0 {
		limit = rate.Inf
		logger.Info("generation of logs isn't being throttled")
	} else {
		logger.Info("generation of logs is limited", zap.Float64("per-second", float64(limit)))
	}

	wg := sync.WaitGroup{}
	res := resource.NewWithAttributes(semconv.SchemaURL, c.GetAttributes()...)

	running := &atomic.Bool{}
	running.Store(true)

	for i := 0; i < c.WorkerCount; i++ {
		wg.Add(1)
		w := worker{
			numLogs:        c.NumLogs,
			limitPerSecond: limit,
			body:           c.Body,
			totalDuration:  c.TotalDuration,
			running:        running,
			wg:             &wg,
			logger:         logger.With(zap.Int("worker", i)),
			index:          i,
		}

		go w.simulateLogs(res, exp, c.GetTelemetryAttributes())
	}
	if c.TotalDuration > 0 {
		time.Sleep(c.TotalDuration)
		running.Store(false)
	}
	wg.Wait()
	return nil
}
