package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nexa/core/orm"
)

// Payroll-hardening workflow actions. Every money-touching step is gated:
//
//   - draft-only recompute / line generation, blocked outright once a run is
//     frozen (approved);
//   - maker-checker approval (the submitter cannot approve their own run) that
//     also freezes the run;
//   - idempotent payment (a payslip can only be disbursed once);
//   - traceable adjustments that POST a new line instead of editing a frozen slip;
//   - manager-only payslip release to the employee.
//
// Invalid transitions return a clear message and change nothing.

const (
	payslipModel   = "hr.payslip"
	lineModel      = "hr.payslip.line"
	runModel       = "hr.payslip.run"
	paymentModel   = "hr.payslip.payment"
	adjustModel    = "hr.payslip.adjustment"
	fileModel      = "hr.payroll.payment.file"
	reconModel     = "hr.payroll.reconciliation"
	ruleModel      = "hr.salary.rule"
	statutoryModel = "hr.payroll.statutory"
	categoryModel  = "hr.payroll.category"
)

func init() {
	orm.RegisterObjectAction(payslipModel, "action_compute", payCompute)
	orm.RegisterObjectAction(payslipModel, "action_generate_lines", payGenerateLines)
	orm.RegisterObjectAction(payslipModel, "action_release", payRelease)

	orm.RegisterObjectAction(runModel, "action_approve", runApprove)
	orm.RegisterObjectAction(runModel, "action_refuse", runRefuse)
	orm.RegisterObjectAction(runModel, "action_generate_file", runGenerateFile)

	orm.RegisterObjectAction(paymentModel, "action_pay", paymentPay)
	orm.RegisterObjectAction(paymentModel, "action_void", paymentVoid)

	orm.RegisterObjectAction(adjustModel, "action_apply", adjustmentApply)
	orm.RegisterObjectAction(adjustModel, "action_refuse", adjustmentRefuse)

	orm.RegisterObjectAction(reconModel, "action_refresh", reconRefresh)
}

// ---------------------------------------------------------------------------
// Payslip: totals + generated lines
// ---------------------------------------------------------------------------

func payCompute(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, payslipModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("payslip not found")
	}
	if err := guardEditableDraft(bypass, rec); err != nil {
		return "", err
	}
	gross, deduction := totalsFromLines(bypass, id)
	net := netOf(gross, deduction)
	return "", orm.UpdateRecordByID(bypass, payslipModel, id, map[string]interface{}{
		"gross":           gross,
		"total_deduction": deduction,
		"net":             net,
	})
}

func payGenerateLines(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, payslipModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("payslip not found")
	}
	if err := guardEditableDraft(bypass, rec); err != nil {
		return "", err
	}
	basic := floatOr(rec["basic"])
	date := dateStr(rec["date_start"])
	structureID := intOr(rec["structure_id"])

	runningGross := basic // percentage_gross rules resolve against accrued gross

	domain := [][]interface{}{{"active", "=", true}}
	if structureID > 0 {
		domain = append(domain, []interface{}{"structure_id", "=", structureID})
	}
	rules, err := orm.Search(bypass, ruleModel, domain)
	if err != nil {
		return "", err
	}
	for _, rule := range rules {
		if !activeInPeriod(dateStr(rule["valid_from"]), dateStr(rule["valid_to"]), date) {
			continue
		}
		amount := ruleAmount(orm.AsString(rule["amount_type"]),
			floatOr(rule["amount_percent"]), floatOr(rule["amount_fixed"]), basic, runningGross)
		catID := intOr(rule["category_id"])
		if categoryKind(bypass, catID) == "earning" {
			runningGross += amount
		}
		if err := upsertRuleLine(bypass, id, intOr(rule["id"]), catID,
			orm.AsString(rule["name"]), intOr(rule["sequence"]), amount); err != nil {
			return "", err
		}
	}

	statutes, err := orm.Search(bypass, statutoryModel, [][]interface{}{{"active", "=", true}})
	if err != nil {
		return "", err
	}
	for _, st := range statutes {
		if !activeInPeriod(dateStr(st["valid_from"]), dateStr(st["valid_to"]), date) {
			continue
		}
		amount := statutoryAmount(orm.AsString(st["amount_type"]), orm.AsString(st["applies_to"]),
			floatOr(st["percent"]), floatOr(st["fixed"]), floatOr(st["cap"]), basic, runningGross)
		if err := upsertStatLine(bypass, id, intOr(st["id"]), intOr(st["category_id"]),
			orm.AsString(st["name"]), intOr(st["sequence"]), amount); err != nil {
			return "", err
		}
	}

	gross, deduction := totalsFromLines(bypass, id)
	net := netOf(gross, deduction)
	return "", orm.UpdateRecordByID(bypass, payslipModel, id, map[string]interface{}{
		"gross":           gross,
		"total_deduction": deduction,
		"net":             net,
	})
}

