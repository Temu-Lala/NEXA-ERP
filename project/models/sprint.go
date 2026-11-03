package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nexa/core/engine/swcmeta"
	"nexa/core/orm"
	"nexa/core/scheduler"
	"nexa/core/sdk"
)

// Sprint delivery layer (Scrum on top of the existing issue engine). A SPRINT is a
// timeboxed commitment over project issues. Planning is a board grouped by sprint:
// dragging a card between the Backlog column and a planned sprint writes sprint_id,
// which is the same mechanism the sales pipeline uses for its stages. STARTING a
// sprint freezes the points on the board as the commitment; CLOSING it returns
// unfinished issues to the backlog while finished issues stay attached, so velocity
// is computed from real closed rows instead of a typed number.
//
// Burndown is recorded forward: a daily cron writes one row per active sprint into
// project.sprint.snapshot. History is never backfilled — a chart drawn from today's
// remaining work would be fabricated data, not a record of the sprint.

const (
	sprintModel        = "project.sprint"
	snapModel          = "project.sprint.snapshot"
	sprintSnapshotCron = "project.sprint.snapshot"
)

// SprintState is the sprint lifecycle. Only one sprint per project may be active.
type SprintState string

const (
	SprintDraft  SprintState = "draft"
	SprintActive SprintState = "active"
	SprintClosed SprintState = "closed"
)

// ProjectSprint is a timeboxed commitment containing a set of issues.
//
// points_committed is written once, when the sprint starts, so the commitment cannot
// drift afterwards. points_completed is written once, when it closes. The live
// planned/done/issue counters are computed from the issues currently attached, which
// is what the board shows while the sprint is running.
type ProjectSprint struct {
	sdk.Model `nexa:"model=project.sprint"`

	Name            sdk.String                   `nexa:"required,size=80,string=Sprint"`
	ProjectID       sdk.Many2One[ProjectProject] `nexa:"required,index,string=Project"`
	State           sdk.Selection[SprintState]   `nexa:"string=Status,default=draft,selection=draft:Planned,active:Active,closed:Closed"`
	Goal            sdk.Text                     `nexa:"string=Sprint Goal"`
	DateStart       sdk.Date                     `nexa:"string=Start Date"`
	DateEnd         sdk.Date                     `nexa:"string=End Date"`
	Sequence        sdk.Integer                  `nexa:"string=Sequence,default=10"`
	Capacity        sdk.Integer                  `nexa:"string=Capacity (points),default=0"`
	PointsCommitted sdk.Integer                  `nexa:"string=Committed Points,default=0"`
	PointsCompleted sdk.Integer                  `nexa:"string=Completed Points,default=0"`
	ClosedOn        sdk.Date                     `nexa:"string=Closed On"`
	Note            sdk.Text                     `nexa:"string=Notes"`

	Tasks     sdk.One2Many[ProjectTask]           `nexa:"string=Issues"`
	Snapshots sdk.One2Many[ProjectSprintSnapshot] `nexa:"string=Daily Snapshot"`

	PointsPlanned sdk.Integer `nexa:"string=Planned Points,compute=computeSprintPlannedPoints"`
	PointsDone    sdk.Integer `nexa:"string=Done Points,compute=computeSprintDonePoints"`
	IssueCount    sdk.Integer `nexa:"string=Issues,compute=computeSprintIssueCount"`
	OpenCount     sdk.Integer `nexa:"string=Open Issues,compute=computeSprintOpenCount"`
}

// ProjectSprintSnapshot is one day of a running sprint, written by the daily cron.
// remaining_points is the burn line: scope of the sprint minus what is finished.
type ProjectSprintSnapshot struct {
	sdk.Model `nexa:"model=project.sprint.snapshot"`

	SprintID        sdk.Many2One[ProjectSprint] `nexa:"required,index,string=Sprint,ondelete=cascade"`
	Date            sdk.Date                    `nexa:"required,index,string=Day"`
	TotalPoints     sdk.Integer                 `nexa:"string=Scope Points,default=0"`
	CompletedPoints sdk.Integer                 `nexa:"string=Completed Points,default=0"`
	RemainingPoints sdk.Integer                 `nexa:"string=Remaining Points,default=0"`
	OpenIssues      sdk.Integer                 `nexa:"string=Open Issues,default=0"`
}

