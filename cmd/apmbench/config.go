// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"flag"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

var cfg struct {
	Count     uint
	Benchtime string
	RunRE     *regexp.Regexp
	SkipRE    *regexp.Regexp
	// Sorted list of agents count to be used for benchmarking
	AgentsList                 []int
	BenchmarkTelemetryEndpoint string
	// CleanupKeys defines a list of telemetry keys that will be
	// used to determine if a specific benchmark funciton has run
	// to completion including draining of all associated data. The
	// cleanup keys, if specified, must be returned by the benchmark
	// telemetry endpoint. The metric associated with each key must
	// go to zero on completion of a benchmark.
	CleanupKeys []string
	Debug       bool
}

func init() {
	cfg.AgentsList = []int{1}

	flag.UintVar(&cfg.Count, "count", 1, "run benchmarks `n` times")
	flag.StringVar(&cfg.Benchtime, "benchtime", "1s", "run each benchmark for duration `d` or N times if `d` is of the form Nx")
	flag.Func("run", "run only benchmarks matching `regexp`", func(restr string) error {
		if restr != "" {
			re, err := regexp.Compile(restr)
			if err != nil {
				return err
			}
			cfg.RunRE = re
		}
		return nil
	})
	flag.Func("skip", "skip benchmarks matching `regexp`", func(restr string) error {
		if restr != "" {
			re, err := regexp.Compile(restr)
			if err != nil {
				return err
			}
			cfg.SkipRE = re
		}
		return nil
	})
	flag.Func("agents", "comma-separated `list` of agent counts to run each benchmark with",
		func(agents string) error {
			var agentsList []int
			for _, val := range strings.Split(agents, ",") {
				val = strings.TrimSpace(val)
				if val == "" {
					continue
				}
				n, err := strconv.Atoi(val)
				if err != nil || n <= 0 {
					return fmt.Errorf("invalid value %q for -agents", val)
				}
				agentsList = append(agentsList, n)
			}
			sort.Ints(agentsList)
			cfg.AgentsList = agentsList
			return nil
		},
	)
	flag.StringVar(&cfg.BenchmarkTelemetryEndpoint, "benchmark-telemetry-endpoint", "", "Telemetry endpoint that exposed benchmark telemetry data with reset capabilities")
	flag.Func("cleanup-keys", "comma-separated `list` of metric keys returned by the telemetry endpoint to monitor cleanup status after a benchmark run",
		func(raw string) error {
			var keys []string
			for _, val := range strings.Split(raw, ",") {
				val = strings.TrimSpace(val)
				if val == "" {
					continue
				}
				keys = append(keys, val)
			}
			cfg.CleanupKeys = keys
			return nil
		},
	)
	flag.BoolVar(&cfg.Debug, "debug", false, "setup debug logging level")
}
