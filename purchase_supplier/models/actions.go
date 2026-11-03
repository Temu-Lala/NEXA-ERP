package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nexa/core/orm"
)

// Supplier onboarding + governance actions (blueprint §3, §19). Transitions are
// validated server-side from the session uid so that separation of duties does
// not depend on the UI: a controlled profile/bank change must be verified by a
// second person, and approval is gated on due-diligence completion, accepted
// mandatory documents and the absence of an unresolved duplicate. This module
// writes only its own records; it never touches Inventory or Accounting.

func init() {
	// Supplier lifecycle.
	orm.RegisterObjectAction(suppModel, "action_submit", suppSubmit)
	orm.RegisterObjectAction(suppModel, "action_start_review", suppStartReview)
	orm.RegisterObjectAction(suppModel, "action_due_diligence", suppDueDiligence)
	orm.RegisterObjectAction(suppModel, "action_approve", suppApprove)
	orm.RegisterObjectAction(suppModel, "action_reject", suppReject)
	orm.RegisterObjectAction(suppModel, "action_activate", suppActivate)
	orm.RegisterObjectAction(suppModel, "action_suspend", suppSuspend)
	orm.RegisterObjectAction(suppModel, "action_block", suppBlock)
	orm.RegisterObjectAction(suppModel, "action_reactivate", suppReactivate)
	orm.RegisterObjectAction(suppModel, "action_archive", suppArchive)
	orm.RegisterObjectAction(suppModel, "action_check_duplicates", suppCheckDuplicates)

	// Documents.
	orm.RegisterObjectAction(docModel, "action_accept", docAccept)
	orm.RegisterObjectAction(docModel, "action_reject", docReject)

	// Qualifications gate sourcing.
	orm.RegisterObjectAction(qualModel, "action_grant", qualGrant)
	orm.RegisterObjectAction(qualModel, "action_revoke", qualRevoke)

	// Controlled change requests (SoD: verifier != requester).
	orm.RegisterObjectAction(chgModel, "action_submit", chgSubmit)
	orm.RegisterObjectAction(chgModel, "action_verify", chgVerify)
	orm.RegisterObjectAction(chgModel, "action_apply", chgApply)
	orm.RegisterObjectAction(chgModel, "action_reject", chgReject)

	// Bank detail independent verification.
	orm.RegisterObjectAction(bankModel, "action_verify", bankVerify)

	// Risk assessment roll-up.
	orm.RegisterObjectAction(riskModel, "action_compute", riskCompute)
}

// ---------------------------------------------------------------------------
// Supplier lifecycle
// ---------------------------------------------------------------------------

func suppSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, suppModel, id)
	if rec == nil {
		return "", fmt.Errorf("supplier not found")
	}
	if SupplierState(str(rec["state"])) != SuppDraft {
		return "", fmt.Errorf("only draft suppliers can be submitted")
	}
	name := str(rec["name"])
	if name == "" || name == "New" {
		name = nextRef(bypass, "SUPP")
	}
	if err := orm.UpdateRecordByID(bypass, suppModel, id, map[string]interface{}{
		"state": string(SuppSubmitted), "name": name,
	}); err != nil {
		return "", err
	}
	return "Supplier submitted for review.", nil
}

func suppStartReview(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return suppAdvance(ctx, id, SuppSubmitted, SuppUnderReview, "Supplier under review.")
}

func suppDueDiligence(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return suppAdvance(ctx, id, SuppUnderReview, SuppDueDiligence, "Due diligence in progress.")
}

// suppApprove gates approval on the evidence the blueprint requires: due
// diligence complete, no expired mandatory document and no unresolved
// duplicate flag. Approval is a compliance/manager act (RBAC via CSV).
func suppApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, suppModel, id)
	if rec == nil {
		return "", fmt.Errorf("supplier not found")
	}
	if SupplierState(str(rec["state"])) != SuppDueDiligence {
		return "", fmt.Errorf("only suppliers in due diligence can be approved")
	}
	if !boolOf(rec["due_diligence"]) {
		return "", fmt.Errorf("due diligence must be completed before approval")
	}
	if boolOf(rec["duplicate_flag"]) {
		return "", fmt.Errorf("resolve the possible-duplicate flag before approval")
	}
	bad, _ := orm.SearchCount(bypass, docModel, [][]interface{}{
		{"supplier_id", "=", id},
		{"mandatory", "=", true},
		{"active", "=", true},
		{"review_state", "in", []interface{}{"expired", "pending", "rejected"}},
	})
	if bad > 0 {
		return "", fmt.Errorf("mandatory documents are missing, expired or rejected")
	}
	if err := orm.UpdateRecordByID(bypass, suppModel, id, map[string]interface{}{
		"state": string(SuppApproved),
	}); err != nil {
		return "", err
	}
	return "Supplier approved.", nil
}