func init() {
	orm.RegisterCompute(sprintModel, "points_planned", []string{"id"}, computeSprintPlannedPoints)
	orm.RegisterCompute(sprintModel, "points_done", []string{"id"}, computeSprintDonePoints)
	orm.RegisterCompute(sprintModel, "issue_count", []string{"id"}, computeSprintIssueCount)
	orm.RegisterCompute(sprintModel, "open_count", []string{"id"}, computeSprintOpenCount)

	orm.RegisterObjectAction(sprintModel, "action_start", sprintStart)
	orm.RegisterObjectAction(sprintModel, "action_close", sprintClose)

	orm.RegisterConstraint(taskModel, "sprint_open", constraintTaskSprintOpen)
	orm.RegisterConstraint(sprintModel, "sprint_frozen", constraintSprintFrozen)

	scheduler.RegisterCronHandler(sprintSnapshotCron, runSprintSnapshot)

	// Planning board: columns are the project's planned and active sprints, with the
	// backlog (no sprint) always leftmost. The default many2one expander would list
	// every closed sprint forever, so this order is built explicitly.
	swcmeta.RegisterKanbanGroupExpander(taskModel, "sprint_id", sprintKanbanColumns)
}

// ---------------------------------------------------------------------------
// Live counters (computed from attached issues)
// ---------------------------------------------------------------------------

// scopeTally is the point/issue breakdown of a set of issues (a sprint or an
// epic). Cancelled issues are out of scope entirely: they neither count toward
// the burn line nor toward velocity.
type scopeTally struct {
	Issues int // issues in scope
	Open   int // issues in scope that are not finished
	Total  int // points in scope
	Done   int // points finished
}

func tallySprint(ctx context.Context, sprintID int) scopeTally {
	if sprintID <= 0 {
		return scopeTally{}
	}
	rows, err := orm.SearchPage(ctx, taskModel, [][]interface{}{{"sprint_id", "=", sprintID}}, 100000, 0, "id asc")
	if err != nil {
		return scopeTally{}
	}
	return tallyRows(rows)
}

// tallyRows is the pure breakdown behind the sprint counters: cancelled issues leave
// scope entirely, finished issues burn points, everything else stays open.
func tallyRows(rows []map[string]interface{}) scopeTally {
	var t scopeTally
	for _, r := range rows {
		if TaskState(str(r["state"])) == TaskStateCancelled {
			continue
		}
		points := intID(r["story_points"])
		t.Issues++
		t.Total += points
		if TaskState(str(r["state"])) == TaskStateDone {
			t.Done += points
			continue
		}
		t.Open++
	}
	return t
}

func computeSprintPlannedPoints(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return tallySprint(ctx, intID(rec["id"])).Total, nil
}

func computeSprintDonePoints(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return tallySprint(ctx, intID(rec["id"])).Done, nil
}

func computeSprintIssueCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return tallySprint(ctx, intID(rec["id"])).Issues, nil
}

func computeSprintOpenCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return tallySprint(ctx, intID(rec["id"])).Open, nil
}

// sprintStartPlan validates a sprint start and returns the normalized start date.
// A team runs one sprint per project at a time, and a sprint needs a real timebox.
func sprintStartPlan(state SprintState, otherActive int, start, end string, today time.Time) (string, error) {
	if state != SprintDraft {
		return "", fmt.Errorf("only planned sprints can be started")
	}
	if otherActive > 0 {
		return "", fmt.Errorf("this project already has an active sprint; close it before starting another")
	}
	start = strings.TrimSpace(start)
	if start == "" {
		start = today.Format(dateLayout)
	}
	end = strings.TrimSpace(end)
	if end == "" {
		return "", fmt.Errorf("set an end date before starting the sprint")
	}
	st, okStart := parseDate(start)
	et, okEnd := parseDate(end)
	if !okStart || !okEnd {
		return "", fmt.Errorf("start and end dates must be valid dates")
	}
	if !et.After(st) {
		return "", fmt.Errorf("the sprint end date must be after its start date")
	}
	return start, nil
}

