package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// Construction engine: BOQ line amounts, BOQ totals, RFI answer flow and
// variation order submit/approve/reject — the site-document workflows.

const (
	boqModel   = "construction.boq"
	boqLineMdl = "construction.boq.line"
	rfiModel   = "construction.rfi"
	varModel   = "construction.variation"
	diaryModel = "construction.diary"
)

func init() {
	orm.RegisterCompute(boqLineMdl, "amount", []string{"quantity", "unit_rate"}, computeBoqLineAmount)
	orm.RegisterCompute(boqModel, "total_amount", []string{"id"}, computeBoqTotal)
	orm.RegisterObjectAction(rfiModel, "action_rfi_answer", rfiAnswer)
	orm.RegisterObjectAction(varModel, "action_var_submit", varSubmit)
	orm.RegisterObjectAction(varModel, "action_var_decide", varDecide)
}

func fnum(v interface{}) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
	return n
}

func oid(v interface{}) int {
	if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
		n, _ := strconv.Atoi(fmt.Sprint(arr[0]))
		return n
	}
	n, _ := strconv.Atoi(fmt.Sprint(v))
	return n
}

func loadRec(ctx context.Context, model string, id int) map[string]interface{} {
	rows, err := orm.SearchPage(ctx, model, [][]interface{}{{"id", "=", id}}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

// BOQ line amount = qty × rate; BOQ total = Σ lines. Live computes.
func computeBoqLineAmount(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	return fnum(rec["quantity"]) * fnum(rec["unit_rate"]), nil
}

func computeBoqTotal(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	lines, err := orm.SearchPage(orm.ContextWithBypass(ctx, true), boqLineMdl,
		[][]interface{}{{"boq_id", "=", oid(rec["id"])}}, 2000, 0, "id asc")
	if err != nil {
		return 0, nil
	}
	total := 0.0
	for _, l := range lines {
		total += fnum(l["quantity"]) * fnum(l["unit_rate"])
	}
	return total, nil
}

// rfiAnswer records the official answer and moves the RFI to answered.
func rfiAnswer(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rfi := loadRec(bypass, rfiModel, id)
	if rfi == nil {
		return "", fmt.Errorf("RFI not found")
	}
	if st := orm.AsString(rfi["state"]); st == "closed" || st == "cancelled" {
		return "", fmt.Errorf("cannot answer a %s RFI", st)
	}
	answer := strings.TrimSpace(args["answer"])
	if answer == "" {
		return "", fmt.Errorf("answer text required")
	}
	_ = orm.UpdateRecordByID(bypass, rfiModel, id, map[string]interface{}{
		"answer":         answer,
		"state":          "answered",
		"answered_by_id": args["user_id"],
		"date_answered":  time.Now().UTC().Format("2006-01-02"),
	})
	return "RFI answered", nil
}

// varSubmit moves a draft variation to submitted for approval.
func varSubmit(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	v := loadRec(bypass, varModel, id)
	if v == nil {
		return "", fmt.Errorf("variation not found")
	}
	if st := orm.AsString(v["state"]); st != "draft" {
		return "", fmt.Errorf("only draft variations can be submitted (state=%s)", st)
	}
	_ = orm.UpdateRecordByID(bypass, varModel, id, map[string]interface{}{
		"state":          "submitted",
		"submitted_by":   args["user_id"],
		"date_submitted": time.Now().UTC().Format("2006-01-02"),
	})
	return "Variation submitted for approval", nil
}

// varDecide approves or rejects a submitted variation.
func varDecide(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	v := loadRec(bypass, varModel, id)
	if v == nil {
		return "", fmt.Errorf("variation not found")
	}
	if st := orm.AsString(v["state"]); st != "submitted" {
		return "", fmt.Errorf("only submitted variations can be decided (state=%s)", st)
	}
	decision := strings.ToLower(strings.TrimSpace(args["decision"]))
	if decision != "approve" && decision != "reject" {
		return "", fmt.Errorf("decision must be approve or reject")
	}
	state := "approved"
	if decision == "reject" {
		state = "rejected"
	}
	_ = orm.UpdateRecordByID(bypass, varModel, id, map[string]interface{}{
		"state":        state,
		"approved_by":  args["user_id"],
		"date_decided": time.Now().UTC().Format("2006-01-02"),
	})
	return "Variation " + state, nil
}

// ---------------------------------------------------------------------------
// Payment certificates: retention/advance math + certify/pay workflow.
// ---------------------------------------------------------------------------

const (
	subModel  = "construction.subcontract"
	certModel = "construction.certificate"
)

func init() {
	orm.RegisterCompute(certModel, "net_payable", []string{"amount_this_period", "retention_held", "advance_recovery"}, computeCertNet)
	orm.RegisterObjectAction(certModel, "action_cert_submit", certSubmit)
	orm.RegisterObjectAction(certModel, "action_cert_certify", certCertify)
	orm.RegisterObjectAction(certModel, "action_cert_pay", certPay)
	orm.RegisterObjectAction(subModel, "action_sub_activate", subActivate)
}

func cfnum(v interface{}) float64 {
	n, _ := strconv.ParseFloat(strings.TrimSpace(fmt.Sprint(v)), 64)
	return n
}

func coid(v interface{}) int {
	if arr, ok := v.([]interface{}); ok && len(arr) > 0 {
		n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(arr[0])))
		return n
	}
	n, _ := strconv.Atoi(strings.TrimSpace(fmt.Sprint(v)))
	return n
}