func suppReject(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return suppSet(ctx, id, []SupplierState{SuppSubmitted, SuppUnderReview, SuppDueDiligence, SuppApproved}, SuppRejected, "Supplier rejected.")
}

func suppActivate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, suppModel, id)
	if rec == nil {
		return "", fmt.Errorf("supplier not found")
	}
	if SupplierState(str(rec["state"])) != SuppApproved {
		return "", fmt.Errorf("only approved suppliers can be activated")
	}
	upd := map[string]interface{}{"state": string(SuppActive)}
	if str(rec["onboard_date"]) == "" {
		upd["onboard_date"] = time.Now().Format(dateLayout)
	}
	if err := orm.UpdateRecordByID(bypass, suppModel, id, upd); err != nil {
		return "", err
	}
	return "Supplier is now active and can be used on sourcing.", nil
}

func suppSuspend(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return suppSet(ctx, id, []SupplierState{SuppActive}, SuppSuspended, "Supplier suspended.")
}

func suppBlock(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	return suppSet(ctx, id, []SupplierState{SuppActive, SuppSuspended, SuppExpired}, SuppBlocked, "Supplier blocked.")
}

func suppReactivate(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, suppModel, id)
	if rec == nil {
		return "", fmt.Errorf("supplier not found")
	}
	switch SupplierState(str(rec["state"])) {
	case SuppSuspended, SuppExpired:
	default:
		return "", fmt.Errorf("only suspended or expired suppliers can be reactivated")
	}
	exp, _ := orm.SearchCount(bypass, docModel, [][]interface{}{
		{"supplier_id", "=", id}, {"mandatory", "=", true},
		{"review_state", "=", "expired"}, {"active", "=", true},
	})
	if exp > 0 {
		return "", fmt.Errorf("reactivation blocked: mandatory documents are still expired")
	}
	if err := orm.UpdateRecordByID(bypass, suppModel, id, map[string]interface{}{"state": string(SuppActive)}); err != nil {
		return "", err
	}
	return "Supplier reactivated.", nil
}

func suppArchive(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	if err := orm.UpdateRecordByID(bypass, suppModel, id, map[string]interface{}{
		"state": string(SuppArchived), "active": false,
	}); err != nil {
		return "", err
	}
	return "Supplier archived.", nil
}

// suppCheckDuplicates flags other active suppliers sharing a normalized legal
// name, registration number or tax id (blueprint §3 duplicate detection).
func suppCheckDuplicates(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, suppModel, id)
	if rec == nil {
		return "", fmt.Errorf("supplier not found")
	}
	normName := normalizeLegalName(str(rec["legal_name"]))
	regNo := str(rec["reg_no"])
	taxID := str(rec["tax_id"])
	others, _ := orm.SearchPage(bypass, suppModel, [][]interface{}{
		{"id", "!=", id}, {"state", "!=", string(SuppArchived)},
	}, 2000, 0, "id asc")
	var hits []string
	for _, o := range others {
		cand := supplierIdentity{
			name: normalizeLegalName(str(o["legal_name"])),
			reg:  str(o["reg_no"]),
			tax:  str(o["tax_id"]),
		}
		if isDuplicateSupplier(normName, regNo, taxID, cand) {
			hits = append(hits, str(o["name"]))
		}
	}
	upd := map[string]interface{}{"duplicate_flag": len(hits) > 0}
	note := ""
	if len(hits) > 0 {
		note = "Possible duplicate of: " + strings.Join(hits, ", ")
	}
	upd["duplicate_note"] = note
	if err := orm.UpdateRecordByID(bypass, suppModel, id, upd); err != nil {
		return "", err
	}
	if len(hits) > 0 {
		return fmt.Sprintf("Flagged: %d possible duplicate(s).", len(hits)), nil
	}
	return "No duplicates found.", nil
}

// ---------------------------------------------------------------------------
// Documents
// ---------------------------------------------------------------------------

func docAccept(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, docModel, id)
	if rec == nil {
		return "", fmt.Errorf("document not found")
	}
	upd := map[string]interface{}{"review_state": string(DocAccepted), "reject_why": ""}
	if uid := orm.UIDFromContext(ctx); uid > 0 {
		upd["reviewed_by"] = uid
	}
	if err := orm.UpdateRecordByID(bypass, docModel, id, upd); err != nil {
		return "", err
	}
	return "Document accepted.", nil
}

func docReject(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, docModel, id)
	if rec == nil {
		return "", fmt.Errorf("document not found")
	}
	upd := map[string]interface{}{"review_state": string(DocRejected)}
	if uid := orm.UIDFromContext(ctx); uid > 0 {
		upd["reviewed_by"] = uid
	}
	if r := strings.TrimSpace(vals["reason"]); r != "" {
		upd["reject_why"] = r
	}
	if err := orm.UpdateRecordByID(bypass, docModel, id, upd); err != nil {
		return "", err
	}
	return "Document rejected.", nil
}

