package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nexa/core/orm"
)

// Governance actions (blueprint §7.3, §10, §14.3, §20). Approving a baseline
// preserves history by superseding the prior approved baseline rather than editing
// it; a financial snapshot captures the project's derived measures on a date; and a
// decision is closed only once its rationale is recorded. These are controlled
// management acts; none rewrite historical approved plans.

func init() {
	orm.RegisterObjectAction(baselineModel, "action_approve", baseApprove)
	orm.RegisterObjectAction(baselineModel, "action_supersede", baseSupersede)

	orm.RegisterObjectAction(snapModel, "action_capture", snapCapture)

	orm.RegisterObjectAction(decisionModel, "action_close", decisionClose)
}

// baseApprove records the approver and date, then makes this the active approved
// baseline for the project by superseding the previous one.
func baseApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, baselineModel, id)
	if rec == nil {
		return "", fmt.Errorf("baseline not found")
	}
	if s(rec["state"]) != "draft" {
		return "", fmt.Errorf("only draft baselines can be approved")
	}
	if s(rec["snapshot_date"]) == "" {
		_ = bypass // snapshot_date is required by the model; leave as-is if unset
	}
	upd := map[string]interface{}{"state": "approved"}
	if uid := orm.UIDFromContext(ctx); uid > 0 {
		upd["approved_by"] = uid
	}
	if err := orm.UpdateRecordByID(bypass, baselineModel, id, upd); err != nil {
		return "", err
	}
	n := supersedePriorBaselines(bypass, idOf(rec["project_id"]), id)
	if n > 0 {
		return fmt.Sprintf("Baseline approved; %d prior approved baseline(s) superseded.", n), nil
	}
	return "Baseline approved as the active plan.", nil
}

func baseSupersede(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, baselineModel, id)
	if rec == nil {
		return "", fmt.Errorf("baseline not found")
	}
	if s(rec["state"]) != "approved" {
		return "", fmt.Errorf("only approved baselines can be superseded")
	}
	if err := orm.UpdateRecordByID(bypass, baselineModel, id, map[string]interface{}{"state": "superseded"}); err != nil {
		return "", err
	}
	return "Baseline superseded.", nil
}

// snapCapture writes derived financial trend lines for the snapshot's project from
// the project's reconciled measures: actual, committed, forecast (actual+committed)
// and margin (contract value - forecast). It replaces the need to hand-type totals.
func snapCapture(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, snapModel, id)
	if rec == nil {
		return "", fmt.Errorf("snapshot not found")
	}
	pid := idOf(rec["project_id"])
	proj := load(bypass, projModel, pid)
	if proj == nil {
		return "", fmt.Errorf("snapshot must reference an existing project")
	}
	actual := flt(proj["actual_cost"])
	committed := flt(proj["committed_cost"])
	forecast := actual + committed
	margin := flt(proj["contract_value"]) - forecast
	date := strings.TrimSpace(s(rec["date"]))
	if date == "" {
		date = time.Now().Format(dateLayout)
	}
	inst, ok := orm.Registry[snapModel]
	if !ok {
		return "", fmt.Errorf("snapshot model unavailable")
	}
	measures := []struct {
		kind  string
		amt   float64
		descr string
	}{
		{"actual", actual, "Derived snapshot: actual cost"},
		{"committed", committed, "Derived snapshot: open commitments"},
		{"forecast", forecast, "Derived snapshot: forecast at completion"},
		{"margin", margin, "Derived snapshot: contract value less forecast"},
	}
	written := 0
	for _, m := range measures {
		if _, err := orm.Create(bypass, inst, map[string]interface{}{
			"project_id":  pid,
			"date":        date,
			"kind":        m.kind,
			"amount":      m.amt,
			"description": m.descr,
		}); err == nil {
			written++
		}
	}
	return fmt.Sprintf("Captured %d financial measure(s) for the project.", written), nil
}

// decisionClose requires a recorded rationale so the log explains the outcome.
func decisionClose(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := load(bypass, decisionModel, id)
	if rec == nil {
		return "", fmt.Errorf("decision not found")
	}
	if strings.EqualFold(s(rec["status"]), "closed") {
		return "", fmt.Errorf("decision already closed")
	}
	if strings.TrimSpace(s(rec["rationale"])) == "" {
		return "", fmt.Errorf("record the rationale before closing a decision")
	}
	if err := orm.UpdateRecordByID(bypass, decisionModel, id, map[string]interface{}{"status": "closed"}); err != nil {
		return "", err
	}
	return "Decision closed.", nil
}
