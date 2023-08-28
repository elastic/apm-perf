// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package version contains metadata for commands.
package version

import (
	"fmt"
	"time"
)

var (
	commitSha string
	buildTime string
)

// CommitSha returns the hash of the git commit used for the build.
func CommitSha() string {
	return commitSha
}

// BuildTime returns the timestamp of the commit used for the build.
func BuildTime() time.Time {
	value, err := time.Parse(time.RFC3339, buildTime)
	if err != nil {
		fmt.Println(err)
	}
	return value
}