// ---------------------------------------------------------------------------
// Qualifications
// ---------------------------------------------------------------------------

func qualGrant(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, qualModel, id)
	if rec == nil {
		return "", fmt.Errorf("qualification not found")
	}
	upd := map[string]interface{}{
		"state": string(QualActive), "approved_date": time.Now().Format(dateLayout),
	}
	if uid := orm.UIDFromContext(ctx); uid > 0 {
		upd["approved_by"] = uid
	}
	if err := orm.UpdateRecordByID(bypass, qualModel, id, upd); err != nil {
		return "", err
	}
	return "Qualification granted.", nil
}

func qualRevoke(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	if err := orm.UpdateRecordByID(bypass, qualModel, id, map[string]interface{}{
		"state": string(QualRevoked),
	}); err != nil {
		return "", err
	}
	return "Qualification revoked.", nil
}

// ---------------------------------------------------------------------------
// Controlled change requests (SoD enforced here)
// ---------------------------------------------------------------------------

func chgSubmit(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, chgModel, id)
	if rec == nil {
		return "", fmt.Errorf("change request not found")
	}
	if ChangeState(str(rec["state"])) != ChgDraft {
		return "", fmt.Errorf("only draft change requests can be submitted")
	}
	name := str(rec["name"])
	if name == "" || name == "New" {
		name = nextRef(bypass, "SCH")
	}
	upd := map[string]interface{}{"state": string(ChgSubmitted), "name": name}
	if uid := orm.UIDFromContext(ctx); uid > 0 {
		upd["requested_by"] = uid
	}
	if err := orm.UpdateRecordByID(bypass, chgModel, id, upd); err != nil {
		return "", err
	}
	return "Change request submitted for independent verification.", nil
}

// chgVerify records an independent verifier. Separation of duties: the person
// who requested the change may not be the one who verifies it.
func chgVerify(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, chgModel, id)
	if rec == nil {
		return "", fmt.Errorf("change request not found")
	}
	if ChangeState(str(rec["state"])) != ChgSubmitted {
		return "", fmt.Errorf("only submitted change requests can be verified")
	}
	uid := orm.UIDFromContext(ctx)
	if uid <= 0 {
		return "", fmt.Errorf("verification requires a logged-in second person")
	}
	if intID(rec["requested_by"]) == uid {
		return "", fmt.Errorf("separation of duties: the requester cannot verify this change")
	}
	if err := orm.UpdateRecordByID(bypass, chgModel, id, map[string]interface{}{
		"state": string(ChgVerified), "verified_by": uid,
	}); err != nil {
		return "", err
	}
	return "Change independently verified.", nil
}

// chgApply writes a verified change. Profile fields update the supplier; a bank
// change creates a new active account that must itself be verified before it
// becomes primary, so payment details never change silently.
func chgApply(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, chgModel, id)
	if rec == nil {
		return "", fmt.Errorf("change request not found")
	}
	if ChangeState(str(rec["state"])) != ChgVerified {
		return "", fmt.Errorf("only verified change requests can be applied")
	}
	sid := intID(rec["supplier_id"])
	kind := strings.ToLower(strings.TrimSpace(str(rec["field_kind"])))
	value := strings.TrimSpace(str(rec["proposed_value"]))
	switch kind {
	case "bank":
		if err := applyBankChange(bypass, sid, str(rec["target_summary"]), value); err != nil {
			return "", err
		}
	default:
		col, ok := profileColumn(kind)
		if !ok {
			return "", fmt.Errorf("unsupported change type %q", kind)
		}
		if sid <= 0 {
			return "", fmt.Errorf("change request has no supplier")
		}
		if err := orm.UpdateRecordByID(bypass, suppModel, sid, map[string]interface{}{col: value}); err != nil {
			return "", err
		}
	}
	if err := orm.UpdateRecordByID(bypass, chgModel, id, map[string]interface{}{
		"state": string(ChgApplied), "applied_on": time.Now().Format(dtLayout),
	}); err != nil {
		return "", err
	}
	return "Change applied and recorded.", nil
}

func chgReject(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, chgModel, id)
	if rec == nil {
		return "", fmt.Errorf("change request not found")
	}
	st := ChangeState(str(rec["state"]))
	if st != ChgSubmitted && st != ChgVerified {
		return "", fmt.Errorf("only submitted or verified changes can be rejected")
	}
	if err := orm.UpdateRecordByID(bypass, chgModel, id, map[string]interface{}{
		"state": string(ChgRejected),
	}); err != nil {
		return "", err
	}
	return "Change request rejected.", nil
}

