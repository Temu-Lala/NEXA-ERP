package models

import (
	"context"
	"fmt"
	"strings"

	"nexa/core/orm"
)

// Operations workflow actions (§16). Tasks move draft → assigned → in_progress
// → done; a scan that does not match the product blocks completion and lands
// in the append-only scan log either way. Waves collect outgoing transfers,
// fan them out into consolidated pick tasks and release them to the floor.

func init() {
	orm.RegisterObjectAction(taskModel, "action_start", taskStart)
	orm.RegisterObjectAction(taskModel, "action_verify_scan", taskVerifyScan)
	orm.RegisterObjectAction(taskModel, "action_suggest_putaway", taskSuggestPutaway)
	orm.RegisterObjectAction(taskModel, "action_complete", taskComplete)
	orm.RegisterObjectAction(taskModel, "action_cancel", taskCancel)
	orm.RegisterObjectAction(waveModel, "action_finalize", waveFinalize)
	orm.RegisterObjectAction(waveModel, "action_generate", waveGenerate)
	orm.RegisterObjectAction(waveModel, "action_release", waveRelease)
	orm.RegisterObjectAction(waveModel, "action_done", waveDone)
	orm.RegisterObjectAction(waveModel, "action_cancel", waveCancel)
}

// ---------------------------------------------------------------------------
// Task workflow
// ---------------------------------------------------------------------------

func taskStart(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, taskModel, id)
	if rec == nil {
		return "", fmt.Errorf("task not found")
	}
	st := TaskState(str(rec["state"]))
	if st != TaskDraft && st != TaskAssigned {
		return "", fmt.Errorf("only open tasks can be started")
	}
	upd := map[string]interface{}{"state": string(TaskDoing)}
	if uid := orm.UIDFromContext(ctx); uid > 0 && intID(rec["assignee_id"]) == 0 {
		upd["assignee_id"] = uid
	}
	if err := orm.UpdateRecordByID(bypass, taskModel, id, upd); err != nil {
		return "", err
	}
	return "Task started.", nil
}

// taskVerifyScan logs the scan attempt and blocks on mismatch: a failed scan
// returns an error (nothing is marked verified) but still records the trail
// (§16.3 — evidence first, prevention by construction).
func taskVerifyScan(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, taskModel, id)
	if rec == nil {
		return "", fmt.Errorf("task not found")
	}
	st := TaskState(str(rec["state"]))
	if st != TaskDoing {
		return "", fmt.Errorf("scans are recorded against tasks in progress")
	}
	scanned := str(rec["scan_code"])
	var barcode, reference string
	if pid := intID(rec["product_id"]); pid > 0 {
		if prod := loadRec(bypass, "product.product", pid); prod != nil {
			barcode = str(prod["barcode"])
			reference = str(prod["default_code"])
		}
	}
	ok := scanMatches(scanned, barcode, reference)
	_, _ = createRec(bypass, scanLogMdl, map[string]interface{}{
		"name":       fmt.Sprintf("SCAN-%d-%s", id, nowStamp()),
		"task_id":    id,
		"product_id": intID(rec["product_id"]),
		"scanned":    scanned,
		"expected":   expectedCodes(barcode, reference),
		"matched":    ok,
		"quantity":   1,
		"scanned_at": nowStamp(),
	})
	if !ok {
		if err := orm.UpdateRecordByID(bypass, taskModel, id,
			map[string]interface{}{"scan_match": false}); err != nil {
			return "", err
		}
		return "", fmt.Errorf("scan mismatch: %q does not match %s — operation blocked",
			scanned, expectedCodes(barcode, reference))
	}
	if err := orm.UpdateRecordByID(bypass, taskModel, id,
		map[string]interface{}{"scan_match": true}); err != nil {
		return "", err
	}
	return "Scan verified against the product.", nil
}

// taskSuggestPutaway ranks internal locations for the task's product and
// quantity and stamps the best one as destination (§3.3, §5.4). Operators may
// change it manually with an override reason on the task.
func taskSuggestPutaway(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, taskModel, id)
	if rec == nil {
		return "", fmt.Errorf("task not found")
	}
	if TaskType(str(rec["type"])) != TaskPutaway {
		return "", fmt.Errorf("putaway suggestions apply to putaway tasks only")
	}
	var needKG, needM3 float64
	qty := flt(rec["quantity"])
	if pid := intID(rec["product_id"]); pid > 0 {
		if prod := loadRec(bypass, "product.product", pid); prod != nil {
			needKG = qty * flt(prod["weight"])
			needM3 = qty * flt(prod["volume"])
		}
	}
	ranked := rankPutaway(putawayCandidates(bypass, 0), needKG, needM3)
	if len(ranked) == 0 {
		return "", fmt.Errorf("no internal location can hold %.3f of this product within capacity", qty)
	}
	if err := orm.UpdateRecordByID(bypass, taskModel, id,
		map[string]interface{}{"dest_location_id": ranked[0].LocID}); err != nil {
		return "", err
	}
	names := make([]string, 0, 3)
	for i, s := range ranked {
		if i == 3 {
			break
		}
		names = append(names, s.Name)
	}
	return "Suggested destination: " + names[0] + " (candidates: " + strings.Join(names, ", ") + ").", nil
}

