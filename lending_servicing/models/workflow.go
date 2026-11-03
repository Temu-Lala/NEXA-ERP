package models

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// Servicing behaviour: an amortization engine that materialises an installment
// schedule from the loan account's contractual terms, a version lifecycle
// (draft -> approved, superseding prior approved versions), and a balance
// recompute that rolls the schedule + payments back onto the anchor's
// loan.account subledger (outstanding principal, accrued interest, paid total,
// next due date, days past due). Non-stored computes expose per-installment
// remaining due and per-schedule progress.

func init() {
	// Display computes.
	orm.RegisterCompute("loan.installment", "amount_due", nil, computeInstallmentAmountDue)
	orm.RegisterCompute("loan.schedule.version", "installment_count", nil, computeScheduleInstallmentCount)
	orm.RegisterCompute("loan.schedule.version", "paid_count", nil, computeSchedulePaidCount)

	// Schedule lifecycle.
	orm.RegisterObjectAction("loan.schedule.version", "action_schedule_generate", scheduleGenerate)
	orm.RegisterObjectAction("loan.schedule.version", "action_schedule_approve", scheduleApprove)

	// Balance recompute onto the anchor loan account.
	orm.RegisterObjectAction("loan.account", "action_account_recompute", accountRecompute)
}

// ---------------------------------------------------------------------------
// Computes
// ---------------------------------------------------------------------------

func computeInstallmentAmountDue(_ context.Context, rec map[string]interface{}) (interface{}, error) {
	rem := numericOf(rec["total_due"]) - numericOf(rec["total_paid"])
	if rem < 0 {
		rem = 0
	}
	return round2(rem), nil
}

func computeScheduleInstallmentCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	id, ok := orm.CoerceInt64(rec["id"])
	if !ok || id <= 0 {
		return 0, nil
	}
	return orm.SearchCount(orm.ContextWithBypass(ctx, true), "loan.installment",
		[][]interface{}{{"schedule_id", "=", int(id)}})
}

func computeSchedulePaidCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	id, ok := orm.CoerceInt64(rec["id"])
	if !ok || id <= 0 {
		return 0, nil
	}
	return orm.SearchCount(orm.ContextWithBypass(ctx, true), "loan.installment",
		[][]interface{}{{"schedule_id", "=", int(id)}, {"state", "=", string(InstallmentStatePaid)}})
}

// ---------------------------------------------------------------------------
// Schedule generation
// ---------------------------------------------------------------------------

// scheduleGenerate builds installment rows from the account's terms. Idempotent:
// it clears any existing lines for a draft version before regenerating.
func scheduleGenerate(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	ver, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
	if err != nil {
		return "", fmt.Errorf("schedule version not found")
	}
	if VersionStatusOf(ver["status"]) == ScheduleStatusSuperseded {
		return "", fmt.Errorf("a superseded schedule cannot be regenerated")
	}

	acctID := intOr(ver["account_id"], 0)
	if acctID <= 0 {
		return "", fmt.Errorf("schedule has no loan account")
	}
	acct, err := orm.SearchOne(bypass, "loan.account", map[string]interface{}{"id": acctID})
	if err != nil {
		return "", fmt.Errorf("loan account not found")
	}

	principal := numericOf(acct["principal"])
	if principal <= 0 {
		principal = numericOf(acct["outstanding_principal"])
	}
	if principal <= 0 {
		return "", fmt.Errorf("loan account has no principal to schedule")
	}
	rate := numericOf(acct["interest_rate"])
	tenor := intOr(acct["tenor"], 0)
	if tenor <= 0 {
		return "", fmt.Errorf("loan account has no tenor")
	}
	method := strOr(acct["interest_method"], "declining_balance")
	start := normalizeDate(strOr(acct["start_date"], time.Now().Format("2006-01-02")))

	// Regenerate from scratch.
	if _, err := orm.UnlinkWhere(bypass, "loan.installment", [][]interface{}{{"schedule_id", "=", id}}); err != nil {
		return "", fmt.Errorf("clear existing installments: %w", err)
	}
	instModel := orm.Registry["loan.installment"]
	if instModel == nil {
		return "", fmt.Errorf("loan.installment model is not available")
	}

	r := rate / 100.0 / 12.0 // monthly periodic rate
	bal := principal
	totP, totI := 0.0, 0.0

	pmt := 0.0
	if method == "declining_balance" {
		if r > 0 {
			pmt = principal * r / (1 - math.Pow(1+r, -float64(tenor)))
		} else {
			pmt = principal / float64(tenor)
		}
	}
	flatInterestPerPeriod := 0.0
	if method == "flat" {
		flatInterestPerPeriod = principal * (rate / 100.0) * (float64(tenor) / 12.0) / float64(tenor)
	}

	for i := 1; i <= tenor; i++ {
		opening := bal
		var intDue, prinDue float64
		last := i == tenor
		switch method {
		case "flat":
			intDue = flatInterestPerPeriod
			prinDue = principal / float64(tenor)
			if last {
				prinDue = opening
			}
		case "interest_only", "bullet":
			intDue = principal * r
			prinDue = 0
			if last {
				prinDue = principal
			}
		default: // declining_balance annuity
			intDue = opening * r
			prinDue = pmt - intDue
			if last {
				prinDue = opening
			}
		}
		if prinDue > opening {
			prinDue = opening
		}
		if prinDue < 0 {
			prinDue = 0
		}
		if intDue < 0 {
			intDue = 0
		}
		bal = opening - prinDue
		if bal < 0 {
			bal = 0
		}
		totalDue := prinDue + intDue

		vals := map[string]interface{}{
			"schedule_id":       id,
			"account_id":        acctID,
			"number":            i,
			"opening_principal": round2(opening),
			"principal_due":     round2(prinDue),
			"interest_due":      round2(intDue),
			"fees_due":          0.0,
			"penalty_due":       0.0,
			"total_due":         round2(totalDue),
			"paid_principal":    0.0,
			"paid_interest":     0.0,
			"paid_fees":         0.0,
			"paid_penalty":      0.0,
			"total_paid":        0.0,
			"balance":           round2(bal),
			"days_past_due":     0,
			"state":             string(InstallmentStateUpcoming),
		}
		if dd := addMonths(start, i); dd != "" {
			vals["due_date"] = dd
		}
		if _, err := orm.Create(bypass, instModel, vals); err != nil {
			return "", fmt.Errorf("create installment %d: %w", i, err)
		}
		totP += prinDue
		totI += intDue
	}

	if err := orm.UpdateRecordByID(bypass, model, id, map[string]interface{}{
		"total_principal": round2(totP),
		"total_interest":  round2(totI),
		"total_amount":    round2(totP + totI),
		"generated_at":    nowDateTime(),
	}); err != nil {
		return "", err
	}
	return "", nil
}

