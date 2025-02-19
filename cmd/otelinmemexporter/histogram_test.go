package otelinmemexporter

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/open-telemetry/opentelemetry-collector-contrib/pkg/pdatatest/pmetrictest"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

func Test_explicitBucketsQuantile_SpecialCases(t *testing.T) {
	simpleBuckets := []explicitBucket{
		{UpperBound: 10, Count: 1},
		{UpperBound: 20, Count: 2},
		{UpperBound: 30, Count: 4},
		{UpperBound: math.Inf(+1), Count: 3},
	}

	type args struct {
		p       float64
		buckets []explicitBucket
	}
	tests := []struct {
		name string
		args args
		want float64
	}{
		{
			name: "quantile NaN",
			args: args{p: math.NaN(), buckets: simpleBuckets},
			want: math.NaN(),
		},
		{
			name: "quantile less than 0",
			args: args{p: -10, buckets: simpleBuckets},
			want: math.Inf(-1),
		},
		{
			name: "quantile greater than 1",
			args: args{p: 4, buckets: simpleBuckets},
			want: math.Inf(+1),
		},
		{
			name: "1 bucket only",
			args: args{p: 0.5, buckets: []explicitBucket{{Count: 1, UpperBound: math.Inf(+1)}}},
			want: math.NaN(),
		},
		{
			name: "highest bucket not +Inf",
			args: args{
				p: 0.2,
				buckets: []explicitBucket{
					{Count: 1, UpperBound: 1},
					{Count: 2, UpperBound: 2},
				},
			},
			want: math.NaN(),
		},
		{
			name: "zero observations",
			args: args{
				p: 0.2,
				buckets: []explicitBucket{
					{Count: 0, UpperBound: 5},
					{Count: 0, UpperBound: 10},
					{Count: 0, UpperBound: math.Inf(+1)},
				},
			},
			want: math.NaN(),
		},
		{
			name: "lowest bucket",
			args: args{p: 0.1, buckets: simpleBuckets},
			want: 10.0,
		},
		{
			name: "highest bucket",
			args: args{p: 0.9, buckets: simpleBuckets},
			want: 30.0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertInDeltaWithInfAndNaN(
				t,
				tt.want,
				explicitBucketsQuantile(tt.args.p, tt.args.buckets),
				0,
				"explicitBucketsQuantile()",
			)
		})
	}
}

func Test_explicitBucketsQuantile(t *testing.T) {

	tests := []struct {
		name       string
		buckets    []explicitBucket
		wantValues map[float64]float64
	}{
		{
			name: "promql example",
			buckets: []explicitBucket{
				{UpperBound: 0, Count: 0},
				{UpperBound: 10, Count: 10},
				{UpperBound: 15, Count: 5},
				{UpperBound: 20, Count: 0},
				{UpperBound: 30, Count: 0},
				{UpperBound: math.Inf(1), Count: 0},
			},
			wantValues: map[float64]float64{
				0.5:  7.5,
				0.9:  13.5,
				0.99: 14.85,
				1.0:  15.0,
			},
		},
		{
			name: "real example",
			buckets: []explicitBucket{
				{UpperBound: 0, Count: 0},
				{UpperBound: 5, Count: 1053},
				{UpperBound: 10, Count: 19},
				{UpperBound: 25, Count: 1},
				{UpperBound: math.Inf(+1), Count: 0},
			},
			wantValues: map[float64]float64{
				0.5:  2.54748338082,
				0.9:  4.58547008547,
				0.95: 4.84021842355,
				0.99: 7.43947368421,
				1.0:  25.0,
			},
		},
	}
	for _, tt := range tests {
		quantiles := slices.Collect(maps.Keys(tt.wantValues))
		sort.Float64s(quantiles)
		for _, q := range quantiles {
			t.Run(fmt.Sprintf("%s %.2f", tt.name, q), func(t *testing.T) {
				assertInDeltaWithInfAndNaN(
					t,
					tt.wantValues[q],
					explicitBucketsQuantile(q, tt.buckets),
					1e-9,
					"explicitBucketsQuantile()",
				)
			})
		}
	}
}

func assertInDeltaWithInfAndNaN(t *testing.T, want, got, delta float64, msgAndArgs ...any) {
	if math.IsNaN(want) {
		assert.True(t, math.IsNaN(got), msgAndArgs...)
		return
	}

	if math.IsInf(want, +1) {
		assert.True(t, math.IsInf(got, +1), msgAndArgs...)
		return
	}

	if math.IsInf(want, -1) {
		assert.True(t, math.IsInf(got, -1), msgAndArgs...)
		return
	}

	assert.InDelta(t, want, got, delta, msgAndArgs...)
}

