// Licensed to The OpenTelemetry Authors under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. The OpenTelemetry Authors licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

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