// taskComplete refuses to finish when the last scan did not match, or when
// the product carries a barcode that was never verified (§16.3).
func taskComplete(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, taskModel, id)
	if rec == nil {
		return "", fmt.Errorf("task not found")
	}
	if TaskState(str(rec["state"])) != TaskDoing {
		return "", fmt.Errorf("only tasks in progress can be completed")
	}
	if str(rec["scan_code"]) != "" && !orm.AsBool(rec["scan_match"]) {
		return "", fmt.Errorf("last scan did not match — re-verify before completing")
	}
	var barcode string
	if pid := intID(rec["product_id"]); pid > 0 {
		if prod := loadRec(bypass, "product.product", pid); prod != nil {
			barcode = str(prod["barcode"])
		}
	}
	if barcode != "" && !orm.AsBool(rec["scan_match"]) {
		return "", fmt.Errorf("this product is barcoded — scan verification is required")
	}
	if TaskType(str(rec["type"])) == TaskPutaway &&
		intID(rec["dest_location_id"]) == 0 && strings.TrimSpace(str(rec["override_reason"])) == "" {
		return "", fmt.Errorf("putaway needs a destination or a recorded override reason")
	}
	if err := orm.UpdateRecordByID(bypass, taskModel, id,
		map[string]interface{}{"state": string(TaskDone)}); err != nil {
		return "", err
	}
	return "Task completed.", nil
}

func taskCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, taskModel, id)
	if rec == nil {
		return "", fmt.Errorf("task not found")
	}
	st := TaskState(str(rec["state"]))
	if st == TaskDone || st == TaskCancelled {
		return "", fmt.Errorf("closed tasks cannot be cancelled")
	}
	if err := orm.UpdateRecordByID(bypass, taskModel, id,
		map[string]interface{}{"state": string(TaskCancelled)}); err != nil {
		return "", err
	}
	return "Task cancelled.", nil
}

// ---------------------------------------------------------------------------
// Wave workflow (§16.2)
// ---------------------------------------------------------------------------

func waveFinalize(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, waveModel, id)
	if rec == nil {
		return "", fmt.Errorf("wave not found")
	}
	if WaveState(str(rec["state"])) != WaveDraft {
		return "", fmt.Errorf("only draft waves can be finalized")
	}
	lines := waveLines(bypass, id)
	if len(lines) == 0 {
		return "", fmt.Errorf("add at least one transfer order before finalizing")
	}
	if err := orm.UpdateRecordByID(bypass, waveModel, id,
		map[string]interface{}{"state": string(WaveFinalized)}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Wave finalized with %d transfer order(s).", len(lines)), nil
}

