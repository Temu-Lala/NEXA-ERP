package models

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// Transaction behaviour: disbursement confirmation (posting funds to the loan
// account), a repayment allocation waterfall that applies cash to the approved
// schedule oldest-installment-first (penalty -> fees -> interest -> principal),
// and reversals that roll those allocations back. Every money event re-rolls the
// anchor loan.account subledger balances. Manager-gated, idempotent, and guarded
// against double-confirmation and reversed records.

func init() {
	orm.RegisterCompute("loan.repayment", "allocated_total", nil, computeRepaymentAllocatedTotal)

	orm.RegisterObjectAction("loan.disbursement", "action_disbursement_confirm", disbursementConfirm)
	orm.RegisterObjectAction("loan.disbursement", "action_disbursement_reverse", disbursementReverse)

	orm.RegisterObjectAction("loan.repayment", "action_repayment_allocate", repaymentAllocate)
	orm.RegisterObjectAction("loan.repayment", "action_repayment_reverse", repaymentReverse)

	orm.RegisterObjectAction("loan.payment.reversal", "action_reversal_post", reversalPost)
}

// ---------------------------------------------------------------------------
// Computes
// ---------------------------------------------------------------------------

func computeRepaymentAllocatedTotal(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	id, ok := orm.CoerceInt64(rec["id"])
	if !ok || id <= 0 {
		return 0.0, nil
	}
	allocs, err := orm.Search(orm.ContextWithBypass(ctx, true), "loan.repayment.allocation",
		[][]interface{}{{"repayment_id", "=", int(id)}})
	if err != nil {
		return nil, err
	}
	var total float64
	for _, a := range allocs {
		total += numericOf(a["total_allocated"])
	}
	return round2(total), nil
}

// ---------------------------------------------------------------------------
// Disbursement
// ---------------------------------------------------------------------------

