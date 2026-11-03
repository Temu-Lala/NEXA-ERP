package models

import (
	"context"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Collaboration engine (blueprint §14). Meetings and documents are driven by their
// lifecycle actions; the only automated signal is action-item overdue status, kept
// as a derived flag refreshed daily so it can be filtered and counted without
// recomputing on every read. A completed or cancelled item is never flagged.

const (
	meetingModel  = "project.meeting"
	attendeeModel = "project.meeting.attendee"
	actionModel   = "project.action.item"
	docModel      = "project.document"

	overdueCron = "project.collab.overdue"
	dateLayout  = "2006-01-02"
)

func init() {
	scheduler.RegisterCronHandler(overdueCron, runOverdueRecompute)
}

// runOverdueRecompute marks open/in-progress action items whose due date has
// passed as overdue, and clears the flag on items that are no longer due-past.
func runOverdueRecompute(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	today := time.Now().Format(dateLayout)
	rows, err := orm.SearchPage(bypass, actionModel, [][]interface{}{
		{"state", "in", []interface{}{"open", "in_progress"}},
	}, 20000, 0, "id asc")
	if err != nil {
		return err
	}
	for _, r := range rows {
		due := s(r["due_date"])
		want := due != "" && due < today
		if boolOf(r["overdue"]) != want {
			_ = orm.UpdateRecordByID(bypass, actionModel, idOf(r["id"]), map[string]interface{}{"overdue": want})
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helpers (package-local)
// ---------------------------------------------------------------------------

func load(ctx context.Context, model string, id int) map[string]interface{} {
	if id <= 0 {
		return nil
	}
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return nil
	}
	return rec
}

func idOf(v interface{}) int {
	n, _ := orm.CoerceInt64(v)
	return int(n)
}

func s(v interface{}) string { return orm.AsString(v) }

func boolOf(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func flt(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	default:
		n, _ := orm.CoerceInt64(v)
		return float64(n)
	}
}