// sprintCloseCheck guards the only legal source state for a close.
func sprintCloseCheck(state SprintState) error {
	if state != SprintActive {
		return fmt.Errorf("only an active sprint can be closed")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Lifecycle
// ---------------------------------------------------------------------------

// sprintStart opens a planned sprint. It refuses a second active sprint on the same
// project (a team runs one sprint at a time) and requires a real date range, then
// freezes the current scope as the committed points and records day zero.
func sprintStart(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, sprintModel, id)
	if rec == nil {
		return "", fmt.Errorf("sprint not found")
	}
	if SprintState(str(rec["state"])) != SprintDraft {
		return "", fmt.Errorf("only planned sprints can be started")
	}
	otherActive := 0
	if pid := intID(rec["project_id"]); pid > 0 {
		otherActive, _ = orm.SearchCount(bypass, sprintModel, [][]interface{}{
			{"project_id", "=", pid},
			{"state", "=", string(SprintActive)},
			{"id", "!=", id},
		})
	}
	start, err := sprintStartPlan(SprintState(str(rec["state"])), otherActive, dateValue(rec["date_start"]), dateValue(rec["date_end"]), time.Now())
	if err != nil {
		return "", err
	}

	t := tallySprint(bypass, id)
	upd := map[string]interface{}{
		"state":            string(SprintActive),
		"date_start":       start,
		"points_committed": t.Total,
	}
	if err := orm.UpdateRecordByID(bypass, sprintModel, id, upd); err != nil {
		return "", err
	}
	writeSprintSnapshotValues(ctx, id, time.Now(), t)
	return fmt.Sprintf("Sprint started with %d committed point(s) across %d issue(s).", t.Total, t.Issues), nil
}

// sprintClose finishes a running sprint. Finished issues keep their sprint so the
// sprint remains a truthful record (and velocity stays computable); unfinished issues
// are returned to the backlog so no work can disappear from planning.
func sprintClose(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, sprintModel, id)
	if rec == nil {
		return "", fmt.Errorf("sprint not found")
	}
	if err := sprintCloseCheck(SprintState(str(rec["state"]))); err != nil {
		return "", err
	}

	t := tallySprint(bypass, id)

	open, err := orm.SearchPage(bypass, taskModel, [][]interface{}{
		{"sprint_id", "=", id},
		{"state", "not in", []interface{}{string(TaskStateDone), string(TaskStateCancelled)}},
	}, 100000, 0, "id asc")
	if err != nil {
		return "", err
	}
	for _, row := range open {
		if err := orm.UpdateRecordByID(bypass, taskModel, intID(row["id"]), map[string]interface{}{"sprint_id": nil}); err != nil {
			return "", err
		}
	}

	upd := map[string]interface{}{
		"state":            string(SprintClosed),
		"closed_on":        time.Now().Format(dateLayout),
		"points_completed": t.Done,
	}
	if err := orm.UpdateRecordByID(bypass, sprintModel, id, upd); err != nil {
		return "", err
	}
	// Final snapshot uses the pre-clear tally: it is the last honest burn position of
	// the sprint, including the work that is now going back to the backlog.
	writeSprintSnapshotValues(ctx, id, time.Now(), t)

	return fmt.Sprintf("Sprint closed with %d of %d point(s) completed; %d unfinished issue(s) returned to the backlog.",
		t.Done, t.Total, len(open)), nil
}

// ---------------------------------------------------------------------------
// Burndown snapshots (recorded forward, never backfilled)
// ---------------------------------------------------------------------------

