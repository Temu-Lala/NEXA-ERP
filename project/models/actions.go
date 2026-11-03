package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nexa/core/orm"
)

// Project, task and milestone lifecycle actions (blueprint §2.1, §5.3, §6.2,
// §7.2, §20). Transitions are validated server-side from the session uid so the
// state machine does not depend on the UI: a project only becomes Active when
// its type does not require approval or an approver has signed off; tasks cannot
// be finished while a blocking dependency is unresolved; and reaching a
// milestone requires its underlying work to be done. Every state change writes a
// best-effort status-history row for the audit timeline. This module writes only
// its own records.

func init() {
	// Project lifecycle.
	orm.RegisterObjectAction(projModel, "action_submit", projSubmit)
	orm.RegisterObjectAction(projModel, "action_approve", projApprove)
	orm.RegisterObjectAction(projModel, "action_activate", projActivate)
	orm.RegisterObjectAction(projModel, "action_hold", projHold)
	orm.RegisterObjectAction(projModel, "action_resume", projResume)
	orm.RegisterObjectAction(projModel, "action_complete", projComplete)
	orm.RegisterObjectAction(projModel, "action_cancel", projCancel)
	orm.RegisterObjectAction(projModel, "action_archive", projArchive)
	orm.RegisterObjectAction(projModel, "action_recompute", projRecompute)

	// Task lifecycle.
	orm.RegisterObjectAction(taskModel, "action_ready", taskReady)
	orm.RegisterObjectAction(taskModel, "action_start", taskStart)
	orm.RegisterObjectAction(taskModel, "action_block", taskBlock)
	orm.RegisterObjectAction(taskModel, "action_unblock", taskUnblock)
	orm.RegisterObjectAction(taskModel, "action_done", taskDone)
	orm.RegisterObjectAction(taskModel, "action_cancel", taskCancel)

	// Milestone lifecycle.
	orm.RegisterObjectAction(mileModel, "action_mark_done", mileDone)
}

// ---------------------------------------------------------------------------
// Project lifecycle
// ---------------------------------------------------------------------------

func projSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, projModel, id)
	if rec == nil {
		return "", fmt.Errorf("project not found")
	}
	if ProjectState(str(rec["state"])) != ProjectStateDraft {
		return "", fmt.Errorf("only draft projects can be submitted")
	}
	name := str(rec["name"])
	if name == "" || name == "New" {
		name = nextRef(bypass, "PRJ")
	}
	if err := setProjState(ctx, id, ProjectStatePlanned, map[string]interface{}{"name": name}); err != nil {
		return "", err
	}
	return "Project submitted for approval.", nil
}

// projApprove is gated by the project type's approval requirement. When the type
// requires approval the actor must differ from the originating manager
// (separation of duties); otherwise approval is a plain lifecycle step.
func projApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, projModel, id)
	if rec == nil {
		return "", fmt.Errorf("project not found")
	}
	if ProjectState(str(rec["state"])) != ProjectStatePlanned {
		return "", fmt.Errorf("only submitted projects can be approved")
	}
	if requiresApproval(bypass, intID(rec["type_id"])) {
		mgr := intID(rec["manager_id"])
		uid := orm.UIDFromContext(ctx)
		if mgr > 0 && uid > 0 && mgr == uid {
			return "", fmt.Errorf("a project that requires approval must be approved by someone other than its manager")
		}
	}
	if err := setProjState(ctx, id, ProjectStateApproved, nil); err != nil {
		return "", err
	}
	return "Project approved.", nil
}

func projActivate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, projModel, id)
	if rec == nil {
		return "", fmt.Errorf("project not found")
	}
	if ProjectState(str(rec["state"])) != ProjectStateApproved {
		return "", fmt.Errorf("only approved projects can be activated")
	}
	upd := map[string]interface{}{}
	if str(rec["actual_start"]) == "" {
		upd["actual_start"] = time.Now().Format(dateLayout)
	}
	if err := setProjState(ctx, id, ProjectStateActive, upd); err != nil {
		return "", err
	}
	return "Project is now active.", nil
}

func projHold(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return projSetFrom(ctx, id, []ProjectState{ProjectStateActive}, ProjectStateOnHold, "Project placed on hold.")
}

