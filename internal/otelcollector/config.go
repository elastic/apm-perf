// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package otelcollector

import (
	"fmt"
	"os"

	"github.com/elastic/apm-perf/internal/otelcollector/exporter/inmemexporter"
	"go.uber.org/zap/zapcore"
	"gopkg.in/yaml.v2"
)

// CollectorConfig defines the configuration to customize the collector.
type CollectorConfig struct {
	HTTPEndpoint        string                            `yaml:"http_endpoint"`
	GRPCEndpoint        string                            `yaml:"grpc_endpoint"`
	InMemoryStoreConfig []inmemexporter.AggregationConfig `yaml:"store"`
}

// LoadConfigFromYamlFile loads collector configuration from an yaml file.
// Can be used with DefaultConfig:
// `DefaultConfig().LoadConfigFromYamlFile(<cfg_file_path>)`
func (cfg *CollectorConfig) LoadConfigFromYamlFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	dec := yaml.NewDecoder(file)
	if err := dec.Decode(cfg); err != nil {
		return fmt.Errorf("failed to decode config file: %w", err)
	}

	return nil
}

// MarshalLogObject implements zapcore.ObjectMarshaler to allow adding
// config to logging context.
func (cfg *CollectorConfig) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("http_endpoint", cfg.HTTPEndpoint)
	enc.AddString("grpc_endpoint", cfg.GRPCEndpoint)
	enc.AddArray("store_config", zapcore.ArrayMarshalerFunc(
		func(enc zapcore.ArrayEncoder) error {
			for _, c := range cfg.InMemoryStoreConfig {
				enc.AppendObject(&c)
			}
			return nil
		},
	))
	return nil
}

// DefaultConfig creates a default collector configuration.
func DefaultConfig() CollectorConfig {
	return CollectorConfig{
		HTTPEndpoint: "127.0.0.1:4318",
		GRPCEndpoint: "127.0.0.1:4317",
	}
}
