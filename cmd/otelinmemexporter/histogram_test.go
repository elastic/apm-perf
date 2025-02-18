package otelinmemexporter

import (
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func Test_mergeHistogramDataPoint(t *testing.T) {
	ts1 := time.Unix(111111111, 0)
	ts2 := time.Unix(222222222, 0)
	ts3 := time.Unix(333333333, 0)

	type args struct {
		from pmetric.HistogramDataPoint
		to   pmetric.HistogramDataPoint
	}
	tests := []struct {
		name     string
		args     args
		expected pmetric.HistogramDataPoint
	}{
		{
			name: "merge to empty",
			args: args{
				from: newTestHistogramDataPoint(ts1, ts2, [5]uint64{0, 1, 2, 3, 4}, 200, 1, 100),
				to:   pmetric.NewHistogramDataPoint(),
			},
			expected: newTestHistogramDataPoint(ts1, ts2, [5]uint64{0, 1, 2, 3, 4}, 200, 1, 100),
		},
		{
			name: "merge from empty",
			args: args{
				from: pmetric.NewHistogramDataPoint(),
				to:   newTestHistogramDataPoint(ts1, ts2, [5]uint64{0, 1, 2, 3, 4}, 200, 1, 100),
			},
			expected: newTestHistogramDataPoint(ts1, ts2, [5]uint64{0, 1, 2, 3, 4}, 200, 1, 100),
		},
		{
			name: "merge",
			args: args{
				from: newTestHistogramDataPoint(ts2, ts3, [5]uint64{0, 1, 2, 3, 4}, 200, 1, 80),
				to:   newTestHistogramDataPoint(ts1, ts2, [5]uint64{0, 1, 0, 2, 0}, 35, 3, 18),
			},
			expected: newTestHistogramDataPoint(ts1, ts3, [5]uint64{0, 2, 2, 5, 4}, 235, 1, 80),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mergeHistogramDataPoint(tt.args.from, tt.args.to)
			if err := pmetrictest.CompareHistogramDataPoints(tt.expected, tt.args.to); err != nil {
				t.Errorf("mergeHistogramDataPoint() compare error = %v", err)
			}
		})
	}
}

func newTestHistogramDataPoint(
	startTs, ts time.Time,
	bucketCounts [5]uint64,
	sum, min, max float64,
) pmetric.HistogramDataPoint {
	countBuckets := func() uint64 {
		var totalCount uint64
		for _, bc := range bucketCounts {
			totalCount += bc
		}
		return totalCount
	}

	dp := pmetric.NewHistogramDataPoint()
	dp.SetCount(countBuckets())
	dp.SetSum(sum)
	dp.SetMin(min)
	dp.SetMax(max)
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(startTs))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(ts))
	dp.ExplicitBounds().FromRaw([]float64{0, 5, 10, 25})
	dp.BucketCounts().FromRaw(bucketCounts[:])

	return dp
}
