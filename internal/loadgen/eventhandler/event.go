package eventhandler

import "time"

type EventCollector interface {
	Filter([]byte) error
	IsMeta([]byte) bool
	Process([]byte) event
}

type EventWriter func(config Config, minTimestamp time.Time, w *pooledWriter, b batch, baseTimestamp time.Time, randomBits uint64) error
