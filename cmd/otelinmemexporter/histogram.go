// Copyright Elasticsearch B.V. and/or licensed to Elasticsearch B.V. under one
// or more contributor license agreements. Licensed under the Elastic License 2.0;
// you may not use this file except in compliance with the Elastic License 2.0.

package otelinmemexporter

import (
	"math"
	"slices"

	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
)

type explicitBucket struct {
	UpperBound float64 // inclusive
	Count      uint64  // beware that this will be converted to float64 in calculations
}

// explicitBucketsFromHistogramDataPoint extracts buckets from histogram data point.
// The length of bucket counts in `dp` is assumed to be the length of explicit bounds plus one.
// Violating this assumption will cause index-out-of-range panic.
func explicitBucketsFromHistogramDataPoint(dp pmetric.HistogramDataPoint) []explicitBucket {
	bucketCounts := dp.BucketCounts().AsRaw()
	explicitBounds := dp.ExplicitBounds().AsRaw()
	buckets := make([]explicitBucket, 0, len(bucketCounts))
	for i, ub := range explicitBounds {
		buckets = append(buckets, explicitBucket{
			UpperBound: ub,
			Count:      bucketCounts[i],
		})
	}

	buckets = append(buckets, explicitBucket{
		UpperBound: math.Inf(+1),
		Count:      bucketCounts[len(bucketCounts)-1],
	})
	return buckets
}

// explicitBucketsQuantile calculates the quantile `q` based on the given buckets.
// The buckets are assumed to be contiguous, e.g. `(-Inf, 5], (5, 10], (10, +Inf]`, but can be
// unsorted since the function will sort the buckets based on upper bound prior to calculation.
// The buckets must also have the highest bound of +Inf.
//
// The quantile value is interpolated assuming a linear distribution within a bucket.
// However, if the quantile falls into the highest bucket i.e. `(x, +Inf]`, the lower bound of that
// bucket (x) is returned . On the other hand, if the quantile falls into the lowest bucket
// i.e. `(-Inf, y]`, the upper bound of that bucket (y) is returned.
//
// Here are some special cases:
//   - If `q` == NaN, NaN is returned.
//   - If `q` < 0, -Inf is returned.
//   - If `q` > 1, +Inf is returned.
//   - If `buckets` has fewer than 2 elements, NaN is returned.
//   - If the highest bucket is not +Inf, NaN is returned.
//   - If `buckets` has 0 observations, NaN is returned.
func explicitBucketsQuantile(q float64, buckets []explicitBucket) float64 {
	if math.IsNaN(q) {
		return math.NaN()
	}
	if q < 0 {
		return math.Inf(-1)
	}
	if q > 1 {
		return math.Inf(+1)
	}

	slices.SortFunc(buckets, func(a, b explicitBucket) int {
		// We don't expect the bucket boundary to be a NaN.
		if a.UpperBound < b.UpperBound {
			return -1
		}
		if a.UpperBound > b.UpperBound {
			return +1
		}
		return 0
	})

	if len(buckets) < 2 {
		return math.NaN()
	}

	// The highest bound must be +Inf, and the lowest bound must be -Inf.
	if !math.IsInf(buckets[len(buckets)-1].UpperBound, +1) {
		return math.NaN()
	}

	// Check if there are any observations.
	var observations uint64
	for _, bucket := range buckets {
		observations += bucket.Count
	}
	if observations == 0 {
		return math.NaN()
	}

	// Find the bucket that the quantile falls into.
	rank := q * float64(observations)
	var countSoFar uint64
	bucketIdx := slices.IndexFunc(buckets, func(bucket explicitBucket) bool {
		countSoFar += bucket.Count
		// Compare using `>=` instead of `>` since upper bound is inclusive.
		return float64(countSoFar) >= rank
	})

	if bucketIdx == len(buckets)-1 {
		return buckets[len(buckets)-2].UpperBound
	}
	if bucketIdx == 0 {
		return buckets[0].UpperBound
	}

	// Interpolate to get quantile in bucket.
	bucketStart := buckets[bucketIdx-1].UpperBound
	bucketEnd := buckets[bucketIdx].UpperBound
	bucketCount := buckets[bucketIdx].Count
	// How the bucket quantile is derived:
	// ==|=======|=======|=======|=======|==
	//   |       |       |       |       |
	//   | b - 2 | b - 1 |   b   | b + 1 |
	// ==|=======|=======|=======|=======|==
	// ----------------------> rank
	// --------------------------> countSoFar
	//                   |-------> bucketCount
	//                   |---> rank - (countSoFar - bucketCount)
	bucketQuantile := (rank - float64(countSoFar-bucketCount)) / float64(bucketCount)
	return bucketStart + (bucketEnd-bucketStart)*bucketQuantile
}

func float64SliceEqual(a, b pcommon.Float64Slice) bool {
	if a.Len() != b.Len() {
		return false
	}
	for i := 0; i < a.Len(); i++ {
		if a.At(i) != b.At(i) {
			return false
		}
	}
	return true
}

// addHistogramDataPoint adds the data from histogram data point `from` into `to`.
// This data includes:
//   - count
//   - sum
//   - min (if `to` has min and `from` min is lower)
//   - max (if `to` has max and `from` max is higher)
//   - bucket counts (assumes that both data points have same explicit bounds)
//   - exemplars
//   - start timestamp (if `from` start timestamp is lower)
//   - timestamp (if `from` timestamp is lower)
//
// Note: If both `from` and `to` does not have the same explicit bounds, `from` will simply
// replace `to` instead of being added.
func addHistogramDataPoint(from, to pmetric.HistogramDataPoint) {
	if from.Count() == 0 {
		// `from` is empty, do nothing.
		return
	}

	if to.Count() == 0 {
		// `to` is new, simply copy over.
		from.CopyTo(to)
		return
	}

	if !float64SliceEqual(from.ExplicitBounds(), to.ExplicitBounds()) {
		// Mismatched explicit bounds, replace observations since we can't simply merge.
		from.CopyTo(to)
		return
	}

	to.SetCount(to.Count() + from.Count())
	to.SetSum(to.Sum() + from.Sum())
	// Overwrite min if lower.
	if to.HasMin() && to.Min() > from.Min() {
		to.SetMin(from.Min())
	}
	// Overwrite max if higher.
	if to.HasMax() && to.Max() < from.Max() {
		to.SetMax(from.Max())
	}
	// Merge buckets.
	bucketCounts := to.BucketCounts()
	for b := 0; b < from.BucketCounts().Len(); b++ {
		bucketCounts.SetAt(b, bucketCounts.At(b)+from.BucketCounts().At(b))
	}
	from.Exemplars().MoveAndAppendTo(to.Exemplars())
	// Overwrite start timestamp if lower.
	if from.StartTimestamp() < to.StartTimestamp() {
		to.SetStartTimestamp(from.StartTimestamp())
	}
	// Overwrite timestamp if higher.
	if from.Timestamp() > to.Timestamp() {
		to.SetTimestamp(from.Timestamp())
	}
}
