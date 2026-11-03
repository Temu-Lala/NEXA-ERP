package models

import (
	"math"
	"testing"
)

func approx(t *testing.T, got, want float64, what string) {
	t.Helper()
	if math.Abs(got-want) > 0.01 {
		t.Errorf("%s = %v want %v", what, got, want)
	}
}

func TestRatePctNotApplicable(t *testing.T) {
	if _, ok := ratePct(5, 0); ok {
		t.Errorf("zero denominator must be not-applicable, not zero")
	}
	if v, ok := ratePct(0, 0); ok {
		t.Errorf("0/0 must be not-applicable (got %v)", v)
	}
	v, ok := ratePct(3, 4)
	if !ok {
		t.Fatalf("3/4 should compute")
	}
	approx(t, v, 75, "3/4 pct")
}

func TestOnTimeDenominator(t *testing.T) {
	v, ok := onTimeScore(8, 10)
	if !ok {
		t.Fatalf("on-time with deliveries should compute")
	}
	approx(t, v, 80, "8/10 on-time")
	if _, ok := onTimeScore(0, 0); ok {
		t.Errorf("no deliveries must be not-applicable")
	}
}

func TestFillScoreCapsAtHundred(t *testing.T) {
	v, ok := fillScore(120, 100)
	if !ok {
		t.Fatalf("fill with ordered>0 should compute")
	}
	approx(t, v, 100, "over-delivery caps at 100")
	v2, _ := fillScore(80, 100)
	approx(t, v2, 80, "80/100 fill")
	if _, ok := fillScore(5, 0); ok {
		t.Errorf("ordered=0 fill must be not-applicable")
	}
}

func TestAcceptanceDenominator(t *testing.T) {
	// inspected = accepted + rejected + damaged, NOT ordered
	v, ok := acceptanceScore(90, 90+5+5)
	if !ok {
		t.Fatalf("acceptance with inspections should compute")
	}
	approx(t, v, 90, "90/100 acceptance")
	if _, ok := acceptanceScore(0, 0); ok {
		t.Errorf("no inspections must be not-applicable")
	}
}

func TestDefectScoreLowerBetter(t *testing.T) {
	v, ok := defectScore(10, 100)
	if !ok {
		t.Fatalf("defect with received>0 should compute")
	}
	approx(t, v, 90, "10% defect -> 90 score")
	v2, _ := defectScore(0, 100)
	approx(t, v2, 100, "no defects -> perfect")
	if _, ok := defectScore(3, 0); ok {
		t.Errorf("received=0 defect must be not-applicable")
	}
}

func TestNormalizeDirection(t *testing.T) {
	approx(t, normalizeDirection(70, DirHigher), 70, "higher keeps value")
	approx(t, normalizeDirection(30, DirLower), 70, "lower inverts")
	if got := normalizeDirection(150, DirHigher); got != 100 {
		t.Errorf("must clamp above 100, got %v", got)
	}
}

func TestGradeFor(t *testing.T) {
	if g := gradeFor(50, false); g != GradeNA {
		t.Errorf("no data must be not_rated, got %v", g)
	}
	cases := []struct {
		score float64
		want  Grade
	}{
		{95, GradeExcellent},
		{80, GradeGood},
		{65, GradeAccept},
		{45, GradePoor},
		{20, GradeCritical},
	}
	for _, c := range cases {
		if g := gradeFor(c.score, true); g != c.want {
			t.Errorf("grade(%v) = %v want %v", c.score, g, c.want)
		}
	}
}

func TestWeightedCompositeDropsAbsent(t *testing.T) {
	// A dimension with no data must not dilute the score toward zero.
	v, ok := weightedComposite([]dimValue{
		{Score: 80, Weight: 50, HasData: true},
		{Score: 0, Weight: 50, HasData: false},
	})
	if !ok {
		t.Fatalf("should compute from the one present dimension")
	}
	approx(t, v, 80, "absent dimension weight dropped")

	if _, ok := weightedComposite([]dimValue{{Score: 90, Weight: 10, HasData: false}}); ok {
		t.Errorf("all-absent composite must be not-applicable")
	}

	v2, _ := weightedComposite([]dimValue{
		{Score: 100, Weight: 30, HasData: true},
		{Score: 50, Weight: 70, HasData: true},
	})
	approx(t, v2, 65, "(100*30 + 50*70)/100")
}

func TestEscalationNeverBlocks(t *testing.T) {
	// Even a critical score only escalates for review; there is no block outcome.
	e := escalationFor(10, true, 70, 50)
	if e != EscEscalate {
		t.Errorf("very low score should escalate, got %v", e)
	}
	if string(e) == "block" || string(e) == "blocked" {
		t.Errorf("escalation must never auto-block")
	}
	if got := escalationFor(60, true, 70, 50); got != EscReview {
		t.Errorf("60 between thresholds should be review, got %v", got)
	}
	if got := escalationFor(85, true, 70, 50); got != EscNone {
		t.Errorf("healthy score needs no escalation, got %v", got)
	}
	if got := escalationFor(10, false, 70, 50); got != EscNone {
		t.Errorf("no data means no signal, got %v", got)
	}
}

func TestMeanScoreIgnoresMissing(t *testing.T) {
	v, ok := meanScore([]float64{80, 0, 60}, []bool{true, false, true})
	if !ok {
		t.Fatalf("mean of present values should compute")
	}
	approx(t, v, 70, "mean of 80 and 60")
	if _, ok := meanScore([]float64{0}, []bool{false}); ok {
		t.Errorf("all-missing mean must be not-applicable")
	}
}
