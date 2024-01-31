package eventhandler

import (
	"bytes"
	"context"
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
	s := `{"metadata":{"system":{"architecture":"arm64","hostname":"3ff988a6070f","platform":"linux"},"process":{"pid":1,"argv":["/opbeans-go","-log-json","-log-level=debug","-listen=:3000","-frontend=/opbeans-frontend","-db=postgres:","-cache=redis://redis:6379"],"ppid":0,"title":"opbeans-go"},"service":{"agent":{"name":"go","version":"2.0.0"},"environment":"production","language":{"name":"go","version":"go1.17.7"},"name":"opbeans-go","runtime":{"name":"gc","version":"go1.17.7"},"version":"None"}}}`
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
