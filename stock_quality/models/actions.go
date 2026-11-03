package models

import (
	"context"
	"fmt"

	"nexa/core/orm"
)

// Quality workflow actions (§5.3, §7.4). Checks move draft → in_progress →
// done with a computed verdict; a non-conforming verdict opens a quarantine
// hold and blocks the lot through the anchor's disposition guard. Holds are
// released only by a second person (separation of duties) — enforced here
// server-side from the session uid, not just in the UI.

func init() {
	orm.RegisterObjectAction(checkModel, "action_start", checkStart)
	orm.RegisterObjectAction(checkModel, "action_record", checkRecord)
	orm.RegisterObjectAction(checkModel, "action_cancel", checkCancel)
	orm.RegisterObjectAction(holdModel, "action_release", holdRelease)
	orm.RegisterObjectAction(holdModel, "action_cancel", holdCancel)
}

func checkStart(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadCheck(bypass, id)
	if rec == nil {
		return "", fmt.Errorf("quality check not found")
	}
	if CheckState(str(rec["state"])) != CheckDraft {
		return "", fmt.Errorf("only draft checks can be started")
	}
	upd := map[string]interface{}{"state": string(CheckDoing)}
	if uid := orm.UIDFromContext(ctx); uid > 0 && intID(rec["inspector_id"]) == 0 {
		upd["inspector_id"] = uid
	}
	if err := orm.UpdateRecordByID(bypass, checkModel, id, upd); err != nil {
		return "", err
	}
	return "Inspection started.", nil
}

// checkRecord computes the verdict from the recorded quantities against the
// plan's pass rule; a fail or conditional result quarantines the lot (when
// tracked) and opens a hold for second-person disposition.
func checkRecord(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadCheck(bypass, id)
	if rec == nil {
		return "", fmt.Errorf("quality check not found")
	}
	st := CheckState(str(rec["state"]))
	if st != CheckDoing {
		return "", fmt.Errorf("only inspections in progress can be recorded")
	}
	sample := flt(rec["sample_qty"])
	if sample <= 0 {
		sample = sampleQty(flt(rec["quantity"]), 100)
	}
	defects := flt(rec["defect_qty"])
	threshold := 100.0
	if pid := intID(rec["plan_id"]); pid > 0 {
		if plan := loadRec(bypass, planModel, pid); plan != nil {
			threshold = flt(plan["pass_threshold_pct"])
		}
	}
	verdict := verdictFor(sample, defects, threshold)
	upd := map[string]interface{}{
		"result": string(verdict),
		"state":  string(CheckDone),
	}
	if uid := orm.UIDFromContext(ctx); uid > 0 && intID(rec["inspector_id"]) == 0 {
		upd["inspector_id"] = uid
	}
	if err := orm.UpdateRecordByID(bypass, checkModel, id, upd); err != nil {
		return "", err
	}
	if verdict == ResultPass || verdict == ResultPending {
		return fmt.Sprintf("Inspection recorded: %s.", verdict), nil
	}
	// Non-conforming: open a hold and block the lot from issue (§5.3, §7.3).
	lotID := intID(rec["lot_id"])
	if lotID > 0 {
		_ = orm.UpdateRecordByID(bypass, "stock.lot", lotID, map[string]interface{}{"state": "quarantined"})
	}
	if err := openHold(bypass, rec, verdict, lotID); err != nil {
		return "", err
	}
	return fmt.Sprintf("Inspection recorded: %s. Quarantine hold opened; a second person must disposition it.", verdict), nil
}

func openHold(ctx context.Context, checkRec map[string]interface{}, verdict CheckResult, lotID int) error {
	if existing, _ := orm.SearchCount(ctx, holdModel, [][]interface{}{
		{"check_id", "=", intID(checkRec["id"])}, {"state", "=", string(HoldOnHold)},
	}); existing > 0 {
		return nil
	}
	inst, ok := orm.Registry[holdModel]
	if !ok {
		return fmt.Errorf("quality hold model unavailable")
	}
	vals := map[string]interface{}{
		"name":        fmt.Sprintf("QH-%d", intID(checkRec["id"])),
		"check_id":    intID(checkRec["id"]),
		"product_id":  intID(checkRec["product_id"]),
		"location_id": intID(checkRec["picking_id"]), // placeholder kept blank below
		"quantity":    flt(checkRec["quantity"]),
		"state":       string(HoldOnHold),
		"reason": fmt.Sprintf("Check %s verdict %s (sample %v, defects %v).",
			str(checkRec["name"]), verdict, flt(checkRec["sample_qty"]), flt(checkRec["defect_qty"])),
	}
	if lotID > 0 {
		vals["lot_id"] = lotID
	}
	delete(vals, "location_id")
	if uid := orm.UIDFromContext(ctx); uid > 0 {
		vals["opened_by"] = uid
	}
	_, err := orm.Create(ctx, inst, vals)
	return err
}

func holdRelease(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, holdModel, id)
	if rec == nil {
		return "", fmt.Errorf("hold not found")
	}
	if HoldState(str(rec["state"])) != HoldOnHold {
		return "", fmt.Errorf("only open holds can be released")
	}
	uid := orm.UIDFromContext(ctx)
	openedBy := intID(rec["opened_by"])
	if !canRelease(openedBy, uid) {
		if uid <= 0 {
			return "", fmt.Errorf("holds must be released by a logged-in second person")
		}
		return "", fmt.Errorf("separation of duties: the person who opened this hold cannot release it")
	}
	if err := orm.UpdateRecordByID(bypass, holdModel, id, map[string]interface{}{
		"state": string(HoldReleased), "released_by": uid,
	}); err != nil {
		return "", err
	}
	// Clear the lot block only if no other open hold keeps it quarantined.
	if lotID := intID(rec["lot_id"]); lotID > 0 {
		open, _ := orm.SearchCount(bypass, holdModel, [][]interface{}{
			{"lot_id", "=", lotID}, {"state", "=", string(HoldOnHold)},
		})
		if open == 0 {
			_ = orm.UpdateRecordByID(bypass, "stock.lot", lotID, map[string]interface{}{"state": "released"})
		}
	}
	return "Hold released; the lot is eligible again.", nil
}

func holdCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, holdModel, id)
	if rec == nil {
		return "", fmt.Errorf("hold not found")
	}
	if HoldState(str(rec["state"])) != HoldOnHold {
		return "", fmt.Errorf("only open holds can be cancelled")
	}
	// Cancelling an incorrect hold must also be a second person, otherwise a
	// failed inspection could be quietly erased.
	uid := orm.UIDFromContext(ctx)
	if !canRelease(intID(rec["opened_by"]), uid) {
		return "", fmt.Errorf("separation of duties: only a second person may cancel this hold")
	}
	if err := orm.UpdateRecordByID(bypass, holdModel, id, map[string]interface{}{
		"state": string(HoldReleased), "released_by": uid, "note": "Hold cancelled as recorded in error.",
	}); err != nil {
		return "", err
	}
	return "Hold cancelled.", nil
}

func checkCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadCheck(bypass, id)
	if rec == nil {
		return "", fmt.Errorf("quality check not found")
	}
	st := CheckState(str(rec["state"]))
	if st != CheckDraft && st != CheckDoing {
		return "", fmt.Errorf("only open checks can be cancelled")
	}
	if err := orm.UpdateRecordByID(bypass, checkModel, id,
		map[string]interface{}{"state": string(CheckCancel)}); err != nil {
		return "", err
	}
	return "Check cancelled.", nil
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

func loadCheck(ctx context.Context, id int) map[string]interface{} {
	return loadRec(ctx, checkModel, id)
}
