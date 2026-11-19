package models

import (
	"context"
	"fmt"
	"time"

	"nexa/core/orm"
	"nexa/core/sdk"
)

// Issue relationships + checklists (spec §2.2/§2.3). A LINK connects two issues
// with a typed, directional relationship (blocks / relates to / duplicates).
// "Blocks" edges form a directed graph: the constraint layer refuses self-links,
// duplicate pairs and — most importantly — cycles, so "A blocks B blocks A" can
// never be stored. CHECKLIST items give every issue an acceptance-criteria /
// definition-of-done list with a live done ratio.
//
// Both models are deliberately flat records (not JSON blobs) so links stay
// queryable for dependency reports and the cycle check stays a real graph walk.

const (
	linkModel      = "project.task.link"
	checkItemModel = "project.task.checklist.item"
)

// TaskLinkType is the directional relationship stored on a link row. The row
// reads "TaskID <type> LinkedTaskID" (e.g. PROJ-1 blocks PROJ-2).
type TaskLinkType string

const (
	LinkBlocks     TaskLinkType = "blocks"
	LinkRelatesTo  TaskLinkType = "relates_to"
	LinkDuplicates TaskLinkType = "duplicates"
)

// ProjectTaskLink is a typed, directional edge between two issues.
type ProjectTaskLink struct {
	sdk.Model `nexa:"model=project.task.link"`
	CompanyID   sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	TaskID       sdk.Many2One[ProjectTask]   `nexa:"required,index,string=Issue"`
	LinkedTaskID sdk.Many2One[ProjectTask]   `nexa:"required,index,string=Linked Issue"`
	LinkType     sdk.Selection[TaskLinkType] `nexa:"required,string=Link Type,default=relates_to"`
	Note         sdk.Text                    `nexa:"string=Note"`
}

// ProjectTaskChecklistItem is one acceptance-criteria / definition-of-done line
// on an issue. Sequence keeps the authored order; Done drives the progress bar.
type ProjectTaskChecklistItem struct {
	sdk.Model `nexa:"model=project.task.checklist.item"`
	CompanyID   sdk.Many2One[CoreCompany] `nexa:"index,string=Company,default=current_company"`

	TaskID   sdk.Many2One[ProjectTask] `nexa:"required,index,ondelete=cascade,string=Issue"`
	Name     sdk.String                `nexa:"required,string=Item"`
	Done     sdk.Boolean               `nexa:"string=Done,default=false"`
	Sequence sdk.Integer               `nexa:"string=Sequence,default=10"`
}

func init() {
	// Overdue is derived, never typed: an open issue past its deadline. The list
	// and board can badge it without a nightly job touching every row.
	orm.RegisterCompute(taskModel, "is_overdue", []string{"deadline", "state"}, computeTaskOverdue)

	// Blocked is derived from "blocks" links: this issue is blocked while any
	// issue that blocks it is still open. Unblocked the moment the blocker
	// finishes — no status field to maintain by hand.
	orm.RegisterCompute(taskModel, "is_blocked", []string{"id"}, computeTaskIsBlocked)

	// Link invariants: no self-links, no duplicate pair (either direction, same
	// type), and no cycles along "blocks" edges.
	orm.RegisterConstraint(linkModel, "link_not_self", constraintLinkNotSelf)
	orm.RegisterConstraint(linkModel, "link_unique_pair", constraintLinkUniquePair)
	orm.RegisterConstraint(linkModel, "link_no_cycle", constraintLinkNoCycle)
}

// computeTaskOverdue reports whether an issue is past its deadline and still open.
func computeTaskOverdue(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	state := TaskState(str(rec["state"]))
	if state == TaskStateDone || state == TaskStateCancelled {
		return false, nil
	}
	deadline, ok := parseDate(rec["deadline"])
	if !ok {
		return false, nil
	}
	today := time.Now()
	today = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, today.Location())
	return deadline.Before(today), nil
}

