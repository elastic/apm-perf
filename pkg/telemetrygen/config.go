// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package telemetrygen

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/elastic/apm-perf/pkg/supportedstacks"
)

func DefaultConfig() Config {
	return Config{
		// default to run 1 replica of 4 agents (one per type).
		AgentReplicas: 1,
		// default to expecting a valid TLS certificate.
		Secure: true,
		// default to 10 events per second
		EventRate: RateFlag{Burst: 10, Interval: 1 * time.Second},
		// default to rewrite ids and timestamp to have new events recorded at the current time.
		RewriteIDs:        true,
		RewriteTimestamps: true,
		// default to 8.x to keep backward compatibility.
		TargetStackVersion: supportedstacks.TargetStackVersionLatest,
	}
}

type Config struct {
	// number of agents replicas to use, each replica launches 4 agents, one for each type
	AgentReplicas int

	ServerURL    *url.URL
	APIKey       string
	Headers      map[string]string
	Secure       bool
	EventRate    RateFlag
	IgnoreErrors bool

	RewriteIDs                bool
	RewriteTimestamps         bool
	RewriteServiceNames       bool
	RewriteServiceNodeNames   bool
	RewriteServiceTargetNames bool
	RewriteSpanNames          bool
	RewriteTransactionNames   bool
	RewriteTransactionTypes   bool

	TargetStackVersion supportedstacks.TargetStackVersion
}

func (c Config) Validate() error {
	errs := []error{}

	if c.ServerURL == nil {
		errs = append(errs, fmt.Errorf("ServerURL is required"))
	}

	return errors.Join(errs...)
}

type RateFlag struct {
	Burst    int
	Interval time.Duration
}

func (f *RateFlag) String() string {
	return fmt.Sprintf("%d/%s", f.Burst, f.Interval)
}

func (f *RateFlag) Set(s string) error {
	before, after, ok := strings.Cut(s, "/")
	if !ok || before == "" || after == "" {
		return fmt.Errorf("invalid rate %q, expected format burst/duration", s)
	}

	burst, err := strconv.Atoi(before)
	if err != nil {
		return fmt.Errorf("invalid burst %s in event rate: %w", before, err)
	}

	if !(after[0] >= '0' && after[0] <= '9') {
		after = "1" + after
	}
	interval, err := time.ParseDuration(after)
	if err != nil {
		return fmt.Errorf("invalid interval %q in event rate: %w", after, err)
	}
	if interval <= 0 {
		return fmt.Errorf("invalid interval %q, must be positive", after)
	}

	f.Burst = burst
	f.Interval = interval
	return nil
}
