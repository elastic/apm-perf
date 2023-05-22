// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

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
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"github.com/elastic/apm-perf/loadgen"
)

type Runner struct {
	logger       *zap.Logger
	configs      []ScenarioConfig
	scenarioName string
}

// NewRunner returns Runner to executes soak test scenario
func NewRunner(path string, scenario string, logger *zap.Logger) (*Runner, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var y Scenarios
	if err := yaml.Unmarshal(f, &y); err != nil {
		return nil, err
	}
	scenarioConfigs := y.Scenarios[scenario]
	if scenarioConfigs == nil {
		return nil, errors.New("unknown scenario " + scenario)
	}

	return &Runner{
		configs:      scenarioConfigs,
		logger:       logger.Named("soaktest"),
		scenarioName: scenario,
	}, nil
}

func (r Runner) Run(ctx context.Context) error {
	g, gCtx := errgroup.WithContext(ctx)
	// Create a Rand with the same seed for each agent, so we randomise their IDs consistently.
	var rngseed int64
	err := binary.Read(cryptorand.Reader, binary.LittleEndian, &rngseed)
	if err != nil {
		return fmt.Errorf("failed to generate seed for math/rand: %w", err)
	}

	r.logger.Info("running scenario `" + r.scenarioName + "`...")
	for _, config := range r.configs {
		config := config
		// when not specified, default to 1
		if config.AgentsReplicas <= 0 {
			config.AgentsReplicas = 1
		}
		for i := 0; i < config.AgentsReplicas; i++ {
			r.logger.Debug(fmt.Sprintf("agent: %s, replica %d", config.AgentName, i))
			g.Go(func() error {
				rng := rand.New(rand.NewSource(rngseed))
				return runAgent(gCtx, config, rng)
			})
		}
	}

	return g.Wait()
}

func runAgent(ctx context.Context, config ScenarioConfig, rng *rand.Rand) error {
	params, err := getHandlerParams(config)
	if err != nil {
		return err
	}
	params.Rand = rng
	handler, err := loadgen.NewEventHandler(params)
	if err != nil {
		return err
	}

	return handler.SendBatchesInLoop(ctx)
}

func getHandlerParams(config ScenarioConfig) (loadgen.EventHandlerParams, error) {
	// if AgentName is not specified, using all the agents,
	// but shares the allowed events numbers sent for given duration(e.g. 4 agents send 10000/s in total)
	path := config.AgentName + "*.ndjson"
	var params loadgen.EventHandlerParams
	if config.Headers == nil {
		config.Headers = make(map[string]string)
	}
	config.Headers["X-Elastic-Project-Id"] = config.ProjectID
	if config.ServerURL == "" {
		config.ServerURL = SoakConfig.ServerURL
	}
	// if <project_id> is specified in the url, replace it
	serverURL, err := url.Parse(strings.Replace(config.ServerURL, "<project_id>", config.ProjectID, 1))
	if err != nil {
		return params, err
	}
	if config.APIKey == "" {
		config.APIKey = SoakConfig.ApiKeys[config.ProjectID]
	}

	burst, interval, err := getEventRate(config.EventRate)
	if err != nil {
		return params, err
	}

	params = loadgen.EventHandlerParams{
		Path:                      path,
		URL:                       serverURL.String(),
		APIKey:                    config.APIKey,
		Token:                     SoakConfig.SecretToken,
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