func runSprintSnapshot(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	sprints, err := orm.SearchPage(bypass, sprintModel, [][]interface{}{
		{"state", "=", string(SprintActive)},
	}, 5000, 0, "id asc")
	if err != nil {
		return err
	}
	day := time.Now()
	for _, sp := range sprints {
		writeSprintSnapshot(bypass, intID(sp["id"]), day)
	}
	return nil
}

func writeSprintSnapshot(ctx context.Context, sprintID int, day time.Time) {
	if sprintID <= 0 {
		return
	}
	writeSprintSnapshotValues(ctx, sprintID, day, tallySprint(ctx, sprintID))
}

// writeSprintSnapshotValues upserts the (sprint, day) row so re-running the cron or a
// manual recompute on the same day corrects the value instead of duplicating the day.
func writeSprintSnapshotValues(ctx context.Context, sprintID int, day time.Time, t scopeTally) {
	if sprintID <= 0 {
		return
	}
	date := day.Format(dateLayout)
	vals := snapshotValues(sprintID, day, t)
	existing, err := orm.SearchPage(ctx, snapModel, [][]interface{}{
		{"sprint_id", "=", sprintID},
		{"date", "=", date},
	}, 1, 0, "id asc")
	if err == nil && len(existing) > 0 {
		_ = orm.UpdateRecordByID(ctx, snapModel, intID(existing[0]["id"]), vals)
		return
	}
	inst, ok := orm.Registry[snapModel]
	if !ok {
		return
	}
	_, _ = orm.Create(ctx, inst, vals)
}

// snapshotValues is the burn row for one day: scope, what burned off it, and what is
// still open. remaining_points is derived, never typed.
func snapshotValues(sprintID int, day time.Time, t scopeTally) map[string]interface{} {
	return map[string]interface{}{
		"sprint_id":        sprintID,
		"date":             day.Format(dateLayout),
		"total_points":     t.Total,
		"completed_points": t.Done,
		"remaining_points": t.Total - t.Done,
		"open_issues":      t.Open,
	}
}

// ---------------------------------------------------------------------------
// Planning board
// ---------------------------------------------------------------------------

// sprintKanbanColumns builds the sprint planning board: the backlog first, then every
// planned and active sprint. Issues still pointing at a closed sprint are history and
// stay off the board (the close action already returned every unfinished issue).
func sprintKanbanColumns(ctx context.Context, _ string, groupField string, rows []map[string]interface{}) ([]swcmeta.KanbanColumn, error) {
	sprints, err := orm.SearchPage(ctx, sprintModel, [][]interface{}{
		{"state", "in", []interface{}{string(SprintDraft), string(SprintActive)}},
	}, 500, 0, "sequence asc, id asc")
	if err != nil {
		return nil, err
	}
	codes := projectCodeIndex(ctx, sprints)

	cols := make([]swcmeta.KanbanColumn, 0, len(sprints)+1)
	index := make(map[int64]int, len(sprints))
	for _, sp := range sprints {
		id, ok := orm.CoerceInt64(sp["id"])
		if !ok || id <= 0 {
			continue
		}
		label := strings.TrimSpace(str(sp["name"]))
		if code := codes[intID(sp["project_id"])]; code != "" {
			label = code + " · " + label
		}
		if SprintState(str(sp["state"])) == SprintActive {
			label += " (active)"
		}
		cols = append(cols, swcmeta.KanbanColumn{
			Value:    id,
			Label:    label,
			Sequence: intID(sp["sequence"]),
			Records:  []map[string]interface{}{},
		})
		index[id] = len(cols) - 1
	}

	backlog := []map[string]interface{}{}
	for _, row := range rows {
		sid, _ := orm.CoerceInt64(row[groupField])
		if sid <= 0 {
			backlog = append(backlog, row)
			continue
		}
		if ix, ok := index[sid]; ok {
			cols[ix].Records = append(cols[ix].Records, row)
		}
	}

	head := swcmeta.KanbanColumn{Value: 0, Label: "Backlog", Sequence: -1, Records: backlog}
	return append([]swcmeta.KanbanColumn{head}, cols...), nil
}

