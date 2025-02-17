package otelinmemexporter

import (
	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatautil"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type histogramDPHash struct {
	attrs          [16]byte
	explicitBounds [16]byte
	startTsNano    int64
	tsNano         int64
	hasMin         bool
	hasMax         bool
	flags          uint32
}

func hashHistogramDataPoint(dp pmetric.HistogramDataPoint) histogramDPHash {
	float64ToAnySlice := func(f []float64) []any {
		res := make([]any, 0, len(f))
		for _, v := range f {
			res = append(res, v)
		}
		return res
	}

	ebSlice := pcommon.NewValueSlice()
	_ = ebSlice.FromRaw(float64ToAnySlice(dp.ExplicitBounds().AsRaw()))
	return histogramDPHash{
		attrs:          pdatautil.MapHash(dp.Attributes()),
		explicitBounds: pdatautil.ValueHash(ebSlice),
		startTsNano:    dp.StartTimestamp().AsTime().UnixNano(),
		tsNano:         dp.Timestamp().AsTime().UnixNano(),
		hasMin:         dp.HasMin(),
		hasMax:         dp.HasMax(),
		flags:          uint32(dp.Flags()),
	}
}
