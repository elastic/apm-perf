// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"go.elastic.co/ecszap"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/elastic/apm-perf/internal/loadgen"
	"github.com/elastic/apm-perf/internal/version"
)

const envVarPrefix = "ELASTIC_APM_"

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	// Register root command in cobra
	var rootCmd = &cobra.Command{
		Use:              "apmtelemetrygen",
		TraverseChildren: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			var err error
			cmd.Flags().VisitAll(func(flag *pflag.Flag) {
				optionName := strings.ToUpper(flag.Name)
				optionName = strings.ReplaceAll(optionName, "-", "_")
				envVar := envVarPrefix + optionName
				if val, ok := os.LookupEnv(envVar); !flag.Changed && ok {
					if flagErr := flag.Value.Set(val); flagErr != nil {
						err = fmt.Errorf("invalid environment variable %s: %w", envVar, flagErr)
					}
				}
			})
			return err
		},
	}
	rootCmd.AddCommand(newRunCmd())
	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Show current version info",
		Run: func(cmd *cobra.Command, _ []string) {
			var buf bytes.Buffer
			fmt.Fprintf(&buf, "%s %s", version.CommitSha(), version.BuildTime())
			fmt.Fprintf(cmd.OutOrStdout(), "%s version %s (%s/%s) [%s]\n",
				rootCmd.Name(), version.Version, runtime.GOOS, runtime.GOARCH,
				buf.String(),
			)
		},
	})

	// Execute commands
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		if !errors.Is(err, context.Canceled) {
			fmt.Println(err)
		}
	}
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

func (f headersFlag) Type() string { return "k=v" }

func newRunCmd() *cobra.Command {
	options := &runOptions{Headers: make(map[string]string)}
	cmd := cobra.Command{
		Use:   "run",
		Short: "Runs the load generator for APM telemetry data",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := getLogger(options.Loglevel)
			config, err := options.toEventHandlerParams(logger)
			if err != nil {
				logger.Fatal("Failed to parse flags", zap.Error(err))
			}

			lg, err := loadgen.NewEventHandler(config)
			if err != nil {
				logger.Fatal("Failed to create event handler", zap.Error(err))
			}

			for i := 0; i < options.Iterations; i++ {
				if _, err := lg.SendBatches(cmd.Context()); err != nil {
					if !options.IgnoreErrors {
						return err
					}
					logger.Error("Failed to send batches", zap.Error(err))
				}
			}
			return nil
		},
	}
	cmd.Flags().Var(headersFlag(options.Headers), "header", "Extra headers to send.	Can be specified multiple times")
	cmd.Flags().StringVar(&options.ServerURL, "server-url", "", "Server URL (default http://127.0.0.1:8200)")
	cmd.Flags().StringVar(&options.SecretToken, "secret-token", "", "Secret token for APM Server. Managed intake service doesn't support secret token")
	cmd.Flags().StringVar(&options.APIKey, "api-key", "", "API key to use for authentication")
	cmd.Flags().StringVar(&options.Loglevel, "log-level", "debug", "Specify the log level to use when running this command. Supported values: debug, info, warn, error")
	cmd.Flags().StringVar(&options.Protocol, "protocol", "apm/http", "Specify the protocol to use when sending events. Supported values: apm/http, otlp/http")
	cmd.Flags().StringVar(&options.Datatype, "data-type", "any", "Specify the data type to use when sending events. Supported values: any, logs, metrics, traces")
	cmd.Flags().StringVar(&options.EventRate, "event-rate", "0/s", "Must be in the format <number of events>/<time>. <time> is parsed")
	cmd.Flags().IntVar(&options.Iterations, "iterations", 1, "The number of times to replay the canned data for")
	cmd.Flags().BoolVar(&options.IgnoreErrors, "ignore-errors", false, "Ignore HTTP errors while sending events")
	return &cmd
}

type runOptions struct {
	Headers      map[string]string
	ServerURL    string
	SecretToken  string
	APIKey       string
	Loglevel     string
	Protocol     string
	Datatype     string
	EventRate    string
	Iterations   int
	IgnoreErrors bool
}

func (opts *runOptions) toEventHandlerParams(logger *zap.Logger) (loadgen.EventHandlerParams, error) {
	burst, interval, err := loadgen.ParseEventRate(opts.EventRate)
	if err != nil {
		return loadgen.EventHandlerParams{}, err
	}

	return loadgen.EventHandlerParams{
		Logger:       logger,
		Path:         "apm*.ndjson",
		URL:          opts.ServerURL,
		Token:        opts.SecretToken,
		APIKey:       opts.APIKey,
		Headers:      opts.Headers,
		IgnoreErrors: opts.IgnoreErrors,
		Protocol:     opts.Protocol,
		Datatype:     opts.Datatype,
		Limiter:      loadgen.GetNewLimiter(burst, interval),
		Rand:         rand.New(rand.NewSource(time.Now().UnixNano())),

		RewriteIDs:        true,
		RewriteTimestamps: true,
	}, nil
}

func getLogger(logLevel string) *zap.Logger {
	level, err := zapcore.ParseLevel(logLevel)
	if err != nil {
		level = zap.InfoLevel
	}

	return zap.New(ecszap.NewCore(
		ecszap.NewDefaultEncoderConfig(), os.Stdout, level,
	), zap.AddCaller())
}
