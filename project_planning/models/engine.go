package models

import (
	"context"
	"fmt"
	"strings"

	"nexa/core/orm"
)

// Planning engine (blueprint §6.4, §7, §13, §22). The scheduling guarantees that
// matter most here are correctness guards rather than a full CPM solver: a
// dependency graph must stay acyclic, and a task must never depend on itself.
// The cycle check walks the predecessor chain over the currently stored edges,
// so an insert that would close a loop is rejected before it is written. This
// module owns only planning structures; tasks and milestones stay in core.

const (
	depModel   = "project.task.dependency"
	delivModel = "project.deliverable"
	taskModel  = "project.task"
	mileModel  = "project.milestone"
)

// createsCycle reports whether adding edge (task -> predecessor) would close a
// dependency loop, i.e. whether the task is reachable from its predecessor by
// following existing predecessor edges. Reaching the task means the new edge
// would make it its own ancestor.
func createsCycle(ctx context.Context, taskID, predecessorID int) bool {
	if taskID <= 0 || predecessorID <= 0 {
		return false
	}
	if taskID == predecessorID {
		return true
	}
	// Walk predecessors-of-predecessor breadth-first, bounded to guard against a
	// pre-existing malformed graph.
	visited := map[int]bool{taskID: true}
	frontier := []int{predecessorID}
	for steps := 0; len(frontier) > 0 && steps < 10000; steps++ {
		next := make([]int, 0, len(frontier))
		for _, cur := range frontier {
			if visited[cur] {
				continue
			}
			if cur == taskID {
				return true
			}
			visited[cur] = true
			edges, err := orm.SearchPage(ctx, depModel, [][]interface{}{
				{"task_id", "=", cur},
			}, 2000, 0, "id asc")
			if err != nil {
				continue
			}
			for _, e := range edges {
				if pid := idOf(e["predecessor_id"]); pid > 0 {
					next = append(next, pid)
				}
			}
		}
		frontier = next
	}
	return false
}

// validateDependency enforces the structural rules for one candidate edge: no
// self-dependency, both endpoints are tasks, and the addition keeps the graph
// acyclic. A blank dependency type defaults to finish-to-start.
func validateDependency(ctx context.Context, taskID, predecessorID int, depType string) error {
	if taskID <= 0 {
		return fmt.Errorf("a dependency must reference a task")
	}
	if predecessorID <= 0 {
		return fmt.Errorf("a dependency must reference a predecessor task")
	}
	if taskID == predecessorID {
		return fmt.Errorf("a task cannot depend on itself")
	}
	if t := strings.TrimSpace(depType); t != "" && !knownDepType(t) {
		return fmt.Errorf("unknown dependency type %q", t)
	}
	if createsCycle(ctx, taskID, predecessorID) {
		return fmt.Errorf("this dependency would create a scheduling cycle; remove the loop first")
	}
	return nil
}

func knownDepType(t string) bool {
	switch t {
	case "finish_to_start", "start_to_start", "finish_to_finish", "start_to_finish":
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// Helpers (package-local; the core project package keeps its own copies)
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
