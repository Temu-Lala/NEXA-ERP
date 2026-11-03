package models

import (
	"context"
	"fmt"
	"strings"

	"nexa/core/orm"
)

// Performance workflow (blueprint §13, §15). Scorecards are evidence and a
// review trigger - a low score escalates for a human decision and NEVER
// auto-blocks a supplier. Corrective-action closure requires independent
// verification (the verifier is not the person responsible for the action).

func init() {
	orm.RegisterObjectAction(periodModel, "action_compute", periodCompute)
	orm.RegisterObjectAction(periodModel, "action_lock", periodLock)
	orm.RegisterObjectAction(periodModel, "action_reset", periodReset)

	orm.RegisterObjectAction(scorecardMdl, "action_recompute", scorecardRecompute)
	orm.RegisterObjectAction(scorecardMdl, "action_review", scorecardReview)
	orm.RegisterObjectAction(scorecardMdl, "action_lock", scorecardLock)

	orm.RegisterObjectAction(correctiveMdl, "action_open", caOpen)
	orm.RegisterObjectAction(correctiveMdl, "action_start", caStart)
	orm.RegisterObjectAction(correctiveMdl, "action_complete", caComplete)
	orm.RegisterObjectAction(correctiveMdl, "action_verify", caVerify)
	orm.RegisterObjectAction(correctiveMdl, "action_reject", caReject)
	orm.RegisterObjectAction(correctiveMdl, "action_cancel", caCancel)
}

// ---------------------------------------------------------------------------
// Period
// ---------------------------------------------------------------------------

func periodCompute(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	p := loadRec(bypass, periodModel, id)
	if p == nil {
		return "", fmt.Errorf("period not found")
	}
	if str(p["state"]) == string(PStateLocked) {
		return "", fmt.Errorf("a locked period cannot be recomputed; reset it first")
	}
	if err := computePeriod(bypass, p); err != nil {
		return "", err
	}
	count, _ := orm.SearchCount(bypass, scorecardMdl, [][]interface{}{{"period_id", "=", id}})
	return fmt.Sprintf("Computed %d supplier scorecard(s).", count), nil
}

func periodLock(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	p := loadRec(bypass, periodModel, id)
	if p == nil {
		return "", fmt.Errorf("period not found")
	}
	if str(p["state"]) != string(PStateComputed) {
		return "", fmt.Errorf("only a computed period can be locked")
	}
	_ = orm.UpdateRecordByID(bypass, periodModel, id, map[string]interface{}{"state": string(PStateLocked)})
	appendHistory(bypass, periodModel, id, "Period locked; scorecards frozen.")
	return "Period locked.", nil
}

func periodReset(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	p := loadRec(bypass, periodModel, id)
	if p == nil {
		return "", fmt.Errorf("period not found")
	}
	if str(p["state"]) == string(PStateCancelled) {
		return "", fmt.Errorf("a cancelled period cannot be reset")
	}
	_ = orm.UpdateRecordByID(bypass, periodModel, id, map[string]interface{}{"state": string(PStateDraft)})
	appendHistory(bypass, periodModel, id, "Reset to draft.")
	return "Period reset to draft.", nil
}

// ---------------------------------------------------------------------------
// Scorecard
// ---------------------------------------------------------------------------