// projectCodeIndex loads the short reference of each sprint's project so a
// cross-project board labels its columns unambiguously.
func projectCodeIndex(ctx context.Context, sprints []map[string]interface{}) map[int]string {
	out := map[int]string{}
	ids := make([]interface{}, 0, len(sprints))
	seen := map[int]bool{}
	for _, sp := range sprints {
		pid := intID(sp["project_id"])
		if pid <= 0 || seen[pid] {
			continue
		}
		seen[pid] = true
		ids = append(ids, pid)
	}
	if len(ids) == 0 {
		return out
	}
	rows, err := orm.Search(ctx, projModel, [][]interface{}{{"id", "in", ids}})
	if err != nil {
		return out
	}
	for _, p := range rows {
		pid := intID(p["id"])
		code := strings.TrimSpace(str(p["code"]))
		if code == "" {
			code = strings.TrimSpace(str(p["name"]))
		}
		if code != "" {
			out[pid] = code
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Invariants
// ---------------------------------------------------------------------------

// constraintTaskSprintOpen keeps open work out of a closed sprint. A closed sprint is
// a historical record and its velocity must not change afterwards, so an issue can
// only leave it (by clearing the sprint) — never enter it.
func constraintTaskSprintOpen(ctx context.Context, rec map[string]interface{}) error {
	sid := intID(rec["sprint_id"])
	if sid <= 0 {
		return nil
	}
	sp := loadRec(orm.ContextWithBypass(ctx, true), sprintModel, sid)
	if sp == nil {
		return nil
	}
	return taskSprintCheck(SprintState(str(sp["state"])), TaskState(str(rec["state"])))
}

// taskSprintCheck is the pure rule behind the constraint: a closed sprint is a
// historical record, so only finished work may sit in it.
func taskSprintCheck(sprintState SprintState, taskState TaskState) error {
	if sprintState != SprintClosed {
		return nil
	}
	switch taskState {
	case TaskStateDone, TaskStateCancelled:
		return nil
	}
	return fmt.Errorf("this issue belongs to a closed sprint; clear the Sprint field to move it back to the backlog")
}

// constraintSprintFrozen keeps a closed sprint's recorded outcome immutable: state,
// committed points and completed points are written only by the start/close actions.
func constraintSprintFrozen(ctx context.Context, rec map[string]interface{}) error {
	id := intID(rec["id"])
	if id <= 0 {
		return nil
	}
	stored := loadRec(orm.ContextWithBypass(ctx, true), sprintModel, id)
	if stored == nil {
		return nil
	}
	return sprintFrozenCheck(
		SprintState(str(stored["state"])), SprintState(str(rec["state"])),
		intID(stored["points_committed"]), intID(rec["points_committed"]),
		intID(stored["points_completed"]), intID(rec["points_completed"]),
	)
}

// sprintFrozenCheck is the pure rule behind the frozen-outcome constraint: once a
// sprint is closed, its state and recorded points are the velocity record.
func sprintFrozenCheck(stored, next SprintState, committedStored, committedNext, completedStored, completedNext int) error {
	if stored != SprintClosed {
		return nil
	}
	if next != SprintClosed {
		return fmt.Errorf("a closed sprint cannot be reopened; plan a new sprint instead")
	}
	if committedStored != committedNext || completedStored != completedNext {
		return fmt.Errorf("a closed sprint's committed and completed points are frozen")
	}
	return nil
}

// nextBacklogRank appends a new issue to the bottom of the project's backlog. The
// backlog view orders by sequence, so a fresh issue must sort after existing work
// instead of interleaving with it.
func nextBacklogRank(ctx context.Context, projectID int) int {
	if projectID <= 0 {
		return 10
	}
	rows, err := orm.SearchPage(ctx, taskModel, [][]interface{}{{"project_id", "=", projectID}}, 1, 0, "sequence desc")
	if err != nil || len(rows) == 0 {
		return 10
	}
	rank := intID(rows[0]["sequence"])
	if rank < 10 {
		rank = 10
	}
	return rank + 10
}