func projResume(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return projSetFrom(ctx, id, []ProjectState{ProjectStateOnHold}, ProjectStateActive, "Project resumed.")
}

// projComplete closes an active or on-hold project and records the finish date.
// Overdue milestones are surfaced as a warning but do not block closure, matching
// the blueprint's guidance that closure is a controlled management decision.
func projComplete(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, projModel, id)
	if rec == nil {
		return "", fmt.Errorf("project not found")
	}
	cur := ProjectState(str(rec["state"]))
	if cur != ProjectStateActive && cur != ProjectStateOnHold {
		return "", fmt.Errorf("only active or on-hold projects can be completed")
	}
	overdue, _ := orm.SearchCount(bypass, mileModel, [][]interface{}{
		{"project_id", "=", id},
		{"due_date", "!=", ""},
		{"due_date", "<", time.Now().Format(dateLayout)},
		{"state", "!=", string(MilestoneDone)},
	})
	upd := map[string]interface{}{"actual_end": time.Now().Format(dateLayout)}
	if err := setProjState(ctx, id, ProjectStateCompleted, upd); err != nil {
		return "", err
	}
	if overdue > 0 {
		return fmt.Sprintf("Project completed with %d overdue milestone(s) still open.", overdue), nil
	}
	return "Project completed.", nil
}

func projCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return projSetFrom(ctx, id, []ProjectState{ProjectStateDraft, ProjectStatePlanned, ProjectStateApproved, ProjectStateActive, ProjectStateOnHold}, ProjectStateCancelled, "Project cancelled.")
}

func projArchive(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	if err := orm.UpdateRecordByID(bypass, projModel, id, map[string]interface{}{
		"state": string(ProjectStateArchived), "active": false,
	}); err != nil {
		return "", err
	}
	writeHistory(ctx, id, 0, EntityProject, "", string(ProjectStateArchived))
	return "Project archived.", nil
}

// projRecompute refreshes progress, costs and health on demand using the same
// derivation as the daily cron.
func projRecompute(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, projModel, id)
	if rec == nil {
		return "", fmt.Errorf("project not found")
	}
	now := time.Now()
	total, _ := orm.SearchCount(bypass, taskModel, [][]interface{}{{"project_id", "=", id}})
	done, _ := orm.SearchCount(bypass, taskModel, [][]interface{}{
		{"project_id", "=", id}, {"state", "=", string(TaskStateDone)},
	})
	overdueTasks, _ := orm.SearchCount(bypass, taskModel, [][]interface{}{
		{"project_id", "=", id}, {"deadline", "!=", ""},
		{"deadline", "<", now.Format(dateLayout)},
		{"state", "not in", []interface{}{string(TaskStateDone), string(TaskStateCancelled)}},
	})
	overdueMiles, _ := orm.SearchCount(bypass, mileModel, [][]interface{}{
		{"project_id", "=", id}, {"due_date", "!=", ""},
		{"due_date", "<", now.Format(dateLayout)}, {"state", "!=", string(MilestoneDone)},
	})
	actual := sumCosts(bypass, costModel, id, "actual")
	committed := sumCosts(bypass, costModel, id, "committed")
	upd := map[string]interface{}{
		"progress":       progressPct(done, total),
		"actual_cost":    actual,
		"committed_cost": committed,
	}
	msg := "Recomputed."
	if str(rec["health_note"]) == "" {
		h := classifyHealth(overdueTasks+overdueMiles, budgetPressure(actual, flt(rec["budget"])), (total-done) > 0)
		upd["health"] = string(h)
		msg = fmt.Sprintf("Recomputed: progress %.0f%%, health %s.", progressPct(done, total), h)
	} else {
		msg += " Health override preserved."
	}
	if err := orm.UpdateRecordByID(bypass, projModel, id, upd); err != nil {
		return "", err
	}
	return msg, nil
}

// ---------------------------------------------------------------------------
// Task lifecycle
// ---------------------------------------------------------------------------

func taskReady(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return taskSet(ctx, id, []TaskState{TaskStateBacklog}, TaskStateReady, "Task is ready.")
}

func taskStart(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return taskSet(ctx, id, []TaskState{TaskStateReady, TaskStateTodo}, TaskStateInProgress, "Task in progress.")
}