// ---------------------------------------------------------------------------
// Schedule approval
// ---------------------------------------------------------------------------

// scheduleApprove moves a draft schedule to approved, superseding any other
// approved schedule on the same account, then recomputes the account balances.
func scheduleApprove(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
		return "", err
	}
	if !orm.UserHasGroupXML(ctx, uid, "lending.group_lending_manager") {
		return "", fmt.Errorf("only a Lending Manager can approve a schedule")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	ver, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
	if err != nil {
		return "", fmt.Errorf("schedule version not found")
	}
	st := VersionStatusOf(ver["status"])
	if st == ScheduleStatusSuperseded {
		return "", fmt.Errorf("a superseded schedule cannot be approved")
	}
	acctID := intOr(ver["account_id"], 0)

	if st != ScheduleStatusApproved && acctID > 0 {
		others, err := orm.Search(bypass, model, [][]interface{}{
			{"account_id", "=", acctID},
			{"status", "=", string(ScheduleStatusApproved)},
		})
		if err == nil {
			for _, o := range others {
				oid, ok := orm.CoerceInt64(o["id"])
				if ok && int(oid) != id {
					_ = orm.UpdateRecordByID(bypass, model, int(oid), map[string]interface{}{"status": string(ScheduleStatusSuperseded)})
				}
			}
		}
	}
	if err := orm.UpdateRecordByID(bypass, model, id, map[string]interface{}{"status": string(ScheduleStatusApproved)}); err != nil {
		return "", err
	}
	if acctID > 0 {
		if err := recomputeAccountBalances(bypass, acctID); err != nil {
			return "", err
		}
		return fmt.Sprintf("/web?action=lending.action_loan_account&view_type=form&id=%d", acctID), nil
	}
	return "", nil
}

// ---------------------------------------------------------------------------
// Balance recompute
// ---------------------------------------------------------------------------

// accountRecompute re-rolls the approved schedule and recorded payments back onto
// the anchor loan account's subledger balances.
func accountRecompute(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if err := orm.CheckModelAccess(ctx, uid, "loan.account", "write"); err != nil {
		return "", err
	}
	if err := recomputeAccountBalances(orm.ContextWithBypass(ctx, true), id); err != nil {
		return "", err
	}
	return "", nil
}

func recomputeAccountBalances(ctx context.Context, acctID int) error {
	scheds, err := orm.Search(ctx, "loan.schedule.version", [][]interface{}{
		{"account_id", "=", acctID},
		{"status", "=", string(ScheduleStatusApproved)},
	})
	if err != nil {
		return err
	}
	if len(scheds) == 0 {
		return fmt.Errorf("no approved schedule to recompute from")
	}
	schedID := intOr(scheds[0]["id"], 0)
	insts, err := orm.Search(ctx, "loan.installment", [][]interface{}{{"schedule_id", "=", schedID}})
	if err != nil {
		return err
	}

	var outstanding, accrued, paidTotal float64
	var nextDue, earliestOverdue string
	today := time.Now().Format("2006-01-02")

	for _, in := range insts {
		remDue := numericOf(in["total_due"]) - numericOf(in["total_paid"])
		paidTotal += numericOf(in["total_paid"])

		remP := numericOf(in["principal_due"]) - numericOf(in["paid_principal"])
		if remP > 0 {
			outstanding += remP
		}
		remI := numericOf(in["interest_due"]) - numericOf(in["paid_interest"])
		if remI > 0 {
			accrued += remI
		}
		if remDue <= 0.005 {
			continue // settled
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

// VersionStatusOf normalizes a stored schedule status value.
func VersionStatusOf(v interface{}) ScheduleStatus {
	return ScheduleStatus(strings.TrimSpace(orm.AsString(v)))
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

func addMonths(dateStr string, months int) string {
	if dateStr == "" || months <= 0 {
		return ""
	}
	d, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return ""
	}
	return d.AddDate(0, months, 0).Format("2006-01-02")
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
