// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package loadgen

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

// GetNewLimiter returns a rate limiter that is configured to rate limit burst per interval.
// eps is calculated as burst/interval(sec), and burst size is set as it is.
// When using it to send events at steady interval, the caller should control
// the events size to be equal to burst, and use `WaitN(ctx, burst)`.
// For example, if the event rate is 100/5s(burst=100,interval=5s) the eps is 20,
// Meaning to send 100 events, the limiter will wait for 5 seconds.
func GetNewLimiter(burst int, interval time.Duration) *rate.Limiter {
	if burst <= 0 || interval <= 0 {
		return rate.NewLimiter(rate.Inf, 0)
	}
	eps := float64(burst) / interval.Seconds()
	return rate.NewLimiter(rate.Limit(eps), burst)
}

// ParseEventRate takes a string in the format of "burst/duration" and returns the burst,
// and the duration, and an error if any. Burst is the number of events and duration is
// the time period in which these events occur. If the string is not in the expected
// format, an error is returned.
func ParseEventRate(eventRate string) (int, time.Duration, error) {
	before, after, ok := strings.Cut(eventRate, "/")
	if !ok || before == "" || after == "" {
		return 0, 0, fmt.Errorf("invalid rate %q, expected format burst/duration", eventRate)
	}

	burst, err := strconv.Atoi(before)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid burst %s in event rate: %w", before, err)
	}

	if !(after[0] >= '0' && after[0] <= '9') {
		after = "1" + after
	}
	interval, err := time.ParseDuration(after)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid interval %q in event rate: %w", after, err)
	}
	if interval <= 0 {
		return 0, 0, fmt.Errorf("invalid interval %q, must be positive", after)
	}

	return burst, interval, nil
}
