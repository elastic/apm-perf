// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package otelinmemexporter

import (
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// AggregationType defines the type of aggregation the store
// will perform on a filtered metrics.
type AggregationType string

const (
	Last AggregationType = "last"
	Sum  AggregationType = "sum"
	Rate AggregationType = "rate"
)

// Store is a in-memory data store for telemetry data. Data
// exported from the in-memory exporter will be aggregated
// in the Store and queried from the store. Store only stores
// a specfic set of entries specified during creation.
type Store struct {
	sync.RWMutex
	nameM map[string][]AggregationConfig // keys are metric names
	keyM  map[string]*AggregationConfig
	nums  map[string]map[string]pmetric.NumberDataPoint

	logger *zap.Logger
}

// AggregationConfig defines the configuration for filtering,
// aggregating, and caching the metrics in the store.
//
// Each metric with a specific name and label values MUST have
// a unique aggregation type i.e. the store will only aggregate
// a single aggregation type for a specific name and label
// values combination. If duplicate entries are provided an
// error will be returned with the creation of a new store.
type AggregationConfig struct {
	// Key is used to describe the aggregated metrics produced by
	// the specified aggregation config. Key must be unique across
	// different aggregation configs.
	Key string `mapstructure:"key"`

	// Name specifies the metric name to include.
	Name string `mapstructure:"name"`

	// MatchLabelValues specifies a subset of attributes that
	// should match to aggregate a metric. All metrics with
	// matching labels subset will be aggregated together.
	MatchLabelValues map[string]string `mapstructure:"match_label_values"`

	// Type defines a type of aggregation that the store will
	// perform on a filtered metric with the given name and
	// label values. Only one type is allowed for a specific
	// combination of name and label values.
	Type AggregationType `mapstructure:"aggregation_type"`

	// GroupBy allows grouping the metrics by a specific key. An
	// empty value for group by signifies no grouping.
	GroupBy string `mapstructure:"group_by"`
}

// MarshalLogObject implements zapcore.ObjectMarshaler to allow adding
// config to logging context.
func (cfg *AggregationConfig) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("name", cfg.Name)
	enc.AddString("type", string(cfg.Type))
	enc.AddString("key", cfg.Key)
	enc.AddString("group_by", cfg.GroupBy)
	enc.AddObject("label_values", zapcore.ObjectMarshalerFunc(
		func(enc zapcore.ObjectEncoder) error {
			for l, v := range cfg.MatchLabelValues {
				enc.AddString("label", l)
				enc.AddString("value", v)
			}
			return nil
		},
	))
	return nil
}

func (cfg *AggregationConfig) isEqualIgnoringType(target AggregationConfig) bool {
	if cfg.Name != target.Name {
		return false
	}
	if len(cfg.MatchLabelValues) != len(target.MatchLabelValues) {
		return false
	}
	for k, v := range cfg.MatchLabelValues {
		targetV, ok := target.MatchLabelValues[k]
		if !ok || v != targetV {
			return false
		}
	}
	return true
}

func (cfg *AggregationConfig) isEqual(
	name string,
	attrs, resAttrs pcommon.Map,
) bool {
	if cfg.Name != name {
		return false
	}
	if len(cfg.MatchLabelValues) > (attrs.Len() + resAttrs.Len()) {
		return false
	}
	for k, v := range cfg.MatchLabelValues {
		targetV := getValueFromMaps(k, attrs, resAttrs)
		if targetV.Type() == pcommon.ValueTypeEmpty || v != targetV.AsString() {
			return false
		}
	}
	return true
}

// NewStore creates a new in memory metric store. Returns an
// error if the provided config is invalid.
func NewStore(aggs []AggregationConfig, logger *zap.Logger) (*Store, error) {
	store, err := validateAggregationConfig(aggs)
	if err != nil {
		return nil, err
	}
	store.logger = logger
	return store, nil
}

// Add adds metrics to the store.
func (s *Store) Add(ld pmetric.Metrics) {
	s.Lock()
	defer s.Unlock()

	rms := ld.ResourceMetrics()
	for i := 0; i < rms.Len(); i++ {
		rm := rms.At(i)
		resAttrs := rm.Resource().Attributes()
		sms := rm.ScopeMetrics()
		for j := 0; j < sms.Len(); j++ {
			ms := sms.At(j).Metrics()
			for k := 0; k < ms.Len(); k++ {
				s.add(ms.At(k), resAttrs)
			}
		}
	}
}

// GetAll returns all the aggregated values for all the configured
// aggregation configs. If `GroupBy` is configured in the aggregation
// config then the results are grouped based on the observed values
// for the grouped by key. No grouping is identified by an empty key.
func (s *Store) GetAll() map[string]map[string]float64 {
	s.RLock()
	defer s.RUnlock()

	m := make(map[string]map[string]float64, len(s.nums))
	for key, cfg := range s.keyM {
		dpByGrp, ok := s.nums[key]
		if !ok {
			m[key] = map[string]float64{"": 0}
			continue
		}
		m[key] = make(map[string]float64, len(dpByGrp))
		for grp, dp := range dpByGrp {
			m[key][grp] = getByType(cfg.Type, dp)
		}
	}
	return m
}

// Get returns the aggregated value of a configured aggregation config.
// If `GroupBy` is configured in the aggregation config then the results
// are grouped based on the observed values for the grouped by key. No
// grouping is identified by an empty key.
func (s *Store) Get(key string) (map[string]float64, error) {
	s.RLock()
	defer s.RUnlock()

	cfg, ok := s.keyM[key]
	if !ok {
		return nil, fmt.Errorf("key %s is not configured", key)
	}

	dpByGrp, ok := s.nums[key]
	if !ok {
		return map[string]float64{"": 0}, nil
	}

	m := make(map[string]float64, len(dpByGrp))
	for k, dp := range dpByGrp {
		m[k] = getByType(cfg.Type, dp)
	}
	return m, nil
}