func taskBlock(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, taskModel, id)
	if rec == nil {
		return "", fmt.Errorf("task not found")
	}
	cur := TaskState(str(rec["state"]))
	if cur == TaskStateDone || cur == TaskStateCancelled {
		return "", fmt.Errorf("a finished task cannot be blocked")
	}
	upd := map[string]interface{}{"state": string(TaskStateBlocked), "kanban_state": string(KanbanBlocked)}
	if r := strings.TrimSpace(vals["reason"]); r != "" {
		upd["description"] = joinNote(str(rec["description"]), "Blocked: "+r)
	}
	if err := orm.UpdateRecordByID(bypass, taskModel, id, upd); err != nil {
		return "", err
	}
	writeTaskHistory(ctx, rec, cur, TaskStateBlocked)
	return "Task blocked.", nil
}

func taskUnblock(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, taskModel, id)
	if rec == nil {
		return "", fmt.Errorf("task not found")
	}
	if TaskState(str(rec["state"])) != TaskStateBlocked {
		return "", fmt.Errorf("only blocked tasks can be unblocked")
	}
	if err := orm.UpdateRecordByID(bypass, taskModel, id, map[string]interface{}{
		"state": string(TaskStateInProgress), "kanban_state": string(KanbanReady),
	}); err != nil {
		return "", err
	}
	writeTaskHistory(ctx, rec, TaskStateBlocked, TaskStateInProgress)
	return "Task unblocked and back in progress.", nil
}

// taskDone refuses completion while any blocking dependency (a predecessor whose
// required relation is Finish-to-Start) is still unfinished, unless an explicit
// exception reason is supplied. This enforces §6.2 without a hard schedule
// engine dependency.
func taskDone(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, taskModel, id)
	if rec == nil {
		return "", fmt.Errorf("task not found")
	}
	cur := TaskState(str(rec["state"]))
	if cur == TaskStateDone {
		return "", fmt.Errorf("task is already done")
	}
	exception := strings.TrimSpace(vals["override"])
	if exception == "" {
		if blocking, n := openPredecessors(bypass, id); blocking {
			return "", fmt.Errorf("%d unfinished predecessor(s) block this task; provide an override reason to complete anyway", n)
		}
	}
	upd := map[string]interface{}{"state": string(TaskStateDone), "remaining_hours": 0}
	if exception != "" {
		upd["description"] = joinNote(str(rec["description"]), "Completed with exception: "+exception)
	}
	if err := orm.UpdateRecordByID(bypass, taskModel, id, upd); err != nil {
		return "", err
	}
	writeTaskHistory(ctx, rec, cur, TaskStateDone)
	if pid := intID(rec["project_id"]); pid > 0 {
		_, _ = projRecompute(ctx, projModel, pid, nil)
	}
	if exception != "" {
		return "Task completed with a recorded exception.", nil
	}
	return "Task completed.", nil
}

func taskCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return taskSet(ctx, id, []TaskState{TaskStateBacklog, TaskStateReady, TaskStateTodo, TaskStateInProgress, TaskStateReview, TaskStateBlocked}, TaskStateCancelled, "Task cancelled.")
}

// ---------------------------------------------------------------------------
// Milestone lifecycle
// ---------------------------------------------------------------------------

// mileDone marks a milestone reached, warning when linked tasks are still open
// (§7.2: a milestone is a checkpoint over real work, not an arbitrary date).
func mileDone(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, mileModel, id)
	if rec == nil {
		return "", fmt.Errorf("milestone not found")
	}
	if MilestoneState(str(rec["state"])) == MilestoneDone {
		return "", fmt.Errorf("milestone already achieved")
	}
	openTasks, _ := orm.SearchCount(bypass, taskModel, [][]interface{}{
		{"milestone_id", "=", id},
		{"state", "not in", []interface{}{string(TaskStateDone), string(TaskStateCancelled)}},
	})
	upd := map[string]interface{}{"state": string(MilestoneDone), "achieved_date": time.Now().Format(dateLayout)}
	if err := orm.UpdateRecordByID(bypass, mileModel, id, upd); err != nil {
		return "", err
	}
	if openTasks > 0 {
		return fmt.Sprintf("Milestone marked achieved with %d linked task(s) still open.", openTasks), nil
	}
	return "Milestone achieved.", nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func setProjState(ctx context.Context, id int, to ProjectState, extra map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, projModel, id)
	from := str(rec["state"])
	upd := map[string]interface{}{"state": string(to)}
	for k, v := range extra {
		upd[k] = v
	}
	if err := orm.UpdateRecordByID(bypass, projModel, id, upd); err != nil {
		return err
	}
	writeHistory(ctx, id, 0, EntityProject, from, string(to))
	return nil
}

