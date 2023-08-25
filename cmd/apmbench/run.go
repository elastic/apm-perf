// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"errors"
	"flag"
	"fmt"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/elastic/apm-perf/internal/loadgen"
	loadgencfg "github.com/elastic/apm-perf/internal/loadgen/config"
	"go.elastic.co/apm/v2/stacktrace"
	"golang.org/x/time/rate"
)

const benchmarkFuncPrefix = "Benchmark"

// BenchmarkFunc is the benchmark function type accepted by Run.
type BenchmarkFunc func(*testing.B, *rate.Limiter)

type result struct {
	benchResult testing.BenchmarkResult
	skipped     bool
	failed      bool
}

// Run runs all the given BenchmarkFunc.
func Run(fns ...BenchmarkFunc) error {
	type benchmark struct {
		name string
		fn   BenchmarkFunc
	}

	// Set `test.benchtime` flag based on the custom `benchtime` flag.
	if err := flag.Set("test.benchtime", cfg.Benchtime.String()); err != nil {
		return fmt.Errorf("failed to set test.benchtime flag: %w", err)
	}

	var maxLenBenchName string
	benchmarks := make([]benchmark, 0, len(fns))
	for _, fn := range fns {
		name, err := benchmarkFuncName(fn)
		if err != nil {
			return err
		}
		if cfg.RunRE == nil || cfg.RunRE.MatchString(name) {
			if len(name) > len(maxLenBenchName) {
				maxLenBenchName = name
			}
			benchmarks = append(benchmarks, benchmark{
				name: name,
				fn:   fn,
			})
		} else {
			fmt.Printf("--- SKIP: %s\n", name)
		}
	}

	// maxLen is the max length of benchmark function that needs to be printed
	maxLen := len(fullBenchmarkName(
		maxLenBenchName, cfg.AgentsList[len(cfg.AgentsList)-1]))

	for _, agents := range cfg.AgentsList {
		runtime.GOMAXPROCS(agents)
		for _, b := range benchmarks {
			name := fullBenchmarkName(b.name, agents)
			result := runOne(b.fn)
			// testing.Benchmark discards all output so the only thing we can
			// retrive is the benchmark status and result.
			if result.skipped {
				fmt.Printf("--- SKIP: %s\n", name)
				continue
			}
			if result.failed {
				fmt.Printf("--- FAIL: %s\n", name)
				return fmt.Errorf("benchmark %q failed", name)
			}
			fmt.Printf("%-*s\t%s\t%s\n", maxLen, name, result.benchResult, result.benchResult.MemString())
		}
	}
	return nil
}

func runOne(fn BenchmarkFunc) (result result) {
	limiter := loadgen.GetNewLimiter(
		loadgencfg.Config.EventRate.Burst,
		loadgencfg.Config.EventRate.Interval,
	)
	result.benchResult = testing.Benchmark(func(b *testing.B) {
		b.ResetTimer()
		signal := make(chan struct{})
		// fn can panic or call runtime.Goexit, stopping the goroutine.
		// When that happens the function won't return and ok=false will
		// be returned, making the benchmark looks like failure.
		go func() {
			defer close(signal)
			fn(b, limiter)
		}()
		<-signal

		result.skipped = b.Skipped()
		result.failed = b.Failed()
	})
	return result
}

func fullBenchmarkName(name string, agents int) string {
	if agents != 1 {
		return fmt.Sprintf("%s-%d", name, agents)
	}
	return name
}

func benchmarkFuncName(f BenchmarkFunc) (string, error) {
	ffunc := runtime.FuncForPC(reflect.ValueOf(f).Pointer())
	if ffunc == nil {
		return "", errors.New("runtime.FuncForPC returned nil")
	}
	fullName := ffunc.Name()
	_, name := stacktrace.SplitFunctionName(fullName)
	if !strings.HasPrefix(name, benchmarkFuncPrefix) {
		return "", fmt.Errorf("benchmark function names must begin with %q (got %q)", fullName, benchmarkFuncPrefix)
	}
	return name, nil
}