func disbursementConfirm(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
		return "", err
	}
	if !orm.UserHasGroupXML(ctx, uid, "lending.group_lending_manager") {
		return "", fmt.Errorf("only a Lending Manager can confirm a disbursement")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
	if err != nil {
		return "", fmt.Errorf("disbursement not found")
	}
	switch DisbursementStatus(orm.AsString(rec["status"])) {
	case DisbursementStatusConfirmed:
		return "", nil
	case DisbursementStatusReversed, DisbursementStatusFailed:
		return "", fmt.Errorf("a %s disbursement cannot be confirmed", orm.AsString(rec["status"]))
	}
	acctID := intOr(rec["account_id"], 0)
	if acctID <= 0 {
		return "", fmt.Errorf("disbursement has no loan account")
	}
	net := numericOf(rec["net_amount"])
	acct, err := orm.SearchOne(bypass, "loan.account", map[string]interface{}{"id": acctID})
	if err != nil {
		return "", fmt.Errorf("loan account not found")
	}
	newDisbursed := round2(numericOf(acct["disbursed_amount"]) + net)
	if err := orm.UpdateRecordByID(bypass, "loan.account", acctID, map[string]interface{}{"disbursed_amount": newDisbursed}); err != nil {
		return "", err
	}
	if err := orm.UpdateRecordByID(bypass, model, id, map[string]interface{}{
		"status":       string(DisbursementStatusConfirmed),
		"disbursed_at": nowDateTime(),
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("/web?action=lending.action_loan_account&view_type=form&id=%d", acctID), nil
}

func disbursementReverse(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
		return "", err
	}
	if !orm.UserHasGroupXML(ctx, uid, "lending.group_lending_manager") {
		return "", fmt.Errorf("only a Lending Manager can reverse a disbursement")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
	if err != nil {
		return "", fmt.Errorf("disbursement not found")
	}
	if DisbursementStatus(orm.AsString(rec["status"])) != DisbursementStatusConfirmed {
		return "", fmt.Errorf("only a confirmed disbursement can be reversed")
	}
	acctID := intOr(rec["account_id"], 0)
	if acctID > 0 {
		if acct, err := orm.SearchOne(bypass, "loan.account", map[string]interface{}{"id": acctID}); err == nil {
			newDisbursed := round2(numericOf(acct["disbursed_amount"]) - numericOf(rec["net_amount"]))
			if newDisbursed < 0 {
				newDisbursed = 0
			}
			_ = orm.UpdateRecordByID(bypass, "loan.account", acctID, map[string]interface{}{"disbursed_amount": newDisbursed})
		}
	}
	if err := orm.UpdateRecordByID(bypass, model, id, map[string]interface{}{"status": string(DisbursementStatusReversed)}); err != nil {
		return "", err
	}
	return "", nil
}

// ---------------------------------------------------------------------------
// Repayment allocation waterfall
// ---------------------------------------------------------------------------

// repaymentAllocate applies the received cash across the approved schedule's
// unpaid installments (oldest first), splitting into penalty/fees/interest/principal,
// writing allocation lines and updating installment + account balances.
func repaymentAllocate(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
		return "", err
	}
	if !orm.UserHasGroupXML(ctx, uid, "lending.group_lending_manager") {
		return "", fmt.Errorf("only a Lending Manager can allocate a repayment")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rp, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
	if err != nil {
		return "", fmt.Errorf("repayment not found")
	}
	if RepaymentStatus(orm.AsString(rp["status"])) == RepaymentStatusReversed {
		return "", fmt.Errorf("a reversed repayment cannot be allocated")
	}
	acctID := intOr(rp["account_id"], 0)
	if acctID <= 0 {
		return "", fmt.Errorf("repayment has no loan account")
	}
	remaining := numericOf(rp["amount_received"])
	if remaining <= 0.005 {
		return "", fmt.Errorf("repayment has no amount received to allocate")
	}

	// Start from a clean slate so re-allocation is idempotent.
	if err := rollbackAllocations(bypass, id); err != nil {
		return "", err
	}

	insts, err := scheduleInstallments(bypass, acctID)
	if err != nil {
		return "", err
	}
	if len(insts) == 0 {
		return "", fmt.Errorf("loan account has no approved schedule to allocate against")
	}

	allocModel := orm.Registry["loan.repayment.allocation"]
	if allocModel == nil {
		return "", fmt.Errorf("loan.repayment.allocation model is not available")
	}
	rpDate := normalizeDate(strOr(rp["date"], time.Now().Format("2006-01-02")))

	allocated := 0.0
	for _, in := range insts {
		if remaining <= 0.005 {
			break
		}
		iid := intOr(in["id"], 0)
		if iid <= 0 {
			continue
		}
		penDue := maxf(numericOf(in["penalty_due"])-numericOf(in["paid_penalty"]), 0)
		feeDue := maxf(numericOf(in["fees_due"])-numericOf(in["paid_fees"]), 0)
		intDue := maxf(numericOf(in["interest_due"])-numericOf(in["paid_interest"]), 0)
		priDue := maxf(numericOf(in["principal_due"])-numericOf(in["paid_principal"]), 0)
		if penDue+feeDue+intDue+priDue <= 0.005 {
			continue
		}

		aPen := take(&remaining, penDue)
		aFee := take(&remaining, feeDue)
		aInt := take(&remaining, intDue)
		aPri := take(&remaining, priDue)
		thisAlloc := aPen + aFee + aInt + aPri
		if thisAlloc <= 0.005 {
			continue
		}
		allocated += thisAlloc

		if _, err := orm.Create(bypass, allocModel, map[string]interface{}{
			"repayment_id":     id,
			"installment_id":   iid,
			"amount_penalty":   round2(aPen),
			"amount_fees":      round2(aFee),
			"amount_interest":  round2(aInt),
			"amount_principal": round2(aPri),
			"total_allocated":  round2(thisAlloc),
			"date":             rpDate,
		}); err != nil {
			return "", fmt.Errorf("create allocation for installment %d: %w", iid, err)
		}

		newPaidPen := numericOf(in["paid_penalty"]) + aPen
		newPaidFee := numericOf(in["paid_fees"]) + aFee
		newPaidInt := numericOf(in["paid_interest"]) + aInt
		newPaidPri := numericOf(in["paid_principal"]) + aPri
		newTotalPaid := newPaidPen + newPaidFee + newPaidInt + newPaidPri
		state := InstallmentStateFromPaid(numericOf(in["total_due"]), newTotalPaid, normalizeDate(orm.AsString(in["due_date"])))
		if err := orm.UpdateRecordByID(bypass, "loan.installment", iid, map[string]interface{}{
			"paid_penalty":   round2(newPaidPen),
			"paid_fees":      round2(newPaidFee),
			"paid_interest":  round2(newPaidInt),
			"paid_principal": round2(newPaidPri),
			"total_paid":     round2(newTotalPaid),
			"state":          string(state),
		}); err != nil {
			return "", err
		}
		in["paid_penalty"], in["paid_fees"], in["paid_interest"], in["paid_principal"] = newPaidPen, newPaidFee, newPaidInt, newPaidPri
		in["total_paid"], in["state"] = newTotalPaid, string(state)
	}

	// Reflect a late installment that received a partial payment as overdue.
	refreshOverdueStates(bypass, acctID)

	if err := orm.UpdateRecordByID(bypass, model, id, map[string]interface{}{
		"status":           string(RepaymentStatusConfirmed),
		"amount_confirmed": round2(allocated),
		"unallocated":      round2(maxf(remaining, 0)),
	}); err != nil {
		return "", err
	}
	if err := recomputeAccountBalances(bypass, acctID); err != nil {
		return "", err
	}
	return fmt.Sprintf("/web?action=lending.action_loan_account&view_type=form&id=%d", acctID), nil
}

// repaymentReverse rolls back every allocation of a repayment and re-rolls the
// account balances.
func repaymentReverse(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
		return "", err
	}
	if !orm.UserHasGroupXML(ctx, uid, "lending.group_lending_manager") {
		return "", fmt.Errorf("only a Lending Manager can reverse a repayment")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rp, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
	if err != nil {
		return "", fmt.Errorf("repayment not found")
	}
	if RepaymentStatus(orm.AsString(rp["status"])) == RepaymentStatusReversed {
		return "", nil
	}
	acctID := intOr(rp["account_id"], 0)
	if err := rollbackAllocations(bypass, id); err != nil {
		return "", err
	}
	if err := orm.UpdateRecordByID(bypass, model, id, map[string]interface{}{
		"status":           string(RepaymentStatusReversed),
		"amount_confirmed": 0.0,
		"unallocated":      numericOf(rp["amount_received"]),
	}); err != nil {
		return "", err
	}
	if acctID > 0 {
		refreshOverdueStates(bypass, acctID)
		if err := recomputeAccountBalances(bypass, acctID); err != nil {
			return "", err
		}
	}
	return "", nil
}

// ---------------------------------------------------------------------------
// Payment reversal record
// ---------------------------------------------------------------------------

// reversalPost posts a reversal record by reversing its underlying repayment.
func reversalPost(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
		return "", err
	}
	if !orm.UserHasGroupXML(ctx, uid, "lending.group_lending_manager") {
		return "", fmt.Errorf("only a Lending Manager can post a reversal")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rev, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
	if err != nil {
		return "", fmt.Errorf("reversal not found")
	}
	if ReversalStatus(orm.AsString(rev["status"])) == ReversalStatusPosted {
		return "", nil
	}
	rpID := intOr(rev["repayment_id"], 0)
	if rpID <= 0 {
		return "", fmt.Errorf("reversal has no repayment")
	}
	if _, err := repaymentReverse(bypass, "loan.repayment", rpID, nil); err != nil {
		return "", err
	}
	upd := map[string]interface{}{"status": string(ReversalStatusPosted), "reversed_by": uid}
	if err := orm.UpdateRecordByID(bypass, model, id, upd); err != nil {
		return "", err
	}
	return "", nil
}

// ---------------------------------------------------------------------------
// Allocation helpers
// ---------------------------------------------------------------------------

// rollbackAllocations subtracts each allocation from its installment and deletes
// the allocation rows for a repayment.
func rollbackAllocations(ctx context.Context, repaymentID int) error {
	allocs, err := orm.Search(ctx, "loan.repayment.allocation", [][]interface{}{{"repayment_id", "=", repaymentID}})
	if err != nil {
		return err
	}
	for _, a := range allocs {
		iid := intOr(a["installment_id"], 0)
		if iid <= 0 {
			continue
		}
		in, err := orm.SearchOne(ctx, "loan.installment", map[string]interface{}{"id": iid})
		if err != nil {
			continue
		}
		newPaidPen := maxf(numericOf(in["paid_penalty"])-numericOf(a["amount_penalty"]), 0)
		newPaidFee := maxf(numericOf(in["paid_fees"])-numericOf(a["amount_fees"]), 0)
		newPaidInt := maxf(numericOf(in["paid_interest"])-numericOf(a["amount_interest"]), 0)
		newPaidPri := maxf(numericOf(in["paid_principal"])-numericOf(a["amount_principal"]), 0)
		newTotal := newPaidPen + newPaidFee + newPaidInt + newPaidPri
		state := InstallmentStateFromPaid(numericOf(in["total_due"]), newTotal, normalizeDate(orm.AsString(in["due_date"])))
		if err := orm.UpdateRecordByID(ctx, "loan.installment", iid, map[string]interface{}{
			"paid_penalty": round2(newPaidPen), "paid_fees": round2(newPaidFee),
			"paid_interest": round2(newPaidInt), "paid_principal": round2(newPaidPri),
			"total_paid": round2(newTotal), "state": string(state),
		}); err != nil {
			return err
		}
	}
	if len(allocs) > 0 {
		if _, err := orm.UnlinkWhere(ctx, "loan.repayment.allocation", [][]interface{}{{"repayment_id", "=", repaymentID}}); err != nil {
			return err
		}
	}
	return nil
}

// scheduleInstallments returns the approved schedule's installments for an account,
// ordered by period number.
func scheduleInstallments(ctx context.Context, acctID int) ([]map[string]interface{}, error) {
	scheds, err := orm.Search(ctx, "loan.schedule.version", [][]interface{}{
		{"account_id", "=", acctID},
		{"status", "=", "approved"},
	})
	if err != nil {
		return nil, err
	}
	if len(scheds) == 0 {
		return nil, nil
	}
	schedID := intOr(scheds[0]["id"], 0)
	insts, err := orm.Search(ctx, "loan.installment", [][]interface{}{{"schedule_id", "=", schedID}})
	if err != nil {
		return nil, err
	}
	sort.SliceStable(insts, func(i, j int) bool {
		return intOr(insts[i]["number"], 0) < intOr(insts[j]["number"], 0)
	})
	return insts, nil
}

// InstallmentStateFromPaid derives an installment state from its payment progress.
func InstallmentStateFromPaid(totalDue, totalPaid float64, dueDate string) string {
	rem := totalDue - totalPaid
	if rem <= 0.005 {
		return "paid"
	}
	if totalPaid > 0.005 {
		return "partial"
	}
	if dueDate != "" && dueDate < time.Now().Format("2006-01-02") {
		return "overdue"
	}
	return "due"
}

// refreshOverdueStates marks unpaid installments past their due date as overdue.
func refreshOverdueStates(ctx context.Context, acctID int) {
	insts, err := scheduleInstallments(ctx, acctID)
	if err != nil {
		return
	}
	today := time.Now().Format("2006-01-02")
	for _, in := range insts {
		rem := numericOf(in["total_due"]) - numericOf(in["total_paid"])
		if rem <= 0.005 {
			continue
		}
		due := normalizeDate(orm.AsString(in["due_date"]))
		st := orm.AsString(in["state"])
		if due != "" && due < today && st != "overdue" {
			_ = orm.UpdateRecordByID(ctx, "loan.installment", intOr(in["id"], 0), map[string]interface{}{"state": "overdue"})
		}
	}
}

// recomputeAccountBalances re-rolls the anchor loan account's subledger balances
// from the approved schedule and recorded payments.
func recomputeAccountBalances(ctx context.Context, acctID int) error {
	insts, err := scheduleInstallments(ctx, acctID)
	if err != nil {
		return err
	}
	if len(insts) == 0 {
		return nil
	}
	var outstanding, accrued, paidTotal float64
	var nextDue, earliestOverdue string
	today := time.Now().Format("2006-01-02")
	for _, in := range insts {
		paidTotal += numericOf(in["total_paid"])
		if remP := numericOf(in["principal_due"]) - numericOf(in["paid_principal"]); remP > 0 {
			outstanding += remP
		}
		if remI := numericOf(in["interest_due"]) - numericOf(in["paid_interest"]); remI > 0 {
			accrued += remI
		}
		if numericOf(in["total_due"])-numericOf(in["total_paid"]) <= 0.005 {
			continue
		}
		due := normalizeDate(orm.AsString(in["due_date"]))
		if due == "" {
			continue
		}
		if nextDue == "" || due < nextDue {
			nextDue = due
		}
		if due < today && (earliestOverdue == "" || due < earliestOverdue) {
			earliestOverdue = due
		}
	}
	dpd := 0
	if earliestOverdue != "" {
		dpd = daysBetween(earliestOverdue, today)
		if dpd < 0 {
			dpd = 0
		}
	}
	upd := map[string]interface{}{
		"outstanding_principal": round2(outstanding),
		"accrued_interest":      round2(accrued),
		"paid_total":            round2(paidTotal),
		"days_past_due":         dpd,
	}
	if nextDue != "" {
		upd["next_due_date"] = nextDue
	}
	return orm.UpdateRecordByID(ctx, "loan.account", acctID, upd)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func take(remaining *float64, due float64) float64 {
	if due <= 0.005 || *remaining <= 0.005 {
		return 0
	}
	if *remaining >= due {
		*remaining -= due
		return due
	}
	v := *remaining
	*remaining = 0
	return v
}

func maxf(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

func nowDateTime() string {
	return time.Now().UTC().Format("2006-01-02 15:04:05")
}

func round2(f float64) float64 {
	v, _ := strconv.ParseFloat(fmt.Sprintf("%.2f", f), 64)
	return v
}

func intOr(v interface{}, def int) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return def
	}
	return int(n)
}

func strOr(v interface{}, def string) string {
	s := strings.TrimSpace(orm.AsString(v))
	if s == "" {
		return def
	}
	return s
}

func numericOf(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case float32:
		return float64(t)
	case int64:
		return float64(t)
	case int:
		return float64(t)
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f
	default:
		return 0
	}
}

func normalizeDate(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 10 {
		s = s[:10]
	}
	if s == "" {
		return ""
	}
	if _, err := time.Parse("2006-01-02", s); err != nil {
		return ""
	}
	return s
}

func daysBetween(fromDate, toDate string) int {
	f, err := time.Parse("2006-01-02", fromDate)
	if err != nil {
		return 0
	}
	t, err := time.Parse("2006-01-02", toDate)
	if err != nil {
		return 0
	}
	return int(t.Sub(f).Hours() / 24)
}
