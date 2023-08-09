// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventhandler

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

// Transport sends the contents of a reader to a remote APM Server.
type Transport struct {
	client        *http.Client
	intakeHeaders http.Header
	intakeV2URL   string
}

// NewTransport initializes a new ReplayTransport.
func NewTransport(c *http.Client, srvURL, token, apiKey string, headers map[string]string) *Transport {
	intakeHeaders := make(http.Header)
	intakeHeaders.Set("Content-Encoding", "deflate")
	intakeHeaders.Set("Content-Type", "application/x-ndjson")
	intakeHeaders.Set("Transfer-Encoding", "chunked")
	intakeHeaders.Set("Authorization", getAuthHeader(token, apiKey))
	for name, header := range headers {
		intakeHeaders.Set(name, header)
	}
	return &Transport{
		client:        c,
		intakeV2URL:   srvURL + `/intake/v2/events`,
		intakeHeaders: intakeHeaders,
	}
}

// SendV2Events sends the reader contents to `/intake/v2/events` as a batch.
func (t *Transport) SendV2Events(ctx context.Context, r io.Reader) error {
	req, err := http.NewRequestWithContext(ctx, "POST", t.intakeV2URL, r)
	if err != nil {
		return err
	}
	// Since the ContentLength will be automatically set on `bytes.Reader`,
	// set it to `-1` just like the agents would.
	req.ContentLength = -1
	req.Header = t.intakeHeaders
	return t.sendEvents(req, r)
}

func (t *Transport) sendEvents(req *http.Request, r io.Reader) error {
	res, err := t.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	switch res.StatusCode {
	case http.StatusOK, http.StatusAccepted:
		return nil
	}

	msg := fmt.Sprintf("unexpected apm server response %d", res.StatusCode)
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return errors.New(msg)
	}
	return fmt.Errorf(msg+": %s", string(b))
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
