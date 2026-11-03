package models

import (
	"testing"
)

// wipRow is a compact fixture for the pure WIP rule.
func wipRow(id int, state string, points int) map[string]interface{} {
	return map[string]interface{}{"id": id, "state": state, "story_points": points}
}

// activeIncoming is an in-flow card arriving at a column.
func activeIncoming(id int, points float64) wipIncoming {
	return wipIncoming{id: id, points: points, inFlow: true}
}

// The card cap counts work still in flow and refuses the card that would breach it.
func TestStageAcceptsAdditionCards(t *testing.T) {
	rule := stageWipRule{name: "In Progress", cardLimit: 2}
	rows := []map[string]interface{}{wipRow(1, "in_progress", 3), wipRow(2, "in_review", 5)}

	// Two already in flow, one arriving: over the cap of 2.
	if _, err := stageAcceptsAddition(rule, rows, activeIncoming(3, 0)); err == nil {
		t.Fatal("expected the third card to be refused at a limit of 2")
	}

	// Only one in flow: the arriving card fills the column exactly.
	if _, err := stageAcceptsAddition(rule, rows[:1], activeIncoming(3, 0)); err != nil {
		t.Fatalf("second card should be allowed to reach the limit: %v", err)
	}

	// Moving a card already in the column onto itself must not count it twice.
	if _, err := stageAcceptsAddition(rule, rows, activeIncoming(1, 0)); err != nil {
		t.Fatalf("rewriting a card in its own column should be allowed: %v", err)
	}
}

// A finished card arriving does not occupy a slot: it can land even in a full column.
func TestStageAcceptsAdditionFinishedIncomingTakesNoSlot(t *testing.T) {
	rule := stageWipRule{name: "In Progress", cardLimit: 1}
	rows := []map[string]interface{}{wipRow(1, "in_progress", 3)}
	// Column already at its limit of 1 in flow.
	if _, err := stageAcceptsAddition(rule, rows, wipIncoming{id: 2, inFlow: false}); err != nil {
		t.Fatalf("an arriving done/cancelled card should take no slot: %v", err)
	}
	// An arriving in-flow card, by contrast, is refused.
	if _, err := stageAcceptsAddition(rule, rows, activeIncoming(2, 0)); err == nil {
		t.Fatal("an arriving in-flow card must breach the full column")
	}
}

// Finished work leaves the budget: a column of done cards is not full.
func TestStageAcceptsAdditionIgnoresFinishedWork(t *testing.T) {
	rule := stageWipRule{name: "In Progress", cardLimit: 1}
	rows := []map[string]interface{}{
		wipRow(1, "done", 8),
		wipRow(2, "cancelled", 8),
	}
	if _, err := stageAcceptsAddition(rule, rows, activeIncoming(3, 0)); err != nil {
		t.Fatalf("done and cancelled cards should not consume the limit: %v", err)
	}

	// Unless the column explicitly says finished work still counts.
	rule.countDone = true
	if _, err := stageAcceptsAddition(rule, rows[:1], activeIncoming(3, 0)); err == nil {
		t.Fatal("with countDone a done card must still hold the column open")
	}
}

// A blocked card is still in flow — that is the whole reason a limit exists.
func TestCountsTowardWipBlockedStillCounts(t *testing.T) {
	if !countsTowardWip("blocked", false) {
		t.Fatal("blocked work must occupy the column")
	}
	if countsTowardWip("done", false) {
		t.Fatal("done work must leave the column")
	}
	if !countsTowardWip("in_progress", false) {
		t.Fatal("active work must occupy the column")
	}
}

// The points cap charges the arriving card its own story points, not a flat one.
func TestStageAcceptsAdditionPoints(t *testing.T) {
	rule := stageWipRule{name: "Review", pointsLimit: 8}
	rows := []map[string]interface{}{wipRow(1, "in_review", 5)}

	// 5 in flow + a 3-point card fits exactly.
	if _, err := stageAcceptsAddition(rule, rows, activeIncoming(2, 3)); err != nil {
		t.Fatalf("5+3 should fit a limit of 8: %v", err)
	}
	// 5 in flow + a 4-point card does not.
	if _, err := stageAcceptsAddition(rule, rows, activeIncoming(2, 4)); err == nil {
		t.Fatal("5+4 should breach a limit of 8")
	}
	// A zero-point card must not create a phantom point of demand.
	if _, err := stageAcceptsAddition(rule, rows, activeIncoming(2, 0)); err != nil {
		t.Fatalf("0-point card should fit: %v", err)
	}
}

// An uncapped column takes anything: zero is no limit, not a limit of zero.
func TestStageAcceptsAdditionUncapped(t *testing.T) {
	rule := stageWipRule{}
	rows := make([]map[string]interface{}, 50)
	for i := range rows {
		rows[i] = wipRow(i+1, "in_progress", 3)
	}
	if _, err := stageAcceptsAddition(rule, rows, activeIncoming(99, 0)); err != nil {
		t.Fatalf("an uncapped column must never refuse: %v", err)
	}
}

// countWip is the reading the badge shows: it must agree with the gate.
func TestCountWipTotals(t *testing.T) {
	rows := []map[string]interface{}{
		wipRow(1, "in_progress", 3),
		wipRow(2, "done", 8),
		wipRow(3, "blocked", 2),
		wipRow(4, "cancelled", 1),
	}
	got := countWip(rows, 0, false)
	if got.cards != 2 || got.points != 5 {
		t.Fatalf("usage = %+v, want 2 cards / 5 points", got)
	}
	// Excluding the card being written.
	got = countWip(rows, 1, false)
	if got.cards != 1 || got.points != 2 {
		t.Fatalf("usage excluding card 1 = %+v, want 1 card / 2 points", got)
	}
}
