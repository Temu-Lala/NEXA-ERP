package models

import (
	"math"
	"testing"
)

func almost(a, b float64) bool { return math.Abs(a-b) < 0.005 }

func TestPercentage(t *testing.T) {
	if got := percentage(85, 100); !almost(got, 85) {
		t.Fatalf("percentage(85,100) = %v, want 85", got)
	}
	if got := percentage(1, 3); !almost(got, 33.33) {
		t.Fatalf("percentage(1,3) = %v, want 33.33", got)
	}
	if got := percentage(50, 0); got != 0 {
		t.Fatalf("percentage with max<=0 = %v, want 0", got)
	}
}

func bands() []band {
	return []band{
		{minPct: 90, grade: "A", points: 4},
		{minPct: 80, grade: "B", points: 3},
		{minPct: 70, grade: "C", points: 2},
		{minPct: 0, grade: "F", points: 0},
	}
}

func TestPickBand(t *testing.T) {
	cases := []struct {
		pct        float64
		wantGrade  string
		wantPoints float64
	}{
		{95, "A", 4},
		{90, "A", 4},
		{89.9, "B", 3},
		{80, "B", 3},
		{72, "C", 2},
		{10, "F", 0},
	}
	for _, c := range cases {
		g, p := pickBand(c.pct, bands())
		if g != c.wantGrade || !almost(p, c.wantPoints) {
			t.Fatalf("pickBand(%v) = (%q,%v), want (%q,%v)", c.pct, g, p, c.wantGrade, c.wantPoints)
		}
	}
	// No band reached (all mins above pct) yields no grade.
	if g, p := pickBand(-1, []band{{minPct: 50, grade: "P", points: 1}}); g != "" || p != 0 {
		t.Fatalf("pickBand below all bounds = (%q,%v), want (\"\",0)", g, p)
	}
	// Empty band set.
	if g, p := pickBand(100, nil); g != "" || p != 0 {
		t.Fatal("pickBand with no bands must return no grade")
	}
}

func TestWeightedMean(t *testing.T) {
	if got := weightedMean(nil, nil); got != 0 {
		t.Fatalf("empty weightedMean = %v, want 0", got)
	}
	// GPA: 4*(3 credits) + 2*(1 credit) = 14/4 = 3.5
	if got := weightedMean([]float64{4, 2}, []float64{3, 1}); !almost(got, 3.5) {
		t.Fatalf("weightedMean GPA = %v, want 3.5", got)
	}
	// Zero/negative weights fall back to unit weight -> simple mean.
	if got := weightedMean([]float64{2, 4}, []float64{0, 0}); !almost(got, 3) {
		t.Fatalf("weightedMean unit fallback = %v, want 3", got)
	}
}

func TestRankFromHigher(t *testing.T) {
	if rankFromHigher(0) != 1 {
		t.Fatal("no better peers should rank 1")
	}
	if rankFromHigher(4) != 5 {
		t.Fatal("4 better peers should rank 5")
	}
}

func TestNonZero(t *testing.T) {
	if nonZero(0, 1) != 1 {
		t.Fatal("zero should fall back")
	}
	if nonZero(-5, 2) != 2 {
		t.Fatal("negative should fall back")
	}
	if nonZero(3, 1) != 3 {
		t.Fatal("positive should be kept")
	}
}
