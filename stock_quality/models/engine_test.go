package models

import (
	"math"
	"testing"
)

func TestSampleQty(t *testing.T) {
	cases := []struct {
		qty, pct, want float64
	}{
		{100, 10, 10},   // plain percentage
		{7, 10, 1},      // ceil(0.7) — never round a sample down to zero
		{0, 50, 0},      // nothing received, nothing sampled
		{-5, 50, 0},     // negative guard
		{100, 100, 100}, // full inspection
		{100, 150, 100}, // over-100% capped at quantity
		{100, 0, 0},     // zero percent samples nothing
		{100, -10, 0},   // negative percent guard
		{2.5, 40, 1},    // ceil(1.0) on fractional receipts
	}
	for _, c := range cases {
		if got := sampleQty(c.qty, c.pct); math.Abs(got-c.want) > 1e-9 {
			t.Errorf("sampleQty(%v, %v) = %v, want %v", c.qty, c.pct, got, c.want)
		}
	}
}

func TestVerdictFor(t *testing.T) {
	cases := []struct {
		sample, defects, threshold float64
		want                       CheckResult
	}{
		{0, 0, 100, ResultPending}, // empty sample cannot decide
		{10, 0, 100, ResultPass},   // perfect sample passes
		{10, 1, 90, ResultPass},    // 90% good meets a 90% threshold
		{10, 2, 90, ResultCond},    // 80% good is below threshold, salvageable
		{10, 10, 50, ResultFail},   // everything defective is a hard fail
		{10, 12, 50, ResultFail},   // over-recorded defects still fail
		{8, 0, 0, ResultPass},      // zero threshold always passes
	}
	for _, c := range cases {
		if got := verdictFor(c.sample, c.defects, c.threshold); got != c.want {
			t.Errorf("verdictFor(%v, %v, %v) = %s, want %s",
				c.sample, c.defects, c.threshold, got, c.want)
		}
	}
}

func TestCanRelease(t *testing.T) {
	if canRelease(5, 0) {
		t.Error("anonymous uid must not release holds")
	}
	if canRelease(5, -1) {
		t.Error("negative uid must not release holds")
	}
	if canRelease(5, 5) {
		t.Error("the opener must not release their own hold (separation of duties)")
	}
	if !canRelease(5, 6) {
		t.Error("a distinct second person must be able to release")
	}
	if canRelease(0, 0) {
		t.Error("both-zero must not release")
	}
	if !canRelease(0, 6) {
		t.Error("a hold without an opener (cron) may be released by any real user")
	}
}
