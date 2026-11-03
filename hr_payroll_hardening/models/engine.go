package models

import (
	"context"
	"strings"

	"nexa/core/orm"
)

// Payroll-hardening behaviour: the pure gross-to-net maths and the reconciliation
// projections live here; the workflow object actions live in actions.go.
//
// All money is rounded to 2 decimals at each posting boundary, and every rule /
// statutory amount is derived by a pure function so it can be unit tested without
// a database.

const (
	moneyEpsilon = 0.005
	dateLayout   = "2006-01-02"
	datetimeFull = "2006-01-02 15:04:05"
	statPrefix   = "STAT:"
)

func init() {
	orm.RegisterCompute("hr.payroll.reconciliation", "expected_net", nil, computeReconExpected)
	orm.RegisterCompute("hr.payroll.reconciliation", "paid_net", nil, computeReconPaid)
	orm.RegisterCompute("hr.payroll.reconciliation", "difference", nil, computeReconDiff)
	orm.RegisterCompute("hr.payroll.reconciliation", "state", nil, computeReconState)
}

// ---------------------------------------------------------------------------
// Pure money maths
// ---------------------------------------------------------------------------

// roundMoney snaps a value to 2 decimals using integer-safe half-up rounding.
func roundMoney(f float64) float64 {
	scaled := f * 100
	if scaled >= 0 {
		return float64(int64(scaled+0.5)) / 100
	}
	return float64(int64(scaled-0.5)) / 100
}

// ruleAmount evaluates a salary rule against a payslip's basic / gross base.
func ruleAmount(amountType string, percent, fixed, basic, gross float64) float64 {
	switch amountType {
	case "percentage_basic":
		return roundMoney(basic * percent / 100)
	case "percentage_gross":
		return roundMoney(gross * percent / 100)
	default: // fixed
		return roundMoney(fixed)
	}
}

// statutoryAmount evaluates a statutory contribution against its chosen base,
// honouring an optional cap (0 = no cap).
func statutoryAmount(amountType, appliesTo string, percent, fixed, cap, basic, gross float64) float64 {
	base := gross
	if appliesTo == "basic" {
		base = basic
	}
	var amount float64
	switch amountType {
	case "fixed":
		amount = fixed
	default: // percentage
		amount = base * percent / 100
	}
	if cap > 0 && amount > cap {
		amount = cap
	}
	return roundMoney(amount)
}

// netOf is gross minus deductions (never below zero on the net floor of a slip).
func netOf(gross, deduction float64) float64 {
	net := gross - deduction
	if net < 0 {
		net = 0
	}
	return roundMoney(net)
}

// ---------------------------------------------------------------------------
// In-period selection
// ---------------------------------------------------------------------------

// activeInPeriod reports whether an effective-dated row covers the target date.
// An empty valid_from is treated as "from the beginning" and an empty valid_to as
// "open ended", so un-dated rows always qualify.
func activeInPeriod(validFrom, validTo, target string) bool {
	if validFrom != "" && target != "" && validFrom > target {
		return false
	}
	if validTo != "" && target != "" && validTo < target {
		return false
	}
	return true
}

func dateStr(v interface{}) string {
	s := strings.TrimSpace(orm.AsString(v))
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func floatOr(v interface{}) float64 {
	f, ok := orm.CoerceFloat64(v)
	if !ok {
		return 0
	}
	return f
}

// ---------------------------------------------------------------------------
// Reconciliation projections (cross-model reads; refreshed on demand)
// ---------------------------------------------------------------------------

func reconRunID(rec map[string]interface{}) int { return intOr(rec["run_id"]) }

func computeReconExpected(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return roundMoney(sumPayslipNet(ctx, reconRunID(rec))), nil
}

func computeReconPaid(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return roundMoney(sumPaidPayments(ctx, reconRunID(rec))), nil
}

func computeReconDiff(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	exp := floatOr(rec["expected_net"])
	paid := floatOr(rec["paid_net"])
	if exp == 0 && paid == 0 {
		exp = sumPayslipNet(ctx, reconRunID(rec))
		paid = sumPaidPayments(ctx, reconRunID(rec))
	}
	return roundMoney(exp - paid), nil
}

func computeReconState(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	diff := floatOr(rec["difference"])
	if diff == 0 {
		// difference may not yet be populated; derive from expected/paid columns.
		exp := floatOr(rec["expected_net"])
		paid := floatOr(rec["paid_net"])
		diff = exp - paid
	}
	if reconRunID(rec) <= 0 {
		return string(ReconDraft), nil
	}
	if diff <= moneyEpsilon && diff >= -moneyEpsilon {
		return string(ReconBalanced), nil
	}
	return string(ReconException), nil
}

// sumPayslipNet totals the net of every payslip belonging to a run.
func sumPayslipNet(ctx context.Context, runID int) float64 {
	if runID <= 0 {
		return 0
	}
	rows, err := orm.Search(orm.ContextWithBypass(ctx, true), "hr.payslip", [][]interface{}{
		{"run_id", "=", runID},
	})
	if err != nil {
		return 0
	}
	var s float64
	for _, r := range rows {
		s += floatOr(r["net"])
	}
	return s
}

// sumPaidPayments totals the amount of every PAID payment for a run.
func sumPaidPayments(ctx context.Context, runID int) float64 {
	if runID <= 0 {
		return 0
	}
	rows, err := orm.Search(orm.ContextWithBypass(ctx, true), "hr.payslip.payment", [][]interface{}{
		{"run_id", "=", runID},
		{"state", "=", string(PayPaid)},
	})
	if err != nil {
		return 0
	}
	var s float64
	for _, r := range rows {
		s += floatOr(r["amount"])
	}
	return s
}
