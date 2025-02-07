// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventhandler

import (
	"context"
	"io"
)

// Transport sends the contents of a reader to a remote APM Server.
type Transport interface {
	SendEvents(ctx context.Context, r io.Reader, ignoreErrs bool) error
}