func projSetFrom(ctx context.Context, id int, from []ProjectState, to ProjectState, msg string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, projModel, id)
	if rec == nil {
		return "", fmt.Errorf("project not found")
	}
	cur := ProjectState(str(rec["state"]))
	if cur == to {
		return msg, nil
	}
	allowed := false
	for _, s := range from {
		if cur == s {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("transition to %s not allowed from %s", to, cur)
	}
	if err := setProjState(ctx, id, to, nil); err != nil {
		return "", err
	}
	return msg, nil
}

func taskSet(ctx context.Context, id int, from []TaskState, to TaskState, msg string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, taskModel, id)
	if rec == nil {
		return "", fmt.Errorf("task not found")
	}
	cur := TaskState(str(rec["state"]))
	if cur == to {
		return msg, nil
	}
	allowed := false
	for _, s := range from {
		if cur == s {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("transition to %s not allowed from %s", to, cur)
	}
	upd := map[string]interface{}{"state": string(to)}
	if to == TaskStateInProgress {
		upd["kanban_state"] = string(KanbanReady)
	}
	if err := orm.UpdateRecordByID(bypass, taskModel, id, upd); err != nil {
		return "", err
	}
	writeTaskHistory(ctx, rec, cur, to)
	return msg, nil
}

// openPredecessors reports whether any Finish-to-Start predecessor of the task
// is unfinished. Dependencies live in project_planning; when that module is not
// installed the search simply returns nothing and completion is unconstrained.
func openPredecessors(ctx context.Context, taskID int) (bool, int) {
	depModel := "project.task.dependency"
	rows, err := orm.SearchPage(ctx, depModel, [][]interface{}{
		{"task_id", "=", taskID},
		{"dependency_type", "=", "finish_to_start"},
	}, 500, 0, "id asc")
	if err != nil {
		return false, 0
	}
	open := 0
	for _, d := range rows {
		pid := intID(d["predecessor_id"])
		pred := loadRec(ctx, taskModel, pid)
		if pred == nil {
			continue
		}
		switch TaskState(str(pred["state"])) {
		case TaskStateDone, TaskStateCancelled:
		default:
			open++
		}
	}
	return open > 0, open
}

// requiresApproval reports whether a project type mandates an approval step.
func requiresApproval(ctx context.Context, typeID int) bool {
	if typeID <= 0 {
		return false
	}
	t := loadRec(ctx, "project.type", typeID)
	if t == nil {
		return false
	}
	b, _ := t["require_approval"].(bool)
	return b
}

func writeHistory(ctx context.Context, projectID, taskID int, entity EntityKind, from, to string) {
	if from == to {
		return
	}
	bypass := orm.ContextWithBypass(ctx, true)
	inst, ok := orm.Registry[histModel]
	if !ok {
		return // project_planning not installed; audit timeline is optional
	}
	vals := map[string]interface{}{
		"project_id": projectID, "task_id": taskID,
		"entity_type": string(entity), "from_state": from, "to_state": to,
	}
	if uid := orm.UIDFromContext(ctx); uid > 0 {
		vals["user_id"] = uid
	}
	_, _ = orm.Create(bypass, inst, vals)
}

func writeTaskHistory(ctx context.Context, rec map[string]interface{}, from, to TaskState) {
	if from == to {
		return
	}
	writeHistory(ctx, intID(rec["project_id"]), intID(rec["id"]), EntityTask, string(from), string(to))
}

func joinNote(existing, add string) string {
	if strings.TrimSpace(existing) == "" {
		return add
	}
	return existing + "\n" + add
}

func nextRef(ctx context.Context, prefix string) string {
	if name, err := orm.NextSequence(ctx, projModel); err == nil && name != "" {
		return name
	}
	return fmt.Sprintf("%s/%05d", prefix, time.Now().Unix()%100000)
}
