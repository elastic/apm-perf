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
	"time"
)

var cfg struct {
	Count     uint
	Benchtime time.Duration
	RunRE     *regexp.Regexp
	// Sorted list of agents count to be used for benchmarking
	AgentsList               []int
	CollectorConfigYaml      string
	ServerMode               bool
	MonitoringAPMServerURL   string
	MonitoringAPMSecretToken string
}

func init() {
	cfg.AgentsList = []int{1}

	flag.UintVar(&cfg.Count, "count", 1, "run benchmarks `n` times")
	flag.DurationVar(&cfg.Benchtime, "benchtime", time.Second, "run each benchmark for duration `d`")
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
	flag.StringVar(&cfg.CollectorConfigYaml, "collector-config-yaml", "", "configuration for otel collector")
	flag.BoolVar(&cfg.ServerMode, "server-mode", false, "continue running otel collector post benchmark run")
	flag.StringVar(&cfg.MonitoringAPMServerURL, "monitoring-apm-server-url", "", "monitoring APM server URL")
	flag.StringVar(&cfg.MonitoringAPMSecretToken, "monitoring-apm-secret-token", "", "monitoring APM secret token")
}
