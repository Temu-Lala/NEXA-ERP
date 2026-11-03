package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nexa/core/orm"
)

// Budget control + approval routing (blueprint §4, §5, §6). All money maths is
// server-side on a precise decimal (scale=2). `account.budget` is read-only;
// procurement keeps its own commitment ledger so available budget =
// approved - actual - openCommitments - proposed. Nothing here writes to
// Accounting.

const (
	reqModel       = "purchase.requisition"
	reqLineModel   = "purchase.requisition.line"
	budgetCheckMdl = "purchase.budget.check"
	commitModel    = "purchase.budget.commitment"
	overrideModel  = "purchase.budget.override"
	matrixModel    = "purchase.approval.matrix"
	instModel      = "purchase.approval.instance"
	stepModel      = "purchase.approval.step"
	eventModel     = "purchase.approval.event"
	budgetComodel  = "account.budget"

	dateLayout = "2006-01-02"
	dtLayout   = "2006-01-02 15:04:05"
)

// ---------------------------------------------------------------------------
// Pure budget maths (unit tested)
// ---------------------------------------------------------------------------

// remainingBudget is the headroom left after actual spend and existing open
// commitments, before the proposed amount is applied.
func remainingBudget(approved, actual, openCommit float64) float64 {
	return approved - actual - openCommit
}

// budgetDecision classifies a proposal against the governing policy. A negative
// headroom is only fatal under a hard policy; soft and conditional allow it as a
// warning (conditional within tolerance), and no-budget skips the check.
func budgetDecision(policy BudgetPolicy, approved, actual, openCommit, proposed, tolerancePct float64) (BudgetResult, bool) {
	if policy == PolicyNoBudget {
		return BudgetNA, false
	}
	headroom := remainingBudget(approved, actual, openCommit) - proposed
	if headroom >= 0 {
		return BudgetOK, false
	}
	// Over budget by an absolute amount; tolerance is a fraction of approved.
	over := -headroom
	limit := approved * tolerancePct / 100.0
	switch policy {
	case PolicyHard:
		return BudgetExceed, true
	case PolicySoft:
		return BudgetWarning, false
	case PolicyConditional:
		if over <= limit {
			return BudgetWarning, false
		}
		return BudgetExceed, true
	default:
		return BudgetExceed, true
	}
}

// netCommitment turns the append-only ledger into the open commitment balance:
// reserves add, consumes and releases subtract.
func netCommitment(reserve, consume, release float64) float64 {
	return reserve - consume - release
}

// amountBand is one mutually-exclusive approval band.
type amountBand struct {
	from  float64
	to    float64 // <=0 means unbounded above
	level int
}

// selectBands returns the levels whose band contains the amount. Bands are
// [from, to); an empty candidate set means no approval routing matched.
func selectBands(bands []amountBand, amount float64) []int {
	var levels []int
	for _, b := range bands {
		if amount >= b.from && (b.to <= 0 || amount < b.to) {
			levels = append(levels, b.level)
		}
	}
	return levels
}

// detectSplitToEvade reports whether several sub-threshold requests together
// exceed a control threshold — a split-to-evade signal, not a verdict.
func detectSplitToEvade(amounts []float64, threshold float64) bool {
	if threshold <= 0 || len(amounts) < 2 {
		return false
	}
	sum := 0.0
	for _, a := range amounts {
		if a >= threshold {
			return false // one already crosses; not a split pattern
		}
		sum += a
	}
	return sum > threshold
}

// ---------------------------------------------------------------------------
// Budget reading + commitment ledger
// ---------------------------------------------------------------------------

func readBudget(ctx context.Context, budgetID int) (approved, actual float64, ok bool) {
	if budgetID <= 0 {
		return 0, 0, false
	}
	rec, err := orm.SearchOne(ctx, budgetComodel, map[string]interface{}{"id": budgetID})
	if err != nil || rec == nil {
		return 0, 0, false
	}
	return flt(rec["amount"]), flt(rec["actual_amount"]), true
}

// openCommitment sums the ledger for a budget into its open balance.
func openCommitment(ctx context.Context, budgetID int) float64 {
	if budgetID <= 0 {
		return 0
	}
	rows, err := orm.SearchPage(ctx, commitModel, [][]interface{}{
		{"budget_id", "=", budgetID}, {"active", "=", true},
	}, 5000, 0, "id asc")
	if err != nil {
		return 0
	}
	var reserve, consume, release float64
	for _, r := range rows {
		amt := flt(r["amount"])
		switch CommitKind(str(r["kind"])) {
		case CommitReserve:
			reserve += amt
		case CommitConsume:
			consume += amt
		case CommitRelease:
			release += amt
		}
	}
	return netCommitment(reserve, consume, release)
}

