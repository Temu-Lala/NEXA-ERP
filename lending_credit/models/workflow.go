package models

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"nexa/core/orm"
)

// Behaviour layer for the Lending - Credit companion. It adds live aggregates over
// the credit-assessment factors and the linked loan account, a scoring/affordability
// recalculation on the assessment, and a maker-checker decision workflow on approvals
// plus an acceptance action on agreements. Approval decisions are segregation-of-duties
// gated: the checker cannot be the maker, and only a Lending Manager can decide.

func init() {
	// Live aggregates (non-stored, recomputed on read).
	orm.RegisterCompute("loan.credit.assessment", "factor_count", nil, computeAssessmentFactorCount)
	orm.RegisterCompute("loan.credit.assessment", "total_weight", nil, computeAssessmentTotalWeight)
	orm.RegisterCompute("loan.agreement", "account_outstanding", nil, computeAgreementAccountOutstanding)

	// Credit assessment scoring / affordability recalculation.
	orm.RegisterObjectAction("loan.credit.assessment", "action_assessment_compute", assessmentCompute)

	// Maker-checker approval workflow.
	orm.RegisterObjectAction("loan.approval", "action_approval_approve", approvalDecide("approve", ApprovalDecisionApproved))
	orm.RegisterObjectAction("loan.approval", "action_approval_reject", approvalDecide("reject", ApprovalDecisionRejected))
	orm.RegisterObjectAction("loan.approval", "action_approval_escalate", approvalEscalate)

	// Agreement acceptance.
	orm.RegisterObjectAction("loan.agreement", "action_agreement_accept", agreementAccept)
}

// ---------------------------------------------------------------------------
// Computed fields
// ---------------------------------------------------------------------------

func computeAssessmentFactorCount(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	aid, ok := orm.CoerceInt64(rec["id"])
	if !ok || aid <= 0 {
		return 0, nil
	}
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), "loan.credit.factor",
		[][]interface{}{{"assessment_id", "=", int(aid)}})
	if err != nil {
		return nil, err
	}
	return n, nil
}

func computeAssessmentTotalWeight(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	aid, ok := orm.CoerceInt64(rec["id"])
	if !ok || aid <= 0 {
		return 0.0, nil
	}
	factors, err := orm.Search(orm.ContextWithBypass(ctx, true), "loan.credit.factor",
		[][]interface{}{{"assessment_id", "=", int(aid)}})
	if err != nil {
		return nil, err
	}
	var total float64
	for _, f := range factors {
		total += numericOf(f["weight"])
	}
	return round2(total), nil
}

func computeAgreementAccountOutstanding(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	acctID := intOr(rec["account_id"], 0)
	if acctID <= 0 {
		return 0.0, nil
	}
	acct, err := orm.SearchOne(orm.ContextWithBypass(ctx, true), "loan.account", map[string]interface{}{"id": acctID})
	if err != nil {
		return 0.0, nil
	}
	return numericOf(acct["outstanding_principal"]), nil
}

// ---------------------------------------------------------------------------
// Credit assessment recalculation
// ---------------------------------------------------------------------------

// assessmentCompute derives the debt-service ratio and affordability verdict from
// the stored income/obligations, stamps the assessor, and is available to any user
// with write access on the assessment.
func assessmentCompute(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
		return "", err
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
	if err != nil {
		return "", fmt.Errorf("credit assessment not found")
	}
	income := numericOf(rec["verified_income"])
	obligations := numericOf(rec["existing_obligations"])

	dsr := 0.0
	if income > 0 {
		dsr = obligations / income * 100.0
	}
	upd := map[string]interface{}{
		"debt_service_ratio":   round2(dsr),
		"affordability_result": affordabilityFor(dsr, income),
		"assessed_by":          uid,
		"assessed_at":          nowDateTime(),
	}
	if err := orm.UpdateRecordByID(bypass, model, id, upd); err != nil {
		return "", err
	}
	return "", nil
}

