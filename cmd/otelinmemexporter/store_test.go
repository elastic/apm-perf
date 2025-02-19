// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package otelinmemexporter

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

const groupKey = "grp_key"

func TestNewStore(t *testing.T) {
	logger := zap.NewExample()
	_, validCfg := getNumTestAggCfg()
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
			name: "duplicate_keys",
			cfgs: []AggregationConfig{
				{
					Name: "agg_cfg_1",
					MatchLabelValues: map[string]string{
						"k1": "v1",
					},
					Type: Sum,
					Key:  "agg",
				},
				{
					Name: "agg_cfg_2",
					MatchLabelValues: map[string]string{
						"k1": "v1",
					},
					Type: Last,
					Key:  "agg",
				},
			},
			errMsg: "key should be unique",
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

func TestAdd_NumberDataPoint(t *testing.T) {
	startTime := time.Unix(0, 0).UTC().Add(time.Second)
	allMetricNames, cfgs := getNumTestAggCfg()
	for _, tt := range []struct {
		name     string
		input    pmetric.Metrics
		expected []map[string]float64 // for each aggregation query 1 output
	}{
		{
			name: "no_config",
			input: newMetrics(nil).addGaugeMetric(
				[]string{"404"}, 1.1,
				nil,
				startTime, startTime,
			).collect(),
			expected: []map[string]float64{
				map[string]float64{"": 0},
				map[string]float64{"": 0},
				map[string]float64{"": 0},
				map[string]float64{"": 0},
				map[string]float64{"": 0},
				map[string]float64{"": 0},
			},
		},
		{
			name: "filtered_input",
			input: newMetrics(nil).
				addGaugeMetric(
					allMetricNames, 1.1,
					map[string]string{"k_1": "v_1", groupKey: "grp1"},
					startTime, startTime.Add(time.Second),
				).
				addGaugeMetric(
					allMetricNames, 2.2,
					map[string]string{"k_1": "v_1", "k_2": "v_2", groupKey: "grp1"},
					startTime.Add(time.Second), startTime.Add(2*time.Second),
				).
				addGaugeMetric(
					allMetricNames, 3.3,
					map[string]string{"k_1": "v_1", "k_2": "v_2"},
					startTime.Add(2*time.Second), startTime.Add(3*time.Second),
				).collect(),
			expected: []map[string]float64{
				map[string]float64{"": 3.3},               // last
				map[string]float64{"": 6.6},               // sum
				map[string]float64{"": 2.2},               // rate
				map[string]float64{"": 3.3, "grp1": 2.2},  // group_by last
				map[string]float64{"": 3.3, "grp1": 3.3},  // group_by sum
				map[string]float64{"": 3.3, "grp1": 1.65}, // group_by rate
			},
		},
		{
			name: "filtered_input_with_resource_attrs",
			input: newMetrics(map[string]string{"k_1": "v_1", "k_2": "v_2", groupKey: "grp1"}).
				addGaugeMetric(
					allMetricNames, 1.1,
					nil,
					startTime, startTime.Add(time.Second),
				).
				addGaugeMetric(
					allMetricNames, 2.2,
					map[string]string{"k_3": "v_3"},
					startTime.Add(time.Second), startTime.Add(2*time.Second),
				).collect(),
			expected: []map[string]float64{
				map[string]float64{"": 2.2},      // last
				map[string]float64{"": 3.3},      // sum
				map[string]float64{"": 1.65},     // rate
				map[string]float64{"grp1": 2.2},  // group_by last
				map[string]float64{"grp1": 3.3},  // group_by sum
				map[string]float64{"grp1": 1.65}, // group_by rate
			},
		},
		{
			name: "unfiltered_input",
			input: newMetrics(map[string]string{groupKey: "grp1"}).
				// no labels
				addGaugeMetric(
					allMetricNames, 1.1,
					nil,
					startTime, startTime,
				).
				// label key doesn't match
				addGaugeMetric(
					allMetricNames, 1.1,
					map[string]string{"k_2": "v_1"},
					startTime, startTime,
				).
				// label value doesn't match
				addGaugeMetric(
					allMetricNames, 2.2,
					map[string]string{"k_1": "v_2"},
					startTime, startTime,
				).
				// name doesn't match
				addGaugeMetric(
					[]string{"404"}, 3.3,
					map[string]string{"k_1": "v_1"},
					startTime, startTime,
				).collect(),
			expected: []map[string]float64{
				map[string]float64{"": 0},
				map[string]float64{"": 0},
				map[string]float64{"": 0},
				map[string]float64{"": 0},
				map[string]float64{"": 0},
				map[string]float64{"": 0},
			},
		},
		{
			name: "mixed_input",
			input: newMetrics(nil).
				addGaugeMetric(
					allMetricNames, 1.1,
					map[string]string{"k_1": "v_1"},
					startTime, startTime.Add(time.Second),
				).
				addGaugeMetric(
					allMetricNames, 2.2,
					map[string]string{"k_2": "v_1", groupKey: "grp1"},
					startTime, startTime.Add(time.Second),
				).
				addGaugeMetric(
					allMetricNames, 3.3,
					map[string]string{"k_1": "v_1", groupKey: "grp1"},
					startTime.Add(time.Second), startTime.Add(2*time.Second),
				).
				addGaugeMetric(
					allMetricNames, 4.4,
					map[string]string{"k_1": "v_1", groupKey: "grp2"},
					startTime.Add(2*time.Second), startTime.Add(4*time.Second),
				).collect(),
			expected: []map[string]float64{
				map[string]float64{"": 4.4},                           // last
				map[string]float64{"": 8.8},                           // sum
				map[string]float64{"": 2.2},                           // rate
				map[string]float64{"": 1.1, "grp1": 3.3, "grp2": 4.4}, // group_by last
				map[string]float64{"": 1.1, "grp1": 3.3, "grp2": 4.4}, // group_by sum
				map[string]float64{"": 1.1, "grp1": 3.3, "grp2": 2.2}, // group_by rate
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runStoreAddTest(t, cfgs, tt.input, tt.expected)
		})
	}
}

func TestAdd_HistogramDataPoint(t *testing.T) {
	startTime := time.Unix(0, 0).UTC().Add(time.Second)
	p50MetricNames, p50Cfgs := getHistTestAggCfg(50)
	p95MetricNames, p95Cfgs := getHistTestAggCfg(95)
	simpleBuckets := []explicitBucket{
		{UpperBound: 0, Count: 0},
		{UpperBound: 10, Count: 1},
		{UpperBound: math.Inf(+1), Count: 0},
	}

	for _, tt := range []struct {
		name     string
		configs  []AggregationConfig
		input    pmetric.Metrics
		expected []map[string]float64 // for each aggregation query 1 output
	}{
		{
			name:    "no_config",
			configs: p95Cfgs,
			input: newMetrics(nil).
				addHistMetric(
					[]string{"404"},
					simpleBuckets,
					5,
					nil,
					startTime, startTime,
				).collect(),
			expected: []map[string]float64{
				{"": 0},
				{"": 0},
				{"": 0},
				{"": 0},
			},
		},
		{
			name: "filtered_input",
			input: newMetrics(nil).
				addHistMetric(
					p95MetricNames,
					[]explicitBucket{
						{UpperBound: 0, Count: 0},
						{UpperBound: 5, Count: 0},
						{UpperBound: 10, Count: 10},
						{UpperBound: math.Inf(+1), Count: 0},
					},
					48,
					map[string]string{"k_1": "v_1", groupKey: "grp1"},
					startTime, startTime.Add(time.Second),
				).
				addHistMetric(
					p95MetricNames,
					[]explicitBucket{
						{UpperBound: 0, Count: 0},
						{UpperBound: 5, Count: 0},
						{UpperBound: 10, Count: 10},
						{UpperBound: math.Inf(+1), Count: 0},
					},
					42,
					map[string]string{"k_1": "v_1", "k_2": "v_2", groupKey: "grp1"},
					startTime.Add(time.Second), startTime.Add(2*time.Second),
				).
				addHistMetric(
					p95MetricNames,
					[]explicitBucket{
						{UpperBound: 0, Count: 0},
						{UpperBound: 5, Count: 10},
						{UpperBound: 10, Count: 9},
						{UpperBound: math.Inf(+1), Count: 1},
					},
					35,
					map[string]string{"k_1": "v_1", "k_2": "v_2"},
					startTime.Add(2*time.Second), startTime.Add(3*time.Second),
				).collect(),
			expected: []map[string]float64{
				{"": 9.8275862068},     // percentile
				{"": 125},              // sum
				{"": 10, "grp1": 9.75}, // group_by percentile
				{"": 35, "grp1": 90},   // group_by sum
			},
		},
		{
			name: "unfiltered_input",
			input: newMetrics(map[string]string{groupKey: "grp1"}).
				// no labels
				addHistMetric(
					p95MetricNames,
					simpleBuckets,
					5,
					nil,
					startTime, startTime,
				).
				// label key doesn't match
				addHistMetric(
					p95MetricNames,
					simpleBuckets,
					5,
					map[string]string{"k_2": "v_1"},
					startTime, startTime,
				).
				// label value doesn't match
				addHistMetric(
					p95MetricNames,
					simpleBuckets,
					5,
					map[string]string{"k_1": "v_2"},
					startTime, startTime,
				).
				// name doesn't match
				addHistMetric(
					[]string{"404"},
					simpleBuckets,
					5,
					map[string]string{"k_1": "v_1"},
					startTime, startTime,
				).collect(),
			expected: []map[string]float64{
				{"": 0},
				{"": 0},
				{"": 0},
				{"": 0},
			},
		},
		{
			name:    "mixed_input",
			configs: p50Cfgs,
			input: newMetrics(nil).
				addHistMetric(
					p50MetricNames,
					[]explicitBucket{
						{UpperBound: 0, Count: 0},
						{UpperBound: 1, Count: 1},
						{UpperBound: 2, Count: 1},
						{UpperBound: 3, Count: 1},
						{UpperBound: math.Inf(+1), Count: 0},
					},
					5,
					map[string]string{"k_1": "v_1"},
					startTime, startTime.Add(time.Second),
				).
				// label key mismatch
				addHistMetric(
					p50MetricNames,
					[]explicitBucket{
						{UpperBound: 0, Count: 0},
						{UpperBound: 1, Count: 1},
						{UpperBound: 2, Count: 0},
						{UpperBound: 3, Count: 0},
						{UpperBound: math.Inf(+1), Count: 0},
					},
					1,
					map[string]string{"k_2": "v_1", groupKey: "grp1"},
					startTime, startTime.Add(time.Second),
				).
				addHistMetric(
					p50MetricNames,
					[]explicitBucket{
						{UpperBound: 0, Count: 0},
						{UpperBound: 1, Count: 0},
						{UpperBound: 2, Count: 1},
						{UpperBound: 3, Count: 0},
						{UpperBound: math.Inf(+1), Count: 0},
					},
					2,
					map[string]string{"k_1": "v_1", groupKey: "grp1"},
					startTime.Add(time.Second), startTime.Add(2*time.Second),
				).
				addHistMetric(
					p50MetricNames,
					[]explicitBucket{
						{UpperBound: 0, Count: 0},
						{UpperBound: 1, Count: 0},
						{UpperBound: 2, Count: 0},
						{UpperBound: 3, Count: 1},
						{UpperBound: math.Inf(+1), Count: 0},
					},
					3,
					map[string]string{"k_1": "v_1", groupKey: "grp2"},
					startTime.Add(2*time.Second), startTime.Add(4*time.Second),
				).collect(),
			expected: []map[string]float64{
				{"": 1.75},                          // percentile
				{"": 10},                            // sum
				{"": 1.5, "grp1": 1.5, "grp2": 2.5}, // group_by percentile
				{"": 5, "grp1": 2, "grp2": 3},       // group_by sum
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runStoreAddTest(t, tt.configs, tt.input, tt.expected)
		})
	}
}

func TestAdd_MixedDataPoints(t *testing.T) {
	startTime := time.Unix(0, 0).UTC().Add(time.Second)
	cfgs := []AggregationConfig{
		{
			Name: "test_num_sum",
			MatchLabelValues: map[string]string{
				"k_1": "v_1",
			},
			Type:    Sum,
			Key:     "k1",
			GroupBy: "",
		},
		{
			Name: "test_hist_sum",
			MatchLabelValues: map[string]string{
				"k_1": "v_1",
			},
			Type:    Sum,
			Key:     "k2",
			GroupBy: "",
		},
	}

	for _, tt := range []struct {
		name     string
		configs  []AggregationConfig
		input    pmetric.Metrics
		expected []map[string]float64 // for each aggregation query 1 output
	}{
		{
			name:    "gauge_and_histogram",
			configs: cfgs,
			input: newMetrics(nil).
				addGaugeMetric(
					[]string{"test_num_sum"}, 1.23,
					map[string]string{"k_1": "v_1"},
					startTime, startTime.Add(time.Second),
				).
				addHistMetric(
					[]string{"test_hist_sum"},
					[]explicitBucket{
						{UpperBound: 0, Count: 0},
						{UpperBound: 10, Count: 1},
						{UpperBound: math.Inf(+1), Count: 0},
					},
					9.58278234,
					map[string]string{"k_1": "v_1"},
					startTime, startTime.Add(time.Second),
				).collect(),
			expected: []map[string]float64{
				{"": 1.23},       // gauge
				{"": 9.58278234}, // hist
			},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			runStoreAddTest(t, tt.configs, tt.input, tt.expected)
		})
	}
}

