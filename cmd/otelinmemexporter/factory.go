// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package otelinmemexporter

import (
	"context"
	"fmt"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		componentID,
		createDefaultConfig,
		exporter.WithMetrics(
			createMetricsExporter,
			component.StabilityLevelDevelopment,
		),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		Server: serverConfig{Endpoint: ":8081"},
	}
}

func createMetricsExporter(
	ctx context.Context,
	settings exporter.CreateSettings,
	rawCfg component.Config,
) (exporter.Metrics, error) {
	cfg := rawCfg.(*Config)
	logger := settings.TelemetrySettings.Logger

	// create in memory metrics store
	store, err := NewStore(cfg.Aggregations, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create in-memory metrics store: %w", err)
	}
	// Start http server
	newServer(store, cfg.Server.Endpoint, logger).Start()

	exp := new(*cfg, store, logger)
	return exporterhelper.NewMetricsExporter(
		ctx, settings, cfg,
		exp.consumeMetrics,
		exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
		// Disable Timeout/RetryOnFailure and SendingQueue
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
		exporterhelper.WithRetry(configretry.BackOffConfig{Enabled: false}),
		exporterhelper.WithQueue(exporterhelper.QueueSettings{Enabled: false}),
	)
}