func applyBankChange(ctx context.Context, supplierID int, label, accountRef string) error {
	if supplierID <= 0 {
		return fmt.Errorf("change request has no supplier")
	}
	inst, ok := orm.Registry[bankModel]
	if !ok {
		return fmt.Errorf("bank account model unavailable")
	}
	if _, err := orm.Create(ctx, inst, map[string]interface{}{
		"supplier_id": supplierID,
		"label":       firstNonEmpty(label, "Bank change"),
		"account_ref": accountRef,
		"state":       string(BankActive),
		"primary":     false,
		"verified":    false,
		"active":      true,
		"note":        "Created from a verified controlled change request; must be independently verified before use as primary.",
	}); err != nil {
		return err
	}
	return nil
}

func profileColumn(kind string) (string, bool) {
	switch kind {
	case "legal_name", "trading_name", "tax_id", "reg_no", "email", "phone", "website", "business_type":
		return kind, true
	default:
		return "", false
	}
}

// ---------------------------------------------------------------------------
// Bank account verification
// ---------------------------------------------------------------------------

func bankVerify(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, bankModel, id)
	if rec == nil {
		return "", fmt.Errorf("bank account not found")
	}
	upd := map[string]interface{}{"verified": true}
	if uid := orm.UIDFromContext(ctx); uid > 0 {
		upd["verified_by"] = uid
	}
	if err := orm.UpdateRecordByID(bypass, bankModel, id, upd); err != nil {
		return "", err
	}
	return "Bank details independently verified.", nil
}

// ---------------------------------------------------------------------------
// Risk assessment roll-up
// ---------------------------------------------------------------------------

func riskCompute(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, riskModel, id)
	if rec == nil {
		return "", fmt.Errorf("risk assessment not found")
	}
	dims := []int{
		int(flt(rec["document_risk"])), int(flt(rec["financial_risk"])),
		int(flt(rec["delivery_risk"])), int(flt(rec["quality_risk"])),
		int(flt(rec["geographic_risk"])), int(flt(rec["concentr_risk"])),
		int(flt(rec["security_risk"])), int(flt(rec["continuity_risk"])),
		int(flt(rec["ethics_risk"])), int(flt(rec["regulatory_risk"])),
	}
	overall := riskOverall(dims)
	level := riskLevel(overall)
	factors := fmt.Sprintf("Document %d, Financial %d, Delivery %d, Quality %d, Geographic %d, Concentration %d, Security %d, Continuity %d, Ethics %d, Regulatory %d.",
		dims[0], dims[1], dims[2], dims[3], dims[4], dims[5], dims[6], dims[7], dims[8], dims[9])
	upd := map[string]interface{}{
		"overall_score": overall, "level": string(level), "factors": factors,
	}
	if uid := orm.UIDFromContext(ctx); uid > 0 {
		upd["assessed_by"] = uid
	}
	if err := orm.UpdateRecordByID(bypass, riskModel, id, upd); err != nil {
		return "", err
	}
	if sid := intID(rec["supplier_id"]); sid > 0 {
		_ = orm.UpdateRecordByID(bypass, suppModel, sid, map[string]interface{}{
			"risk": string(level), "risk_score": overall,
		})
	}
	return fmt.Sprintf("Risk level: %s (score %.2f).", level, overall), nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func suppAdvance(ctx context.Context, id int, from, to SupplierState, msg string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, suppModel, id)
	if rec == nil {
		return "", fmt.Errorf("supplier not found")
	}
	if SupplierState(str(rec["state"])) != from {
		return "", fmt.Errorf("transition requires state %s", from)
	}
	if err := orm.UpdateRecordByID(bypass, suppModel, id, map[string]interface{}{"state": string(to)}); err != nil {
		return "", err
	}
	return msg, nil
}

func suppSet(ctx context.Context, id int, from []SupplierState, to SupplierState, msg string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, suppModel, id)
	if rec == nil {
		return "", fmt.Errorf("supplier not found")
	}
	cur := SupplierState(str(rec["state"]))
	allowed := false
	for _, s := range from {
		if cur == s {
			allowed = true
			break
		}
	}
	if !allowed {
		return "", fmt.Errorf("transition to %s not allowed from %s", to, cur)
	}
	if err := orm.UpdateRecordByID(bypass, suppModel, id, map[string]interface{}{"state": string(to)}); err != nil {
		return "", err
	}
	return msg, nil
}

func nextRef(ctx context.Context, prefix string) string {
	if name, err := orm.NextSequence(ctx, suppModel); err == nil && name != "" {
		return name
	}
	return fmt.Sprintf("%s/%05d", prefix, time.Now().Unix()%100000)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
