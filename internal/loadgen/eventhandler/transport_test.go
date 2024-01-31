package eventhandler

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/klauspost/compress/zlib"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestTransport_SendEventV2Logs(t *testing.T) {
	s := `{"metadata":{"service":{"name":"foo","version":"bar"}}}`
	var w bytes.Buffer
	_, err := zlib.NewWriter(&w).Write([]byte(s))
	require.NoError(t, err)

	ms := &mockServer{got: &bytes.Buffer{}}
	srv := httptest.NewServer(ms)
	defer srv.Close()

	t.Run("always log status code in debug", func(t *testing.T) {
		core, logs := observer.New(zap.DebugLevel)
		err = NewTransport(zap.New(core), srv.Client(), srv.URL, "", "", nil).
			SendV2Events(context.Background(), &w, false)

		assert.NoError(t, err)
		assert.True(t, logs.FilterFieldKey("status_code").Len() == 1)
		assert.True(t, logs.FilterMessageSnippet("request completed").Len() == 1)
	})

	t.Run("log error if status code is 400 without ignoreErrors", func(t *testing.T) {
		testBadRequestWithIgnoreErrors(t, srv, false, zap.ErrorLevel, assert.Error)
	})

	t.Run("log error if status code is 400 with ignoreErrors", func(t *testing.T) {
		testBadRequestWithIgnoreErrors(t, srv, true, zap.ErrorLevel, assert.NoError)
	})
}

func TestTransport_logResponseOutcome(t *testing.T) {
	core, logs := observer.New(zap.DebugLevel)

	t.Run("non-nil response with empty body", func(t *testing.T) {
		logResponseOutcome(
			zap.New(core),
			&http.Response{
				Body:       http.NoBody,
				StatusCode: http.StatusCreated,
			})
		assert.NotEmpty(t, logs.TakeAll())
	})

	t.Run("non-nil response with body", func(t *testing.T) {
		logResponseOutcome(
			zap.New(core),
			&http.Response{
				Body:       io.NopCloser(bytes.NewBufferString("anything")),
				StatusCode: http.StatusCreated,
			})
		assert.NotEmpty(t, logs.TakeAll())
	})

	t.Run("nil response early return", func(t *testing.T) {
		logResponseOutcome(
			zap.New(core),
			nil)
		assert.Empty(t, logs.TakeAll())
	})
}

func testBadRequestWithIgnoreErrors(
	t *testing.T,
	srv *httptest.Server,
	ignoreErrors bool,
	logLevel zapcore.Level,
	errorCheck assert.ErrorAssertionFunc) {
	t.Helper()

	core, logs := observer.New(logLevel)
	err := NewTransport(zap.New(core), srv.Client(), srv.URL, "", "", nil).
		// Always make the call fail due to HTTP 400, because the body is not compressed
		SendV2Events(context.Background(), bytes.NewBufferString("foo"), ignoreErrors)

	errorCheck(t, err)
	var (
		loggedStatusCode any
		ok               bool
	)
	assert.NotPanics(t, func() {
		loggedStatusCode, ok = logs.FilterFieldKey("status_code").TakeAll()[0].ContextMap()["status_code"]
	})
	assert.True(t, ok)
	assert.Equal(t, loggedStatusCode.(int64), int64(400))
	assert.True(t, logs.FilterMessageSnippet("request failed").Len() == 1)
}
