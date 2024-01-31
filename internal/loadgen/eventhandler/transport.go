// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventhandler

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"

	"go.uber.org/zap"
)

// Transport sends the contents of a reader to a remote APM Server.
type Transport struct {
	logger        *zap.Logger
	client        *http.Client
	intakeHeaders http.Header
	intakeV2URL   string
}

// NewTransport initializes a new ReplayTransport.
func NewTransport(logger *zap.Logger, c *http.Client, srvURL, token, apiKey string, headers map[string]string) *Transport {
	intakeHeaders := make(http.Header)
	intakeHeaders.Set("Content-Encoding", "deflate")
	intakeHeaders.Set("Content-Type", "application/x-ndjson")
	intakeHeaders.Set("Transfer-Encoding", "chunked")
	intakeHeaders.Set("Authorization", getAuthHeader(token, apiKey))
	for name, header := range headers {
		intakeHeaders.Set(name, header)
	}
	return &Transport{
		logger:        logger.Named("transport"),
		client:        c,
		intakeV2URL:   srvURL + `/intake/v2/events`,
		intakeHeaders: intakeHeaders,
	}
}

// SendV2Events sends the reader contents to `/intake/v2/events` as a batch.
func (t *Transport) SendV2Events(ctx context.Context, r io.Reader, ignoreErrs bool) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.intakeV2URL, r)
	if err != nil {
		return err
	}
	// Since the ContentLength will be automatically set on `bytes.Reader`,
	// set it to `-1` just like the agents would.
	req.ContentLength = -1
	req.Header = t.intakeHeaders
	return t.sendEvents(req, r, ignoreErrs)
}

func (t *Transport) sendEvents(req *http.Request, r io.Reader, ignoreErrs bool) error {
	t.logger.Debug("sending request")
	res, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot complete request: %w", err)
	}
	defer res.Body.Close()
	defer logResponseOutcome(t.logger, res)

	if !ignoreErrs {
		switch res.StatusCode / 100 {
		case 4:
			return fmt.Errorf("unexpected client error: %d", res.StatusCode)
		case 5:
			return fmt.Errorf("unexpected server error: %d", res.StatusCode)
		}
	}

	return nil
}

func logResponseOutcome(logger *zap.Logger, res *http.Response) {
	var body bytes.Buffer
	if _, err := body.ReadFrom(res.Body); err != nil {
		logger.Error("cannot read body", zap.Error(err))
	}
	if res.StatusCode >= http.StatusBadRequest {
		logger.Error("request failed",
			zap.Int("status_code", res.StatusCode), zap.String("response", body.String()))
	} else {
		logger.Debug("request completed",
			zap.Int("status_code", res.StatusCode), zap.String("response", body.String()))
	}
}

func getAuthHeader(token string, apiKey string) string {
	var auth string
	if token != "" {
		auth = "Bearer " + token
	}
	if apiKey != "" {
		auth = "ApiKey " + apiKey
	}
	return auth
}
