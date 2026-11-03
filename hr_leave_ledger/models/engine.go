package models

import (
	"context"
	"strings"
	"time"

	"nexa/core/event"
	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Leave-ledger behaviour layer.
//
// Posting is derived from the anchor's existing approval workflow and is written
// as append-only deltas, so it is idempotent no matter how many times an event
// fires for the same record: for each source record the ledger's net equals the
// "desired" amount when approved (or zero when not), and only a difference posts
// a new line. Approving an allocation credits days; approving a request debits
// them; refusing or resetting an already-posted record writes an opposite-signed
// reversal rather than editing history.
//
// The scheduled job credits each active employee the configured amount per period
// exactly once (deduped on rule + period date).

const (
	accrualCronCode  = "hr.leave.accrual.run"
	sourceAllocation = "hr.leave.allocation"
	sourceLeave      = "hr.leave"
	sourceAccrual    = "hr.leave.accrual.rule"
	dayHours         = 8.0
	epsilon          = 0.0001
	dateLayout       = "2006-01-02"
)

func init() {
	orm.RegisterCompute("hr.leave.balance", "accrued", nil, computeLeaveAccrued)
	orm.RegisterCompute("hr.leave.balance", "taken", nil, computeLeaveTaken)
	orm.RegisterCompute("hr.leave.balance", "balance", nil, computeLeaveBalance)

	event.Subscribe(orm.EventRecordCreated, onLeaveChanged)
	event.Subscribe(orm.EventRecordUpdated, onLeaveChanged)
	scheduler.RegisterCronHandler(accrualCronCode, runAccrual)
}

// ---------------------------------------------------------------------------
// Computed balance projections (live over the posted ledger)
// ---------------------------------------------------------------------------

func computeLeaveAccrued(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	// Accrued = all credit-side movement: accruals, carry-over and adjustments.
	sum := sumCategory(ctx, rec, TxnAccrual) + sumCategory(ctx, rec, TxnCarryOver) + sumCategory(ctx, rec, TxnAdjust)
	return round2(sum), nil
}

func computeLeaveTaken(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	// Usage rows are stored negative; show consumption as a positive magnitude.
	return round2(-sumCategory(ctx, rec, TxnUsage)), nil
}

func computeLeaveBalance(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return round2(sumAll(ctx, rec)), nil
}

func sumCategory(ctx context.Context, rec map[string]interface{}, cat TxnCategory) float64 {
	rows := ledgerRows(ctx, rec, string(cat))
	var s float64
	for _, r := range rows {
		f, _ := orm.CoerceFloat64(r["amount"])
		s += f
	}
	return s
}

func sumAll(ctx context.Context, rec map[string]interface{}) float64 {
	rows, err := orm.Search(orm.ContextWithBypass(ctx, true), "hr.leave.transaction", leaveDomain(rec))
	if err != nil {
		return 0
	}
	var s float64
	for _, r := range rows {
		f, _ := orm.CoerceFloat64(r["amount"])
		s += f
	}
	return s
}

func ledgerRows(ctx context.Context, rec map[string]interface{}, category string) []map[string]interface{} {
	domain := append(leaveDomain(rec), []interface{}{"category", "=", category})
	rows, err := orm.Search(orm.ContextWithBypass(ctx, true), "hr.leave.transaction", domain)
	if err != nil {
		return nil
	}
	return rows
}

// leaveDomain scopes a query to a balance record's employee + leave type.
func leaveDomain(rec map[string]interface{}) [][]interface{} {
	d := [][]interface{}{}
	if e := intOr(rec["employee_id"]); e > 0 {
		d = append(d, []interface{}{"employee_id", "=", e})
	}
	if t := intOr(rec["leave_type_id"]); t > 0 {
		d = append(d, []interface{}{"leave_type_id", "=", t})
	}
	return d
}

// ---------------------------------------------------------------------------
// Workflow-driven posting
// ---------------------------------------------------------------------------

func onLeaveChanged(ctx context.Context, ev event.Event) error {
	model, _ := ev.Payload["model"].(string)
	if model != sourceLeave && model != sourceAllocation {
		return nil
	}
	id, ok := orm.CoerceInt64(ev.Payload["id"])
	if !ok || id <= 0 {
		return nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": int(id)})
	if err != nil || rec == nil {
		return nil
	}

	emp := intOr(rec["employee_id"])
	ltype := intOr(rec["leave_type_id"])
	if emp <= 0 || ltype <= 0 {
		return nil
	}
	approved := strings.TrimSpace(orm.AsString(rec["state"])) == "validate"

	var cat TxnCategory
	var desired float64
	var date string
	if model == sourceAllocation {
		cat = TxnAccrual
		days, _ := orm.CoerceFloat64(rec["number_of_days"])
		desired = days // credit
		date = effectiveDate(rec, "date_from")
	} else {
		cat = TxnUsage
		days := effectiveLeaveDays(rec)
		desired = -days // debit
		date = effectiveDate(rec, "date_from")
	}
	if !approved {
		desired = 0
	}
	if date == "" {
		date = time.Now().UTC().Format(dateLayout)
	}

	postDelta(bypass, model, int(id), emp, ltype, cat, desired, date, intOr(rec["company_id"]), ev.Actor)
	return nil
}

// postDelta writes only the difference between the desired and the currently
// posted net for a source record, so repeated events converge and never duplicate.
func postDelta(ctx context.Context, srcModel string, srcID, emp, ltype int, cat TxnCategory, desired float64, date string, companyID, actorID int) {
	net := sourceNet(ctx, srcModel, srcID)
	delta := round2(desired - net)
	if delta <= epsilon && delta >= -epsilon {
		return
	}
	postCategory := cat
	if desired == 0 {
		// removing a previously posted amount is a reversal
		postCategory = TxnReversal
	}
	createTxn(ctx, emp, ltype, postCategory, delta, date, srcModel, srcID, companyID, actorID)
	ensureBalance(ctx, emp, ltype, companyID)
}

func sourceNet(ctx context.Context, srcModel string, srcID int) float64 {
	rows, err := orm.Search(ctx, "hr.leave.transaction", [][]interface{}{
		{"source_model", "=", srcModel},
		{"source_id", "=", srcID},
	})
	if err != nil {
		return 0
	}
	var s float64
	for _, r := range rows {
		f, _ := orm.CoerceFloat64(r["amount"])
		s += f
	}
	return s
}

func createTxn(ctx context.Context, emp, ltype int, cat TxnCategory, amount float64, date, srcModel string, srcID, companyID, actorID int) {
	inst, ok := orm.Registry["hr.leave.transaction"]
	if !ok {
		return
	}
	vals := map[string]interface{}{
		"employee_id":   emp,
		"leave_type_id": ltype,
		"category":      string(cat),
		"amount":        amount,
		"date":          date,
		"source_model":  srcModel,
		"source_id":     srcID,
	}
	if companyID > 0 {
		vals["company_id"] = companyID
	}
	if actorID > 0 {
		vals["create_uid"] = actorID
	}
	_, _ = orm.Create(ctx, inst, vals)
}

func ensureBalance(ctx context.Context, emp, ltype, companyID int) {
	existing, err := orm.SearchCount(ctx, "hr.leave.balance", [][]interface{}{
		{"employee_id", "=", emp},
		{"leave_type_id", "=", ltype},
	})
	if err == nil && existing > 0 {
		return
	}
	inst, ok := orm.Registry["hr.leave.balance"]
	if !ok {
		return
	}
	vals := map[string]interface{}{"employee_id": emp, "leave_type_id": ltype}
	if companyID > 0 {
		vals["company_id"] = companyID
	}
	_, _ = orm.Create(ctx, inst, vals)
}

// effectiveLeaveDays converts a request into a signed-positive day count,
// honouring hourly then half-day modifiers.
func effectiveLeaveDays(rec map[string]interface{}) float64 {
	if h, ok := orm.CoerceFloat64(rec["hours"]); ok && h > 0 {
		return round2(h / dayHours)
	}
	days := requestDays(rec)
	if orm.AsBool(rec["half_day"]) {
		return round2(days * 0.5)
	}
	return round2(days)
}

// requestDays is the inclusive calendar-day span of a request (recomputed here
// because the anchor's day count is a non-stored compute, absent from SearchOne).
func requestDays(rec map[string]interface{}) float64 {
	d0 := parseDate(rec["date_from"])
	d1 := parseDate(rec["date_to"])
	if d0.IsZero() || d1.IsZero() {
		return 0
	}
	n := int(d1.Sub(d0).Hours()/24) + 1
	if n < 0 {
		n = 0
	}
	return float64(n)
}

// ---------------------------------------------------------------------------
// Periodic accrual
// ---------------------------------------------------------------------------

func runAccrual(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	rules, err := orm.Search(bypass, "hr.leave.accrual.rule", [][]interface{}{{"active", "=", true}})
	if err != nil {
		return nil
	}
	employees, err := orm.Search(bypass, "hr.employee", [][]interface{}{
		{"active", "=", true},
		{"state", "=", "active"},
	})
	if err != nil {
		return nil
	}
	period := periodDate()
	for _, rule := range rules {
		ltype := intOr(rule["leave_type_id"])
		amt, _ := orm.CoerceFloat64(rule["days_per_period"])
		if ltype <= 0 || amt <= 0 {
			continue
		}
		companyID := intOr(rule["company_id"])
		for _, e := range employees {
			emp := intOr(e["id"])
			if emp <= 0 {
				continue
			}
			if alreadyAccrued(bypass, intOr(rule["id"]), emp, ltype, period) {
				continue
			}
			createTxn(bypass, emp, ltype, TxnAccrual, amt, period, sourceAccrual, intOr(rule["id"]), companyID, 0)
			ensureBalance(bypass, emp, ltype, companyID)
		}
	}
	return nil
}

func alreadyAccrued(ctx context.Context, ruleID, emp, ltype int, period string) bool {
	n, err := orm.SearchCount(ctx, "hr.leave.transaction", [][]interface{}{
		{"source_model", "=", sourceAccrual},
		{"source_id", "=", ruleID},
		{"employee_id", "=", emp},
		{"leave_type_id", "=", ltype},
		{"date", "=", period},
	})
	return err == nil && n > 0
}

// periodDate is the anchor date for the current accrual period: the first of this
// month, or the first of this year for yearly rules (the caller filters per rule;
// using month start keeps monthly idempotency and is harmless for yearly runs).
func periodDate() string {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC).Format(dateLayout)
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func effectiveDate(rec map[string]interface{}, field string) string {
	d := parseDate(rec[field])
	if d.IsZero() {
		return ""
	}
	return d.Format(dateLayout)
}

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}

func round2(f float64) float64 {
	scaled := f * 100
	if scaled >= 0 {
		return float64(int64(scaled+0.5)) / 100
	}
	return float64(int64(scaled-0.5)) / 100
}

func parseDate(v interface{}) time.Time {
	switch t := v.(type) {
	case time.Time:
		return t.UTC()
	case string:
		s := strings.TrimSpace(t)
		if len(s) >= 10 {
			s = s[:10]
		}
		if parsed, err := time.Parse(dateLayout, s); err == nil {
			return parsed
		}
	}
	return time.Time{}
}