// waveGenerate fans the wave out into consolidated pick tasks: one task per
// product+source zone across all pickings in the wave (§16.2 — pickers walk
// once for the same bin, sort later).
func waveGenerate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, waveModel, id)
	if rec == nil {
		return "", fmt.Errorf("wave not found")
	}
	if WaveState(str(rec["state"])) != WaveFinalized {
		return "", fmt.Errorf("finalize the wave before generating pick tasks")
	}
	open, _ := orm.SearchCount(bypass, taskModel, [][]interface{}{{"wave_id", "=", id}})
	if open > 0 {
		return "", fmt.Errorf("pick tasks already exist for this wave")
	}
	lines := waveLines(bypass, id)
	type aggKey struct {
		prod, loc int
	}
	type aggVal struct {
		qty      float64
		picking  int
		products []string
	}
	groups := map[aggKey]*aggVal{}
	var order []aggKey
	created := 0
	for _, ln := range lines {
		pickID := intID(ln["picking_id"])
		if pickID <= 0 {
			continue
		}
		moves, _ := orm.SearchPage(bypass, "stock.move", [][]interface{}{
			{"picking_id", "=", pickID},
		}, 500, 0, "id asc")
		for _, mv := range moves {
			if s := str(mv["state"]); s != "confirmed" && s != "assigned" {
				continue
			}
			k := aggKey{intID(mv["product_id"]), intID(mv["source_location_id"])}
			g := groups[k]
			if g == nil {
				g = &aggVal{picking: pickID}
				groups[k] = g
				order = append(order, k)
			}
			g.qty += flt(mv["quantity"])
			if pickID < g.picking {
				g.picking = pickID
			}
		}
	}
	n := 0
	for _, k := range order {
		g := groups[k]
		if g.qty <= 0 {
			continue
		}
		n++
		vals := map[string]interface{}{
			"name":       fmt.Sprintf("WAVE-%d-PICK-%02d", id, n),
			"type":       string(TaskPick),
			"state":      string(TaskDraft),
			"wave_id":    id,
			"picking_id": g.picking,
			"quantity":   g.qty,
			"priority":   5,
		}
		if k.prod > 0 {
			vals["product_id"] = k.prod
		}
		if k.loc > 0 {
			vals["location_id"] = k.loc
		}
		if _, err := createRec(bypass, taskModel, vals); err != nil {
			return "", err
		}
		created++
	}
	if created == 0 {
		return "", fmt.Errorf("no confirmed/assigned moves to pick in this wave")
	}
	return fmt.Sprintf("Generated %d consolidated pick task(s).", created), nil
}

func waveRelease(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, waveModel, id)
	if rec == nil {
		return "", fmt.Errorf("wave not found")
	}
	if WaveState(str(rec["state"])) != WaveFinalized {
		return "", fmt.Errorf("only finalized waves can be released")
	}
	tasks, _ := orm.SearchPage(bypass, taskModel, [][]interface{}{
		{"wave_id", "=", id}, {"state", "=", string(TaskDraft)},
	}, 1000, 0, "id asc")
	for _, t := range tasks {
		_ = orm.UpdateRecordByID(bypass, taskModel, intID(t["id"]),
			map[string]interface{}{"state": string(TaskAssigned)})
	}
	if err := orm.UpdateRecordByID(bypass, waveModel, id,
		map[string]interface{}{"state": string(WaveReleased)}); err != nil {
		return "", err
	}
	return fmt.Sprintf("Wave released: %d task(s) assigned to the floor.", len(tasks)), nil
}

func waveDone(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, waveModel, id)
	if rec == nil {
		return "", fmt.Errorf("wave not found")
	}
	if WaveState(str(rec["state"])) != WaveReleased {
		return "", fmt.Errorf("release the wave before closing it")
	}
	left, _ := orm.SearchCount(bypass, taskModel, [][]interface{}{
		{"wave_id", "=", id},
		{"state", "not in", []interface{}{string(TaskDone), string(TaskCancelled)}},
	})
	if left > 0 {
		return "", fmt.Errorf("%d task(s) from this wave are still open", left)
	}
	if err := orm.UpdateRecordByID(bypass, waveModel, id,
		map[string]interface{}{"state": string(WaveDone)}); err != nil {
		return "", err
	}
	return "Wave closed.", nil
}

func waveCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, waveModel, id)
	if rec == nil {
		return "", fmt.Errorf("wave not found")
	}
	st := WaveState(str(rec["state"]))
	if st == WaveDone || st == WaveCancelled {
		return "", fmt.Errorf("closed waves cannot be cancelled")
	}
	tasks, _ := orm.SearchPage(bypass, taskModel, [][]interface{}{
		{"wave_id", "=", id},
		{"state", "not in", []interface{}{string(TaskDone), string(TaskCancelled)}},
	}, 1000, 0, "id asc")
	for _, t := range tasks {
		_ = orm.UpdateRecordByID(bypass, taskModel, intID(t["id"]),
			map[string]interface{}{"state": string(TaskCancelled)})
	}
	if err := orm.UpdateRecordByID(bypass, waveModel, id,
		map[string]interface{}{"state": string(WaveCancelled)}); err != nil {
		return "", err
	}
	return "Wave cancelled; open tasks cancelled.", nil
}

// ---------------------------------------------------------------------------
// Read helpers
// ---------------------------------------------------------------------------

func loadRec(ctx context.Context, model string, id int) map[string]interface{} {
	if id <= 0 {
		return nil
	}
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil {
		return nil
	}
	return rec
}

func waveLines(ctx context.Context, waveID int) []map[string]interface{} {
	rows, err := orm.SearchPage(ctx, waveLineMdl, [][]interface{}{
		{"wave_id", "=", waveID},
	}, 500, 0, "id asc")
	if err != nil {
		return nil
	}
	return rows
}
