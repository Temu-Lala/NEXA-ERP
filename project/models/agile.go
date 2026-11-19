package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/event"
	"nexa/core/orm"
	"nexa/core/sdk"
)

// Agile board layer (Jira-class delivery, ERP-native). A governed BOARD STAGE is a
// kanban column that DRIVES the task's own lifecycle state: dragging a card between
// columns is the CRM-pipeline mechanism (kanban default_group_by on a Many2One, drop
// writes stage_id via rpc), and a server-side reconciler maps the column to the
// project task state machine so the board and the state machine can never diverge.
// An ISSUE TYPE classifies work (story/task/bug/improvement/epic). Each issue carries
// a stable human KEY (PROJECT-12) allocated on create from a per-project counter, a
// story-point estimate, and a RESOLUTION once it leaves the board. Everything here is
// additive to the existing project.task (nullable columns) so the eight companion
// project modules are unaffected. Story points and stage counts are computed from real
// rows; no value is fabricated.

const (
	stageModel     = "project.board.stage"
	issueTypeModel = "project.issue.type"
)

// IssueKind is the category of work an issue type represents.
type IssueKind string

const (
	KindStory       IssueKind = "story"
	KindTask        IssueKind = "task"
	KindBug         IssueKind = "bug"
	KindImprovement IssueKind = "improvement"
	KindSubtask     IssueKind = "subtask"
	KindEpic        IssueKind = "epic"
)

// Resolution is how a finished issue came to rest. None means still open.
type Resolution string

const (
	ResNone            Resolution = "none"
	ResDone            Resolution = "done"
	ResWontFix         Resolution = "wont_fix"
	ResDuplicate       Resolution = "duplicate"
	ResCannotReproduce Resolution = "cannot_reproduce"
	ResCancelled       Resolution = "cancelled"
)

// ProjectBoardStage is one column of the agile board. ToState maps the column onto
// the task lifecycle state the reconciler applies when a card is dropped here; Done
// marks the column as a finishing column (which zeroes remaining hours and, when a
// resolution is not yet set, resolves the issue as Done). Fold hides an empty column.
// A column may be capped: WipLimit is the most cards it may hold and WipPointsLimit
// the most story points it may hold, both counting work still in flow. The cap is
// enforced on write, so a column physically cannot be over-filled (see wip.go).
type ProjectBoardStage struct {
	sdk.Model `nexa:"model=project.board.stage"`
	CompanyID   sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name           sdk.String               `nexa:"required,size=80,index,string=Stage"`
	Sequence       sdk.Integer              `nexa:"string=Sequence,default=10"`
	ToState        sdk.Selection[TaskState] `nexa:"string=Sets Status,selection=backlog:Backlog,ready:Ready,todo:To Do,in_progress:In Progress,in_review:In Review,blocked:Blocked,done:Done,cancelled:Cancelled"`
	Done           sdk.Boolean              `nexa:"string=Finishing Column,default=false"`
	Fold           sdk.Boolean              `nexa:"string=Folded when Empty,default=false"`
	Color          sdk.Integer              `nexa:"string=Color Index,default=0"`
	WipLimit       sdk.Integer              `nexa:"string=WIP Limit (cards),default=0"`
	WipPointsLimit sdk.Numeric              `nexa:"string=WIP Limit (points),precision=18,scale=2,default=0"`
	CountDoneInWip sdk.Boolean              `nexa:"string=Done Cards Count Toward Limit,default=false"`
	Active         sdk.Boolean              `nexa:"string=Active,default=true"`
	Note           sdk.Text                 `nexa:"string=Description"`

	TaskCount  sdk.Integer `nexa:"string=Issues,compute=computeStageTaskCount"`
	WipUsage   sdk.Integer `nexa:"string=In Flow,compute=computeStageWipUsage"`
	PointsUsed sdk.Numeric `nexa:"string=Points in Flow,compute=computeStagePointsUsed"`
}

// ProjectIssueType classifies a board issue (story, bug, task, …) and can be made
// the default for newly created issues.
type ProjectIssueType struct {
	sdk.Model `nexa:"model=project.issue.type"`
	CompanyID   sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	Name      sdk.String               `nexa:"required,size=60,string=Name"`
	Code      sdk.String               `nexa:"size=10,index,string=Symbol"`
	Kind      sdk.Selection[IssueKind] `nexa:"string=Kind,default=task,index,selection=story:Story,task:Task,bug:Bug,improvement:Improvement,subtask:Sub-task,epic:Epic"`
	Color     sdk.Integer              `nexa:"string=Color Index,default=0"`
	IsDefault sdk.Boolean              `nexa:"string=Default Type,default=false"`
	Active    sdk.Boolean              `nexa:"string=Active,default=true"`
	Note      sdk.Text                 `nexa:"string=Description"`
}

