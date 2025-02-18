package otelinmemexporter

import "go.opentelemetry.io/collector/pdata/pmetric"

type bucket struct {
	count      int
	lowerBound float64 // exclusive
	upperBound float64 // inclusive
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
