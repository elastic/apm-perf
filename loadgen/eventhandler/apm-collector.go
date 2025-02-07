// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventhandler

import (
	"bytes"
	"fmt"
	"time"

	"github.com/tidwall/gjson"
)

var (
	metaHeader    = []byte(`{"metadata":`)
	rumMetaHeader = []byte(`{"m":`)
)

// APMEventCollector extracts relevant metadata and event from
// single line scans.
type APMEventCollector struct{}

// Filter skips processing RUM related events.
func (a *APMEventCollector) Filter(line []byte) error {
	if bytes.HasPrefix(line, rumMetaHeader) {
		return fmt.Errorf("rum data support not implemented")
	}

	return nil
}

// IsMeta identifies metadata lines from APM protocol.
func (a *APMEventCollector) IsMeta(line []byte) bool {
	return bytes.HasPrefix(line, metaHeader)
}

// Process processes single lines extracting APM events.
// It uniforms events timestamp.
func (a *APMEventCollector) Process(linecopy []byte) event {
	event := event{payload: linecopy}
	result := gjson.ParseBytes(linecopy)
	result.ForEach(func(key, value gjson.Result) bool {
		event.objectType = key.Str // lines look like {"span":{...}}
		timestampResult := value.Get("timestamp")
		if timestampResult.Exists() {
			switch timestampResult.Type {
			case gjson.Number:
				us := timestampResult.Int()
				if us >= 0 {
					s := us / 1000000
					ns := (us - (s * 1000000)) * 1000
					event.timestamp = time.Unix(s, ns)
				}
			case gjson.String:
				tstr := timestampResult.Str
				for _, f := range supportedTSFormats {
					if t, err := time.Parse(f, tstr); err == nil {
						event.timestamp = t
						break
					}
				}
			}
		}
		return false
	})

	return event
}
