// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// This file is forked from https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/790e18f1733e71debc7608aed98ace654ac76a60/cmd/telemetrygen/internal/metrics/exporter.go,
// which is licensed under Apache-2 and Copyright The OpenTelemetry Authors.
//
// It modifies the original implementation to remove unnecessary files,
// and to accept a configurable logger.

package metrics

import (
	"fmt"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"google.golang.org/grpc"

	"github.com/elastic/apm-perf/internal/telemetrygen/common"
)

// grpcExporterOptions creates the configuration options for a gRPC-based OTLP metric exporter.
// It configures the exporter with the provided endpoint, connection security settings, and headers.
func grpcExporterOptions(cfg *Config) ([]otlpmetricgrpc.Option, error) {
	grpcExpOpt := []otlpmetricgrpc.Option{
		otlpmetricgrpc.WithEndpoint(cfg.Endpoint()),
		otlpmetricgrpc.WithDialOption(
			grpc.WithBlock(),
		),
	}

	if cfg.Insecure {
		grpcExpOpt = append(grpcExpOpt, otlpmetricgrpc.WithInsecure())
	} else {
		credentials, err := common.GetTLSCredentialsForGRPCExporter(cfg.CaFile, cfg.ClientAuth)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS credentials: %w", err)
		}
		grpcExpOpt = append(grpcExpOpt, otlpmetricgrpc.WithTLSCredentials(credentials))
	}

	if len(cfg.Headers) > 0 {
		grpcExpOpt = append(grpcExpOpt, otlpmetricgrpc.WithHeaders(cfg.Headers))
	}

	return grpcExpOpt, nil
}

// httpExporterOptions creates the configuration options for an HTTP-based OTLP metric exporter.
// It configures the exporter with the provided endpoint, URL path, connection security settings, and headers.
func httpExporterOptions(cfg *Config) ([]otlpmetrichttp.Option, error) {
	httpExpOpt := []otlpmetrichttp.Option{
		otlpmetrichttp.WithEndpoint(cfg.Endpoint()),
		otlpmetrichttp.WithURLPath(cfg.HTTPPath),
	}

	if cfg.Insecure {
		httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithInsecure())
	} else {
		tlsCfg, err := common.GetTLSCredentialsForHTTPExporter(cfg.CaFile, cfg.ClientAuth)
		if err != nil {
			return nil, fmt.Errorf("failed to get TLS credentials: %w", err)
		}
		httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithTLSClientConfig(tlsCfg))
	}

	if len(cfg.Headers) > 0 {
		httpExpOpt = append(httpExpOpt, otlpmetrichttp.WithHeaders(cfg.Headers))
	}

	return httpExpOpt, nil
}
