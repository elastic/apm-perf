// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package loadgen

import (
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
