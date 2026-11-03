package models

import (
	"context"
	"fmt"
	"math"

	"nexa/core/orm"
	"nexa/core/sdk"
)

// Time tracking. A worklog row is the record of hours one person spent on an
// issue. The issue's spent and remaining hours are computed from those rows on
// read, so the effort panel always agrees with the work actually logged — the
// roll-up is never a number somebody typed that can quietly drift from the truth.
// This is the same principle as the epic roll-up: nothing cached, nothing hand-
// entered, so a child change is reflected the moment the parent is read again.
//
// Only time that exists is counted. A logged entry can never be negative, and the
// remaining figure bottoms out at zero rather than going under: an issue can be
// over-spent, but "negative hours left" is a fiction the system refuses to state.

const worklogModel = "project.worklog"

// ProjectWorklog is one logged unit of work on an issue.
type ProjectWorklog struct {
	sdk.Model `nexa:"model=project.worklog"`

	TaskID  sdk.Many2One[ProjectTask] `nexa:"required,index,string=Task"`
	UserID  sdk.Many2One[CoreUser]    `nexa:"required,index,string=User"`
	Date    sdk.Date                  `nexa:"required,index,string=Date"`
	Hours   sdk.Numeric               `nexa:"required,string=Time Spent,precision=18,scale=2,default=0"`
	Summary sdk.String                `nexa:"size=200,string=Description"`
}

func init() {
	orm.RegisterCompute(taskModel, "actual_hours", []string{"id"}, computeTaskSpentHours)
	orm.RegisterCompute(taskModel, "remaining_hours", []string{"id", "est_hours"}, computeTaskRemainingHours)
	orm.RegisterConstraint(worklogModel, "worklog_hours_positive", constraintWorklogHours)
}

// sumWorklogHours totals the logged time on one issue. It reads the caller's own
// view of the worklogs: someone who cannot see an issue's time entries sees zero,
// not a wrong positive.
func sumWorklogHours(ctx context.Context, taskID int) float64 {
	if taskID <= 0 {
		return 0
	}
	rows, err := orm.Search(ctx, worklogModel, [][]interface{}{{"task_id", "=", taskID}})
	if err != nil {
		return 0
	}
	var total float64
	for _, r := range rows {
		if h, ok := orm.CoerceFloat64(r["hours"]); ok && h > 0 {
			total += h
		}
	}
	return total
}

func computeTaskSpentHours(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return round2(sumWorklogHours(ctx, intID(rec["id"]))), nil
}

// computeTaskRemainingHours subtracts logged time from the estimate and never lets
// the result fall below zero: an over-spent issue reads as 0 remaining, honestly
// reporting there is nothing left rather than inventing negative time.
func computeTaskRemainingHours(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	est, _ := orm.CoerceFloat64(rec["est_hours"])
	remaining := est - sumWorklogHours(ctx, intID(rec["id"]))
	if remaining < 0 {
		remaining = 0
	}
	return round2(remaining), nil
}

// round2 holds the roll-up at the two-decimal scale hours are entered on.
func round2(v float64) float64 {
	return math.Round(v*100) / 100
}

// constraintWorklogHours refuses a negative or non-numeric time entry. Logged time
// is a fact about work done; a negative would subtract effort that was spent, so it
// is prevented at the write rather than corrected after the fact.
func constraintWorklogHours(_ context.Context, rec map[string]interface{}) error {
	h, ok := orm.CoerceFloat64(rec["hours"])
	if !ok {
		return fmt.Errorf("time spent must be a number")
	}
	if h < 0 {
		return fmt.Errorf("logged time cannot be negative")
	}
	return nil
}
