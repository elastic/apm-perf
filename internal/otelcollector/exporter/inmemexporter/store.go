// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package inmemexporter

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"go.elastic.co/fastjson"
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
	nums map[string]pmetric.NumberDataPoint

	aggCfgM map[string][]AggregationConfig // keys are metric names
	logger  *zap.Logger
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
	// Name specifies the metric name to include.
	Name string `yaml:"name"`

	// MatchLabelValues specifies a subset of attributes that
	// should match to aggregate a metric. All metrics with
	// matching labels subset will be aggregated together.
	MatchLabelValues map[string]string `yaml:"match_label_values"`

	// Type defines a type of aggregation that the store will
	// perform on a filtered metric with the given name and
	// label values. Only one type is allowed for a specific
	// combination of name and label values.
	Type AggregationType `yaml:"aggregation_type"`

	// Alias is used to describe the aggregated metrics produced
	// by the aggregation config.
	Alias string `yaml:"alias"`
}

// MarshalLogObject implements zapcore.ObjectMarshaler to allow adding
// config to logging context.
func (cfg *AggregationConfig) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("name", cfg.Name)
	enc.AddString("type", string(cfg.Type))
	enc.AddString("alias", cfg.Alias)
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
		targetV, ok := attrs.Get(k)
		if !ok {
			targetV, ok = resAttrs.Get(k)
		}
		if !ok || v != targetV.AsString() {
			return false
		}
	}
	return true
}

// NewStore creates a new in memory metric store. Returns an
// error if the provided config is invalid.
func NewStore(aggs []AggregationConfig, logger *zap.Logger) (*Store, error) {
	aggCfgM, err := validateAggregationConfig(aggs)
	if err != nil {
		return nil, err
	}
	return &Store{
		aggCfgM: aggCfgM,
		logger:  logger,
	}, nil
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

// Get returns the aggregated value of a configured aggregation config.
func (s *Store) Get(cfg AggregationConfig) (float64, error) {
	var w fastjson.Writer
	key := getHashKey(cfg, &w)

	s.RLock()
	defer s.RUnlock()

	dp, ok := s.nums[key]
	if !ok {
		return 0, nil
	}
	switch cfg.Type {
	case Rate:
		duration := time.Duration(dp.Timestamp() - dp.StartTimestamp()).Seconds()
		if duration <= 0 {
			return 0, nil
		}
		return dp.DoubleValue() / duration, nil
	default:
		return dp.DoubleValue(), nil
	}
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
	_, ok := s.aggCfgM[m.Name()]
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
		s.nums = make(map[string]pmetric.NumberDataPoint)
	}

	var w fastjson.Writer
	for i := 0; i < from.Len(); i++ {
		dp := from.At(i)
		for _, cfg := range s.filterCfgs(name, dp.Attributes(), resAttrs) {
			key := getHashKey(cfg, &w)
			if _, ok := s.nums[key]; !ok {
				s.nums[key] = pmetric.NewNumberDataPoint()
			}
			to := s.nums[key]
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
	cfgs, ok := s.aggCfgM[name]
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

func validateAggregationConfig(src []AggregationConfig) (map[string][]AggregationConfig, error) {
	to := make(map[string][]AggregationConfig)
	for _, srcCfg := range src {
		if toCfgs, ok := to[srcCfg.Name]; ok {
			for _, toCfg := range toCfgs {
				if toCfg.isEqualIgnoringType(srcCfg) {
					if toCfg.Type != srcCfg.Type {
						return nil, fmt.Errorf("cannot record same metric with different types: %s", srcCfg.Name)
					}
					return nil, fmt.Errorf("duplicate config found: %s", srcCfg.Name)
				}
			}
		}
		to[srcCfg.Name] = append(to[srcCfg.Name], srcCfg)
	}
	return to, nil
}

func getHashKey(aggCfg AggregationConfig, w *fastjson.Writer) string {
	w.Reset()
	w.RawByte('{')

	w.RawString("\"n\":")
	w.String(aggCfg.Name)

	// TODO (lahsivjar): Can be optimized by caching sorted keys.
	sortedKeys := make([]string, 0, len(aggCfg.MatchLabelValues))
	for k := range aggCfg.MatchLabelValues {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, k := range sortedKeys {
		w.RawByte(',')
		w.String(k)
		w.RawByte(':')
		w.String(aggCfg.MatchLabelValues[k])
	}
	w.RawByte('}')
	return string(w.Bytes())
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
