// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"strings"

	"github.com/spf13/cobra"
	"go.elastic.co/ecszap"
	"go.uber.org/zap"

	"github.com/elastic/apm-perf/internal/soaktest"
)

type RunOptions struct {
	Scenario      string
	ScenariosPath string
	ServerURL     string
	SecretToken   string
	APIKeys       string
	BypassProxy   bool
	Loglevel      string
}

func (opts *RunOptions) toRunnerConfig() (*soaktest.RunnerConfig, error) {
	apiKeys := make(map[string]string)
	if opts.APIKeys != "" {
		pairs := strings.Split(opts.APIKeys, ",")
		for _, pair := range pairs {
			kv := strings.Split(pair, ":")
			if len(kv) != 2 {
				return nil, errors.New("invalid api keys provided. example: project_id:my_api_key")
			}
			apiKeys[kv[0]] = kv[1]
		}
	}
	return &soaktest.RunnerConfig{
		Scenario:      opts.Scenario,
		ScenariosPath: opts.ScenariosPath,
		ServerURL:     opts.ServerURL,
		SecretToken:   opts.SecretToken,
		APIKeys:       apiKeys,
		BypassProxy:   opts.BypassProxy,
	}, nil
}

func NewCmdRun() *cobra.Command {
	options := &RunOptions{}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run apmsoak",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := getLogger(options.Loglevel)

			config, err := options.toRunnerConfig()
			if err != nil {
				logger.Fatal("Fail to parse flags", zap.Error(err))
			}

			runner, err := soaktest.NewRunner(config, logger)
			if err != nil {
				logger.Fatal("Fail to initialize runner", zap.Error(err))
			}

			// Graceful shutdown driven by SIGINT.
			// Nothing else is expected to stop the runner.
			ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt)
			defer cancel()

			if err := runner.Run(ctx); err != nil {
				if !errors.Is(err, context.Canceled) {
					logger.Fatal("runner exited with error", zap.Error(err))
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&options.ServerURL, "server-url", "", "Server URL (default http://127.0.0.1:8200), if specified <project_id>, it will be replaced with the project_id provided by the config, (example: https://<project_id>.apm.elastic.cloud)")
	cmd.Flags().StringVar(&options.Scenario, "scenario", "steady", "Specify which scenario to use. the value should match one of the scenario key defined in given scenarios YAML file")
	cmd.Flags().StringVarP(&options.ScenariosPath, "file", "f", "./scenarios.yml", "Path to scenarios file")
	cmd.Flags().StringVar(&options.SecretToken, "secret-token", "", "Secret token for APM Server. Managed intake service doesn't support secret token")
	cmd.Flags().StringVar(&options.APIKeys, "api-keys", "", "API keys for managed service. Specify key value pairs as `project_id_1:my_api_key,project_id_2:my_key`")
	cmd.Flags().BoolVar(&options.BypassProxy, "bypass-proxy", false, "Detach from proxy dependency and provide projectID via header. Useful when testing locally")
	cmd.Flags().StringVar(&options.Loglevel, "log-level", "info", "Specify the log level to use when running this command. Supported values: debug, info, warn, error")
	return cmd
}

func getLogger(logLevel string) *zap.Logger {
	encoderConfig := ecszap.NewDefaultEncoderConfig()

	level := zap.InfoLevel
	switch logLevel {
	case "debug":
		level = zap.DebugLevel
	case "warn":
		level = zap.WarnLevel
	case "error":
		level = zap.ErrorLevel
	}

	core := ecszap.NewCore(encoderConfig, os.Stdout, level)
	logger := zap.New(core, zap.AddCaller())
	return logger
}