// scorecardRecompute re-derives the dimension and composite scores, the grade and
// the escalation from the scorecard's own stored KPI metric rows.
func scorecardRecompute(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	sc := loadRec(bypass, scorecardMdl, id)
	if sc == nil {
		return "", fmt.Errorf("scorecard not found")
	}
	if str(sc["state"]) == string(ScLocked) {
		return "", fmt.Errorf("a locked scorecard cannot be recomputed")
	}
	rows, _ := orm.SearchPage(bypass, metricModel, [][]interface{}{
		{"scorecard_id", "=", id},
	}, 200, 0, "id asc")
	dimScores := map[Dimension][]float64{}
	dimFlags := map[Dimension][]bool{}
	for _, r := range rows {
		d := Dimension(str(r["dimension"]))
		dimScores[d] = append(dimScores[d], flt(r["score"]))
		dimFlags[d] = append(dimFlags[d], boolOf(r["has_data"]))
	}
	dimScore := map[Dimension]float64{}
	dimHas := map[Dimension]bool{}
	for d := range dimScores {
		s, ok := meanScore(dimScores[d], dimFlags[d])
		dimScore[d] = s
		dimHas[d] = ok
	}
	dims := []dimValue{
		{dimScore[DimQuality], flt(sc["weight_quality"]), dimHas[DimQuality]},
		{dimScore[DimDelivery], flt(sc["weight_delivery"]), dimHas[DimDelivery]},
		{dimScore[DimCost], flt(sc["weight_cost"]), dimHas[DimCost]},
		{dimScore[DimService], flt(sc["weight_service"]), dimHas[DimService]},
		{dimScore[DimCompliance], flt(sc["weight_compliance"]), dimHas[DimCompliance]},
	}
	composite, hasComp := weightedComposite(dims)
	grade := gradeFor(composite, hasComp)
	esc := escalationFor(composite, hasComp, 70, 50)
	uid := orm.UIDFromContext(ctx)
	_ = orm.UpdateRecordByID(bypass, scorecardMdl, id, map[string]interface{}{
		"quality_score":    dimScore[DimQuality],
		"delivery_score":   dimScore[DimDelivery],
		"cost_score":       dimScore[DimCost],
		"service_score":    dimScore[DimService],
		"compliance_score": dimScore[DimCompliance],
		"composite_score":  composite,
		"grade":            string(grade),
		"escalation":       string(esc),
		"evaluated_by":     uid,
		"evaluated_on":     nowStr(),
		"state":            string(ScCalculated),
	})
	note := fmt.Sprintf("Recomputed: composite %.2f (%s).", composite, string(grade))
	if esc != EscNone {
		note += " Escalated for review (no automatic block)."
	}
	appendHistory(bypass, scorecardMdl, id, note)
	return fmt.Sprintf("Scorecard recomputed: %.2f, grade %s, escalation %s.", composite, string(grade), string(esc)), nil
}

func scorecardReview(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	sc := loadRec(bypass, scorecardMdl, id)
	if sc == nil {
		return "", fmt.Errorf("scorecard not found")
	}
	if str(sc["state"]) != string(ScCalculated) {
		return "", fmt.Errorf("only a calculated scorecard can be marked reviewed")
	}
	uid := orm.UIDFromContext(ctx)
	_ = orm.UpdateRecordByID(bypass, scorecardMdl, id, map[string]interface{}{"state": string(ScReviewed)})
	appendHistory(bypass, scorecardMdl, id, fmt.Sprintf("Reviewed by user %d.", uid))
	return "Scorecard reviewed.", nil
}

func scorecardLock(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	sc := loadRec(bypass, scorecardMdl, id)
	if sc == nil {
		return "", fmt.Errorf("scorecard not found")
	}
	if str(sc["state"]) != string(ScReviewed) {
		return "", fmt.Errorf("only a reviewed scorecard can be locked")
	}
	_ = orm.UpdateRecordByID(bypass, scorecardMdl, id, map[string]interface{}{"state": string(ScLocked)})
	appendHistory(bypass, scorecardMdl, id, "Scorecard locked.")
	return "Scorecard locked.", nil
}

// ---------------------------------------------------------------------------
// Corrective actions
// ---------------------------------------------------------------------------

func caOpen(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ca := loadRec(bypass, correctiveMdl, id)
	if ca == nil {
		return "", fmt.Errorf("corrective action not found")
	}
	if str(ca["state"]) != string(CaDraft) {
		return "", fmt.Errorf("only a draft corrective action can be opened")
	}
	if strings.TrimSpace(str(ca["title"])) == "" || intID(ca["supplier_id"]) <= 0 {
		return "", fmt.Errorf("a corrective action needs a title and a supplier")
	}
	if strings.TrimSpace(str(ca["name"])) == "" || str(ca["name"]) == "New" {
		_ = orm.UpdateRecordByID(bypass, correctiveMdl, id, map[string]interface{}{"name": nextRef(bypass, "CAPA")})
	}
	_ = orm.UpdateRecordByID(bypass, correctiveMdl, id, map[string]interface{}{"state": string(CaOpen)})
	appendHistory(bypass, correctiveMdl, id, "Opened.")
	return "Corrective action opened.", nil
}

