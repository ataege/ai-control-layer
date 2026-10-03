package main

import (
	"testing"
	"time"
)

func TestSummarizeReportsNothingBeforeObservations(t *testing.T) {
	empty := summarize(nil)
	if empty.Count != 0 || empty.Median != nil || empty.P95 != nil || empty.Max != nil || empty.Mean != nil {
		t.Fatalf("statistics without observations: %+v", empty)
	}
}

func TestSummarizeUsesNearestRankOnObservedValues(t *testing.T) {
	// 1..20 ms in reverse order: the median is the 10th value, p95 the 19th.
	observations := make([]time.Duration, 0, 20)
	for value := 20; value >= 1; value-- {
		observations = append(observations, time.Duration(value)*time.Millisecond)
	}
	summary := summarize(observations)
	if summary.Count != 20 || *summary.Median != 10_000 || *summary.P95 != 19_000 || *summary.Max != 20_000 || *summary.Mean != 10_500 {
		t.Fatalf("summary %+v", summary)
	}
	if observations[0] != 20*time.Millisecond {
		t.Fatal("summarize reordered the caller's observations")
	}
	single := summarize([]time.Duration{7 * time.Microsecond})
	if *single.Median != 7 || *single.P95 != 7 || *single.Max != 7 {
		t.Fatalf("single observation %+v", single)
	}
}

func TestWorkloadIsThePermittedReadInvoiceResult(t *testing.T) {
	input, workload, err := buildWorkload()
	if err != nil {
		t.Fatal(err)
	}
	if workload.PayloadBytes != len(input.ResultJSON) || len(input.Untrusted) != 1 || input.Untrusted[0].Source.Classification != "internal_only" {
		t.Fatalf("workload %+v, input %+v", workload, input)
	}
}
