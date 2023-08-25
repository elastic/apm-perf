// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"flag"
	"fmt"
	"log"
	"testing"
)

func main() {
	flag.Parse()

	flag.VisitAll(func(f *flag.Flag) {
		fmt.Printf("%s: %s\n", f.Name, f.Value)
	})

	if err := Run(
		Benchmark1000Transactions,
		BenchmarkOTLPTraces,
		BenchmarkAgentAll,
		BenchmarkAgentGo,
		BenchmarkAgentNodeJS,
		BenchmarkAgentPython,
		BenchmarkAgentRuby,
		Benchmark10000AggregationGroups,
	); err != nil {
		log.Fatal(err)
	}
}

func init() {
	testing.Init()
}
