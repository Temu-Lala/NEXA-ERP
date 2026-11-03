package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// Lifecycle workflow for the lending anchor. State transitions are guarded
// object actions (Submit / Approve / Reject / Activate / Close / Write-off) and
// the Approve action materialises the authoritative loan.account subledger from
// the approved application. Money fields are only seeded here (principal/outstanding);
// servicing owns schedule-driven balance updates.

func init() {
	// Live product stats (non-stored, recomputed on read).
	orm.RegisterCompute("lending.product", "application_count", nil, computeProductApplications)
	orm.RegisterCompute("lending.product", "active_loan_count", nil, computeProductActiveLoans)

	// Product version activation (manager only).
	orm.RegisterObjectAction("lending.product.version", "action_version_activate", versionActivate)

	// Application state machine.
	orm.RegisterObjectAction("loan.application", "action_application_submit",
		appTransition("submit", false, []ApplicationState{ApplicationStateDraft}, ApplicationStateSubmitted, true))
	orm.RegisterObjectAction("loan.application", "action_application_review",
		appTransition("send for review", false, []ApplicationState{ApplicationStateSubmitted}, ApplicationStateUnderReview, false))
	orm.RegisterObjectAction("loan.application", "action_application_request_approval",
		appTransition("request approval", false, []ApplicationState{ApplicationStateUnderReview, ApplicationStateAssessment}, ApplicationStatePendingApproval, false))
	orm.RegisterObjectAction("loan.application", "action_application_approve", applicationApprove)
	orm.RegisterObjectAction("loan.application", "action_application_reject",
		appTransition("reject", true, []ApplicationState{ApplicationStateSubmitted, ApplicationStateUnderReview, ApplicationStateAssessment, ApplicationStatePendingApproval}, ApplicationStateRejected, false))
	orm.RegisterObjectAction("loan.application", "action_application_cancel",
		appTransition("cancel", false, []ApplicationState{ApplicationStateDraft, ApplicationStateSubmitted, ApplicationStateUnderReview, ApplicationStateAssessment, ApplicationStatePendingApproval}, ApplicationStateCancelled, false))

	// Loan account lifecycle.
	orm.RegisterObjectAction("loan.account", "action_account_activate",
		accountTransition("activate", []AccountStatus{AccountStatusPending}, AccountStatusActive, false, false))
	orm.RegisterObjectAction("loan.account", "action_account_close",
		accountTransition("close", []AccountStatus{AccountStatusActive, AccountStatusRestructured}, AccountStatusClosed, true, false))
	orm.RegisterObjectAction("loan.account", "action_account_writeoff",
		accountTransition("write off", []AccountStatus{AccountStatusActive, AccountStatusDelinquent, AccountStatusRestructured}, AccountStatusWrittenOff, false, true))
}

// ---------------------------------------------------------------------------
// Computed product statistics
// ---------------------------------------------------------------------------

func computeProductApplications(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	return countLinked(ctx, "loan.application", rec["id"], nil, nil)
}

func computeProductActiveLoans(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	status := string(AccountStatusActive)
	return countLinked(ctx, "loan.account", rec["id"], &status, nil)
}

// countLinked counts records of model that point at the given product id,
// optionally filtered by a single status value.
func countLinked(ctx context.Context, model string, rawProduct interface{}, status *string, _ *string) (interface{}, error) {
	id, ok := orm.CoerceInt64(rawProduct)
	if !ok || id <= 0 {
		return 0, nil
	}
	bypass := orm.ContextWithBypass(ctx, true)
	domain := [][]interface{}{{"product_id", "=", int(id)}}
	if status != nil {
		domain = append(domain, []interface{}{"status", "=", *status})
	}
	n, err := orm.SearchCount(bypass, model, domain)
	if err != nil {
		return nil, err
	}
	return n, nil
}

// ---------------------------------------------------------------------------
// Product version
// ---------------------------------------------------------------------------

