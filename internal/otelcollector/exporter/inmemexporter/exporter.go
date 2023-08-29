// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package inmemexporter contains code for creating an in-memory OTEL exporter.
package inmemexporter

import (
	"context"

	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

const componentID = "inmem"

type Config struct{}

type inMemExporter struct {
	cfg    Config
	store  *Store
	logger *zap.Logger
}

func new(cfg Config, store *Store, logger *zap.Logger) *inMemExporter {
	return &inMemExporter{
		cfg:    cfg,
		store:  store,
		logger: logger,
	}
}

func (e *inMemExporter) consumeMetrics(_ context.Context, ld pmetric.Metrics) error {
	e.logger.Debug("received metrics", zap.Int("count", ld.MetricCount()))
	e.store.Add(ld)
	return nil
}

func (e *inMemExporter) consumeTraces(_ context.Context, ld ptrace.Traces) error {
	e.logger.Debug("received traces", zap.Int("count", ld.SpanCount()))
	return nil
}