func runStoreAddTest(
	t *testing.T,
	configs []AggregationConfig,
	input pmetric.Metrics,
	expected []map[string]float64,
) {
	store, err := NewStore(configs, zap.NewNop())
	require.NoError(t, err)

	store.Add(input)
	// Assert GetAll
	assert.NotPanics(t, func() { store.GetAll() })
	// Assert if data is correctly handled
	for i := 0; i < len(configs); i++ {
		actual, err := store.Get(configs[i].Key)
		assert.NoError(t, err)
		if assert.Equal(t, len(expected[i]), len(actual)) {
			assert.InDeltaMapValues(t, expected[i], actual, 1e-9)
		}
	}
}

type testMetricSlice struct {
	m  pmetric.Metrics
	ms pmetric.MetricSlice
}

func newMetrics(resAttrs map[string]string) *testMetricSlice {
	m := pmetric.NewMetrics()
	rm := m.ResourceMetrics().AppendEmpty()
	for k, v := range resAttrs {
		rm.Resource().Attributes().PutStr(k, v)
	}
	return &testMetricSlice{
		m:  m,
		ms: rm.ScopeMetrics().AppendEmpty().Metrics(),
	}
}

func (tms *testMetricSlice) addGaugeMetric(
	names []string,
	val float64,
	attrs map[string]string,
	startTime, endTime time.Time,
) *testMetricSlice {
	for _, name := range names {
		m := tms.ms.AppendEmpty()
		m.SetName(name)
		dp := m.SetEmptyGauge().DataPoints().AppendEmpty()
		dp.SetDoubleValue(val)
		dp.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
		dp.SetTimestamp(pcommon.NewTimestampFromTime(endTime))
		for k, v := range attrs {
			dp.Attributes().PutStr(k, v)
		}
	}
	return tms
}