// availableBudget returns approved, actual, open commitments and the available
// headroom for a budget. Used to fill a purchase.budget.check snapshot.
func availableBudget(ctx context.Context, budgetID int) (approved, actual, open, available float64) {
	approved, actual, ok := readBudget(ctx, budgetID)
	if !ok {
		return 0, 0, 0, 0
	}
	open = openCommitment(ctx, budgetID)
	return approved, actual, open, remainingBudget(approved, actual, open)
}

func recordCommitment(ctx context.Context, reqID, budgetID int, kind CommitKind, amount float64, reason string) error {
	if budgetID <= 0 || amount <= 0 {
		return nil
	}
	inst, ok := orm.Registry[commitModel]
	if !ok {
		return fmt.Errorf("commitment model unavailable")
	}
	req := loadRec(ctx, reqModel, reqID)
	companyID := 0
	if req != nil {
		companyID = intID(req["company_id"])
	}
	vals := map[string]interface{}{
		"name":       nextRef(ctx, "CMT"),
		"budget_id":  budgetID,
		"kind":       string(kind),
		"amount":     round2(amount),
		"reason":     reason,
		"active":     true,
		"occured_on": time.Now().Format(dateLayout),
	}
	if reqID > 0 {
		vals["requisition_id"] = reqID
	}
	if companyID > 0 {
		vals["company_id"] = companyID
	}
	_, err := orm.Create(ctx, inst, vals)
	return err
}

// ---------------------------------------------------------------------------
// Approval matrix resolution
// ---------------------------------------------------------------------------

// matrixRow captures the routing fields of a matching matrix rule.
type matrixRow struct {
	level    int
	role     string
	approver int
	mode     MatrixMode
	minAppr  int
	priority int
}

// resolveMatrix finds the active matrix rows matching a requisition's amount and
// dimensions for the newest applicable version, ordered by level.
func resolveMatrix(ctx context.Context, req map[string]interface{}) []matrixRow {
	amount := flt(req["est_total"])
	rows, err := orm.SearchPage(ctx, matrixModel, [][]interface{}{
		{"active", "=", true},
	}, 5000, 0, "version desc, level asc, priority asc")
	if err != nil {
		return nil
	}
	var topVersion int
	seen := false
	var out []matrixRow
	for _, r := range rows {
		ver := int(flt(r["version"]))
		if !seen {
			topVersion = ver
			seen = true
		} else if ver != topVersion {
			break // only the newest version governs
		}
		if !rowMatches(r, req, amount) {
			continue
		}
		out = append(out, matrixRow{
			level:    int(flt(r["level"])),
			role:     str(r["approver_role"]),
			approver: intID(r["approver_id"]),
			mode:     MatrixMode(strOr(r["mode"], "sequential")),
			minAppr:  intOrDefault(r["min_approvers"], 1),
			priority: int(flt(r["priority"])),
		})
	}
	return out
}

func rowMatches(rule map[string]interface{}, req map[string]interface{}, amount float64) bool {
	if amount < flt(rule["amount_from"]) {
		return false
	}
	if to := flt(rule["amount_to"]); to > 0 && amount >= to {
		return false
	}
	if s := strings.TrimSpace(str(rule["req_type"])); s != "" && !strings.EqualFold(s, str(req["req_type"])) {
		return false
	}
	if s := strings.TrimSpace(str(rule["department"])); s != "" && !strings.EqualFold(s, str(req["department"])) {
		return false
	}
	if s := strings.TrimSpace(str(rule["branch"])); s != "" && !strings.EqualFold(s, str(req["branch"])) {
		return false
	}
	if s := strings.TrimSpace(str(rule["category"])); s != "" && !strings.EqualFold(s, str(req["category"])) {
		return false
	}
	if boolOf(rule["emergency_only"]) && !strings.EqualFold(str(req["req_type"]), "emergency") {
		return false
	}
	if boolOf(rule["capex_only"]) && !strings.EqualFold(str(req["req_type"]), "capex") {
		return false
	}
	if boolOf(rule["single_source_only"]) && !boolOf(req["single_source"]) {
		return false
	}
	return true
}

