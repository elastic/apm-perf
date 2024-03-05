// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// This file is forked from https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/790e18f1733e71debc7608aed98ace654ac76a60/cmd/telemetrygen/internal/logs/config.go,
// which is licensed under Apache-2 and Copyright The OpenTelemetry Authors.
//
// It modifies the original implementation to remove unnecessary files,
// and to accept a configurable logger.

package logs

import (
	"github.com/spf13/pflag"

	"github.com/elastic/apm-perf/internal/telemetrygen/common"
)

// Config describes the test scenario.
type Config struct {
	common.Config
	NumLogs int
	Body    string
}

// Flags registers config flags.
func (c *Config) Flags(fs *pflag.FlagSet) {
	c.CommonFlags(fs)

	fs.StringVar(&c.HTTPPath, "otlp-http-url-path", "/v1/logs", "Which URL path to write to")

	fs.IntVar(&c.NumLogs, "logs", 1, "Number of logs to generate in each worker (ignored if duration is provided)")
	fs.StringVar(&c.Body, "body", "the message", "Body of the log")
}
