// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package inmemexporter

import (
	"context"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

func NewFactory(store *Store) exporter.Factory {
	return exporter.NewFactory(
		componentID,
		createDefaultConfig,
		exporter.WithMetrics(
			createMetricsExporter(store),
			component.StabilityLevelDevelopment,
		),
		exporter.WithTraces(
			createTracesExporter(store),
			component.StabilityLevelDevelopment,
		),
	)
}

func createDefaultConfig() component.Config {
	return &Config{}
}

func createMetricsExporter(store *Store) func(context.Context, exporter.CreateSettings, component.Config) (exporter.Metrics, error) {
	return func(
		ctx context.Context,
		settings exporter.CreateSettings,
		rawCfg component.Config,
	) (exporter.Metrics, error) {
		cfg := rawCfg.(*Config)
		exp := new(*cfg, store, settings.TelemetrySettings.Logger)
		return exporterhelper.NewMetricsExporter(
			ctx, settings, cfg,
			exp.consumeMetrics,
			exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
			// Disable Timeout/RetryOnFailure and SendingQueue
			exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
			exporterhelper.WithRetry(exporterhelper.RetrySettings{Enabled: false}),
			exporterhelper.WithQueue(exporterhelper.QueueSettings{Enabled: false}),
		)
	}
}

func createTracesExporter(store *Store) func(context.Context, exporter.CreateSettings, component.Config) (exporter.Traces, error) {
	return func(
		ctx context.Context,
		settings exporter.CreateSettings,
		rawCfg component.Config,
	) (exporter.Traces, error) {
		cfg := rawCfg.(*Config)
		exp := new(*cfg, store, settings.TelemetrySettings.Logger)
		return exporterhelper.NewTracesExporter(
			ctx, settings, cfg,
			exp.consumeTraces,
			exporterhelper.WithCapabilities(consumer.Capabilities{MutatesData: false}),
			// Disable Timeout/RetryOnFailure and SendingQueue
			exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: 0}),
			exporterhelper.WithRetry(exporterhelper.RetrySettings{Enabled: false}),
			exporterhelper.WithQueue(exporterhelper.QueueSettings{Enabled: false}),
		)
	}
}
