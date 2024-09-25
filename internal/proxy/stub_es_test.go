// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package proxy

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"
	"github.com/stretchr/testify/assert"
)

func TestHandlerStubES(t *testing.T) {
	tests := map[string]struct {
		method       string
		path         string
		payload      io.Reader
		headers      http.Header
		options      []StubESOption
		expectedCode int
		expectedBody string
	}{
		"path /": {
			method:       "GET",
			path:         "/",
			expectedCode: http.StatusOK,
			expectedBody: defaultInfo,
		},
		"path /_license": {
			method:       "GET",
			path:         "/_license",
			expectedCode: http.StatusOK,
			expectedBody: defaultLicense,
		},
		"path /_bulk fail auth": {
			method:       "POST",
			path:         "/_bulk",
			options:      []StubESOption{StubESWithAuth("user", "pass")},
			expectedCode: http.StatusUnauthorized,
		},
		"path /_bulk empty payload": {
			method: "POST",
			path:   "/_bulk",
			headers: http.Header{
				"Authorization": []string{
					"Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass")),
				},
			},
			options:      []StubESOption{StubESWithAuth("user", "pass")},
			expectedBody: `{"items":[]}`,
			expectedCode: http.StatusOK,
		},
		"path /_bulk invalid payload": {
			method: "POST",
			path:   "/_bulk",
			payload: strings.NewReader(`
....
....
....
			`),
			headers: http.Header{
				"Authorization": []string{
					"Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass")),
				},
			},
			options:      []StubESOption{StubESWithAuth("user", "pass")},
			expectedBody: "expected source\n",
			expectedCode: http.StatusBadRequest,
		},
		"path /_bulk valid payload": {
			method: "POST",
			path:   "/_bulk",
			payload: strings.NewReader(`
{ "create" : { "_index" : "test", "_id" : "1" } }
{ "create" : { "_index" : "test", "_id" : "2" } }
{ "create" : { "_index" : "test", "_id" : "3" } }

			`),
			headers: http.Header{
				"Authorization": []string{
					"Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass")),
				},
			},
			options:      []StubESOption{StubESWithAuth("user", "pass")},
			expectedBody: `{"items":[{"create":{"status":201}},{"create":{"status":201}},{"create":{"status":201}}]}`,
			expectedCode: http.StatusOK,
		},
		"path /_bulk valid gzip payload": {
			method: "POST",
			path:   "/_bulk",
			payload: compress(strings.NewReader(`
{ "create" : { "_index" : "test", "_id" : "1" } }
{ "create" : { "_index" : "test", "_id" : "2" } }
{ "create" : { "_index" : "test", "_id" : "3" } }

			`), func(w io.Writer) io.WriteCloser {
				return gzip.NewWriter(w)
			}),
			headers: http.Header{
				"Authorization": []string{
					"Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass")),
				},
				"Content-Encoding": []string{"gzip"},
			},
			options:      []StubESOption{StubESWithAuth("user", "pass")},
			expectedBody: `{"items":[{"create":{"status":201}},{"create":{"status":201}},{"create":{"status":201}}]}`,
			expectedCode: http.StatusOK,
		},
		"path /_bulk valid zstd payload": {
			method: "POST",
			path:   "/_bulk",
			payload: compress(strings.NewReader(`
{ "create" : { "_index" : "test", "_id" : "1" } }
{ "create" : { "_index" : "test", "_id" : "2" } }
{ "create" : { "_index" : "test", "_id" : "3" } }

			`), func(w io.Writer) io.WriteCloser {
				wc, _ := zstd.NewWriter(w)
				return wc
			}),
			headers: http.Header{
				"Authorization": []string{
					"Basic " + base64.StdEncoding.EncodeToString([]byte("user:pass")),
				},
				"Content-Encoding": []string{"zstd"},
			},
			options:      []StubESOption{StubESWithAuth("user", "pass")},
			expectedBody: `{"items":[{"create":{"status":201}},{"create":{"status":201}},{"create":{"status":201}}]}`,
			expectedCode: http.StatusOK,
		},
	}
	for tname, tcase := range tests {
		t.Run(tname, func(t *testing.T) {
			req := httptest.NewRequest(tcase.method, tcase.path, tcase.payload)
			req.Header = tcase.headers
			rec := httptest.NewRecorder()
			h := NewHandlerStubES(tcase.options...)
			h.ServeHTTP(rec, req)
			assert.Equal(t, "Elasticsearch", rec.Header().Get("X-Elastic-Product"))
			assert.Equal(t, tcase.expectedCode, rec.Code)
			assert.Equal(t, tcase.expectedBody, rec.Body.String())
		})
	}
}

func compress(from io.Reader, f func(out io.Writer) io.WriteCloser) io.Reader {
	var b bytes.Buffer
	enc := f(&b)
	defer enc.Close()
	_, _ = io.Copy(enc, from)
	return &b
}
