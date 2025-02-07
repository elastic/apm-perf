// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package loadgen

import (
	"testing"
	"time"
)

func TestParseEventRate(t *testing.T) {
	tests := []struct {
		name      string
		eventRate string
		burst     int
		rate      time.Duration
		wantErr   bool
	}{
		{
			name:      "valid event rate",
			eventRate: "10/1s",
			burst:     10,
			rate:      time.Second,
			wantErr:   false,
		},
		{
			name:      "valid event rate with milliseconds",
			eventRate: "100/1ms",
			burst:     100,
			rate:      time.Millisecond,
			wantErr:   false,
		},
		{
			name:      "infinite event rate removes limiter",
			eventRate: "-1/s",
			burst:     -1,
			rate:      time.Second,
			wantErr:   false,
		},
		{

			name:      "invalid event rate format",
			eventRate: "0/abc",
			burst:     0,
			rate:      0,
			wantErr:   true,
		},
		{
			name:      "invalid burst format",
			eventRate: "abc/1s",
			burst:     0,
			rate:      0,
			wantErr:   true,
		},
		{
			name:      "invalid format missing slash",
			eventRate: "100000",
			burst:     0,
			rate:      0,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, err := ParseEventRate(tt.eventRate)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseEventRate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.burst {
				t.Errorf("ParseEventRate() got = %v, want %v", got, tt.burst)
			}
			if got1 != tt.rate {
				t.Errorf("ParseEventRate() got1 = %v, want %v", got1, tt.rate)
			}
		})
	}
}
