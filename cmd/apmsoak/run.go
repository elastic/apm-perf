// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

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
	Headers       map[string]string
	BypassProxy   bool
	Loglevel      string
	IgnoreErrors  bool
	RunForever    bool
	RunDuration   time.Duration
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
		Headers:       opts.Headers,
		BypassProxy:   opts.BypassProxy,
		IgnoreErrors:  opts.IgnoreErrors,
		RunForever:    opts.RunForever,
		RunDuration:   opts.RunDuration,
	}, nil
}

type headersFlag map[string]string

func (f headersFlag) String() string {
	keys := make([]string, 0, len(f))
	for k := range f {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		keys[i] = fmt.Sprintf("%s=%s", k, f[k])
	}
	return strings.Join(keys, ",")
}

func (f headersFlag) Set(s string) error {
	k, v, ok := strings.Cut(s, "=")
	if !ok {
		return fmt.Errorf("expected k=v, got %q", s)
	}
	f[k] = v
	return nil
}

func (f headersFlag) Type() string {
	return "k=v"
}

func NewCmdRun() *cobra.Command {
	options := &RunOptions{
		Headers: make(map[string]string),
	}
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Run apmsoak",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := getLogger(options.Loglevel)

			config, err := options.toRunnerConfig()
			if err != nil {
				logger.Fatal("Fail to parse flags", zap.Error(err))
			}

			logger.Debug("parsed configs", zap.Object("config", config))

			runner, err := soaktest.NewRunner(config, logger)
			if err != nil {
				logger.Fatal("Fail to initialize runner", zap.Error(err))
			}

			if err := runner.Run(cmd.Context()); err != nil {
				if !errors.Is(err, context.Canceled) {
					logger.Error("runner exited with error", zap.Error(err))
					return err
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
	cmd.Flags().Var(headersFlag(options.Headers), "header", "Extra headers to send. <project_id> will be replaced in header values.")
	cmd.Flags().BoolVar(&options.BypassProxy, "bypass-proxy", false, "Detach from proxy dependency and provide projectID via header. Useful when testing locally")
	cmd.Flags().StringVar(&options.Loglevel, "log-level", "info", "Specify the log level to use when running this command. Supported values: debug, info, warn, error")
	cmd.Flags().BoolVar(&options.IgnoreErrors, "ignore-errors", false, "Ignore HTTP errors while sending events")
	cmd.Flags().BoolVar(&options.RunForever, "run-forever", false, "Continue running the soak test until a signal is received to stop it")
	cmd.Flags().DurationVar(&options.RunDuration, "duration", 0, "duration of the run")
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
