// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package otelcollector contains code for OTEL based in-memory telemetry
// collector responsible for exposing OTEL receiver endpoints, caching telemetry
// data sentand providing queryable interface for the collected data.
package otelcollector

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/cumulativetodeltaprocessor"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/configopaque"
	"go.opentelemetry.io/collector/config/configtelemetry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/otlpexporter"
	"go.opentelemetry.io/collector/otelcol"
	"go.opentelemetry.io/collector/processor"
	"go.opentelemetry.io/collector/processor/batchprocessor"
	"go.opentelemetry.io/collector/receiver"
	otlpreceiver "go.opentelemetry.io/collector/receiver/otlpreceiver"
	"go.opentelemetry.io/collector/service"
	"go.opentelemetry.io/collector/service/pipelines"
	"go.opentelemetry.io/collector/service/telemetry"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/elastic/apm-perf/internal/otelcollector/exporter/inmemexporter"
)

// Collector defines an OTEL collector for collecting metrics. Services
// can be configured to send metrics to the collector and the collector
// can be configured to record aggregated values for a set of metrics.
// The collector can be queried for the recorded metrics as required.
type Collector struct {
	collector *otelcol.Collector
	store     *inmemexporter.Store
}

// New creates a new instance of the Collector.
func New(cfg CollectorConfig, logger *zap.Logger) (*Collector, error) {
	store, err := inmemexporter.NewStore(cfg.InMemoryStoreConfig, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create in memory store: %w", err)
	}

	factories := otelcol.Factories{}
	otlpReceiverFactory := otlpreceiver.NewFactory()
	factories.Receivers, err = receiver.MakeFactoryMap(otlpReceiverFactory)
	if err != nil {
		return nil, fmt.Errorf("failed to create collector: %w", err)
	}

	factories.Processors, err = processor.MakeFactoryMap(
		cumulativetodeltaprocessor.NewFactory(),
		batchprocessor.NewFactory(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create collector: %w", err)
	}

	factories.Exporters, err = exporter.MakeFactoryMap(
		inmemexporter.NewFactory(store),
		otlpexporter.NewFactory(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create collector: %w", err)
	}

	otlpReceiverCfg := otlpReceiverFactory.CreateDefaultConfig().(*otlpreceiver.Config)
	if cfg.GRPCEndpoint != "" {
		otlpReceiverCfg.GRPC.NetAddr.Endpoint = cfg.GRPCEndpoint
	}
	if cfg.HTTPEndpoint != "" {
		otlpReceiverCfg.HTTP.Endpoint = cfg.HTTPEndpoint
	}
	collector, err := otelcol.NewCollector(otelcol.CollectorSettings{
		BuildInfo: component.NewDefaultBuildInfo(),
		Factories: factories,
		ConfigProvider: newStaticConfigProvider(
			otlpReceiverCfg,
			cfg.OTLPExporterEndpoint,
			cfg.OTLPExporterHeaders,
		),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create collector: %w", err)
	}

	return &Collector{
		collector: collector,
		store:     store,
	}, nil
}

// Run runs the collector and waits for it to complete. Consecutive calls
// to Run are not allowed and it shouldn't be called after Shutdown.
func (c *Collector) Run(ctx context.Context) error {
	return c.collector.Run(ctx)
}

// Wait waits for the collector to be in a ready state.
func (c *Collector) Wait(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("failed waiting for collector to be ready: %w", ctx.Err())
		default:
			switch c.collector.GetState() {
			case otelcol.StateRunning:
				return nil
			case otelcol.StateClosing, otelcol.StateClosed:
				return errors.New("collector is closing")
			}
			time.Sleep(time.Millisecond)
		}
	}
}

// Shutdown shuts down the collector.
func (c *Collector) Shutdown() error {
	if c.collector != nil {
		c.collector.Shutdown()
	}
	return nil
}

// GetAggregatedMetric returns a aggregated value for the given
// aggregation config.
func (c *Collector) GetAggregatedMetric(
	cfg inmemexporter.AggregationConfig,
) (float64, error) {
	return c.store.Get(cfg)
}

// Reset resets the collector.
func (c *Collector) Reset() {
	c.store.Reset()
}

type staticConfigProvider struct {
	otlpReceiverConfig   *otlpreceiver.Config
	otlpExporterEndpoint string
	otlpExporterHeaders  map[string]configopaque.String
}

func newStaticConfigProvider(
	otlpReceiverConfig *otlpreceiver.Config,
	otlpExporterEndpoint string,
	otlpExporterHeaders map[string]string,
) staticConfigProvider {
	exporterHeaders := make(map[string]configopaque.String, len(otlpExporterHeaders))
	for k, v := range otlpExporterHeaders {
		exporterHeaders[k] = configopaque.String(v)
	}
	return staticConfigProvider{
		otlpReceiverConfig:   otlpReceiverConfig,
		otlpExporterEndpoint: otlpExporterEndpoint,
		otlpExporterHeaders:  exporterHeaders,
	}
}

func (p staticConfigProvider) Get(
	ctx context.Context,
	factories otelcol.Factories,
) (*otelcol.Config, error) {
	return &otelcol.Config{
		Receivers:  p.getReceivers(),
		Processors: p.getProcessors(),
		Exporters:  p.getExporters(),
		Service: service.Config{
			Pipelines: p.getPipelines(),
			Telemetry: telemetry.Config{
				Logs: telemetry.LogsConfig{
					Level:            zapcore.InfoLevel,
					Development:      false,
					Encoding:         "console",
					OutputPaths:      []string{"stderr"},
					ErrorOutputPaths: []string{"stderr"},
				},
				Metrics: telemetry.MetricsConfig{
					Level: configtelemetry.LevelNone,
				},
			},
		},
	}, nil
}

func (p staticConfigProvider) Watch() <-chan error {
	// disable reload
	return nil
}

func (p staticConfigProvider) Shutdown(ctx context.Context) error {
	return nil
}

func (p staticConfigProvider) getReceivers() map[component.ID]component.Config {
	return map[component.ID]component.Config{
		component.NewID("otlp"): p.otlpReceiverConfig,
	}
}

func (p staticConfigProvider) getProcessors() map[component.ID]component.Config {
	return map[component.ID]component.Config{
		component.NewID("cumulativetodelta"): &cumulativetodeltaprocessor.Config{},
		component.NewID("batch"): &batchprocessor.Config{
			SendBatchMaxSize: 4 << 20, // 4MB
		},
	}
}

func (p staticConfigProvider) getExporters() map[component.ID]component.Config {
	exporters := map[component.ID]component.Config{
		component.NewID("inmem"): &inmemexporter.Config{},
	}
	if p.otlpExporterEndpoint != "" {
		exporters[component.NewID("otlp")] = &otlpexporter.Config{
			GRPCClientSettings: configgrpc.GRPCClientSettings{
				Endpoint: p.otlpExporterEndpoint,
				Headers:  p.otlpExporterHeaders,
			},
		}
	}
	return exporters
}

func (p staticConfigProvider) getPipelines() map[component.ID]*pipelines.PipelineConfig {
	pipes := map[component.ID]*pipelines.PipelineConfig{
		component.NewID("metrics"): &pipelines.PipelineConfig{
			Receivers:  []component.ID{component.NewID("otlp")},
			Processors: []component.ID{component.NewID("cumulativetodelta")},
			Exporters:  []component.ID{component.NewID("inmem")},
		},
	}
	if p.otlpExporterEndpoint != "" {
		pipes[component.NewIDWithName("metrics", "2")] = &pipelines.PipelineConfig{
			Receivers: []component.ID{component.NewID("otlp")},
			Processors: []component.ID{
				component.NewID("cumulativetodelta"),
				component.NewID("batch"),
			},
			Exporters: []component.ID{component.NewID("otlp")},
		}
		pipes[component.NewID("traces")] = &pipelines.PipelineConfig{
			Receivers:  []component.ID{component.NewID("otlp")},
			Processors: []component.ID{component.NewID("batch")},
			Exporters:  []component.ID{component.NewID("otlp")},
		}
	} else {
		// Enable traces endpoint with a noOp if no otlp exporter is configured
		pipes[component.NewID("traces")] = &pipelines.PipelineConfig{
			Receivers: []component.ID{component.NewID("otlp")},
			Exporters: []component.ID{component.NewID("inmem")},
		}
	}
	return pipes
}
