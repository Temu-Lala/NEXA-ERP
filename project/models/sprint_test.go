package models

import (
	"strings"
	"testing"
	"time"

	"nexa/core/engine/swcmeta"
)

func taskRow(state TaskState, points int) map[string]interface{} {
	return map[string]interface{}{"state": string(state), "story_points": int64(points)}
}

// tallyRows defines the sprint counters: cancelled work leaves scope, finished work
// burns points off the sprint, everything else stays open.
func TestTallyRowsCountsScopeAndBurn(t *testing.T) {
	rows := []map[string]interface{}{
		taskRow(TaskStateDone, 5),
		taskRow(TaskStateInProgress, 3),
		taskRow(TaskStateBlocked, 2),
		taskRow(TaskStateCancelled, 8),
	}
	got := tallyRows(rows)
	if got.Issues != 3 {
		t.Fatalf("issues = %d, want 3 (cancelled excluded)", got.Issues)
	}
	if got.Total != 10 {
		t.Fatalf("total = %d, want 10", got.Total)
	}
	if got.Done != 5 {
		t.Fatalf("done = %d, want 5", got.Done)
	}
	if got.Open != 2 {
		t.Fatalf("open = %d, want 2", got.Open)
	}
}

func TestTallyRowsEmpty(t *testing.T) {
	got := tallyRows(nil)
	if got != (scopeTally{}) {
		t.Fatalf("empty tally = %+v, want zero", got)
	}
}

func TestSprintStartPlanRequiresDraft(t *testing.T) {
	today := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	if _, err := sprintStartPlan(SprintActive, 0, "", "2026-11-14", today); err == nil {
		t.Fatal("starting an active sprint must fail")
	}
	if _, err := sprintStartPlan(SprintClosed, 0, "", "2026-11-14", today); err == nil {
		t.Fatal("starting a closed sprint must fail")
	}
}

// A team runs one sprint at a time: a second active sprint in the same project is
// refused rather than silently allowed.
func TestSprintStartPlanRefusesSecondActiveSprint(t *testing.T) {
	today := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	_, err := sprintStartPlan(SprintDraft, 1, "2026-11-01", "2026-11-14", today)
	if err == nil || !strings.Contains(err.Error(), "already has an active sprint") {
		t.Fatalf("expected active-sprint refusal, got %v", err)
	}
}

func TestSprintStartPlanValidatesTimebox(t *testing.T) {
	today := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	if _, err := sprintStartPlan(SprintDraft, 0, "", "", today); err == nil {
		t.Fatal("a sprint without an end date must not start")
	}
	if _, err := sprintStartPlan(SprintDraft, 0, "2026-11-14", "2026-11-01", today); err == nil {
		t.Fatal("an end date before the start must not start")
	}
	if _, err := sprintStartPlan(SprintDraft, 0, "not-a-date", "2026-11-14", today); err == nil {
		t.Fatal("an unparseable start date must not start")
	}
}

// A blank start date is the day the sprint really starts, not an invented date.
func TestSprintStartPlanDefaultsStartToToday(t *testing.T) {
	today := time.Date(2026, 10, 31, 9, 30, 0, 0, time.UTC)
	start, err := sprintStartPlan(SprintDraft, 0, "  ", "2026-11-14", today)
	if err != nil {
		t.Fatal(err)
	}
	if start != "2026-10-31" {
		t.Fatalf("start = %q, want today", start)
	}
}

func TestSprintCloseCheckOnlyFromActive(t *testing.T) {
	if err := sprintCloseCheck(SprintActive); err != nil {
		t.Fatalf("closing an active sprint must be allowed: %v", err)
	}
	if err := sprintCloseCheck(SprintDraft); err == nil {
		t.Fatal("a draft sprint cannot be closed")
	}
	if err := sprintCloseCheck(SprintClosed); err == nil {
		t.Fatal("a closed sprint cannot be closed twice")
	}
}

