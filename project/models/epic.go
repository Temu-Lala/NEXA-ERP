package models

import (
	"context"
	"fmt"

	"nexa/core/orm"
	"nexa/core/sdk"
)

// Epic hierarchy (Jira-class containers over the issue engine). An EPIC groups
// issues that outlive a single sprint; issues point at one epic through
// project.task.epic_id, so every roll-up number (issues, open issues, points,
// progress) is computed from the real child rows on read. Nothing here is typed
// by hand and nothing is cached, so the epic panel can never disagree with the
// issues it contains.
//
// Epics are project-scoped on purpose: a cross-project link would let one epic's
// progress silently mix two projects' work, so the link is refused instead of
// being displayed and then explained.

const epicModel = "project.epic"

// EpicState is the epic lifecycle. It is declared by the epic owner, not derived:
// an epic is done when the owner says the outcome shipped, even if a cancelled
// issue is still sitting inside it.
type EpicState string

const (
	EpicOpen       EpicState = "open"
	EpicInProgress EpicState = "in_progress"
	EpicDone       EpicState = "done"
)

// ProjectEpic is a body of work grouping issues across sprints.
type ProjectEpic struct {
	sdk.Model `nexa:"model=project.epic"`

	Name        sdk.String                   `nexa:"required,size=120,index,string=Epic"`
	ProjectID   sdk.Many2One[ProjectProject] `nexa:"required,index,string=Project"`
	OwnerID     sdk.Many2One[CoreUser]       `nexa:"string=Epic Owner"`
	State       sdk.Selection[EpicState]     `nexa:"string=Status,default=open,selection=open:Open,in_progress:In Progress,done:Done"`
	Color       sdk.Integer                  `nexa:"string=Color Index,default=0"`
	Sequence    sdk.Integer                  `nexa:"string=Sequence,default=10"`
	StartDate   sdk.Date                     `nexa:"string=Start Date"`
	TargetDate  sdk.Date                     `nexa:"string=Target Date"`
	Description sdk.Text                     `nexa:"string=Description"`
	Issues      sdk.One2Many[ProjectTask]    `nexa:"string=Issues"`

	IssueCount  sdk.Integer `nexa:"string=Issues,compute=computeEpicIssueCount"`
	OpenCount   sdk.Integer `nexa:"string=Open Issues,compute=computeEpicOpenCount"`
	PointsTotal sdk.Integer `nexa:"string=Total Points,compute=computeEpicPointsTotal"`
	PointsDone  sdk.Integer `nexa:"string=Done Points,compute=computeEpicPointsDone"`
	Progress    sdk.Integer `nexa:"string=Progress %,compute=computeEpicProgress"`
}

func init() {
	orm.RegisterCompute(epicModel, "issue_count", []string{"id"}, computeEpicIssueCount)
	orm.RegisterCompute(epicModel, "open_count", []string{"id"}, computeEpicOpenCount)
	orm.RegisterCompute(epicModel, "points_total", []string{"id"}, computeEpicPointsTotal)
	orm.RegisterCompute(epicModel, "points_done", []string{"id"}, computeEpicPointsDone)
	orm.RegisterCompute(epicModel, "progress", []string{"id"}, computeEpicProgress)

	orm.RegisterConstraint(taskModel, "epic_link", constraintTaskEpic)
	orm.RegisterConstraint(epicModel, "epic_project", constraintEpicProject)
}

// ---------------------------------------------------------------------------
// Roll-up (computed from the issues actually attached)
// ---------------------------------------------------------------------------

func tallyEpic(ctx context.Context, epicID int) scopeTally {
	if epicID <= 0 {
		return scopeTally{}
	}
	rows, err := orm.SearchPage(ctx, taskModel, [][]interface{}{{"epic_id", "=", epicID}}, 100000, 0, "id asc")
	if err != nil {
		return scopeTally{}
	}
	return tallyRows(rows)
}

func computeEpicIssueCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return tallyEpic(ctx, intID(rec["id"])).Issues, nil
}

func computeEpicOpenCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return tallyEpic(ctx, intID(rec["id"])).Open, nil
}

func computeEpicPointsTotal(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return tallyEpic(ctx, intID(rec["id"])).Total, nil
}

func computeEpicPointsDone(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return tallyEpic(ctx, intID(rec["id"])).Done, nil
}

func computeEpicProgress(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return epicProgressPct(tallyEpic(ctx, intID(rec["id"]))), nil
}

// epicProgressPct prefers story points, because that is the unit the epic was
// planned in; an epic whose issues carry no estimate falls back to the finished
// issue ratio rather than showing a permanent 0%.
func epicProgressPct(t scopeTally) int {
	if t.Total > 0 {
		return percent(t.Done, t.Total)
	}
	if t.Issues > 0 {
		return percent(t.Issues-t.Open, t.Issues)
	}
	return 0
}

// percent rounds part/whole to a whole percentage, clamped to 0..100.
func percent(part, whole int) int {
	if whole <= 0 || part <= 0 {
		return 0
	}
	if part >= whole {
		return 100
	}
	return (part*100 + whole/2) / whole
}

// ---------------------------------------------------------------------------
// Invariants
// ---------------------------------------------------------------------------

// constraintTaskEpic keeps an epic's scope honest: the epic must belong to the
// issue's own project. Because the linked epic is stored per issue, editing the
// project of an issue that still carries an epic is refused by the same rule
// instead of leaving the issue stranded in another project's epic.
func constraintTaskEpic(ctx context.Context, rec map[string]interface{}) error {
	eid := intID(rec["epic_id"])
	if eid <= 0 {
		return nil
	}
	epic := loadRec(orm.ContextWithBypass(ctx, true), epicModel, eid)
	if epic == nil {
		return nil
	}
	return epicProjectCheck(intID(rec["project_id"]), intID(epic["project_id"]))
}

// epicProjectCheck is the pure rule behind the epic link. A missing project on
// either side is not a disagreement: an issue can join an epic and receive its
// project later.
func epicProjectCheck(taskProject, epicProject int) error {
	if taskProject <= 0 || epicProject <= 0 || taskProject == epicProject {
		return nil
	}
	return fmt.Errorf("this epic belongs to a different project; an epic only groups issues from its own project")
}

// constraintEpicProject refuses to move an epic to another project while it
// still groups issues: the children would keep a link to an epic they are no
// longer allowed to join.
func constraintEpicProject(ctx context.Context, rec map[string]interface{}) error {
	id := intID(rec["id"])
	if id <= 0 {
		return nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	stored := loadRec(bypass, epicModel, id)
	if stored == nil {
		return nil
	}
	next, prev := intID(rec["project_id"]), intID(stored["project_id"])
	if next == prev {
		return nil
	}
	issues, _ := orm.SearchCount(bypass, taskModel, [][]interface{}{{"epic_id", "=", id}})
	return epicMoveCheck(prev, next, int(issues))
}

// epicMoveCheck is the pure rule behind the project guard.
func epicMoveCheck(prevProject, nextProject, issues int) error {
	if prevProject == nextProject || issues <= 0 {
		return nil
	}
	return fmt.Errorf("this epic still groups %d issue(s); move them to another epic before changing its project", issues)
}
