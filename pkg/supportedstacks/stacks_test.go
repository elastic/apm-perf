package supportedstacks_test

import (
	"testing"

	"github.com/elastic/apm-perf/pkg/supportedstacks"
	"github.com/stretchr/testify/require"
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
