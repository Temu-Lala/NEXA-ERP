package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// Healthcare engine: one-open-encounter-per-patient guard, encounter close,
// appointment check-in → encounter creation, patient age compute.

const (
	patientModel  = "healthcare.patient"
	providerModel = "healthcare.provider"
	apptModel     = "healthcare.appointment"
	encModel      = "healthcare.encounter"
)

func init() {
	orm.RegisterCompute(patientModel, "age_years", []string{"date_of_birth"}, computePatientAge)
	orm.RegisterObjectAction(apptModel, "action_appt_checkin", apptCheckIn)
	orm.RegisterObjectAction(encModel, "action_encounter_close", encClose)
}

func fnum(v interface{}) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
	return n
}

func oid(v interface{}) int {
	if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
		n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(arr[0])))
		return n
	}
	n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
	return n
}

func loadRec(ctx context.Context, model string, id int) map[string]interface{} {
	rows, err := orm.SearchPage(ctx, model, [][]interface{}{{"id", "=", id}}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

// computePatientAge from date_of_birth.
func computePatientAge(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	dob := strings.TrimSpace(fmt.Sprint(rec["date_of_birth"]))
	if len(dob) < 10 {
		return 0, nil
	}
	d, err := time.Parse("2006-01-02", dob[:10])
	if err != nil {
		return 0, nil
	}
	return int(time.Since(d).Hours() / 24 / 365.25), nil
}

// apptCheckIn: confirmed → checked_in AND opens an encounter for the visit.
// One open encounter per patient is enforced.
func apptCheckIn(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	a := loadRec(bypass, apptModel, id)
	if a == nil {
		return "", fmt.Errorf("appointment not found")
	}
	if st := orm.AsString(a["state"]); st != "confirmed" && st != "scheduled" {
		return "", fmt.Errorf("cannot check in from state %s", st)
	}
	patientID := oid(a["patient_id"])
	open, _ := orm.SearchPage(bypass, encModel,
		[][]interface{}{{"patient_id", "=", patientID}, {"state", "=", "open"}}, 10, 0, "id asc")
	var encID int
	if len(open) > 0 {
		encID = oid(open[0]["id"])
	} else {
		inst, ok := orm.Registry[encModel]
		if !ok {
			return "", fmt.Errorf("encounter model not registered")
		}
		encID, _ = orm.Create(bypass, inst, map[string]interface{}{
			"name":           fmt.Sprintf("ENC-%d", id),
			"patient_id":     patientID,
			"provider_id":    oid(a["provider_id"]),
			"appointment_id": id,
			"encounter_date": time.Now().UTC().Format(time.RFC3339),
			"type":           "outpatient",
			"state":          "open",
		})
	}
	_ = orm.UpdateRecordByID(bypass, apptModel, id, map[string]interface{}{
		"state":        "checked_in",
		"encounter_id": encID,
	})
	return fmt.Sprintf("Checked in — encounter %d open", encID), nil
}

// encClose: open → closed. Only one transition; notes remain restricted.
func encClose(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	e := loadRec(bypass, encModel, id)
	if e == nil {
		return "", fmt.Errorf("encounter not found")
	}
	if st := orm.AsString(e["state"]); st != "open" {
		return "", fmt.Errorf("only open encounters can be closed (state=%s)", st)
	}
	_ = orm.UpdateRecordByID(bypass, encModel, id, map[string]interface{}{"state": "closed"})
	return "Encounter closed", nil
}

// ---------------------------------------------------------------------------
// Healthcare finance: claim totals, submit/adjudicate/pay workflow.
// ---------------------------------------------------------------------------

const (
	tariffModel = "healthcare.tariff"
	claimModel  = "healthcare.claim"
	clineModel  = "healthcare.claim.line"
)

func init() {
	orm.RegisterCompute(claimModel, "amount_total", []string{"id"}, computeClaimTotal)
	orm.RegisterCompute(clineModel, "amount", []string{"quantity", "unit_price"}, computeClaimLineAmount)
	orm.RegisterObjectAction(claimModel, "action_claim_submit", claimSubmit)
	orm.RegisterObjectAction(claimModel, "action_claim_adjudicate", claimAdjudicate)
	orm.RegisterObjectAction(claimModel, "action_claim_pay", claimPay)
}

func fnum2(v interface{}) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
	return n
}

func oid2(v interface{}) int {
	if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
		n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(arr[0])))
		return n
	}
	n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
	return n
}

