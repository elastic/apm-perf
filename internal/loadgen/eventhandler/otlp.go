// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventhandler

import (
	"context"
	"fmt"
	"io"
	"net/http"
)

type OTLPTransport struct {
	client  *http.Client
	headers http.Header
	url     string
}

func (o *OTLPTransport) SendEvents(ctx context.Context, r io.Reader, ignoreErrs bool) error {
	req, err := http.NewRequestWithContext(ctx, "POST", o.url, r)
	if err != nil {
		return err
	}
	req.Header = o.headers

	res, err := o.client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

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

func newOTLPTransport(c *http.Client, srvURL, u, apiKey string, headers map[string]string) *OTLPTransport {
	h := make(http.Header)
	h.Set("Content-Encoding", "deflate")
	h.Set("Content-Type", "application/x-json")
	h.Set("Authorization", "ApiKey "+apiKey)

	for name, header := range headers {
		h.Set(name, header)
	}

	return &OTLPTransport{
		client:  c,
		url:     srvURL + u,
		headers: h,
	}
}

func NewOTLPLogsTransport() {
	panic("not implemented")
}

func NewOTLPMetricsTransport(c *http.Client, srvURL, apiKey string, headers map[string]string) Transport {
	return newOTLPTransport(c, srvURL, "/v1/metrics", apiKey, headers)
}

func NewOTLPTracesTransport(c *http.Client, srvURL, apiKey string, headers map[string]string) Transport {
	return newOTLPTransport(c, srvURL, "/v1/traces", apiKey, headers)
}