// versionActivate promotes a draft version to active, superseding any other
// active version of the same product so a product keeps exactly one live term set.
func versionActivate(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
		return "", err
	}
	if !orm.UserHasGroupXML(ctx, uid, "lending.group_lending_manager") {
		return "", fmt.Errorf("only a Lending Manager can activate a product version")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
	if err != nil {
		return "", fmt.Errorf("product version not found")
	}
	current := VersionStatus(orm.AsString(rec["status"]))
	if current == VersionStatusActive {
		return "", nil
	}
	if current == VersionStatusSuperseded {
		return "", fmt.Errorf("a superseded version cannot be re-activated; create a new version")
	}
	if productID := intOr(rec["product_id"], 0); productID > 0 {
		others, err := orm.Search(bypass, model, [][]interface{}{
			{"product_id", "=", productID},
			{"status", "=", string(VersionStatusActive)},
		})
		if err == nil {
			for _, o := range others {
				oid, ok := orm.CoerceInt64(o["id"])
				if ok && int(oid) != id {
					_ = orm.UpdateRecordByID(bypass, model, int(oid), map[string]interface{}{"status": string(VersionStatusSuperseded)})
				}
			}
		}
	}
	if err := orm.UpdateRecordByID(bypass, model, id, map[string]interface{}{"status": string(VersionStatusActive)}); err != nil {
		return "", err
	}
	return "", nil
}

// ---------------------------------------------------------------------------
// Application transitions
// ---------------------------------------------------------------------------

// appTransition builds a guarded state-move handler; when ensureNumber is set it
// stamps a reference number on first transition if none exists.
func appTransition(verb string, managerOnly bool, from []ApplicationState, to ApplicationState, ensureNumber bool) func(context.Context, string, int, map[string]string) (string, error) {
	return func(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
		uid := orm.SecurityUID(ctx)
		if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
			return "", err
		}
		if managerOnly && !orm.UserHasGroupXML(ctx, uid, "lending.group_lending_manager") {
			return "", fmt.Errorf("only a Lending Manager can %s this application", verb)
		}
		bypass := orm.ContextWithBypass(ctx, true)
		rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
		if err != nil {
			return "", fmt.Errorf("application not found")
		}
		current := ApplicationState(orm.AsString(rec["state"]))
		if !appStateAllowed(current, from) {
			return "", fmt.Errorf("cannot %s an application in the %q state", verb, current)
		}
		vals := map[string]interface{}{"state": string(to)}
		if ensureNumber && strings.TrimSpace(orm.AsString(rec["number"])) == "" {
			vals["number"] = fmt.Sprintf("APP-%05d", id)
		}
		if err := orm.UpdateRecordByID(bypass, model, id, vals); err != nil {
			return "", err
		}
		return "", nil
	}
}

// applicationApprove marks the application approved and creates (or reuses) the
// loan account, then redirects the browser to the new account form.
func applicationApprove(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
		return "", err
	}
	if !orm.UserHasGroupXML(ctx, uid, "lending.group_lending_manager") {
		return "", fmt.Errorf("only a Lending Manager can approve an application")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	app, err := orm.SearchOne(bypass, "loan.application", map[string]interface{}{"id": id})
	if err != nil {
		return "", fmt.Errorf("application not found")
	}
	current := ApplicationState(orm.AsString(app["state"]))
	if !appStateAllowed(current, []ApplicationState{ApplicationStateUnderReview, ApplicationStateAssessment, ApplicationStatePendingApproval}) {
		return "", fmt.Errorf("cannot approve an application in the %q state", current)
	}

	accountID := 0
	if existing, err := orm.Search(bypass, "loan.account", [][]interface{}{{"application_id", "=", id}}); err == nil && len(existing) > 0 {
		if aid, ok := orm.CoerceInt64(existing[0]["id"]); ok {
			accountID = int(aid)
		}
	} else {
		accountID, err = createAccountFromApplication(bypass, app)
		if err != nil {
			return "", err
		}
	}

	upd := map[string]interface{}{"state": string(ApplicationStateApproved)}
	if strings.TrimSpace(orm.AsString(app["number"])) == "" {
		upd["number"] = fmt.Sprintf("APP-%05d", id)
	}
	if err := orm.UpdateRecordByID(bypass, "loan.application", id, upd); err != nil {
		return "", err
	}
	if accountID <= 0 {
		return "", nil
	}
	return fmt.Sprintf("/web?action=lending.action_loan_account&view_type=form&id=%d", accountID), nil
}

