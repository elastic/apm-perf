// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"flag"
	"fmt"
	"net/http"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/elastic/apm-perf/internal/proxy"
)

func main() {
	logLevel := zap.LevelFlag(
		"loglevel", zapcore.InfoLevel,
		"set log level to one of: DEBUG, INFO (default), WARN, ERROR, DPANIC, PANIC, FATAL",
	)
	username := flag.String("username", "elastic", "authentication username to mimic ES")
	password := flag.String("password", "", "authentication username to mimic ES")
	port := flag.Int("port", 9200, "http port to listen on")
	flag.Parse()
	zapcfg := zap.NewProductionConfig()
	zapcfg.EncoderConfig.EncodeTime = zapcore.RFC3339TimeEncoder
	zapcfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	zapcfg.Encoding = "console"
	zapcfg.Level = zap.NewAtomicLevelAt(*logLevel)
	logger, err := zapcfg.Build()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	options := []proxy.StubESOption{
		proxy.StubESWithLogger(logger),
	}
	if *username != "" && *password != "" {
		options = append(options, proxy.StubESWithAuth(*username, *password))
	}
	s := http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: proxy.NewHandlerStubES(options...),
	}
	if err := s.ListenAndServe(); err != nil {
		logger.Fatal("listen error", zap.Error(err))
	}
}