func init() {
	orm.RegisterCompute(stageModel, "task_count", []string{"id"}, computeStageTaskCount)
	event.Subscribe(orm.EventRecordCreated, onIssueCreated)
	event.Subscribe(orm.EventRecordUpdated, onIssueUpdated)
}

// computeStageTaskCount counts the issues currently sitting in a board column.
func computeStageTaskCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	id := intID(rec["id"])
	if id <= 0 {
		return 0, nil
	}
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), taskModel, [][]interface{}{
		{"stage_id", "=", id},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

// onIssueCreated provisions a fresh task for the board: a stable PROJECT-n key, the
// first active stage when none was chosen, and the default issue type. Runs off the
// create event so the values are truthfully allocated server-side, not typed.
func onIssueCreated(ctx context.Context, ev event.Event) error {
	if str(ev.Payload["model"]) != taskModel {
		return nil
	}
	id := intID(ev.Payload["id"])
	if id <= 0 {
		return nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, taskModel, id)
	if rec == nil {
		return nil
	}
	patch := map[string]interface{}{}
	if strings.TrimSpace(str(rec["key"])) == "" {
		patch["key"] = allocateIssueKey(bypass, rec)
	}
	if intID(rec["reporter_id"]) == 0 {
		if uid := orm.UIDFromContext(ctx); uid > 0 {
			patch["reporter_id"] = uid
		}
	}
	if intID(rec["stage_id"]) == 0 {
		if sid := firstStageID(bypass); sid > 0 {
			patch["stage_id"] = sid
		}
	}
	if intID(rec["issue_type_id"]) == 0 {
		if tid := defaultIssueTypeID(bypass); tid > 0 {
			patch["issue_type_id"] = tid
		}
	}
	// New issues enter at the bottom of the project's backlog: the backlog view orders
	// by sequence (rank), so a fresh issue must sort after existing work rather than
	// interleave with it.
	if pid := intID(rec["project_id"]); pid > 0 && intID(rec["sequence"]) <= 10 {
		if rank := nextBacklogRank(bypass, pid); rank > 0 {
			patch["sequence"] = rank
		}
	}
	if len(patch) > 0 {
		_ = orm.UpdateRecordByID(bypass, taskModel, id, patch)
	}
	return nil
}

// onIssueUpdated reconciles the board column with the task state machine. Dropping a
// card onto a stage applies that stage's mapped state; a finishing column resolves the
// issue and zeroes remaining hours. The write is guarded to only fire on a real
// difference, so the follow-up updated event settles without looping.
func onIssueUpdated(ctx context.Context, ev event.Event) error {
	if str(ev.Payload["model"]) != taskModel {
		return nil
	}
	id := intID(ev.Payload["id"])
	if id <= 0 {
		return nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, taskModel, id)
	if rec == nil {
		return nil
	}
	patch := map[string]interface{}{}

	// Backfill key/stage/type for rows created outside the board path.
	if strings.TrimSpace(str(rec["key"])) == "" {
		patch["key"] = allocateIssueKey(bypass, rec)
	}

	if sid := intID(rec["stage_id"]); sid > 0 {
		stage := loadRec(bypass, stageModel, sid)
		if stage != nil {
			want := TaskState(str(stage["to_state"]))
			// Workflow guard: a finishing column is refused while the issue still
			// has open subtasks or unchecked checklist items — Jira's "definition
			// of done" enforcement. The card stays in its previous state.
			if boolv(stage["done"]) && !doneBlockersClear(bypass, id) {
				want = ""
			}
			if want != "" && TaskState(str(rec["state"])) != want {
				patch["state"] = string(want)
			}
			if boolv(stage["done"]) && want != "" {
				if Resolution(str(rec["resolution"])) == "" || Resolution(str(rec["resolution"])) == ResNone {
					patch["resolution"] = string(ResDone)
					patch["resolved_date"] = time.Now().Format("2006-01-02")
				}
				patch["remaining_hours"] = 0
			} else {
				if Resolution(str(rec["resolution"])) == ResDone {
					patch["resolution"] = string(ResNone)
					patch["resolved_date"] = false
				}
			}
		}
	}
	if len(patch) > 0 {
		prev := TaskState(str(rec["state"]))
		_ = orm.UpdateRecordByID(bypass, taskModel, id, patch)
		if st, ok := patch["state"]; ok {
			writeTaskHistory(ctx, rec, prev, TaskState(str(st)))
		}
	}
	return nil
}

// doneBlockersClear reports whether an issue may enter a done stage: it has no
// open subtasks and no unchecked checklist items — the definition-of-done gate.
func doneBlockersClear(ctx context.Context, taskID int) bool {
	if taskID <= 0 {
		return true
	}
	open, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), taskModel,
		[][]interface{}{
			{"parent_id", "=", taskID},
			{"state", "not in", []interface{}{string(TaskStateDone), string(TaskStateCancelled)}},
		})
	if err != nil || open > 0 {
		return false
	}
	open, err = orm.SearchCount(orm.ContextWithBypass(ctx, true), checkItemModel,
		[][]interface{}{{"task_id", "=", taskID}, {"done", "=", false}})
	if err != nil || open > 0 {
		return false
	}
	// Open "blocks" links targeting this issue also gate Done — you cannot
	// finish work that an unfinished blocker still holds.
	links, err := orm.SearchPage(ctx, linkModel, [][]interface{}{
		{"linked_task_id", "=", taskID},
		{"link_type", "=", "blocks"},
	}, 200, 0, "id asc")
	if err != nil {
		return false
	}
	for _, l := range links {
		bid := intID(l["task_id"])
		if bid <= 0 {
			continue
		}
		blk := loadRec(ctx, taskModel, bid)
		if blk == nil {
			continue
		}
		st := TaskState(str(blk["state"]))
		if st != TaskStateDone && st != TaskStateCancelled {
			return false
		}
	}
	return true
}

