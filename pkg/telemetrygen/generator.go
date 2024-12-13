// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package telemetrygen

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/binary"
	"fmt"
	"math/rand"

	"go.uber.org/zap"

	"github.com/elastic/apm-perf/internal/loadgen"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"
)

type Generator struct {
	Config Config
	Logger *zap.Logger
}

func New(cfg Config) (*Generator, error) {
	if err := cfg.Validate(); err != nil {
		return &Generator{}, fmt.Errorf("cannot create generator, configuration is invalid: %w", err)
	}

	return &Generator{Config: cfg, Logger: zap.NewNop()}, nil
}

func (g *Generator) RunBlocking(ctx context.Context) error {
	limiter := loadgen.GetNewLimiter(g.Config.EventRate.Burst, g.Config.EventRate.Interval)
	errg, gCtx := errgroup.WithContext(ctx)

	var rngseed int64
	err := binary.Read(cryptorand.Reader, binary.LittleEndian, &rngseed)
	if err != nil {
		return fmt.Errorf("failed to generate seed for math/rand: %w", err)
	}

	for i := 0; i < g.Config.AgentReplicas; i++ {
		for _, expr := range []string{`apm-go*.ndjson`, `apm-nodejs*.ndjson`, `apm-python*.ndjson`, `apm-ruby*.ndjson`} {
			expr := expr
			errg.Go(func() error {
				rng := rand.New(rand.NewSource(rngseed))
				return runAgent(gCtx, g.Logger, expr, limiter, rng, g.Config)
			})
		}
	}

	return errg.Wait()
}

func runAgent(ctx context.Context, l *zap.Logger, expr string, limiter *rate.Limiter, rng *rand.Rand, cfg Config) error {
	handler, err := loadgen.NewEventHandler(loadgen.EventHandlerParams{
		Logger:                    l,
		URL:                       cfg.ServerURL.String(),
		Path:                      expr,
		APIKey:                    cfg.APIKey,
		Limiter:                   limiter,
		Rand:                      rng,
		RewriteIDs:                cfg.RewriteIDs,
		RewriteServiceNames:       cfg.RewriteServiceNames,
		RewriteServiceNodeNames:   cfg.RewriteServiceNodeNames,
		RewriteServiceTargetNames: cfg.RewriteServiceTargetNames,
		RewriteSpanNames:          cfg.RewriteSpanNames,
		RewriteTransactionNames:   cfg.RewriteTransactionNames,
		RewriteTransactionTypes:   cfg.RewriteTransactionTypes,
		RewriteTimestamps:         cfg.RewriteTimestamps,
		Headers:                   cfg.Headers,
		Protocol:                  "apm/http",
	})
	if err != nil {
		return err
	}

	if _, err := handler.SendBatches(ctx); err != nil {
		return err
	}

	return nil
}