// Reset resets the store by deleting all cached data.
func (s *Store) Reset() {
	s.Lock()
	defer s.Unlock()

	for k := range s.nums {
		delete(s.nums, k)
	}
}

func (s *Store) add(m pmetric.Metric, resAttrs pcommon.Map) {
	// Fast fail if metric name is not filtered
	_, ok := s.nameM[m.Name()]
	if !ok {
		s.logger.Debug(
			"skipping metric, no config matched",
			zap.String("name", m.Name()),
			zap.String("type", m.Type().String()),
		)
		return
	}

	switch m.Type() {
	case pmetric.MetricTypeGauge:
		s.mergeNumberDataPoints(m.Name(), m.Gauge().DataPoints(), resAttrs)
	case pmetric.MetricTypeSum:
		if m.Sum().AggregationTemporality() == pmetric.AggregationTemporalityCumulative {
			s.logger.Warn(
				"unexpected, all cumulative temporality should be converted to delta",
				zap.String("name", m.Name()),
				zap.String("type", m.Type().String()),
			)
			return
		}
		s.mergeNumberDataPoints(m.Name(), m.Sum().DataPoints(), resAttrs)
	default:
		s.logger.Warn(
			"metric type not implemented",
			zap.String("type", m.Type().String()),
		)
	}
}

func (s *Store) mergeNumberDataPoints(
	name string,
	from pmetric.NumberDataPointSlice,
	resAttrs pcommon.Map,
) {
	if s.nums == nil {
		s.nums = make(map[string]map[string]pmetric.NumberDataPoint)
	}

	for i := 0; i < from.Len(); i++ {
		dp := from.At(i)
		attrs := dp.Attributes()
		for _, cfg := range s.filterCfgs(name, attrs, resAttrs) {
			grp := getValueFromMaps(cfg.GroupBy, attrs, resAttrs).AsString()
			if _, ok := s.nums[cfg.Key]; !ok {
				s.nums[cfg.Key] = make(map[string]pmetric.NumberDataPoint)
			}
			if _, ok := s.nums[cfg.Key][grp]; !ok {
				s.nums[cfg.Key][grp] = pmetric.NewNumberDataPoint()
			}
			to := s.nums[cfg.Key][grp]
			switch cfg.Type {
			case Last:
				to.SetDoubleValue(doubleValue(dp))
			case Sum:
				to.SetDoubleValue(to.DoubleValue() + doubleValue(dp))
			case Rate:
				val := doubleValue(dp)
				if val != 0 {
					to.SetDoubleValue(to.DoubleValue() + val)
					// We will use to#StartTimestamp and to#Timestamp fields to
					// cache the lowest and the highest timestamps. This will be
					// used at query time to calculate rate.
					if to.StartTimestamp() == 0 {
						// If the data point has a start timestamp then use that
						// as the start timestamp, else use the end timestamp.
						if dp.StartTimestamp() != 0 {
							to.SetStartTimestamp(dp.StartTimestamp())
						} else {
							to.SetStartTimestamp(dp.Timestamp())
						}
					}
					if to.Timestamp() < dp.Timestamp() {
						to.SetTimestamp(dp.Timestamp())
					}
				}
			}
		}
	}
}

func (s *Store) filterCfgs(
	name string,
	attrs, resAttrs pcommon.Map,
) []AggregationConfig {
	cfgs, ok := s.nameM[name]
	if !ok {
		return nil
	}
	var result []AggregationConfig
	for _, cfg := range cfgs {
		if cfg.isEqual(name, attrs, resAttrs) {
			result = append(result, cfg)
		}
	}
	return result
}

func getByType(typ AggregationType, dp pmetric.NumberDataPoint) float64 {
	switch typ {
	case Rate:
		if dp.DoubleValue() == 0 {
			return 0
		}
		duration := time.Duration(dp.Timestamp() - dp.StartTimestamp()).Seconds()
		if duration <= 0 {
			return 0
		}
		return dp.DoubleValue() / duration
	default:
		return dp.DoubleValue()
	}
}

func validateAggregationConfig(src []AggregationConfig) (*Store, error) {
	nameM := make(map[string][]AggregationConfig)
	keyM := make(map[string]*AggregationConfig)
	for i := range src {
		srcCfg := src[i]
		if toCfgs, ok := nameM[srcCfg.Name]; ok {
			for _, toCfg := range toCfgs {
				if toCfg.isEqualIgnoringType(srcCfg) {
					if toCfg.Type != srcCfg.Type {
						return nil, fmt.Errorf("cannot record same metric with different types: %s", srcCfg.Name)
					}
					return nil, fmt.Errorf("duplicate config found: %s", srcCfg.Name)
				}
			}
		}
		if _, seen := keyM[srcCfg.Key]; seen {
			return nil, fmt.Errorf("key should be unique, found duplicate: %s", srcCfg.Key)
		}
		nameM[srcCfg.Name] = append(nameM[srcCfg.Name], srcCfg)
		keyM[srcCfg.Key] = &srcCfg
	}
	return &Store{
		nameM: nameM,
		keyM:  keyM,
	}, nil
}

func doubleValue(dp pmetric.NumberDataPoint) float64 {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleValue()
	case pmetric.NumberDataPointValueTypeInt:
		return float64(dp.IntValue())
	}
	return 0
}

func getValueFromMaps(key string, maps ...pcommon.Map) pcommon.Value {
	for _, m := range maps {
		v, ok := m.Get(key)
		if ok {
			return v
		}
	}
	return pcommon.NewValueEmpty()
}
