package models

import (
	"context"
	"fmt"

	"nexa/core/orm"
)

// Template actions (blueprint §16). Instantiating a template creates a real draft
// project and reproduces its phase and task blueprint beneath it, preserving order
// and each task's phase. The template itself is never mutated, so it stays a
// reusable standard. A template with no task blueprint cannot be instantiated
// because it would produce an empty project.

var validTaskPriority = map[string]bool{
	"low": true, "normal": true, "high": true, "urgent": true,
}

func init() {
	orm.RegisterObjectAction(tplModel, "action_instantiate", tplInstantiate)
}

// tplInstantiate builds a new draft project from the template's blueprint and
// returns a human-readable summary of what was created.
func tplInstantiate(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	tpl := load(bypass, tplModel, id)
	if tpl == nil {
		return "", fmt.Errorf("template not found")
	}

	phases, err := orm.SearchPage(bypass, tplPhaseMdl, [][]interface{}{
		{"template_id", "=", id},
	}, 5000, 0, "sequence asc, id asc")
	if err != nil {
		return "", err
	}
	tasks, err := orm.SearchPage(bypass, tplTaskMdl, [][]interface{}{
		{"template_id", "=", id},
	}, 5000, 0, "sequence asc, id asc")
	if err != nil {
		return "", err
	}
	if len(tasks) == 0 {
		return "", fmt.Errorf("template has no task blueprint to instantiate")
	}

	projInst, ok := orm.Registry[destProjectModel]
	if !ok {
		return "", fmt.Errorf("project model unavailable")
	}
	name := s(tpl["name"])
	if v := vals["name"]; v != "" {
		name = v
	}
	projVals := map[string]interface{}{
		"name":        name,
		"state":       "draft",
		"description": s(tpl["description"]),
	}
	if uid := orm.UIDFromContext(ctx); uid > 0 {
		projVals["manager_id"] = uid
	}
	projectID, err := orm.Create(bypass, projInst, projVals)
	if err != nil {
		return "", err
	}

	// Reproduce phases, remembering the mapping from blueprint phase to new phase.
	phaseInst, ok := orm.Registry[destPhaseModel]
	phaseMap := map[int]int{}
	createdPhases := 0
	if ok {
		for _, p := range phases {
			newID, err := orm.Create(bypass, phaseInst, map[string]interface{}{
				"name":       s(p["name"]),
				"project_id": projectID,
				"sequence":   idOf(p["sequence"]),
				"state":      "planned",
			})
			if err == nil {
				phaseMap[idOf(p["id"])] = newID
				createdPhases++
			}
		}
	}

	// Reproduce tasks, attaching each to its mapped phase when one exists.
	taskInst, ok := orm.Registry[destTaskModel]
	createdTasks := 0
	if ok {
		for _, t := range tasks {
			tv := map[string]interface{}{
				"name":        s(t["name"]),
				"project_id":  projectID,
				"sequence":    idOf(t["sequence"]),
				"state":       "backlog",
				"est_hours":   flt(t["est_hours"]),
				"description": s(t["description"]),
			}
			if pr := s(t["priority"]); validTaskPriority[pr] {
				tv["priority"] = pr
			}
			if pid := phaseMap[idOf(t["phase_id"])]; pid > 0 {
				tv["phase_id"] = pid
			}
			if _, err := orm.Create(bypass, taskInst, tv); err == nil {
				createdTasks++
			}
		}
	}

	return fmt.Sprintf("Created draft project with %d phase(s) and %d task(s).", createdPhases, createdTasks), nil
}
