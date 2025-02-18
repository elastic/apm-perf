package otelinmemexporter

import (
	"math"
	"slices"

	"go.opentelemetry.io/collector/pdata/pmetric"
)

type explicitBucket struct {
	UpperBound float64 // inclusive
	Count      uint    // beware that this will be converted to float64 in calculations
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
	var observations uint
	for _, bucket := range buckets {
		observations += bucket.Count
	}
	if observations == 0 {
		return math.NaN()
	}

	// Find the bucket that the quantile falls into.
	rank := q * float64(observations)
	var countSoFar uint
	bucketIdx := slices.IndexFunc(buckets, func(bucket explicitBucket) bool {
		countSoFar += bucket.Count
		// Compare using GTE instead of GT since upper bound is inclusive.
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
	count := buckets[bucketIdx].Count
	bucketQuantile := (rank - float64(countSoFar-count)) / float64(count)
	return bucketStart + (bucketEnd-bucketStart)*bucketQuantile
}

func mergeHistogramDataPoint(from, to pmetric.HistogramDataPoint) {
	if from.Count() == 0 {
		// from is empty, do nothing
		return
	}

	if to.Count() == 0 {
		// to is new, simply copy over
		from.CopyTo(to)
		return
	}

	to.SetCount(to.Count() + from.Count())
	to.SetSum(to.Sum() + from.Sum())
	// Overwrite min if lower
	if to.HasMin() && to.Min() > from.Min() {
		to.SetMin(from.Min())
	}
	// Overwrite max if higher
	if to.HasMax() && to.Max() < from.Max() {
		to.SetMax(from.Max())
	}
	// Merge buckets
	bucketCounts := to.BucketCounts()
	for b := 0; b < from.BucketCounts().Len(); b++ {
		bucketCounts.SetAt(b, bucketCounts.At(b)+from.BucketCounts().At(b))
	}
	from.Exemplars().MoveAndAppendTo(to.Exemplars())
	// Overwrite start timestamp if lower
	if from.StartTimestamp() < to.StartTimestamp() {
		to.SetStartTimestamp(from.StartTimestamp())
	}
	// Overwrite timestamp if higher
	if from.Timestamp() > to.Timestamp() {
		to.SetTimestamp(from.Timestamp())
	}
}
