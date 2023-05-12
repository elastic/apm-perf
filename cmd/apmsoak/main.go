// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"errors"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"go.elastic.co/ecszap"
	"go.uber.org/zap"

	"gopkg.in/yaml.v3"

	"github.com/elastic/apm-perf/soaktest"
)

func main() {
	encoderConfig := ecszap.NewDefaultEncoderConfig()
	core := ecszap.NewCore(encoderConfig, os.Stdout, zap.InfoLevel)
	logger := zap.New(core, zap.AddCaller())

	flag.Parse()
	f, err := os.ReadFile(soaktest.SoakConfig.ScenariosPath)
	if err != nil {
		logger.Fatal("failed to read scenarios.yml", zap.Error(err))
	}

	var y soaktest.Scenarios
	if err := yaml.Unmarshal(f, &y); err != nil {
		logger.Fatal("failed to unmarshal scenarios", zap.Error(err))
	}
	scenarioConfigs := y.Scenarios[soaktest.SoakConfig.Scenario]
	if scenarioConfigs == nil {
		logger.Fatal("unknown scenario "+soaktest.SoakConfig.Scenario, zap.Error(err))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	logger.Info("running scenario `" + soaktest.SoakConfig.Scenario + "`...")
	if err := soaktest.Run(ctx, scenarioConfigs); err != nil {
		if !errors.Is(err, context.Canceled) {
			logger.Fatal("runner exited with error", zap.Error(err))
		}
	}
}