func (tms *testMetricSlice) addHistMetric(
	names []string,
	buckets []explicitBucket,
	sum float64,
	attrs map[string]string,
	startTime, endTime time.Time,
) *testMetricSlice {
	for _, name := range names {
		m := tms.ms.AppendEmpty()
		m.SetName(name)
		dp := m.SetEmptyHistogram().DataPoints().AppendEmpty()
		h := newTestHistogramDataPoint(buckets, sum, 0, 0, attrs, startTime, endTime)
		h.CopyTo(dp)
	}
	return tms
}

func (tms *testMetricSlice) collect() pmetric.Metrics {
	return tms.m
}

func getNumTestAggCfg() ([]string, []AggregationConfig) {
	cfgs := []AggregationConfig{
		{
			Name: "test_last",
			MatchLabelValues: map[string]string{
				"k_1": "v_1",
			},
			Type:    Last,
			Key:     "k1",
			GroupBy: "",
		},
		{
			Name: "test_sum",
			MatchLabelValues: map[string]string{
				"k_1": "v_1",
			},
			Type:    Sum,
			Key:     "k2",
			GroupBy: "",
		},
		{
			Name: "test_rate",
			MatchLabelValues: map[string]string{
				"k_1": "v_1",
			},
			Type:    Rate,
			Key:     "k3",
			GroupBy: "",
		},
		{
			Name: "test_last_groupby",
			MatchLabelValues: map[string]string{
				"k_1": "v_1",
			},
			Type:    Last,
			Key:     "k4",
			GroupBy: groupKey,
		},
		{
			Name: "test_sum_groupby",
			MatchLabelValues: map[string]string{
				"k_1": "v_1",
			},
			Type:    Sum,
			Key:     "k5",
			GroupBy: groupKey,
		},
		{
			Name: "test_rate_groupby",
			MatchLabelValues: map[string]string{
				"k_1": "v_1",
			},
			Type:    Rate,
			Key:     "k6",
			GroupBy: groupKey,
		},
	}

	names := make([]string, 0, len(cfgs))
	for _, cfg := range cfgs {
		names = append(names, cfg.Name)
	}
	return names, cfgs
}

func getHistTestAggCfg(percentile float64) ([]string, []AggregationConfig) {
	cfgs := []AggregationConfig{
		{
			Name: "test_percentile",
			MatchLabelValues: map[string]string{
				"k_1": "v_1",
			},
			Type:       Percentile,
			Percentile: percentile,
			Key:        "k1",
			GroupBy:    "",
		},
		{
			Name: "test_sum",
			MatchLabelValues: map[string]string{
				"k_1": "v_1",
			},
			Type:    Sum,
			Key:     "k2",
			GroupBy: "",
		},
		{
			Name: "test_percentile_groupby",
			MatchLabelValues: map[string]string{
				"k_1": "v_1",
			},
			Type:       Percentile,
			Percentile: percentile,
			Key:        "k3",
			GroupBy:    groupKey,
		},
		{
			Name: "test_sum_groupby",
			MatchLabelValues: map[string]string{
				"k_1": "v_1",
			},
			Type:    Sum,
			Key:     "k4",
			GroupBy: groupKey,
		},
	}

	names := make([]string, 0, len(cfgs))
	for _, cfg := range cfgs {
		names = append(names, cfg.Name)
	}
	return names, cfgs
}
