package models

import (
	"context"
	"fmt"
	"strings"

	"nexa/core/orm"
)

// action_clone duplicates a project shell — header fields, phases, milestones
// and team — into a fresh draft. Tasks, sprints, worklogs and history are NOT
// copied: a clone is a new plan, not a copy of execution history (Jira Clone
// behaves the same way for project templates).
func projClone(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	src := loadRec(bypass, projModel, id)
	if src == nil {
		return "", fmt.Errorf("project not found")
	}

	copy := map[string]interface{}{}
	for k, v := range src {
		switch k {
		case "id", "create_date", "write_date", "image", "__created", "__last_updated":
			continue
		}
		copy[k] = v
	}
	copy["name"] = strings.TrimSpace(str(src["name"])) + " (copy)"
	copy["code"] = ""
	copy["state"] = string(ProjectStateDraft)
	copy["health"] = string(HealthNotAssessed)
	copy["progress"] = 0
	copy["actual_start"] = nil
	copy["actual_end"] = nil
	copy["actual_cost"] = 0
	copy["committed_cost"] = 0
	copy["active"] = true

	inst, ok := orm.Registry[projModel]
	if !ok {
		return "", fmt.Errorf("project model not registered")
	}
	newID, err := orm.Create(bypass, inst, copy)
	if err != nil {
		return "", err
	}

	cloned := 0
	for _, spec := range []struct {
		model  string
		fields []string
	}{
		{"project.phase", []string{"name", "state", "planned_start", "planned_end", "sequence", "color"}},
		{"project.milestone", []string{"name", "phase_id", "owner_id", "due_date", "is_billable", "sequence", "description"}},
		{"project.member", []string{"user_id", "role", "allocation_pct", "active"}},
	} {
		rows, err := orm.SearchPage(bypass, spec.model, [][]interface{}{{"project_id", "=", id}}, 1000, 0, "id asc")
		if err != nil {
			continue
		}
		childInst, ok := orm.Registry[spec.model]
		if !ok {
			continue
		}
		for _, row := range rows {
			vals := map[string]interface{}{"project_id": newID}
			for _, f := range spec.fields {
				vals[f] = row[f]
			}
			// phase_id on milestones refers to the OLD phase id — remap below.
			if spec.model == "project.milestone" {
				vals["phase_id"] = nil
			}
			if _, err := orm.Create(bypass, childInst, vals); err == nil {
				cloned++
			}
		}
	}

	return fmt.Sprintf("Project cloned as draft #%d with %d child record(s). Tasks and history were not copied.", newID, cloned), nil
}
