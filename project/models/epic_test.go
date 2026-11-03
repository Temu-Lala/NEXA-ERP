package models

import (
	"strings"
	"testing"

	"nexa/core/modelreg"
	"nexa/core/orm"
)

// The epic roll-up reuses the same breakdown as a sprint: cancelled issues leave
// scope, finished issues burn points, everything else stays open.
func TestEpicRollUpFromRows(t *testing.T) {
	rows := []map[string]interface{}{
		{"state": string(TaskStateDone), "story_points": 5},
		{"state": string(TaskStateInProgress), "story_points": 3},
		{"state": string(TaskStateTodo), "story_points": 0},
		{"state": string(TaskStateCancelled), "story_points": 100},
	}
	got := tallyRows(rows)
	if got != (scopeTally{Issues: 3, Open: 2, Total: 8, Done: 5}) {
		t.Fatalf("roll-up = %+v, want 3 issues / 2 open / 8 points / 5 done", got)
	}
}

// Progress is a percentage of points when the epic was estimated, otherwise of
// finished issues. It never reports more than 100 or a negative number.
func TestEpicProgressPct(t *testing.T) {
	cases := []struct {
		name  string
		tally scopeTally
		want  int
	}{
		{"empty", scopeTally{}, 0},
		{"points half", scopeTally{Issues: 4, Open: 2, Total: 8, Done: 4}, 50},
		{"points partial", scopeTally{Issues: 4, Open: 2, Total: 8, Done: 5}, 63},
		{"points complete", scopeTally{Issues: 4, Open: 0, Total: 8, Done: 8}, 100},
		{"issue fallback", scopeTally{Issues: 4, Open: 1}, 75},
		{"issue fallback complete", scopeTally{Issues: 3, Open: 0}, 100},
		{"nothing done", scopeTally{Issues: 2, Open: 2, Total: 3}, 0},
	}
	for _, tc := range cases {
		if got := epicProgressPct(tc.tally); got != tc.want {
			t.Fatalf("%s: progress = %d, want %d", tc.name, got, tc.want)
		}
	}
}

// An issue may only join an epic from its own project; an epic groups its own
// project's work. A missing project on either side is not a disagreement.
func TestEpicProjectCheck(t *testing.T) {
	if err := epicProjectCheck(4, 4); err != nil {
		t.Fatalf("same project must be allowed: %v", err)
	}
	if err := epicProjectCheck(0, 9); err != nil {
		t.Fatalf("issue without project must be allowed: %v", err)
	}
	if err := epicProjectCheck(9, 0); err != nil {
		t.Fatalf("epic without project must be allowed: %v", err)
	}
	err := epicProjectCheck(4, 9)
	if err == nil || !strings.Contains(err.Error(), "different project") {
		t.Fatalf("cross-project link must be refused, got %v", err)
	}
}

// Moving an epic that still groups issues would strand them outside their
// project, so it is refused until the children are moved away.
func TestEpicMoveCheck(t *testing.T) {
	if err := epicMoveCheck(4, 9, 0); err != nil {
		t.Fatalf("moving an empty epic must be allowed: %v", err)
	}
	if err := epicMoveCheck(4, 4, 3); err != nil {
		t.Fatalf("saving an epic in its own project must be allowed: %v", err)
	}
	err := epicMoveCheck(4, 9, 3)
	if err == nil || !strings.Contains(err.Error(), "3 issue(s)") {
		t.Fatalf("moving an epic with issues must be refused, got %v", err)
	}
}

// The epic's child list must bind to epic_id. project.task also points at itself
// through parent_id, so the resolver would otherwise pick the parent link and the
// Issues page would silently show the wrong rows.
func TestEpicIssuesBindToEpicLink(t *testing.T) {
	if err := modelreg.ActivateAll(nil); err != nil {
		t.Fatalf("activate models: %v", err)
	}
	if got := orm.ResolveInverseOne2ManyField(epicModel, taskModel); got != "epic_id" {
		t.Fatalf("epic children resolve to %q, want epic_id", got)
	}
}
