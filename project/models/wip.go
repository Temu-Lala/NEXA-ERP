package models

import (
	"context"
	"fmt"

	"nexa/core/orm"
)

// Work-in-progress limits. A board column with a limit is a promise about how much
// work the team keeps in flow at once, and a promise the system should not let people
// break by accident: the cap is enforced where the write happens, so a drag or an edit
// that would over-fill the column is refused rather than performed and then reported.
// That is the difference between a limit and a warning.
//
// Only work still in flow counts. A finished issue occupies no attention, so done and
// cancelled cards leave the column's budget unless the stage is explicitly configured
// to count them. Both the card cap and the points cap are optional; zero means the
// column is uncapped, which is the honest default rather than a fabricated number.

// stageWipRule is a column's configured capacity, as pure data.
type stageWipRule struct {
	name        string
	cardLimit   int
	pointsLimit float64
	countDone   bool
}

// wipUsage is how much of a column's capacity is currently taken.
type wipUsage struct {
	cards  int
	points float64
}

// wipIncoming is the card about to enter a column: the issue itself (never counted
// against itself), the points it will spend once there, and whether it is even in flow.
// A finished card moving in occupies nothing, so it neither takes a slot nor spends points.
type wipIncoming struct {
	id     int
	points float64
	inFlow bool
}

// stageAcceptsAddition is the whole WIP decision, with no database and no clock: given
// a column's limits, the cards already sitting in it, and the issue about to enter, it
// reports whether the column can take the card. Keeping it pure makes the rule testable
// in isolation and impossible to disagree with the enforcement that calls it.
func stageAcceptsAddition(rule stageWipRule, rows []map[string]interface{}, incoming wipIncoming) (wipUsage, error) {
	if rule.cardLimit <= 0 && rule.pointsLimit <= 0 {
		return wipUsage{}, nil
	}
	usage := countWip(rows, incoming.id, rule.countDone)
	added := 0
	addedPoints := 0.0
	if incoming.inFlow {
		added = 1
		addedPoints = incoming.points
	}
	if rule.cardLimit > 0 && usage.cards+added > rule.cardLimit {
		return usage, fmt.Errorf("%q is at its WIP limit of %d card(s): %d are already in flow. Finish one, or raise the limit on the column.",
			rule.name, rule.cardLimit, usage.cards)
	}
	if rule.pointsLimit > 0 && usage.points+addedPoints > rule.pointsLimit {
		return usage, fmt.Errorf("%q would exceed its WIP limit of %g point(s): %g are already in flow and this card takes %g.",
			rule.name, rule.pointsLimit, usage.points, incoming.points)
	}
	return usage, nil
}

// countWip totals the capacity used by the rows that are genuinely in flow: the issue
// being written is excluded, and finished work is only counted where the column says to.
func countWip(rows []map[string]interface{}, excludeID int, countDone bool) wipUsage {
	var u wipUsage
	for _, r := range rows {
		id := intID(r["id"])
		if id <= 0 || id == excludeID {
			continue
		}
		if !countsTowardWip(str(r["state"]), countDone) {
			continue
		}
		u.cards++
		if p, ok := orm.CoerceFloat64(r["story_points"]); ok && p > 0 {
			u.points += p
		}
	}
	return u
}

// countsTowardWip states which work occupies a column. Anything finished is out of
// flow; everything else, including a blocked card, still holds the column open — a
// blocked issue is the very reason a WIP limit exists.
func countsTowardWip(state string, countDone bool) bool {
	switch TaskState(state) {
	case TaskStateDone:
		return countDone
	case TaskStateCancelled:
		return false
	case "":
		return true
	}
	return true
}

// ---------------------------------------------------------------------------
// Wiring: enforce on write, report on read
// ---------------------------------------------------------------------------

func init() {
	orm.RegisterCompute(stageModel, "wip_usage", []string{"id"}, computeStageWipUsage)
	orm.RegisterCompute(stageModel, "points_used", []string{"id"}, computeStagePointsUsed)
	orm.RegisterConstraint(taskModel, "task_wip_limit", constraintTaskWipLimit)
}

// constraintTaskWipLimit refuses a write that would push a capped column past its
// limit. It only reacts when the write actually touches the column: a task edited in
// place (a renamed card, a new comment) must not be blocked by its own column's
// traffic, which would make the board unusable.
func constraintTaskWipLimit(ctx context.Context, rec map[string]interface{}) error {
	sid := intID(rec["stage_id"])
	if sid <= 0 {
		return nil
	}
	// A create always places a new card, so it is always an addition; a write counts
	// only when it moves the card into this column from somewhere else.
	_, isMove := rec["stage_id"]
	id := intID(rec["id"])
	if id == 0 {
		isMove = true
	}
	if !isMove {
		return nil
	}
	var rule stageWipRule
	row, err := orm.SearchOne(orm.WithElevatedReader(ctx, "project.wip"), stageModel, map[string]interface{}{"id": sid})
	if err == nil && row != nil {
		rule = stageWipRule{
			name:        str(row["name"]),
			cardLimit:   int(intID(row["wip_limit"])),
			pointsLimit: floatOf(row["wip_points_limit"]),
			countDone:   boolv(row["count_done_in_wip"]),
		}
	}
	if rule.cardLimit <= 0 && rule.pointsLimit <= 0 {
		return nil
	}
	if rule.name == "" {
		rule.name = "this column"
	}
	// Read the column's own contents with the kernel's view: the limit is a property of
	// the board, and it must not be possible to slip a card past the cap simply by being
	// unable to see the cards already there.
	bypass := orm.WithElevatedReader(orm.ContextWithBypass(ctx, true), "project.wip")
	rows, err := orm.SearchPage(bypass, taskModel, [][]interface{}{{"stage_id", "=", sid}}, 100000, 0, "id asc")
	if err != nil {
		// Unable to establish the column's contents is not permission to over-fill it.
		return fmt.Errorf("cannot verify the column's work-in-progress limit; try again")
	}
	incoming := wipIncoming{
		id:     id,
		points: floatOf(rec["story_points"]),
		inFlow: countsTowardWip(str(rec["state"]), rule.countDone),
	}
	_, err = stageAcceptsAddition(rule, rows, incoming)
	return err
}

// computeStageWipUsage reports how many cards currently occupy a column, so the limit
// can be read next to the number it is measured against.
func computeStageWipUsage(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return stageUsage(ctx, rec).cards, nil
}

// computeStagePointsUsed is the same reading in story points.
func computeStagePointsUsed(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return round2(stageUsage(ctx, rec).points), nil
}

// stageUsage reads one column's in-flow total through the caller's own view, so what a
// person is told is occupied is what they can actually see on the board.
func stageUsage(ctx context.Context, rec map[string]interface{}) wipUsage {
	sid := intID(rec["id"])
	if sid <= 0 {
		return wipUsage{}
	}
	countDone := boolv(rec["count_done_in_wip"])
	rows, err := orm.SearchPage(ctx, taskModel, [][]interface{}{{"stage_id", "=", sid}}, 100000, 0, "id asc")
	if err != nil {
		return wipUsage{}
	}
	return countWip(rows, 0, countDone)
}

// floatOf reads a numeric field that may arrive as a number, a driver string or JSON.
func floatOf(v interface{}) float64 {
	f, _ := orm.CoerceFloat64(v)
	return f
}
