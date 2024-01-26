// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventhandler

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestOTLPEventCollector_Process(t *testing.T) {
	o := OTLPEventCollector{}

	minTimestamp := "1581452773000000789"
	s, err := strconv.ParseInt(minTimestamp, 10, 64)
	assert.NoError(t, err)
	et := time.Unix(0, s)

	line := `{"resourceLogs":[{"resource":{"attributes":[{"key":"resource-attr","value":{"stringValue":"resource-attr-val-1"}}]},"scopeLogs":[{"scope":{},"logRecords":[{"timeUnixNano":"1581452773000009875","severityNumber":9,"severityText":"Info","name":"logA","body":{"stringValue":"This is a log message"},"attributes":[{"key":"app","value":{"stringValue":"server"}},{"key":"instance_num","value":{"intValue":"1"}}],"droppedAttributesCount":1,"traceId":"08040201000000000000000000000000","spanId":"0102040800000000"},{"timeUnixNano":"1581452773000000789","severityNumber":9,"severityText":"Info","name":"logB","body":{"stringValue":"something happened"},"attributes":[{"key":"customer","value":{"stringValue":"acme"}},{"key":"env","value":{"stringValue":"dev"}}],"droppedAttributesCount":1,"traceId":"","spanId":""}]}]}]}`

	event := o.Process([]byte(line))

	assert.Equal(t, "resourceLogs", event.objectType)
	assert.Equal(t, et, event.timestamp)
}
