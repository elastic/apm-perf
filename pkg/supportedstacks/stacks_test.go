// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package supportedstacks_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/elastic/apm-perf/pkg/supportedstacks"
)

func TestFromString(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		want      supportedstacks.TargetStackVersion
		wantError bool
	}{
		{"empty", "", supportedstacks.TargetStackVersionUnknown, true},
		{"latest", "latest", supportedstacks.TargetStackVersion8x, false},
		{"7x", "7x", supportedstacks.TargetStackVersion7x, false},
		{"8x", "8x", supportedstacks.TargetStackVersion8x, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := supportedstacks.FromStringVersion(tt.version)
			if tt.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, tt.want, v)
		})
	}
}
