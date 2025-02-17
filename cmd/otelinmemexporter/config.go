// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package otelinmemexporter contains code for creating an in-memory OTEL exporter.
package otelinmemexporter

import (
	"errors"
	"fmt"

	"go.opentelemetry.io/collector/component"
)

type serverConfig struct {
	Endpoint string `mapstructure:"endpoint"`
}

type Config struct {
	Aggregations []AggregationConfig `mapstructure:"aggregations"`
	Server       serverConfig        `mapstructure:"server"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if _, _, err := groupAggregationConfigsByKeyAndName(cfg.Aggregations); err != nil {
		return fmt.Errorf("failed to validate aggregation config: %w", err)
	}
	if cfg.Server.Endpoint == "" {
		return errors.New("failed to validate server config: address cannot be empty")
	}
	return nil
}
