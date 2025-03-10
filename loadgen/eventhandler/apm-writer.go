// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package eventhandler

import (
	"fmt"
	"time"

	"github.com/tidwall/gjson"
)

// writeAPMEvents writes to buffers in eventWriter JSON formatted events that can be replayed.
// Implements EventWriter interface.
func writeAPMEvents(config Config, minTimestamp time.Time, w *eventWriter, b batch, baseTimestamp time.Time, randomBits uint64) error {
	rewriteAny := config.RewriteTimestamps ||
		config.RewriteIDs ||
		config.RewriteServiceNames ||
		config.RewriteServiceNodeNames ||
		config.RewriteServiceTargetNames ||
		config.RewriteSpanNames ||
		config.RewriteTransactionNames ||
		config.RewriteTransactionTypes

	var err error
	metadata := b.metadata
	if config.RewriteServiceNames {
		metadata, err = randomizeASCIIField(metadata, "metadata.service.name", randomBits, &w.idBuf)
		if err != nil {
			return fmt.Errorf("failed to rewrite `service.name`: %w", err)
		}
	}
	if config.RewriteServiceNodeNames {
		// The intakev2 field name is `service.node.configured_name`,
		// this is translated to `service.node.name` in the ES documents.
		metadata, err = randomizeASCIIField(metadata, "metadata.service.node.configured_name", randomBits, &w.idBuf)
		if err != nil {
			return fmt.Errorf("failed to rewrite `service.node.name`: %w", err)
		}
	}
	w.Write(metadata)
	w.Write(newlineBytes)

	for _, event := range b.events {
		if !rewriteAny {
			w.Write(event.payload)
			w.Write(newlineBytes)
			continue
		}
		w.rewriteBuf.RawByte('{')
		w.rewriteBuf.String(event.objectType)
		w.rewriteBuf.RawString(":")
		rewriteJSONObject(w, gjson.GetBytes(event.payload, event.objectType), func(key, value gjson.Result) bool {
			switch key.Str {
			case "timestamp":
				if config.RewriteTimestamps && !event.timestamp.IsZero() {
					// We always encode rewritten timestamps as strings,
					// so we don't lose any precision when offsetting by
					// either the base timestamp, or the minimum timestamp
					// across all the batches; string-formatted timestamps
					// may have nanosecond precision.
					// Note that this prevents using RewriteTimestamps when
					// targeting a 7.x APM Server, as it will fail deconding
					// incoming events.
					offset := event.timestamp.Sub(minTimestamp)
					timestamp := baseTimestamp.Add(offset)
					w.rewriteBuf.RawByte('"')
					w.rewriteBuf.Time(timestamp, time.RFC3339Nano)
					w.rewriteBuf.RawByte('"')
				} else {
					w.rewriteBuf.RawString(value.Raw)
				}
			case "id", "parent_id", "trace_id", "transaction_id":
				if config.RewriteIDs && randomizeTraceID(&w.idBuf, value.Str, randomBits) {
					w.rewriteBuf.RawByte('"')
					w.rewriteBuf.RawBytes(w.idBuf.Bytes())
					w.rewriteBuf.RawByte('"')
					w.idBuf.Reset()
				} else {
					w.rewriteBuf.RawString(value.Raw)
				}
			case "name":
				randomizeASCII(&w.idBuf, value.Str, randomBits)
				switch {
				case config.RewriteSpanNames && event.objectType == "span":
					w.rewriteBuf.String(w.idBuf.String())
				case config.RewriteTransactionNames && event.objectType == "transaction":
					w.rewriteBuf.String(w.idBuf.String())
				default:
					w.rewriteBuf.RawString(value.Raw)
				}
				w.idBuf.Reset()
			case "type":
				switch {
				case config.RewriteTransactionTypes && event.objectType == "transaction":
					randomizeASCII(&w.idBuf, value.Str, randomBits)
					w.rewriteBuf.String(w.idBuf.String())
					w.idBuf.Reset()
				default:
					w.rewriteBuf.RawString(value.Raw)
				}
			case "context":
				if !config.RewriteServiceTargetNames {
					w.rewriteBuf.RawString(value.Raw)
					break
				}
				rewriteJSONObject(w, value, func(key, value gjson.Result) bool {
					if key.Str != "service" {
						w.rewriteBuf.RawString(value.Raw)
						return true
					}
					rewriteJSONObject(w, value, func(key, value gjson.Result) bool {
						if key.Str != "target" {
							w.rewriteBuf.RawString(value.Raw)
							return true
						}
						rewriteJSONObject(w, value, func(key, value gjson.Result) bool {
							if key.Str != "name" {
								w.rewriteBuf.RawString(value.Raw)
								return true
							}
							randomizeASCII(&w.idBuf, value.Str, randomBits)
							w.rewriteBuf.String(w.idBuf.String())
							w.idBuf.Reset()
							return true
						})
						return true
					})
					return true
				})
			default:
				w.rewriteBuf.RawString(value.Raw)
			}
			return true
		})
		w.rewriteBuf.RawString("}")
		w.Write(w.rewriteBuf.Bytes())
		w.Write(newlineBytes)
		w.rewriteBuf.Reset()
	}
	return nil
}
