package models

import "testing"

func TestScoreAggregate(t *testing.T) {
	lines := []map[string]interface{}{
		{"weight": "2", "rating": "5", "scale_max": "10"}, // 10 achieved, 20 max
		{"weight": "1", "rating": "8", "scale_max": "10"}, // 8 achieved, 10 max
		{"weight": "3", "rating": "0", "scale_max": "10"}, // 0 achieved, 30 max
	}
	total, max := scoreAggregate(lines)
	if total != 18 {
		t.Fatalf("total = %v, want 18", total)
	}
	if max != 60 {
		t.Fatalf("max = %v, want 60", max)
	}
}

func TestScoreAggregateEmpty(t *testing.T) {
	total, max := scoreAggregate(nil)
	if total != 0 || max != 0 {
		t.Fatalf("empty aggregate = (%v,%v), want (0,0)", total, max)
	}
}

func TestBandIndex(t *testing.T) {
	cases := []struct {
		name string
		tot  float64
		max  float64
		want CriticalityLevel
	}{
		{"zero_max", 0, 0, LevelLow},
		{"low", 10, 100, LevelLow},                    // 10%
		{"normal_boundary", 25, 100, LevelNormal},     // 25%
		{"high_boundary", 50, 100, LevelHigh},         // 50%
		{"critical_boundary", 75, 100, LevelCritical}, // 75%
		{"mid_normal", 40, 100, LevelNormal},
		{"full", 60, 60, LevelCritical}, // 100%
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := bandIndex(c.tot, c.max); got != c.want {
				t.Fatalf("bandIndex(%v,%v) = %v, want %v", c.tot, c.max, got, c.want)
			}
		})
	}
}

func TestRound(t *testing.T) {
	if got := round4(1.23456789); got != 1.2346 {
		t.Fatalf("round4 = %v, want 1.2346", got)
	}
	if got := round2(33.3333); got != 33.33 {
		t.Fatalf("round2 = %v, want 33.33", got)
	}
}