// createAccountFromApplication builds the authoritative contractual subledger row
// from the approved application, inheriting pricing from the product version when set.
func createAccountFromApplication(ctx context.Context, app map[string]interface{}) (int, error) {
	m := orm.Registry["loan.account"]
	if m == nil {
		return 0, fmt.Errorf("loan.account model is not available")
	}
	appID := intOr(app["id"], 0)
	borrower := intOr(app["borrower_id"], 0)
	if borrower <= 0 {
		return 0, fmt.Errorf("application has no borrower")
	}
	principal := numericOf(app["requested_amount"])

	vals := map[string]interface{}{
		"name":                  fmt.Sprintf("Loan %s", strOr(app["name"], fmt.Sprintf("APP-%d", appID))),
		"number":                fmt.Sprintf("LN-%05d", appID),
		"principal":             principal,
		"outstanding_principal": principal,
		"disbursed_amount":      0.0,
		"accrued_interest":      0.0,
		"paid_total":            0.0,
		"status":                string(AccountStatusPending),
	}
	setM2O(vals, "application_id", appID)
	setM2O(vals, "borrower_id", borrower)
	setM2O(vals, "currency_id", intOr(app["currency_id"], 0))
	setM2O(vals, "officer_id", intOr(app["officer_id"], 0))
	setM2O(vals, "company_id", intOr(app["company_id"], 0))

	tenor := intOr(app["tenor"], 0)
	vals["tenor"] = tenor

	productID := intOr(app["product_id"], 0)
	versionID := intOr(app["version_id"], 0)
	setM2O(vals, "product_id", productID)
	setM2O(vals, "version_id", versionID)

	// Pricing: version overrides product.
	var rate float64
	var method, dayCount string
	if prod := loadProductTerms(ctx, productID); prod != nil {
		rate = numericOf(prod["interest_rate"])
		method = orm.AsString(prod["interest_method"])
		dayCount = orm.AsString(prod["day_count"])
	}
	if versionID > 0 {
		if ver, err := orm.SearchOne(ctx, "lending.product.version", map[string]interface{}{"id": versionID}); err == nil {
			if r := numericOf(ver["interest_rate"]); r > 0 {
				rate = r
			}
			if mth := orm.AsString(ver["interest_method"]); mth != "" {
				method = mth
			}
			if dc := orm.AsString(ver["day_count"]); dc != "" {
				dayCount = dc
			}
		}
	}
	if rate > 0 {
		vals["interest_rate"] = rate
	}
	if method != "" {
		vals["interest_method"] = method
	}
	if dayCount != "" {
		vals["day_count"] = dayCount
	}

	start := normalizeDate(strOr(app["applied_date"], time.Now().Format("2006-01-02")))
	if start != "" {
		vals["start_date"] = start
		if md := addMonths(start, tenor); md != "" {
			vals["maturity_date"] = md
		}
	}

	return orm.Create(ctx, m, vals)
}

func loadProductTerms(ctx context.Context, productID int) map[string]interface{} {
	if productID <= 0 {
		return nil
	}
	prod, err := orm.SearchOne(ctx, "lending.product", map[string]interface{}{"id": productID})
	if err != nil {
		return nil
	}
	return prod
}

// ---------------------------------------------------------------------------
// Account lifecycle
// ---------------------------------------------------------------------------

// accountTransition moves a loan account between lifecycle states. requireSettled
// blocks the move until outstanding principal is cleared; managerOnly gates the verb.
func accountTransition(verb string, from []AccountStatus, to AccountStatus, requireSettled, managerOnly bool) func(context.Context, string, int, map[string]string) (string, error) {
	return func(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
		uid := orm.SecurityUID(ctx)
		if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
			return "", err
		}
		if managerOnly && !orm.UserHasGroupXML(ctx, uid, "lending.group_lending_manager") {
			return "", fmt.Errorf("only a Lending Manager can %s this loan", verb)
		}
		bypass := orm.ContextWithBypass(ctx, true)
		rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
		if err != nil {
			return "", fmt.Errorf("loan account not found")
		}
		current := AccountStatus(orm.AsString(rec["status"]))
		if !acctStateAllowed(current, from) {
			return "", fmt.Errorf("cannot %s a loan in the %q status", verb, current)
		}
		if requireSettled && numericOf(rec["outstanding_principal"]) > 0.005 {
			return "", fmt.Errorf("loan still has an outstanding balance; settle it before %sing", verb)
		}
		if err := orm.UpdateRecordByID(bypass, model, id, map[string]interface{}{"status": string(to)}); err != nil {
			return "", err
		}
		return "", nil
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func appStateAllowed(current ApplicationState, allowed []ApplicationState) bool {
	for _, s := range allowed {
		if current == s {
			return true
		}
	}
	return false
}

func acctStateAllowed(current AccountStatus, allowed []AccountStatus) bool {
	for _, s := range allowed {
		if current == s {
			return true
		}
	}
	return false
}

func setM2O(vals map[string]interface{}, key string, id int) {
	if id > 0 {
		vals[key] = id
	}
}

func strOr(v interface{}, def string) string {
	s := strings.TrimSpace(orm.AsString(v))
	if s == "" {
		return def
	}
	return s
}

func intOr(v interface{}, def int) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return def
	}
	return int(n)
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