func caStart(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ca := loadRec(bypass, correctiveMdl, id)
	if ca == nil {
		return "", fmt.Errorf("corrective action not found")
	}
	if str(ca["state"]) != string(CaOpen) && str(ca["state"]) != string(CaRejected) {
		return "", fmt.Errorf("only an open or rejected action can be started")
	}
	plan := strings.TrimSpace(str(ca["corrective_plan"]))
	if plan == "" {
		return "", fmt.Errorf("a corrective plan is required before starting")
	}
	_ = orm.UpdateRecordByID(bypass, correctiveMdl, id, map[string]interface{}{"state": string(CaInProgress)})
	appendHistory(bypass, correctiveMdl, id, "Work started.")
	return "Corrective action in progress.", nil
}

func caComplete(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ca := loadRec(bypass, correctiveMdl, id)
	if ca == nil {
		return "", fmt.Errorf("corrective action not found")
	}
	if str(ca["state"]) != string(CaInProgress) {
		return "", fmt.Errorf("only an in-progress action can be submitted for closure")
	}
	evidence := strings.TrimSpace(vals["closure_evidence"])
	if evidence == "" {
		evidence = strings.TrimSpace(str(ca["closure_evidence"]))
	}
	if evidence == "" {
		return "", fmt.Errorf("closure evidence is required before verification")
	}
	_ = orm.UpdateRecordByID(bypass, correctiveMdl, id, map[string]interface{}{
		"state": string(CaPendingV), "closure_evidence": evidence, "closure_date": todayStr(),
	})
	appendHistory(bypass, correctiveMdl, id, "Submitted for independent verification.")
	return "Closure submitted for verification.", nil
}

func caVerify(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ca := loadRec(bypass, correctiveMdl, id)
	if ca == nil {
		return "", fmt.Errorf("corrective action not found")
	}
	if str(ca["state"]) != string(CaPendingV) {
		return "", fmt.Errorf("only a pending-verification action can be verified")
	}
	uid := orm.UIDFromContext(ctx)
	if uid <= 0 {
		return "", fmt.Errorf("verifier identity required")
	}
	if resp := intID(ca["responsible_id"]); resp > 0 && resp == uid {
		return "", fmt.Errorf("separation of duties: the responsible person cannot verify their own closure")
	}
	_ = orm.UpdateRecordByID(bypass, correctiveMdl, id, map[string]interface{}{
		"state": string(CaClosed), "verified_by": uid,
	})
	appendHistory(bypass, correctiveMdl, id, fmt.Sprintf("Closure verified by user %d.", uid))
	return "Corrective action closed (independently verified).", nil
}

func caReject(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ca := loadRec(bypass, correctiveMdl, id)
	if ca == nil {
		return "", fmt.Errorf("corrective action not found")
	}
	if str(ca["state"]) != string(CaPendingV) {
		return "", fmt.Errorf("only a pending-verification action can be rejected")
	}
	reason := strings.TrimSpace(vals["reason"])
	if reason == "" {
		reason = "Closure evidence insufficient."
	}
	_ = orm.UpdateRecordByID(bypass, correctiveMdl, id, map[string]interface{}{"state": string(CaRejected), "closure_date": ""})
	appendHistory(bypass, correctiveMdl, id, "Verification rejected: "+reason)
	return "Closure rejected; returned to the responsible owner.", nil
}

func caCancel(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ca := loadRec(bypass, correctiveMdl, id)
	if ca == nil {
		return "", fmt.Errorf("corrective action not found")
	}
	if str(ca["state"]) == string(CaClosed) {
		return "", fmt.Errorf("a closed corrective action cannot be cancelled")
	}
	_ = orm.UpdateRecordByID(bypass, correctiveMdl, id, map[string]interface{}{"state": string(CaCancelled), "active": false})
	appendHistory(bypass, correctiveMdl, id, "Cancelled.")
	return "Corrective action cancelled.", nil
}