// affordabilityFor maps a debt-service ratio (and income availability) to a verdict.
func affordabilityFor(dsr, income float64) string {
	if income <= 0 {
		return string(AffordabilityResultRefer)
	}
	switch {
	case dsr <= 40:
		return string(AffordabilityResultPass)
	case dsr <= 50:
		return string(AffordabilityResultRefer)
	default:
		return string(AffordabilityResultFail)
	}
}

// ---------------------------------------------------------------------------
// Maker-checker approval workflow
// ---------------------------------------------------------------------------

// approvalDecide records a final decision (approved/rejected), enforcing that the
// checker is not the maker and that only a Lending Manager can decide.
func approvalDecide(verb string, to ApprovalDecision) func(context.Context, string, int, map[string]string) (string, error) {
	return func(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
		uid := orm.SecurityUID(ctx)
		if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
			return "", err
		}
		if !orm.UserHasGroupXML(ctx, uid, "lending.group_lending_manager") {
			return "", fmt.Errorf("only a Lending Manager can %s this approval", verb)
		}
		bypass := orm.ContextWithBypass(ctx, true)
		rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
		if err != nil {
			return "", fmt.Errorf("approval not found")
		}
		current := ApprovalDecision(orm.AsString(rec["decision"]))
		if current != ApprovalDecisionPending && current != ApprovalDecisionEscalated {
			return "", fmt.Errorf("cannot %s an approval already %q", verb, current)
		}
		makerID := intOr(rec["maker_id"], 0)
		if makerID > 0 && makerID == uid {
			return "", fmt.Errorf("segregation of duties: the maker cannot be the checker")
		}
		upd := map[string]interface{}{
			"decision":   string(to),
			"decided_at": nowDateTime(),
		}
		if intOr(rec["checker_id"], 0) <= 0 {
			upd["checker_id"] = uid
		}
		if intOr(rec["approver_id"], 0) <= 0 {
			upd["approver_id"] = uid
		}
		if err := orm.UpdateRecordByID(bypass, model, id, upd); err != nil {
			return "", err
		}
		if appID := intOr(rec["application_id"], 0); appID > 0 {
			return fmt.Sprintf("/web?action=lending.action_loan_application&view_type=form&id=%d", appID), nil
		}
		return "", nil
	}
}

// approvalEscalate moves a pending approval to escalated for a higher authority.
func approvalEscalate(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
		return "", err
	}
	if !orm.UserHasGroupXML(ctx, uid, "lending.group_lending_manager") {
		return "", fmt.Errorf("only a Lending Manager can escalate this approval")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
	if err != nil {
		return "", fmt.Errorf("approval not found")
	}
	if ApprovalDecision(orm.AsString(rec["decision"])) != ApprovalDecisionPending {
		return "", fmt.Errorf("only a pending approval can be escalated")
	}
	upd := map[string]interface{}{
		"decision": string(ApprovalDecisionEscalated),
		"level":    intOr(rec["level"], 0) + 1,
	}
	if err := orm.UpdateRecordByID(bypass, model, id, upd); err != nil {
		return "", err
	}
	return "", nil
}

// ---------------------------------------------------------------------------
// Agreement acceptance
// ---------------------------------------------------------------------------

// agreementAccept marks the loan agreement as accepted by the borrower.
func agreementAccept(ctx context.Context, model string, id int, _ map[string]string) (string, error) {
	uid := orm.SecurityUID(ctx)
	if err := orm.CheckModelAccess(ctx, uid, model, "write"); err != nil {
		return "", err
	}
	if !orm.UserHasGroupXML(ctx, uid, "lending.group_lending_manager") {
		return "", fmt.Errorf("only a Lending Manager can accept an agreement")
	}
	bypass := orm.ContextWithBypass(ctx, true)
	rec, err := orm.SearchOne(bypass, model, map[string]interface{}{"id": id})
	if err != nil {
		return "", fmt.Errorf("agreement not found")
	}
	if b, ok := rec["accepted"].(bool); ok && b {
		return "", nil
	}
	if err := orm.UpdateRecordByID(bypass, model, id, map[string]interface{}{"accepted": true}); err != nil {
		return "", err
	}
	return "", nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

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
