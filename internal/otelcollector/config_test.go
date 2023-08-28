// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package otelcollector

import (
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-perf/internal/otelcollector/exporter/inmemexporter"
)

func TestLoadConfigFromYamlFile(t *testing.T) {
	for _, tt := range []struct {
		name   string
		input  []byte
		output CollectorConfig
		errMsg string
	}{
		{
			name:   "invalid_yaml",
			input:  []byte("invalid_yaml"),
			errMsg: "failed to decode config file",
		},
		{
			name: "valid_yaml",
			input: []byte(`
http_endpoint: localhost:8181
grpc_endpoint: localhost:8182
store:
  - name: test.metric.count
    alias: events/s
    match_label_values:
      k_1: v_1
      k_2: v_2`),
			output: CollectorConfig{
				HTTPEndpoint: "localhost:8181",
				GRPCEndpoint: "localhost:8182",
				InMemoryStoreConfig: []inmemexporter.AggregationConfig{
					{
						Name:  "test.metric.count",
						Alias: "events/s",
						MatchLabelValues: map[string]string{
							"k_1": "v_1",
							"k_2": "v_2",
						},
					},
				},
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			testFile := path.Join(t.TempDir(), fmt.Sprintf("test-%s.yaml", tt.name))
			require.NoError(t, os.WriteFile(testFile, tt.input, 0777))

			var cfg CollectorConfig
			err := cfg.LoadConfigFromYamlFile(testFile)
			if tt.errMsg != "" {
				assert.ErrorContains(t, err, tt.errMsg)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.output, cfg)
		})
	}
}