// Open work must never sit in a closed sprint, or velocity would keep changing after
// the fact.
func TestTaskSprintCheckBlocksOpenWorkInClosedSprint(t *testing.T) {
	if err := taskSprintCheck(SprintClosed, TaskStateInProgress); err == nil {
		t.Fatal("open work in a closed sprint must be refused")
	}
	if err := taskSprintCheck(SprintClosed, TaskStateDone); err != nil {
		t.Fatalf("finished work may stay in a closed sprint: %v", err)
	}
	if err := taskSprintCheck(SprintClosed, TaskStateCancelled); err != nil {
		t.Fatalf("cancelled work may stay in a closed sprint: %v", err)
	}
	if err := taskSprintCheck(SprintActive, TaskStateTodo); err != nil {
		t.Fatalf("an active sprint accepts open work: %v", err)
	}
	if err := taskSprintCheck(SprintDraft, TaskStateTodo); err != nil {
		t.Fatalf("a planned sprint accepts open work: %v", err)
	}
}

func TestSprintFrozenCheckProtectsClosedRecord(t *testing.T) {
	if err := sprintFrozenCheck(SprintClosed, SprintActive, 20, 20, 12, 12); err == nil {
		t.Fatal("a closed sprint must not be reopened")
	}
	if err := sprintFrozenCheck(SprintClosed, SprintClosed, 20, 20, 12, 15); err == nil {
		t.Fatal("completed points of a closed sprint must be frozen")
	}
	if err := sprintFrozenCheck(SprintClosed, SprintClosed, 20, 25, 12, 12); err == nil {
		t.Fatal("committed points of a closed sprint must be frozen")
	}
	if err := sprintFrozenCheck(SprintClosed, SprintClosed, 20, 20, 12, 12); err != nil {
		t.Fatalf("an unchanged closed sprint must be writable: %v", err)
	}
	if err := sprintFrozenCheck(SprintActive, SprintClosed, 20, 0, 0, 12); err != nil {
		t.Fatalf("the close action writes the record while the stored state is active: %v", err)
	}
}

// The planning board must use the sprint expander, not the generic many2one lanes:
// closed sprints would otherwise pile up as permanent columns.
func TestSprintKanbanExpanderIsWired(t *testing.T) {
	if !swcmeta.HasKanbanGroupExpanderForTest(taskModel, "sprint_id") {
		t.Fatalf("no kanban expander registered for %s:sprint_id", taskModel)
	}
}

func TestSnapshotValuesDeriveRemaining(t *testing.T) {
	day := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	vals := snapshotValues(7, day, scopeTally{Issues: 4, Open: 1, Total: 21, Done: 13})
	if vals["remaining_points"] != 8 {
		t.Fatalf("remaining = %v, want 8", vals["remaining_points"])
	}
	if vals["date"] != "2026-11-03" {
		t.Fatalf("date = %v, want 2026-11-03", vals["date"])
	}
	if vals["sprint_id"] != 7 {
		t.Fatalf("sprint_id = %v, want 7", vals["sprint_id"])
	}
	if vals["open_issues"] != 1 {
		t.Fatalf("open_issues = %v, want 1", vals["open_issues"])
	}
}

// Dates scanned from the database arrive as time.Time. The start action must read a
// stored date as a plain date, not fmt.Sprint of a timestamp, or a valid timebox is
// rejected on the second read.
func TestDateValueFormatsStoredDate(t *testing.T) {
	stored := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	if got := dateValue(stored); got != "2026-10-31" {
		t.Fatalf("dateValue(time.Time) = %q, want 2026-10-31", got)
	}
	if got := dateValue("2026-11-13"); got != "2026-11-13" {
		t.Fatalf("dateValue(string) = %q, want 2026-11-13", got)
	}
	if got := dateValue(nil); got != "" {
		t.Fatalf("dateValue(nil) = %q, want empty", got)
	}
}

// parseDate accepts both a scanned time.Time and the fmt.Sprint form of a timestamp.
func TestParseDateAcceptsStoredTimestamp(t *testing.T) {
	stored := time.Date(2026, 10, 31, 0, 0, 0, 0, time.UTC)
	if got, ok := parseDate(stored); !ok || !got.Equal(stored) {
		t.Fatalf("parseDate(time.Time) = %v, %v", got, ok)
	}
	if _, ok := parseDate(stored.String()); !ok {
		t.Fatalf("parseDate(%q) must parse the fmt.Sprint form", stored.String())
	}
}
