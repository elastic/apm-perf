// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventhandler

import "time"

type EventCollector interface {
	Filter([]byte) error
	IsMeta([]byte) bool
	Process([]byte) event
}

type EventWriter func(config Config, minTimestamp time.Time, w *eventWriter, b batch, baseTimestamp time.Time, randomBits uint64) error
