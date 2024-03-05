// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// This file is forked from https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/cmd/telemetrygen,
// which is licensed under Apache-2 and Copyright The OpenTelemetry Authors.
//
// It modifies the original implementation to remove unnecessary files,
// and to accept a configurable logger.

package common

import (
	"fmt"

	grpcZap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	"go.uber.org/zap"
)

// CreateLogger creates a logger for use by telemetrygen
func CreateLogger(skipSettingGRPCLogger bool) (*zap.Logger, error) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain logger: %w", err)
	}
	if !skipSettingGRPCLogger {
		grpcZap.ReplaceGrpcLoggerV2WithVerbosity(logger.WithOptions(
			zap.AddCallerSkip(3),
		), 1) // set to warn verbosity to avoid copious logging from grpc framework
	}
	return logger, err
}
