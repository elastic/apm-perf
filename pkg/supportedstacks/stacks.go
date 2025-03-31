// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package supportedstacks exposes the supported stacks for telemetrygen and
// apm-perf.
//
// This is a separate package to allow reusing it across the packages without
// risking or forcing dependencies between unrelated packages.
package supportedstacks

import (
	"fmt"
)

type TargetStackVersion int

const (
	// TargetStackVersionUnknown identifies an unknown version.
	// The expected behavior for such version is to trigger errors
	// or panics to alert the caller and user that an undefined
	// or unexpected behavior may happen.
	TargetStackVersionUnknown TargetStackVersion = iota
	TargetStackVersionLatest
	// TargetStackVersion7x identifies a generic 7.x.y version
	TargetStackVersion7x
)

const (
	latest = "latest"

	generic7x = "7x"
)

// FromStringVersion returns the appropriate TargetStackVersion from
// a string.
//
// Valid values are:
//   - "latest", to automatically specify the latest version as determined
//     by this function
//   - "7x", to select the generic major version
//
// If no version is matched will return TargetStackVersionUnknown.
func FromStringVersion(version string) (TargetStackVersion, error) {
	switch version {
	case latest:
		return TargetStackVersionLatest, nil
	case generic7x:
		return TargetStackVersion7x, nil
	}

	return TargetStackVersionUnknown, fmt.Errorf("cannot determine stack version from string: %s", version)
}