func payRelease(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can release a payslip to the employee")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, payslipModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("payslip not found")
	}
	run := loadRun(bypass, intOr(rec["run_id"]))
	if run == nil || !orm.AsBool(run["approved"]) || !orm.AsBool(run["frozen"]) {
		return "", fmt.Errorf("the payslip's run must be approved and frozen before release")
	}
	return "", orm.UpdateRecordByID(bypass, payslipModel, id, map[string]interface{}{
		"released_to_employee": true,
		"released_on":          time.Now().UTC().Format(datetimeFull),
	})
}

// ---------------------------------------------------------------------------
// Run: maker-checker approval + freeze, payment-file generation
// ---------------------------------------------------------------------------

func runApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can approve a payroll run")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	run, err := orm.SearchOne(bypass, runModel, map[string]interface{}{"id": id})
	if err != nil || run == nil {
		return "", fmt.Errorf("run not found")
	}
	if orm.AsString(run["state"]) != "confirmed" {
		return "", fmt.Errorf("only a confirmed run can be approved")
	}
	if creator := intOr(run["create_uid"]); creator != 0 && creator == uid {
		return "", fmt.Errorf("maker-checker: the run submitter cannot approve their own run")
	}
	return "", orm.UpdateRecordByID(bypass, runModel, id, map[string]interface{}{
		"approved":    true,
		"frozen":      true,
		"approver_id": uid,
		"approved_on": time.Now().UTC().Format(datetimeFull),
	})
}

func runRefuse(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can refuse a payroll run")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	run, err := orm.SearchOne(bypass, runModel, map[string]interface{}{"id": id})
	if err != nil || run == nil {
		return "", fmt.Errorf("run not found")
	}
	if orm.AsString(run["state"]) == "closed" {
		return "", fmt.Errorf("a closed run cannot be refused")
	}
	return "", orm.UpdateRecordByID(bypass, runModel, id, map[string]interface{}{
		"approved":    false,
		"frozen":      false,
		"approver_id": uid,
	})
}

