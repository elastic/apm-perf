// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// This file is forked from https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/cmd/telemetrygen,
// which is licensed under Apache-2 and Copyright The OpenTelemetry Authors.
//
// It modifies the original implementation to remove unnecessary files,
// and to accept a configurable logger.

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestKeyValueSet(t *testing.T) {
	tests := []struct {
		flag     string
		expected KeyValue
		err      error
	}{
		{
			flag:     "key=\"value\"",
			expected: KeyValue(map[string]string{"key": "value"}),
		},
		{
			flag:     "key=\"\"",
			expected: KeyValue(map[string]string{"key": ""}),
		},
		{
			flag: "key=\"",
			err:  errDoubleQuotesOTLPAttributes,
		},
		{
			flag: "key=value",
			err:  errDoubleQuotesOTLPAttributes,
		},
		{
			flag: "key",
			err:  errFormatOTLPAttributes,
		},
	}

	for _, tt := range tests {
		t.Run(tt.flag, func(t *testing.T) {
			kv := KeyValue(make(map[string]string))
			err := kv.Set(tt.flag)
			if err != nil || tt.err != nil {
				assert.Equal(t, err, tt.err)
			} else {
				assert.Equal(t, tt.expected, kv)
			}
		})
	}
}

func TestEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		http     bool
		expected string
	}{
		{
			"default-no-http",
			"",
			false,
			defaultGRPCEndpoint,
		},
		{
			"default-with-http",
			"",
			true,
			defaultHTTPEndpoint,
		},
		{
			"custom-endpoint-no-http",
			"collector:4317",
			false,
			"collector:4317",
		},
		{
			"custom-endpoint-with-http",
			"collector:4317",
			true,
			"collector:4317",
		},
		{
			"wrong-custom-endpoint-with-http",
			defaultGRPCEndpoint,
			true,
			defaultGRPCEndpoint,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				CustomEndpoint: tc.endpoint,
				UseHTTP:        tc.http,
			}

			assert.Equal(t, tc.expected, cfg.Endpoint())
		})
	}
}
