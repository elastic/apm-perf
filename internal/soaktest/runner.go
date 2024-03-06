// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

// Package soaktest contains code for defining soak testing scenarios and providing
// an interface for running them.
package soaktest

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"github.com/elastic/apm-perf/internal/loadgen"
)

type RunnerConfig struct {
	Scenario      string
	ScenariosPath string
	ServerURL     string
	SecretToken   string
	APIKeys       map[string]string
	BypassProxy   bool
	IgnoreErrors  bool
	// RunForever when set to true, will keep the handler running
	// until a signal is received to stop it.
	RunForever bool
}

type Runner struct {
	config          *RunnerConfig
	logger          *zap.Logger
	scenarioConfigs []ScenarioConfig
	scenarioName    string
}

// NewRunner returns Runner to executes soak test scenario
func NewRunner(config *RunnerConfig, logger *zap.Logger) (*Runner, error) {
	f, err := os.ReadFile(config.ScenariosPath)
	if err != nil {
		return nil, err
	}

	var y Scenarios
	if err := yaml.Unmarshal(f, &y); err != nil {
		return nil, err
	}
	scenarioConfigs := y.Scenarios[config.Scenario]
	if scenarioConfigs == nil {
		return nil, errors.New("unknown scenario " + config.Scenario)
	}

	return &Runner{
		config:          config,
		scenarioConfigs: scenarioConfigs,
		logger:          logger.Named("soaktest"),
		scenarioName:    config.Scenario,
	}, nil
}

func (runner *Runner) Run(ctx context.Context) error {
	// Apply graceful shutdown driven by SIGINT or SIGTERM
	// in case the config is set.
	if runner.config.RunForever {
		var cancel context.CancelFunc
		ctx, cancel = signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
		defer cancel()
	}

	g, gCtx := errgroup.WithContext(ctx)
	// Create a Rand with the same seed for each agent, so we randomise their IDs consistently.
	var rngseed int64
	err := binary.Read(cryptorand.Reader, binary.LittleEndian, &rngseed)
	if err != nil {
		return fmt.Errorf("failed to generate seed for math/rand: %w", err)
	}

	runner.logger.Info("running scenario `" + runner.scenarioName + "`...")
	for _, config := range runner.scenarioConfigs {
		config := config
		// when not specified, default to 1
		if config.AgentsReplicas <= 0 {
			config.AgentsReplicas = 1
		}
		for i := 0; i < config.AgentsReplicas; i++ {
			runner.logger.Debug(fmt.Sprintf("agent: %s, replica %d", config.AgentName, i))
			g.Go(func() error {
				rng := rand.New(rand.NewSource(rngseed))
				return runAgent(gCtx, runner, config, rng)
			})
		}
	}

	return g.Wait()
}

func runAgent(ctx context.Context, runner *Runner, config ScenarioConfig, rng *rand.Rand) error {
	params, err := getHandlerParams(runner.config, config)
	if err != nil {
		return err
	}
	params.Rand = rng
	runner.logger.Debug("computed load generation parameters", zap.Object("params", params))

	params.Logger = runner.logger
	handler, err := loadgen.NewEventHandler(params)
	if err != nil {
		return err
	}

	runner.logger.Debug("created event handler")

	return handler.SendBatchesInLoop(ctx)
}

func getHandlerParams(runnerConfig *RunnerConfig, config ScenarioConfig) (loadgen.EventHandlerParams, error) {
	// if AgentName is not specified, using all the APM agents,
	// but shares the allowed events numbers sent for given duration(e.g. 4 agents send 10000/s in total)
	if config.AgentName == "" {
		config.AgentName = "apm-"
	}
	extension := ".ndjson"
	if strings.HasPrefix(config.AgentName, "otlp-") {
		extension = ".jsonl"
	}
	path := config.AgentName + "*" + extension

	var params loadgen.EventHandlerParams
	if config.Headers == nil {
		config.Headers = make(map[string]string)
	}
	config.Headers["X-Elastic-Project-Id"] = config.ProjectID
	if config.ServerURL == "" {
		config.ServerURL = runnerConfig.ServerURL
	}
	// if <project_id> is specified in the url, replace it
	serverURL, err := url.Parse(strings.Replace(config.ServerURL, "<project_id>", config.ProjectID, 1))
	if err != nil {
		return params, err
	}
	if config.APIKey == "" {
		config.APIKey = runnerConfig.APIKeys[config.ProjectID]
	}

	burst, interval, err := getEventRate(config.EventRate)
	if err != nil {
		return params, err
	}

	protocol := "apm/http"
	if strings.HasPrefix(config.AgentName, "otlp-") {
		protocol = "otlp/http"
	}

	datatype := "any"
	if protocol == "otlp/http" {
		if strings.HasPrefix(config.AgentName, "otlp-logs") {
			datatype = "logs"
		} else if strings.HasPrefix(config.AgentName, "otlp-metrics") {
			datatype = "metrics"
		} else if strings.HasPrefix(config.AgentName, "otlp-traces") {
			datatype = "traces"
		}
	}

	params = loadgen.EventHandlerParams{
		Path:                      path,
		URL:                       serverURL.String(),
		APIKey:                    config.APIKey,
		Token:                     runnerConfig.SecretToken,
		IgnoreErrors:              runnerConfig.IgnoreErrors,
		RunForever:                runnerConfig.RunForever,
		Limiter:                   loadgen.GetNewLimiter(burst, interval),
		RewriteIDs:                true,
		RewriteServiceNames:       config.RewriteServiceNames,
		RewriteServiceNodeNames:   config.RewriteServiceNodeNames,
		RewriteServiceTargetNames: config.RewriteServiceTargetNames,
		RewriteSpanNames:          config.RewriteSpanNames,
		RewriteTransactionNames:   config.RewriteTransactionNames,
		RewriteTransactionTypes:   config.RewriteTransactionTypes,
		RewriteTimestamps:         true,
		Headers:                   config.Headers,

		Protocol: protocol,
		Datatype: datatype,
	}

	return params, nil
}

func getEventRate(eventRate string) (int, time.Duration, error) {
	before, after, ok := strings.Cut(eventRate, "/")
	if !ok || before == "" || after == "" {
		return 0, 0, fmt.Errorf("invalid rate %q, expected format burst/duration", eventRate)
	}

	burst, err := strconv.Atoi(before)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid burst %s in event rate: %w", before, err)
	}

	if !(after[0] >= '0' && after[0] <= '9') {
		after = "1" + after
	}
	interval, err := time.ParseDuration(after)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid interval %q in event rate: %w", after, err)
	}
	if interval <= 0 {
		return 0, 0, fmt.Errorf("invalid interval %q, must be positive", after)
	}

	return burst, interval, nil
}