// computeTaskIsBlocked reports whether any open issue blocks this one. A
// "blocks" edge A -> B means A blocks B, so B is blocked while A is unfinished.
// Cancelled blockers do not block; done blockers release the issue immediately.
func computeTaskIsBlocked(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	id := intID(rec["id"])
	if id <= 0 {
		return false, nil
	}
	state := TaskState(str(rec["state"]))
	if state == TaskStateDone || state == TaskStateCancelled {
		return false, nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	links, err := orm.SearchPage(bypass, linkModel, [][]interface{}{
		{"linked_task_id", "=", id},
		{"link_type", "=", string(LinkBlocks)},
	}, 200, 0, "id asc")
	if err != nil {
		return false, nil
	}
	for _, l := range links {
		blocker := loadRec(bypass, taskModel, intID(l["task_id"]))
		if blocker == nil {
			continue
		}
		switch TaskState(str(blocker["state"])) {
		case TaskStateDone, TaskStateCancelled:
		default:
			return true, nil
		}
	}
	return false, nil
}

// constraintLinkNotSelf rejects an issue linked to itself.
func constraintLinkNotSelf(_ context.Context, rec map[string]interface{}) error {
	if intID(rec["task_id"]) == intID(rec["linked_task_id"]) && intID(rec["task_id"]) > 0 {
		return fmt.Errorf("an issue cannot be linked to itself")
	}
	return nil
}

// constraintLinkUniquePair rejects the same typed edge stored twice — in either
// direction. A->B "blocks" and B->A "blocks" are different edges (opposite
// meaning) and both may exist; what is rejected is an exact duplicate.
func constraintLinkUniquePair(ctx context.Context, rec map[string]interface{}) error {
	taskID := intID(rec["task_id"])
	linkedID := intID(rec["linked_task_id"])
	linkType := str(rec["link_type"])
	if taskID <= 0 || linkedID <= 0 || linkType == "" {
		return nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	selfID := intID(rec["id"])
	rows, err := orm.SearchPage(bypass, linkModel, [][]interface{}{
		{"task_id", "=", taskID},
		{"linked_task_id", "=", linkedID},
		{"link_type", "=", linkType},
	}, 10, 0, "id asc")
	if err != nil {
		return err
	}
	for _, row := range rows {
		if intID(row["id"]) != selfID {
			return fmt.Errorf("this link already exists")
		}
	}
	return nil
}

// constraintLinkNoCycle keeps the "blocks" graph acyclic. Adding A -blocks-> B is
// rejected when B can already reach A through blocks edges (B blocks … blocks A),
// which would make the dependency unsatisfiable.
func constraintLinkNoCycle(ctx context.Context, rec map[string]interface{}) error {
	if TaskLinkType(str(rec["link_type"])) != LinkBlocks {
		return nil
	}
	from := intID(rec["task_id"])
	to := intID(rec["linked_task_id"])
	if from <= 0 || to <= 0 {
		return nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	if blocksPathExists(bypass, to, from, map[int]bool{}) {
		return fmt.Errorf("circular dependency: the linked issue already blocks this one (directly or through other issues)")
	}
	return nil
}

// blocksPathExists is a bounded DFS over "blocks" edges: does `from` reach `to`
// by following task_id -> linked_task_id hops? Depth is capped so a pathological
// graph cannot stall a write.
func blocksPathExists(ctx context.Context, from, to int, seen map[int]bool) bool {
	if from <= 0 || to <= 0 {
		return false
	}
	if from == to {
		return true
	}
	if seen[from] || len(seen) > 500 {
		return false
	}
	seen[from] = true
	rows, err := orm.SearchPage(ctx, linkModel, [][]interface{}{
		{"task_id", "=", from},
		{"link_type", "=", string(LinkBlocks)},
	}, 200, 0, "id asc")
	if err != nil {
		return false
	}
	for _, row := range rows {
		if blocksPathExists(ctx, intID(row["linked_task_id"]), to, seen) {
			return true
		}
	}
	return false
}