// buildApprovalInstance materializes an instance with one step per matched
// level and an opening audit event carrying the reviewed version snapshot.
func buildApprovalInstance(ctx context.Context, req map[string]interface{}, snapshot string) (int, error) {
	reqID := intID(req["id"])
	amount := flt(req["est_total"])
	levels := resolveMatrix(ctx, req)
	if len(levels) == 0 {
		return 0, fmt.Errorf("no approval routing matched this requisition")
	}
	im, ok := orm.Registry[instModel]
	if !ok {
		return 0, fmt.Errorf("approval instance model unavailable")
	}
	version := 1
	if rows, _ := orm.SearchPage(ctx, matrixModel, [][]interface{}{{"active", "=", true}}, 1, 0, "version desc"); len(rows) > 0 {
		version = int(flt(rows[0]["version"]))
	}
	instID, err := orm.Create(ctx, im, map[string]interface{}{
		"name":             nextRef(ctx, "APR"),
		"requisition_id":   reqID,
		"matrix_version":   version,
		"requested_amount": round2(amount),
		"currency":         strOr(req["currency"], "USD"),
		"state":            string(InstPending),
		"requester_id":     intID(req["requester_id"]),
		"total_levels":     len(levels),
		"snapshot":         snapshot,
		"started_on":       time.Now().Format(dtLayout),
		"active":           true,
	})
	if err != nil {
		return 0, err
	}
	sm, _ := orm.Registry[stepModel]
	for i, lv := range levels {
		if _, err := orm.Create(ctx, sm, map[string]interface{}{
			"instance_id":   instID,
			"level":         lv.level,
			"approver_role": lv.role,
			"approver_id":   lv.approver,
			"mode":          string(lv.mode),
			"min_approvers": lv.minAppr,
			"state":         string(StepPending),
			"decision":      string(DecNone),
			"sequence":      (i + 1) * 10,
			"active":        true,
		}); err != nil {
			return 0, err
		}
	}
	logEvent(ctx, int(instID), 0, intID(req["requester_id"]), "submitted", snapshot, fmt.Sprintf("Amount %.2f routed to %d level(s).", amount, len(levels)))
	return int(instID), nil
}

func logEvent(ctx context.Context, instID, stepID, actorID int, action, snapshot, detail string) {
	em, ok := orm.Registry[eventModel]
	if !ok || instID <= 0 {
		return
	}
	vals := map[string]interface{}{
		"instance_id":      instID,
		"timestamp":        time.Now().Format(dtLayout),
		"actor_id":         actorID,
		"action":           action,
		"version_snapshot": snapshot,
		"detail":           detail,
	}
	if stepID > 0 {
		vals["step_id"] = stepID
	}
	_, _ = orm.Create(ctx, em, vals)
}

// requisitionSnapshot records the reviewed content so every approver's decision
// is tied to a specific version (evidence for reapproval on material change).
func requisitionSnapshot(ctx context.Context, req map[string]interface{}) string {
	lines, _ := orm.SearchPage(ctx, reqLineModel, [][]interface{}{
		{"requisition_id", "=", intID(req["id"])},
	}, 200, 0, "sequence asc, id asc")
	var b strings.Builder
	fmt.Fprintf(&b, "%s | type=%s dept=%s cat=%s amount=%.2f %s | lines=%d",
		str(req["name"]), str(req["req_type"]), str(req["department"]),
		str(req["category"]), flt(req["est_total"]), strOr(req["currency"], "USD"), len(lines))
	if boolOf(req["single_source"]) {
		b.WriteString(" | single-source")
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

func loadRec(ctx context.Context, model string, id int) map[string]interface{} {
	if id <= 0 {
		return nil
	}
	rec, err := orm.SearchOne(ctx, model, map[string]interface{}{"id": id})
	if err != nil || rec == nil {
		return nil
	}
	return rec
}

func str(v interface{}) string { return orm.AsString(v) }

func strOr(v interface{}, def string) string {
	if s := strings.TrimSpace(orm.AsString(v)); s != "" {
		return s
	}
	return def
}

func flt(v interface{}) float64 {
	switch t := v.(type) {
	case float64:
		return t
	case int64:
		return float64(t)
	case int:
		return float64(t)
	default:
		n, _ := orm.CoerceInt64(v)
		return float64(n)
	}
}

func intID(v interface{}) int {
	n, _ := orm.CoerceInt64(v)
	return int(n)
}

func boolOf(v interface{}) bool {
	b, _ := v.(bool)
	return b
}

func intOrDefault(v interface{}, def int) int {
	if n, ok := orm.CoerceInt64(v); ok && n != 0 {
		return int(n)
	}
	if s := str(v); s == "" {
		return def
	}
	return def
}

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

func nextRef(ctx context.Context, prefix string) string {
	if name, err := orm.NextSequence(ctx, prefix); err == nil && name != "" {
		return name
	}
	return fmt.Sprintf("%s/%05d", prefix, time.Now().Unix()%100000)
}
