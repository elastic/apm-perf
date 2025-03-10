// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package supportedstacks exposes the supported stacks for telemetrygen and
// apm-perf.
//
// This is a separate package to allow reusing it across the packages without
// risking or forcing dependencies between unrelated packages.
package supportedstacks

type TargetStackVersion int

const (
	TargetStackVersionUnknown TargetStackVersion = iota
	TargetStackVersion7x
	TargetStackVersion8x
)
