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

	"github.com/elastic/apm-perf/soaktest"
)

func main() {
	encoderConfig := ecszap.NewDefaultEncoderConfig()
	core := ecszap.NewCore(encoderConfig, os.Stdout, zap.InfoLevel)
	logger := zap.New(core, zap.AddCaller())

	flag.Parse()
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	runner, err := soaktest.NewRunner(soaktest.SoakConfig.ScenariosPath, soaktest.SoakConfig.Scenario, logger)
	if err != nil {
		logger.Fatal("Fail to initialize runner", zap.Error(err))
	}

	if err := runner.Run(ctx); err != nil {
		if !errors.Is(err, context.Canceled) {
			logger.Fatal("runner exited with error", zap.Error(err))
		}
	}
}
