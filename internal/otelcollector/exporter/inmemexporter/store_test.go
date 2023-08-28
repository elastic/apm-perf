// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package inmemexporter

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestNewStore(t *testing.T) {
	logger := zap.NewExample()
	_, validCfg := getTestAggCfg()
	for _, tt := range []struct {
		name   string
		cfgs   []AggregationConfig
		errMsg string
	}{
		{
			name: "duplicate_config",
			cfgs: []AggregationConfig{
				{
					Name: "agg_cfg_1",
					MatchLabelValues: map[string]string{
						"k1": "v1",
					},
					Type: Sum,
				},
				{
					Name: "agg_cfg_1",
					MatchLabelValues: map[string]string{
						"k1": "v1",
					},
					Type: Sum,
				},
			},
			errMsg: "duplicate config found",
		},
		{
			name: "duplicate_config_differnt_type",
			cfgs: []AggregationConfig{
				{
					Name: "agg_cfg_1",
					MatchLabelValues: map[string]string{
						"k1": "v1",
					},
					Type: Sum,
				},
				{
					Name: "agg_cfg_1",
					MatchLabelValues: map[string]string{
						"k1": "v1",
					},
					Type: Last,
				},
			},
			errMsg: "cannot record same metric with different types",
		},
		{
			name: "valid",
			cfgs: validCfg,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewStore(tt.cfgs, logger)
			if tt.errMsg != "" {
				assert.ErrorContains(t, err, tt.errMsg)
				return
			}
			assert.NoError(t, err)
			assert.NotNil(t, store)
		})
	}
}

func TestAdd(t *testing.T) {
	allMetricNames, cfgs := getTestAggCfg()
	for _, tt := range []struct {
		name     string
		input    pmetric.Metrics
		expected []float64 // for each aggregation query 1 output
	}{
		{
			name: "no_config",
			input: newMetrics().addMetric(
				[]string{"404"}, 1.1,
				nil,
				pmetric.MetricTypeGauge,
			).get(),
			expected: []float64{0, 0},
		},
		{
			name: "filtered_input",
			input: newMetrics().
				addMetric(
					allMetricNames, 1.1,
					map[string]string{"k_1": "v_1"},
					pmetric.MetricTypeGauge).
				addMetric(
					allMetricNames, 2.2,
					map[string]string{"k_1": "v_1", "k_2": "v_2"},
					pmetric.MetricTypeGauge).
				get(),
			expected: []float64{
				3.3, // sum
				2.2, // last
			},
		},
		{
			name: "unfiltered_input",
			input: newMetrics().
				// no labels
				addMetric(
					allMetricNames, 1.1,
					nil,
					pmetric.MetricTypeGauge).
				// label key doesn't match
				addMetric(
					allMetricNames, 1.1,
					map[string]string{"k_2": "v_1"},
					pmetric.MetricTypeGauge).
				// label value doesn't match
				addMetric(
					allMetricNames, 2.2,
					map[string]string{"k_1": "v_2"},
					pmetric.MetricTypeGauge).
				// name doesn't match
				addMetric(
					[]string{"404"}, 3.3,
					map[string]string{"k_1": "v_1"},
					pmetric.MetricTypeGauge).
				get(),
			expected: []float64{0, 0},
		},
		{
			name: "mixed_input",
			input: newMetrics().
				addMetric(
					allMetricNames, 1.1,
					map[string]string{"k_1": "v_1"},
					pmetric.MetricTypeGauge).
				addMetric(
					allMetricNames, 2.2,
					map[string]string{"k_2": "v_1"},
					pmetric.MetricTypeGauge).
				addMetric(
					allMetricNames, 3.3,
					map[string]string{"k_1": "v_1"},
					pmetric.MetricTypeGauge).
				get(),
			expected: []float64{
				4.4, // sum
				3.3, // last
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			store, err := NewStore(cfgs, zap.NewExample())
			require.NoError(t, err)

			store.Add(tt.input)
			for i := 0; i < len(cfgs); i++ {
				actual, err := store.Get(cfgs[i])
				assert.NoError(t, err)
				assert.InDelta(t, tt.expected[i], actual, 1e-9)
			}
		})
	}
}

type testMetricSlice struct {
	m  pmetric.Metrics
	ms pmetric.MetricSlice
}

func newMetrics() testMetricSlice {
	m := pmetric.NewMetrics()
	return testMetricSlice{
		m: m,
		ms: m.ResourceMetrics().AppendEmpty().
			ScopeMetrics().AppendEmpty().
			Metrics(),
	}
}

func (tms testMetricSlice) addMetric(
	names []string,
	val float64,
	attrs map[string]string,
	t pmetric.MetricType,
) testMetricSlice {
	for _, name := range names {
		m := tms.ms.AppendEmpty()
		m.SetName(name)
		switch t {
		case pmetric.MetricTypeGauge:
			dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
			dp.SetDoubleValue(val)
			for k, v := range attrs {
				dp.Attributes().PutStr(k, v)
			}
		}
	}
	return tms
}

func (tms testMetricSlice) get() pmetric.Metrics {
	return tms.m
}

func getTestAggCfg() ([]string, []AggregationConfig) {
	cfgs := []AggregationConfig{
		{
			Name: "test_sum",
			MatchLabelValues: map[string]string{
				"k_1": "v_1",
			},
			Type: Sum,
		},
		{
			Name: "test_last",
			MatchLabelValues: map[string]string{
				"k_1": "v_1",
			},
			Type: Last,
		},
	}

	names := make([]string, 0, len(cfgs))
	for _, cfg := range cfgs {
		names = append(names, cfg.Name)
	}
	return names, cfgs
}