// runGenerateFile builds a transfer batch from an approved, frozen run. It is
// idempotent: payslips that already have a payment row are skipped.
func runGenerateFile(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can generate a payment file")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	run, err := orm.SearchOne(bypass, runModel, map[string]interface{}{"id": id})
	if err != nil || run == nil {
		return "", fmt.Errorf("run not found")
	}
	if !orm.AsBool(run["approved"]) || !orm.AsBool(run["frozen"]) {
		return "", fmt.Errorf("the run must be approved and frozen before generating a payment file")
	}
	slips, err := orm.Search(bypass, payslipModel, [][]interface{}{{"run_id", "=", id}})
	if err != nil {
		return "", err
	}
	var total float64
	count := 0
	for _, s := range slips {
		if hasPayment(bypass, intOr(s["id"])) {
			continue
		}
		net := floatOr(s["net"])
		if net <= 0 {
			continue
		}
		ref := fmt.Sprintf("PAY-%d-%d", id, intOr(s["id"]))
		if _, err := createRecord(bypass, paymentModel, map[string]interface{}{
			"payslip_id":  intOr(s["id"]),
			"employee_id": intOr(s["employee_id"]),
			"run_id":      id,
			"amount":      net,
			"payment_ref": ref,
			"date":        dateStr(run["period_end"]),
			"state":       string(PayDraft),
		}); err != nil {
			return "", err
		}
		total += net
		count++
	}
	fileID, err := createRecord(bypass, fileModel, map[string]interface{}{
		"name":         fmt.Sprintf("Payment file - %s", orm.AsString(run["name"])),
		"run_id":       id,
		"generated_on": time.Now().UTC().Format(datetimeFull),
		"total_amount": roundMoney(total),
		"line_count":   count,
		"state":        string(FileDraft),
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("payment file #%d created with %d line(s)", fileID, count), nil
}

// ---------------------------------------------------------------------------
// Payment: idempotent disbursement
// ---------------------------------------------------------------------------

func paymentPay(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can mark a payment as paid")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	pay, err := orm.SearchOne(bypass, paymentModel, map[string]interface{}{"id": id})
	if err != nil || pay == nil {
		return "", fmt.Errorf("payment not found")
	}
	if orm.AsString(pay["state"]) == string(PayPaid) {
		return "", fmt.Errorf("this payment is already marked paid")
	}
	if orm.AsString(pay["state"]) == string(PayVoid) {
		return "", fmt.Errorf("a voided payment cannot be paid")
	}
	payslipID := intOr(pay["payslip_id"])
	// Idempotency: reject if another paid row already exists for this payslip.
	if alreadyPaid(bypass, payslipID) {
		return "", fmt.Errorf("payslip %d has already been disbursed (duplicate payment blocked)", payslipID)
	}
	if err := orm.UpdateRecordByID(bypass, paymentModel, id, map[string]interface{}{
		"state": string(PayPaid),
	}); err != nil {
		return "", err
	}
	return "", orm.UpdateRecordByID(bypass, payslipModel, payslipID, map[string]interface{}{
		"paid":        true,
		"paid_on":     time.Now().UTC().Format(datetimeFull),
		"payment_ref": orm.AsString(pay["payment_ref"]),
	})
}

func paymentVoid(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can void a payment")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	pay, err := orm.SearchOne(bypass, paymentModel, map[string]interface{}{"id": id})
	if err != nil || pay == nil {
		return "", fmt.Errorf("payment not found")
	}
	if orm.AsString(pay["state"]) == string(PayPaid) {
		return "", fmt.Errorf("a paid payment cannot be voided; post a reversal adjustment instead")
	}
	return "", orm.UpdateRecordByID(bypass, paymentModel, id, map[string]interface{}{"state": string(PayVoid)})
}

// ---------------------------------------------------------------------------
// Adjustment: traceable post-approval correction
// ---------------------------------------------------------------------------

func adjustmentApply(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can apply a payroll adjustment")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	adj, err := orm.SearchOne(bypass, adjustModel, map[string]interface{}{"id": id})
	if err != nil || adj == nil {
		return "", fmt.Errorf("adjustment not found")
	}
	if orm.AsString(adj["state"]) == string(AdjValidate) {
		return "", fmt.Errorf("this adjustment is already applied")
	}
	payslipID := intOr(adj["payslip_id"])
	amount := floatOr(adj["amount"])
	if amount <= 0 {
		return "", fmt.Errorf("adjustment amount must be positive")
	}
	if orm.AsString(adj["direction"]) == string(AdjDecrease) {
		amount = -amount
	}
	name := fmt.Sprintf("Adjustment #%d", id)
	catID := intOr(adj["category_id"])
	if _, err := createRecord(bypass, lineModel, map[string]interface{}{
		"payslip_id":  payslipID,
		"name":        name,
		"category_id": catID,
		"sequence":    900,
		"amount":      roundMoney(amount),
	}); err != nil {
		return "", err
	}
	gross, deduction := totalsFromLines(bypass, payslipID)
	net := netOf(gross, deduction)
	if err := orm.UpdateRecordByID(bypass, payslipModel, payslipID, map[string]interface{}{
		"gross":           gross,
		"total_deduction": deduction,
		"net":             net,
	}); err != nil {
		return "", err
	}
	return "", orm.UpdateRecordByID(bypass, adjustModel, id, map[string]interface{}{
		"state":   string(AdjValidate),
		"applied": true,
	})
}

func adjustmentRefuse(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if !orm.UserHasGroupXML(ctx, uid, "hr.group_hr_manager") {
		return "", fmt.Errorf("only an HR Manager can refuse a payroll adjustment")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	adj, err := orm.SearchOne(bypass, adjustModel, map[string]interface{}{"id": id})
	if err != nil || adj == nil {
		return "", fmt.Errorf("adjustment not found")
	}
	if orm.AsString(adj["state"]) == string(AdjValidate) {
		return "", fmt.Errorf("an applied adjustment cannot be refused")
	}
	return "", orm.UpdateRecordByID(bypass, adjustModel, id, map[string]interface{}{"state": string(AdjRefuse)})
}

// ---------------------------------------------------------------------------
// Reconciliation refresh
// ---------------------------------------------------------------------------

func reconRefresh(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, reconModel, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return "", fmt.Errorf("reconciliation not found")
	}
	expected := roundMoney(sumPayslipNet(bypass, intOr(rec["run_id"])))
	paid := roundMoney(sumPaidPayments(bypass, intOr(rec["run_id"])))
	diff := roundMoney(expected - paid)
	state := string(ReconException)
	if diff <= moneyEpsilon && diff >= -moneyEpsilon {
		state = string(ReconBalanced)
	}
	return "", orm.UpdateRecordByID(bypass, reconModel, id, map[string]interface{}{
		"expected_net": expected,
		"paid_net":     paid,
		"difference":   diff,
		"state":        state,
	})
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// guardEditableDraft blocks edits once a payslip leaves draft or its run freezes.
func guardEditableDraft(ctx context.Context, slip map[string]interface{}) error {
	if orm.AsString(slip["state"]) != "draft" {
		return fmt.Errorf("only a draft payslip can be recalculated")
	}
	if run := loadRun(ctx, intOr(slip["run_id"])); run != nil && orm.AsBool(run["frozen"]) {
		return fmt.Errorf("the payslip's run is frozen (approved); use an adjustment instead")
	}
	return nil
}

func loadRun(ctx context.Context, runID int) map[string]interface{} {
	if runID <= 0 {
		return nil
	}
	run, err := orm.SearchOne(ctx, runModel, map[string]interface{}{"id": runID})
	if err != nil || run == nil {
		return nil
	}
	return run
}

// totalsFromLines sums a payslip's lines by their category kind.
func totalsFromLines(ctx context.Context, payslipID int) (gross, deduction float64) {
	lines, err := orm.Search(ctx, lineModel, [][]interface{}{{"payslip_id", "=", payslipID}})
	if err != nil {
		return 0, 0
	}
	for _, l := range lines {
		amt := floatOr(l["amount"])
		switch categoryKind(ctx, intOr(l["category_id"])) {
		case "deduction":
			deduction += amt
		default: // earning (and uncategorised lines add to gross)
			gross += amt
		}
	}
	return roundMoney(gross), roundMoney(deduction)
}

var kindCache = map[int]string{}

func categoryKind(ctx context.Context, catID int) string {
	if catID <= 0 {
		return ""
	}
	if k, ok := kindCache[catID]; ok {
		return k
	}
	cat, err := orm.SearchOne(ctx, categoryModel, map[string]interface{}{"id": catID})
	if err != nil || cat == nil {
		return ""
	}
	k := strings.TrimSpace(orm.AsString(cat["kind"]))
	kindCache[catID] = k
	return k
}

func upsertRuleLine(ctx context.Context, payslipID, ruleID, catID int, name string, seq int, amount float64) error {
	existing, err := orm.SearchPage(ctx, lineModel, [][]interface{}{
		{"payslip_id", "=", payslipID},
		{"rule_id", "=", ruleID},
	}, 1, 0, "id ASC")
	if err != nil {
		return err
	}
	vals := map[string]interface{}{
		"name":        name,
		"category_id": catID,
		"sequence":    seq,
		"amount":      roundMoney(amount),
	}
	if len(existing) > 0 {
		vals["payslip_id"] = payslipID
		vals["rule_id"] = ruleID
		return orm.UpdateRecordByID(ctx, lineModel, intOr(existing[0]["id"]), vals)
	}
	vals["payslip_id"] = payslipID
	vals["rule_id"] = ruleID
	_, err = createRecord(ctx, lineModel, vals)
	return err
}

func upsertStatLine(ctx context.Context, payslipID, statID, catID int, name string, seq int, amount float64) error {
	lineName := statPrefix + fmt.Sprintf("%d", statID)
	existing, err := orm.SearchPage(ctx, lineModel, [][]interface{}{
		{"payslip_id", "=", payslipID},
		{"name", "=", lineName},
	}, 1, 0, "id ASC")
	if err != nil {
		return err
	}
	vals := map[string]interface{}{
		"name":        lineName,
		"category_id": catID,
		"sequence":    seq + 500,
		"amount":      roundMoney(amount),
		"payslip_id":  payslipID,
	}
	if len(existing) > 0 {
		return orm.UpdateRecordByID(ctx, lineModel, intOr(existing[0]["id"]), vals)
	}
	_, err = createRecord(ctx, lineModel, vals)
	return err
}

func hasPayment(ctx context.Context, payslipID int) bool {
	n, err := orm.SearchCount(ctx, paymentModel, [][]interface{}{
		{"payslip_id", "=", payslipID},
		{"state", "!=", string(PayVoid)},
	})
	return err == nil && n > 0
}

func alreadyPaid(ctx context.Context, payslipID int) bool {
	n, err := orm.SearchCount(ctx, paymentModel, [][]interface{}{
		{"payslip_id", "=", payslipID},
		{"state", "=", string(PayPaid)},
	})
	return err == nil && n > 0
}

func createRecord(ctx context.Context, model string, vals map[string]interface{}) (int, error) {
	inst, ok := orm.Registry[model]
	if !ok {
		return 0, fmt.Errorf("model %s is not registered", model)
	}
	return orm.Create(ctx, inst, vals)
}
