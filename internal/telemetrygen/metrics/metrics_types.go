// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// This file is forked from https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/790e18f1733e71debc7608aed98ace654ac76a60/cmd/telemetrygen/internal/metrics/metrics_types.go,
// which is licensed under Apache-2 and Copyright The OpenTelemetry Authors.
//
// This file does not contain significant modifications.

package metrics

import (
	"errors"
)

type metricType string

const (
	metricTypeGauge = "Gauge"
	metricTypeSum   = "Sum"
)

// String is used both by fmt.Print and by Cobra in help text
func (e *metricType) String() string {
	return string(*e)
}

// Set must have pointer receiver so it doesn't change the value of a copy
func (e *metricType) Set(v string) error {
	switch v {
	case "Gauge", "Sum":
		*e = metricType(v)
		return nil
	default:
		return errors.New(`must be one of "Gauge" or "Sum"`)
	}
}

// Type is only used in help text
func (e *metricType) Type() string {
	return "metricType"
}
