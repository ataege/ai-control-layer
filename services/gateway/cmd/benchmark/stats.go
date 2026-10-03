package main

import (
	"math"
	"slices"
	"time"
)

// Distribution summarizes observed durations in microseconds. Every statistic is null until
// there is at least one observation: a benchmark never reports a value it did not measure.
type Distribution struct {
	Count  int    `json:"count"`
	Median *int64 `json:"medianMicroseconds"`
	P95    *int64 `json:"p95Microseconds"`
	Max    *int64 `json:"maxMicroseconds"`
	Mean   *int64 `json:"meanMicroseconds"`
}

// summarize returns the nearest-rank distribution of the observations.
func summarize(observations []time.Duration) Distribution {
	distribution := Distribution{Count: len(observations)}
	if len(observations) == 0 {
		return distribution
	}
	sorted := slices.Clone(observations)
	slices.Sort(sorted)
	var total time.Duration
	for _, observation := range sorted {
		total += observation
	}
	distribution.Median = microseconds(nearestRank(sorted, 0.50))
	distribution.P95 = microseconds(nearestRank(sorted, 0.95))
	distribution.Max = microseconds(sorted[len(sorted)-1])
	distribution.Mean = microseconds(total / time.Duration(len(sorted)))
	return distribution
}

// nearestRank returns the smallest observation with at least the quantile's share at or below it.
func nearestRank(sorted []time.Duration, quantile float64) time.Duration {
	rank := int(math.Ceil(quantile * float64(len(sorted))))
	if rank < 1 {
		rank = 1
	}
	return sorted[rank-1]
}

func microseconds(duration time.Duration) *int64 {
	value := duration.Microseconds()
	return &value
}