func Test_addHistogramDataPoint(t *testing.T) {
	ts1 := time.Unix(111111111, 0)
	ts2 := time.Unix(222222222, 0)
	ts3 := time.Unix(333333333, 0)
	genBuckets := func(bucketCounts [5]uint64) []explicitBucket {
		return []explicitBucket{
			{UpperBound: 0, Count: bucketCounts[0]},
			{UpperBound: 5, Count: bucketCounts[1]},
			{UpperBound: 10, Count: bucketCounts[2]},
			{UpperBound: 25, Count: bucketCounts[3]},
			{UpperBound: math.Inf(+1), Count: bucketCounts[4]},
		}
	}

	boundsMismatchDP := newTestHistogramDataPoint(
		genBuckets([5]uint64{0, 1, 2, 3, 4}),
		200, 1, 80, nil, ts1, ts2,
	)
	boundsMismatchDP.ExplicitBounds().FromRaw([]float64{0, 100, 200, 300})

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
			name: "explicit bounds mismatch",
			args: args{
				from: boundsMismatchDP,
				to: newTestHistogramDataPoint(
					genBuckets([5]uint64{0, 1, 0, 2, 0}),
					35, 3, 18, nil, ts1, ts2,
				),
			},
			expected: boundsMismatchDP,
		},
		{
			name: "merge to empty",
			args: args{
				from: newTestHistogramDataPoint(
					genBuckets([5]uint64{0, 1, 2, 3, 4}),
					200, 1, 100, nil, ts1, ts2,
				),
				to: pmetric.NewHistogramDataPoint(),
			},
			expected: newTestHistogramDataPoint(
				genBuckets([5]uint64{0, 1, 2, 3, 4}),
				200, 1, 100, nil, ts1, ts2,
			),
		},
		{
			name: "merge from empty",
			args: args{
				from: pmetric.NewHistogramDataPoint(),
				to: newTestHistogramDataPoint(
					genBuckets([5]uint64{0, 1, 2, 3, 4}),
					200, 1, 100, nil, ts1, ts2,
				),
			},
			expected: newTestHistogramDataPoint(
				genBuckets([5]uint64{0, 1, 2, 3, 4}),
				200, 1, 100, nil, ts1, ts2,
			),
		},
		{
			name: "merge",
			args: args{
				from: newTestHistogramDataPoint(
					genBuckets([5]uint64{0, 1, 2, 3, 4}),
					200, 1, 80, nil, ts2, ts3),
				to: newTestHistogramDataPoint(
					genBuckets([5]uint64{0, 1, 0, 2, 0}),
					35, 3, 18, nil, ts1, ts2,
				),
			},
			expected: newTestHistogramDataPoint(
				genBuckets([5]uint64{0, 2, 2, 5, 4}),
				235, 1, 80, nil, ts1, ts3,
			),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			addHistogramDataPoint(tt.args.from, tt.args.to)
			if err := pmetrictest.CompareHistogramDataPoints(tt.expected, tt.args.to); err != nil {
				t.Errorf("addHistogramDataPoint() compare error = %v", err)
			}
		})
	}
}

func newTestHistogramDataPoint(
	buckets []explicitBucket,
	sum, min, max float64,
	attrs map[string]string,
	startTime, endTime time.Time,
) pmetric.HistogramDataPoint {
	explicitBounds := make([]float64, len(buckets)-1)
	bucketCounts := make([]uint64, len(buckets))
	for i, bucket := range buckets {
		if i == len(buckets)-1 {
			bucketCounts[i] = bucket.Count
		} else {
			explicitBounds[i] = bucket.UpperBound
			bucketCounts[i] = bucket.Count
		}
	}

	var totalCount uint64
	for _, bc := range bucketCounts {
		totalCount += bc
	}

	dp := pmetric.NewHistogramDataPoint()
	dp.ExplicitBounds().FromRaw(explicitBounds)
	dp.BucketCounts().FromRaw(bucketCounts)
	dp.SetCount(totalCount)
	dp.SetSum(sum)
	dp.SetStartTimestamp(pcommon.NewTimestampFromTime(startTime))
	dp.SetTimestamp(pcommon.NewTimestampFromTime(endTime))
	for k, v := range attrs {
		dp.Attributes().PutStr(k, v)
	}

	return dp
}
