package models

import (
	"context"
	"encoding/json"
	"net/url"

	"nexa/core/orm"
)

// The Space shell (Jira-class, ERP-native). A project IS a space: its form opens
// with a row of section buttons — Issues, Open, Backlog, Board, Blocked, Sprints,
// Epics, Milestones, Team, Risks, Documents — each carrying a live count and each
// diving into that section already filtered to this project.
//
// Two honest mechanisms, no fakery:
//   - The counts are computed from real rows the instant the record is read (a
//     RegisterCompute per field). Companion models (risks, documents) are searched
//     by their model string; when that module is absent the search errors and the
//     count falls to 0 rather than inventing a number.
//   - Navigation uses an object button that returns a redirect URL to /web carrying
//     a domain JSON ([["project_id","=",thisProject], …]). The workspace reads the
//     domain and AND-merges it into whatever the section's base view does, for list,
//     kanban and every collection view alike — so the same project filter is applied
//     server-side, not faked in the browser.
//
// Nothing here writes project records or mutates the board; it is a read-only lens
// over data the rest of the module already owns.

const (
	memberModel   = "project.member"
	riskModel     = "project.risk"
	documentModel = "project.document"
)

func init() {
	orm.RegisterCompute(projModel, "issue_count", []string{"id"}, computeProjectIssueCount)
	orm.RegisterCompute(projModel, "open_issue_count", []string{"id"}, computeProjectOpenIssueCount)
	orm.RegisterCompute(projModel, "backlog_count", []string{"id"}, computeProjectBacklogCount)
	orm.RegisterCompute(projModel, "blocked_count", []string{"id"}, computeProjectBlockedCount)
	orm.RegisterCompute(projModel, "sprint_count", []string{"id"}, computeProjectSprintCount)
	orm.RegisterCompute(projModel, "epic_count", []string{"id"}, computeProjectEpicCount)
	orm.RegisterCompute(projModel, "milestone_count", []string{"id"}, computeProjectMilestoneCount)
	orm.RegisterCompute(projModel, "member_count", []string{"id"}, computeProjectMemberCount)
	orm.RegisterCompute(projModel, "risk_count", []string{"id"}, computeProjectRiskCount)
	orm.RegisterCompute(projModel, "document_count", []string{"id"}, computeProjectDocumentCount)

	orm.RegisterObjectAction(projModel, "action_space_issues", projSpaceIssues)
	orm.RegisterObjectAction(projModel, "action_space_open", projSpaceOpen)
	orm.RegisterObjectAction(projModel, "action_space_backlog", projSpaceBacklog)
	orm.RegisterObjectAction(projModel, "action_space_board", projSpaceBoard)
	orm.RegisterObjectAction(projModel, "action_space_blocked", projSpaceBlocked)
	orm.RegisterObjectAction(projModel, "action_space_sprints", projSpaceSprints)
	orm.RegisterObjectAction(projModel, "action_space_epics", projSpaceEpics)
	orm.RegisterObjectAction(projModel, "action_space_milestones", projSpaceMilestones)
	orm.RegisterObjectAction(projModel, "action_space_team", projSpaceTeam)
	orm.RegisterObjectAction(projModel, "action_space_risks", projSpaceRisks)
	orm.RegisterObjectAction(projModel, "action_space_documents", projSpaceDocuments)
}

// ---------------------------------------------------------------------------
// Live section counts (computed on read)
// ---------------------------------------------------------------------------

// rowCountWithin counts rows of model scoped to this project, plus any extra
// domain clauses. An unreadable or uninstalled model yields 0, never an error.
func rowCountWithin(ctx context.Context, rec map[string]interface{}, model string, extra ...[]interface{}) int {
	id := intID(rec["id"])
	if id <= 0 {
		return 0
	}
	domain := append([][]interface{}{{"project_id", "=", id}}, extra...)
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), model, domain)
	if err != nil {
		return 0
	}
	return n
}

func computeProjectIssueCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return rowCountWithin(ctx, rec, taskModel), nil
}

func computeProjectOpenIssueCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return rowCountWithin(ctx, rec, taskModel,
		[]interface{}{"state", "not in", []interface{}{string(TaskStateDone), string(TaskStateCancelled)}}), nil
}

func computeProjectBacklogCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return rowCountWithin(ctx, rec, taskModel,
		[]interface{}{"sprint_id", "=", false},
		[]interface{}{"state", "not in", []interface{}{string(TaskStateDone), string(TaskStateCancelled)}}), nil
}

func computeProjectBlockedCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return rowCountWithin(ctx, rec, taskModel,
		[]interface{}{"state", "=", string(TaskStateBlocked)}), nil
}

func computeProjectSprintCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return rowCountWithin(ctx, rec, sprintModel), nil
}

func computeProjectEpicCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return rowCountWithin(ctx, rec, epicModel), nil
}

func computeProjectMilestoneCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return rowCountWithin(ctx, rec, mileModel), nil
}

func computeProjectMemberCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return rowCountWithin(ctx, rec, memberModel), nil
}

func computeProjectRiskCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return rowCountWithin(ctx, rec, riskModel), nil
}

func computeProjectDocumentCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return rowCountWithin(ctx, rec, documentModel), nil
}

// ---------------------------------------------------------------------------
// Section navigation (object → project-scoped redirect)
// ---------------------------------------------------------------------------

// spaceRedirect builds a /web URL that opens model in viewType already filtered to
// projectID by domain. The workspace AND-merges this domain into the view's own,
// server-side, so every card shown genuinely belongs to the space.
func spaceRedirect(model, viewType string, projectID int, extra ...[]interface{}) (string, error) {
	domain := append([][]interface{}{{"project_id", "=", projectID}}, extra...)
	b, err := json.Marshal(domain)
	if err != nil {
		return "", err
	}
	q := url.Values{}
	q.Set("model", model)
	q.Set("view_type", viewType)
	q.Set("domain", string(b))
	return "/web?" + q.Encode(), nil
}

func projSpaceIssues(_ context.Context, _ string, id int, _ map[string]string) (string, error) {
	return spaceRedirect(taskModel, "list", id)
}

func projSpaceOpen(_ context.Context, _ string, id int, _ map[string]string) (string, error) {
	return spaceRedirect(taskModel, "list", id,
		[]interface{}{"state", "not in", []interface{}{string(TaskStateDone), string(TaskStateCancelled)}})
}

func projSpaceBacklog(_ context.Context, _ string, id int, _ map[string]string) (string, error) {
	return spaceRedirect(taskModel, "list", id,
		[]interface{}{"sprint_id", "=", false},
		[]interface{}{"state", "not in", []interface{}{string(TaskStateDone), string(TaskStateCancelled)}})
}

func projSpaceBoard(_ context.Context, _ string, id int, _ map[string]string) (string, error) {
	return spaceRedirect(taskModel, "kanban", id)
}

func projSpaceBlocked(_ context.Context, _ string, id int, _ map[string]string) (string, error) {
	return spaceRedirect(taskModel, "list", id, []interface{}{"state", "=", string(TaskStateBlocked)})
}

func projSpaceSprints(_ context.Context, _ string, id int, _ map[string]string) (string, error) {
	return spaceRedirect(sprintModel, "list", id)
}

func projSpaceEpics(_ context.Context, _ string, id int, _ map[string]string) (string, error) {
	return spaceRedirect(epicModel, "list", id)
}

func projSpaceMilestones(_ context.Context, _ string, id int, _ map[string]string) (string, error) {
	return spaceRedirect(mileModel, "list", id)
}

func projSpaceTeam(_ context.Context, _ string, id int, _ map[string]string) (string, error) {
	return spaceRedirect(memberModel, "list", id)
}

func projSpaceRisks(_ context.Context, _ string, id int, _ map[string]string) (string, error) {
	return spaceRedirect(riskModel, "list", id)
}

func projSpaceDocuments(_ context.Context, _ string, id int, _ map[string]string) (string, error) {
	return spaceRedirect(documentModel, "list", id)
}