func loadRec2(ctx context.Context, model string, id int) map[string]interface{} {
	rows, err := orm.SearchPage(ctx, model, [][]interface{}{{"id", "=", id}}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

func computeClaimLineAmount(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	return fnum2(rec["quantity"]) * fnum2(rec["unit_price"]), nil
}

// computeClaimTotal sums line amounts + copay into billed total.
func computeClaimTotal(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	lines, err := orm.SearchPage(bypass, clineModel,
		[][]interface{}{{"claim_id", "=", oid2(rec["id"])}}, 500, 0, "id asc")
	if err != nil {
		return 0, nil
	}
	total := 0.0
	for _, l := range lines {
		total += fnum2(l["quantity"]) * fnum2(l["unit_price"])
	}
	return total + fnum2(rec["copay"]), nil
}

// claimSubmit: draft → submitted with timestamp.
func claimSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := loadRec2(bypass, claimModel, id)
	if c == nil {
		return "", fmt.Errorf("claim not found")
	}
	if st := orm.AsString(c["state"]); st != "draft" {
		return "", fmt.Errorf("only draft claims can be submitted (state=%s)", st)
	}
	lines, _ := orm.SearchPage(bypass, clineModel, [][]interface{}{{"claim_id", "=", id}}, 500, 0, "id asc")
	if len(lines) == 0 {
		return "", fmt.Errorf("claim has no line items")
	}
	_ = orm.UpdateRecordByID(bypass, claimModel, id, map[string]interface{}{
		"state":        "submitted",
		"submitted_at": time.Now().UTC().Format(time.RFC3339),
	})
	return "Claim submitted to payer", nil
}

// claimAdjudicate: submitted → adjudicated with approved total.
func claimAdjudicate(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := loadRec2(bypass, claimModel, id)
	if c == nil {
		return "", fmt.Errorf("claim not found")
	}
	if st := orm.AsString(c["state"]); st != "submitted" {
		return "", fmt.Errorf("only submitted claims can be adjudicated (state=%s)", st)
	}
	approved := fnum2(args["approved"])
	if approved <= 0 {
		approved = fnum2(c["amount_total"])
	}
	upd := map[string]interface{}{
		"state":           "adjudicated",
		"amount_approved": approved,
		"decided_at":      time.Now().UTC().Format(time.RFC3339),
	}
	if r := strings.TrimSpace(args["reason"]); r != "" {
		upd["denial_reason"] = r
	}
	_ = orm.UpdateRecordByID(bypass, claimModel, id, upd)
	return fmt.Sprintf("Claim adjudicated — approved %.2f", approved), nil
}

// claimPay: adjudicated → paid with paid amount.
func claimPay(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := loadRec2(bypass, claimModel, id)
	if c == nil {
		return "", fmt.Errorf("claim not found")
	}
	if st := orm.AsString(c["state"]); st != "adjudicated" {
		return "", fmt.Errorf("only adjudicated claims can be paid (state=%s)", st)
	}
	paid := fnum2(args["paid"])
	if paid <= 0 {
		paid = fnum2(c["amount_approved"])
	}
	_ = orm.UpdateRecordByID(bypass, claimModel, id, map[string]interface{}{
		"state":       "paid",
		"amount_paid": paid,
	})
	return fmt.Sprintf("Claim paid — %.2f received", paid), nil
}