// allocateIssueKey builds PROJECT-n using the project's short reference and the next
// free ordinal among that project's existing keys. Falls back to a global TSK counter.
func allocateIssueKey(ctx context.Context, rec map[string]interface{}) string {
	pid := intID(rec["project_id"])
	prefix := "TSK"
	if pid > 0 {
		proj := loadRec(ctx, projModel, pid)
		if proj != nil {
			if c := sanitizeKeyPrefix(str(proj["code"])); c != "" {
				prefix = c
			} else if n := sanitizeKeyPrefix(str(proj["name"])); n != "" {
				prefix = n
			}
		}
	}
	rows, err := orm.SearchPage(ctx, taskModel, [][]interface{}{{"project_id", "=", pid}}, 100000, 0, "id asc")
	if err != nil {
		rows = nil
	}
	maxN := 0
	for _, r := range rows {
		if n := trailingNumber(str(r["key"])); n > maxN {
			maxN = n
		}
	}
	return fmt.Sprintf("%s-%d", prefix, maxN+1)
}

// firstStageID returns the lowest-sequence active stage, the column new issues enter.
func firstStageID(ctx context.Context) int {
	rows, err := orm.SearchPage(ctx, stageModel, [][]interface{}{{"active", "=", true}}, 1, 0, "sequence asc, id asc")
	if err != nil || len(rows) == 0 {
		return 0
	}
	return intID(rows[0]["id"])
}

// defaultIssueTypeID returns the flagged default issue type, or the lowest active one.
func defaultIssueTypeID(ctx context.Context) int {
	rows, err := orm.SearchPage(ctx, issueTypeModel, [][]interface{}{{"active", "=", true}}, 500, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return 0
	}
	for _, r := range rows {
		if boolv(r["is_default"]) {
			return intID(r["id"])
		}
	}
	return intID(rows[0]["id"])
}

func sanitizeKeyPrefix(s string) string {
	var b strings.Builder
	for _, r := range strings.ToUpper(s) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if len(out) > 8 {
		out = out[:8]
	}
	return out
}

func trailingNumber(s string) int {
	i := strings.LastIndexAny(s, "-_ ")
	if i < 0 {
		return 0
	}
	digits := strings.TrimLeft(s[i:], "-_ ")
	if digits == "" {
		return 0
	}
	n, err := strconv.Atoi(digits)
	if err != nil {
		return 0
	}
	return n
}

func boolv(v interface{}) bool {
	b, _ := v.(bool)
	return b
}