func cload(ctx context.Context, model string, id int) map[string]interface{} {
	rows, err := orm.SearchPage(ctx, model, [][]interface{}{{"id", "=", id}}, 1, 0, "id asc")
	if err != nil || len(rows) == 0 {
		return nil
	}
	return rows[0]
}

// computeCertNet = period amount − retention held − advance recovery.
func computeCertNet(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	return cfnum(rec["amount_this_period"]) - cfnum(rec["retention_held"]) - cfnum(rec["advance_recovery"]), nil
}

// subActivate: draft → active.
func subActivate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	s := cload(bypass, subModel, id)
	if s == nil {
		return "", fmt.Errorf("subcontract not found")
	}
	if st := orm.AsString(s["state"]); st != "draft" {
		return "", fmt.Errorf("only draft subcontracts can be activated (state=%s)", st)
	}
	_ = orm.UpdateRecordByID(bypass, subModel, id, map[string]interface{}{"state": "active"})
	return "Subcontract activated", nil
}

// certSubmit: draft → submitted.
func certSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := cload(bypass, certModel, id)
	if c == nil {
		return "", fmt.Errorf("certificate not found")
	}
	if st := orm.AsString(c["state"]); st != "draft" {
		return "", fmt.Errorf("only draft certificates can be submitted (state=%s)", st)
	}
	if cfnum(c["amount_this_period"]) <= 0 {
		return "", fmt.Errorf("amount_this_period must be > 0")
	}
	_ = orm.UpdateRecordByID(bypass, certModel, id, map[string]interface{}{"state": "submitted"})
	return "Certificate submitted for certification", nil
}

// certCertify: submitted → certified; records certifier + cumulative paid.
func certCertify(ctx context.Context, _ string, id int, args map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := cload(bypass, certModel, id)
	if c == nil {
		return "", fmt.Errorf("certificate not found")
	}
	if st := orm.AsString(c["state"]); st != "submitted" {
		return "", fmt.Errorf("only submitted certificates can be certified (state=%s)", st)
	}
	// cumulative paid = sum of prior certified/paid certs on this subcontract + this one
	subID := coid(c["subcontract_id"])
	prior, _ := orm.SearchPage(bypass, certModel,
		[][]interface{}{{"subcontract_id", "=", subID}, {"id", "!=", id},
			{"state", "in", []interface{}{"certified", "paid"}}}, 500, 0, "id asc")
	cum := cfnum(c["net_payable"])
	for _, p := range prior {
		cum += cfnum(p["net_payable"])
	}
	_ = orm.UpdateRecordByID(bypass, certModel, id, map[string]interface{}{
		"state":           "certified",
		"certified_by_id": args["user_id"],
		"cumulative_paid": cum,
	})
	return fmt.Sprintf("Certificate certified — net %.2f, cumulative %.2f", cfnum(c["net_payable"]), cum), nil
}

// certPay: certified → paid with timestamp.
func certPay(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	c := cload(bypass, certModel, id)
	if c == nil {
		return "", fmt.Errorf("certificate not found")
	}
	if st := orm.AsString(c["state"]); st != "certified" {
		return "", fmt.Errorf("only certified certificates can be paid (state=%s)", st)
	}
	_ = orm.UpdateRecordByID(bypass, certModel, id, map[string]interface{}{
		"state":   "paid",
		"paid_at": time.Now().UTC().Format(time.RFC3339),
	})
	return "Certificate paid", nil
}
