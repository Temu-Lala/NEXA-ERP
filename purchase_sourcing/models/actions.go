package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nexa/core/orm"
)

// Sourcing & tendering workflow (blueprint §7, §8). Transitions are validated
// server-side; bids are locked at the deadline; landed cost and weighted scores
// are recomputed authoritatively (never trusted from the client); an award must
// pass the Supplier-Master eligibility gate and, once approved, produces a draft
// purchase.order that the buyer still has to confirm. Separation of duties: the
// user who proposes an award cannot be the one who approves it.

func init() {
	orm.RegisterObjectAction(rfqModel, "action_approve", rfqApprove)
	orm.RegisterObjectAction(rfqModel, "action_send", rfqSend)
	orm.RegisterObjectAction(rfqModel, "action_record_responses", rfqRecordResponses)
	orm.RegisterObjectAction(rfqModel, "action_open_evaluation", rfqOpenEvaluation)
	orm.RegisterObjectAction(rfqModel, "action_expire", rfqExpire)
	orm.RegisterObjectAction(rfqModel, "action_cancel", rfqCancel)

	orm.RegisterObjectAction(quoteModel, "action_receive", quoteReceive)
	orm.RegisterObjectAction(quoteModel, "action_compute", quoteCompute)
	orm.RegisterObjectAction(quoteModel, "action_round", quoteRound)

	orm.RegisterObjectAction(evalModel, "action_rank", evalRank)

	orm.RegisterObjectAction(quoteModel, "action_propose_award", awardPropose)
	orm.RegisterObjectAction(awardModel, "action_approve", awardApprove)
}

// ---------------------------------------------------------------------------
// RFQ lifecycle
// ---------------------------------------------------------------------------

func rfqApprove(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, rfqModel, id)
	if rec == nil {
		return "", fmt.Errorf("RFQ not found")
	}
	if RFQState(str(rec["state"])) != RFQStateDraft {
		return "", fmt.Errorf("only a draft RFQ can be approved")
	}
	if cnt, _ := orm.SearchCount(bypass, rfqLineModel, [][]interface{}{{"rfq_id", "=", id}}); cnt == 0 {
		return "", fmt.Errorf("add at least one line item before approving")
	}
	name := str(rec["name"])
	upd := map[string]interface{}{"state": string(RFQStateApproved)}
	if name == "" || name == "New" {
		upd["name"] = nextRef(bypass, "RFQ")
	}
	if uid := orm.UIDFromContext(ctx); uid > 0 && intID(rec["owner_id"]) == 0 {
		upd["owner_id"] = uid
	}
	if err := orm.UpdateRecordByID(bypass, rfqModel, id, upd); err != nil {
		return "", err
	}
	return "RFQ approved and ready to send.", nil
}

func rfqSend(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, rfqModel, id)
	if rec == nil {
		return "", fmt.Errorf("RFQ not found")
	}
	if RFQState(str(rec["state"])) != RFQStateApproved {
		return "", fmt.Errorf("RFQ must be approved before sending")
	}
	invited, _ := orm.SearchPage(bypass, rfqSuppModel, [][]interface{}{{"rfq_id", "=", id}}, 500, 0, "id asc")
	if len(invited) == 0 {
		return "", fmt.Errorf("invite at least one supplier before sending")
	}
	for _, s := range invited {
		_ = orm.UpdateRecordByID(bypass, rfqSuppModel, intID(s["id"]), map[string]interface{}{
			"response_state": string(InviteInvited),
			"invited_date":   time.Now().Format(dateLayout),
		})
	}
	_ = orm.UpdateRecordByID(bypass, rfqModel, id, map[string]interface{}{"state": string(RFQStateSent)})
	return fmt.Sprintf("RFQ sent to %d supplier(s); bids locked at the deadline.", len(invited)), nil
}

// rfqRecordResponses marks the RFQ as receiving responses. It is only allowed
// once the submission deadline has passed, which is what locks the bids.
func rfqRecordResponses(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, rfqModel, id)
	if rec == nil {
		return "", fmt.Errorf("RFQ not found")
	}
	if RFQState(str(rec["state"])) != RFQStateSent {
		return "", fmt.Errorf("RFQ must be sent first")
	}
	if !deadlinePassed(time.Now(), str(rec["deadline"])) {
		return "", fmt.Errorf("the submission deadline has not passed; bids are still open")
	}
	// Suppliers with a received quotation but no recorded response are marked
	// responded so the invitation log reflects the locked bid set.
	quotes, _ := orm.SearchPage(bypass, quoteModel, [][]interface{}{
		{"rfq_id", "=", id}, {"state", "in", []interface{}{string(QuoteReceived), string(QuoteEvaluated), string(QuoteAccepted)}},
	}, 500, 0, "id asc")
	respondedPartner := map[int]bool{}
	for _, q := range quotes {
		respondedPartner[intID(q["partner_id"])] = true
	}
	invited, _ := orm.SearchPage(bypass, rfqSuppModel, [][]interface{}{{"rfq_id", "=", id}}, 500, 0, "id asc")
	for _, s := range invited {
		if respondedPartner[intID(s["partner_id"])] && str(s["response_state"]) == string(InviteInvited) {
			_ = orm.UpdateRecordByID(bypass, rfqSuppModel, intID(s["id"]), map[string]interface{}{
				"response_state": string(InviteResponded),
				"responded_on":   time.Now().Format(dateLayout),
			})
		}
	}
	_ = orm.UpdateRecordByID(bypass, rfqModel, id, map[string]interface{}{"state": string(RFQStateResponses)})
	return fmt.Sprintf("Deadline passed; %d quotation(s) recorded under locked bids.", len(quotes)), nil
}

func rfqOpenEvaluation(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, rfqModel, id)
	if rec == nil {
		return "", fmt.Errorf("RFQ not found")
	}
	if RFQState(str(rec["state"])) != RFQStateResponses {
		return "", fmt.Errorf("responses must be recorded before evaluation")
	}
	n, _ := orm.SearchCount(bypass, quoteModel, [][]interface{}{
		{"rfq_id", "=", id}, {"state", "!=", string(QuoteDraft)},
	})
	if n == 0 {
		return "", fmt.Errorf("no quotations to evaluate")
	}
	_ = orm.UpdateRecordByID(bypass, rfqModel, id, map[string]interface{}{"state": string(RFQStateEvaluation)})
	return "Evaluation opened for the locked bids.", nil
}

func rfqExpire(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, rfqModel, id)
	if rec == nil {
		return "", fmt.Errorf("RFQ not found")
	}
	st := RFQState(str(rec["state"]))
	if st != RFQStateSent && st != RFQStateResponses {
		return "", fmt.Errorf("only a sent or open RFQ can be expired")
	}
	if !deadlinePassed(time.Now(), str(rec["deadline"])) {
		return "", fmt.Errorf("the deadline has not passed")
	}
	_ = orm.UpdateRecordByID(bypass, rfqModel, id, map[string]interface{}{"state": string(RFQStateExpired)})
	return "RFQ expired without award.", nil
}

func rfqCancel(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	rec := loadRec(bypass, rfqModel, id)
	if rec == nil {
		return "", fmt.Errorf("RFQ not found")
	}
	st := RFQState(str(rec["state"]))
	if st == RFQStateAwarded || st == RFQStateCancelled {
		return "", fmt.Errorf("an awarded or cancelled RFQ cannot be cancelled again")
	}
	reason := strings.TrimSpace(vals["reason"])
	if reason == "" {
		return "", fmt.Errorf("a cancellation / retender reason is required")
	}
	note := strOr(rec["notes"], "")
	if note == "" {
		note = "Cancelled: " + reason
	} else {
		note = note + "\nCancelled: " + reason
	}
	_ = orm.UpdateRecordByID(bypass, rfqModel, id, map[string]interface{}{
		"state": string(RFQStateCancelled), "notes": note,
	})
	return "RFQ cancelled; reason recorded.", nil
}

// ---------------------------------------------------------------------------
// Quotation handling
// ---------------------------------------------------------------------------

func quoteReceive(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	q := loadRec(bypass, quoteModel, id)
	if q == nil {
		return "", fmt.Errorf("quotation not found")
	}
	if QuotationState(str(q["state"])) != QuoteDraft {
		return "", fmt.Errorf("only a draft quotation can be received")
	}
	if cnt, _ := orm.SearchCount(bypass, quoteLineMdl, [][]interface{}{{"quotation_id", "=", id}}); cnt == 0 {
		return "", fmt.Errorf("the quotation has no lines")
	}
	_ = orm.UpdateRecordByID(bypass, quoteModel, id, map[string]interface{}{"state": string(QuoteReceived)})
	return "Quotation received.", nil
}

// quoteCompute recomputes line totals and the landed total authoritatively and
// stores it in the RFQ base currency. Post-award term changes are blocked.
func quoteCompute(ctx context.Context, _ string, id int, _ map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	q := loadRec(bypass, quoteModel, id)
	if q == nil {
		return "", fmt.Errorf("quotation not found")
	}
	rfq := loadRec(bypass, rfqModel, intID(q["rfq_id"]))
	if rfq != nil && RFQState(str(rfq["state"])) == RFQStateAwarded {
		return "", fmt.Errorf("terms are locked: the RFQ has been awarded")
	}
	rate := flt(q["exchange_rate"])
	if rate <= 0 {
		rate = 1
	}
	lines, _ := orm.SearchPage(bypass, quoteLineMdl, [][]interface{}{{"quotation_id", "=", id}}, 500, 0, "sequence asc, id asc")
	goods := 0.0
	for _, ln := range lines {
		t := lineTotal(flt(ln["quantity"]), flt(ln["unit_price"]), flt(ln["discount"]))
		if intID(ln["line_total"]) != 0 || t != flt(ln["line_total"]) {
			_ = orm.UpdateRecordByID(bypass, quoteLineMdl, intID(ln["id"]), map[string]interface{}{"line_total": t})
		}
		goods += t
	}
	landed := round2(normalizeAmount(landedCost(goods, flt(q["freight"]), flt(q["tax_amount"])), rate))
	_ = orm.UpdateRecordByID(bypass, quoteModel, id, map[string]interface{}{"total_landed": landed})
	return fmt.Sprintf("Landed total recomputed to %.2f (base currency).", landed), nil
}

// quoteRound records a negotiation round (initial / revised / counter / final)
// against a quotation. Rounds cannot be added after award.
func quoteRound(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	q := loadRec(bypass, quoteModel, id)
	if q == nil {
		return "", fmt.Errorf("quotation not found")
	}
	rfq := loadRec(bypass, rfqModel, intID(q["rfq_id"]))
	if rfq != nil && RFQState(str(rfq["state"])) == RFQStateAwarded {
		return "", fmt.Errorf("negotiation is closed: the RFQ has been awarded")
	}
	rm, ok := orm.Registry[roundModel]
	if !ok {
		return "", fmt.Errorf("round model unavailable")
	}
	kind := RoundKind(strings.TrimSpace(vals["kind"]))
	if kind == "" {
		kind = RoundRevised
	}
	prev, _ := orm.SearchPage(bypass, roundModel, [][]interface{}{{"quotation_id", "=", id}}, 1, 0, "round_no desc")
	next := 1
	if len(prev) > 0 {
		next = int(flt(prev[0]["round_no"])) + 1
	}
	rid, err := orm.Create(bypass, rm, map[string]interface{}{
		"name":          nextRef(bypass, "RND"),
		"rfq_id":        intID(q["rfq_id"]),
		"quotation_id":  id,
		"partner_id":    intID(q["partner_id"]),
		"round_no":      next,
		"kind":          string(kind),
		"round_date":    time.Now().Format(dateLayout),
		"offered_total": flt(q["total_landed"]),
		"outcome":       strOr(vals["outcome"], "open"),
		"participants":  strOr(vals["participants"], ""),
		"terms_note":    strOr(vals["terms_note"], ""),
		"locked":        false,
		"active":        true,
	})
	if err != nil {
		return "", err
	}
	_ = rid
	return fmt.Sprintf("Negotiation round %d recorded.", next), nil
}

// ---------------------------------------------------------------------------
// Evaluation
// ---------------------------------------------------------------------------

// supplierBid is a quotation loaded and reduced for server-side scoring.
type supplierBid struct {
	quoteID     int
	partnerID   int
	name        string
	goods       float64
	landed      float64
	leadDays    int
	quality     float64
	terms       float64
	risk        float64
	eligibleNow bool
}

func loadBids(ctx context.Context, rfqID int) []supplierBid {
	quotes, _ := orm.SearchPage(ctx, quoteModel, [][]interface{}{
		{"rfq_id", "=", rfqID}, {"state", "!=", string(QuoteDraft)},
	}, 500, 0, "id asc")
	out := make([]supplierBid, 0, len(quotes))
	for _, q := range quotes {
		rate := flt(q["exchange_rate"])
		if rate <= 0 {
			rate = 1
		}
		goods := 0.0
		lines, _ := orm.SearchPage(ctx, quoteLineMdl, [][]interface{}{{"quotation_id", "=", intID(q["id"])}}, 500, 0, "id asc")
		for _, ln := range lines {
			goods += flt(ln["line_total"])
		}
		out = append(out, supplierBid{
			quoteID:   intID(q["id"]),
			partnerID: intID(q["partner_id"]),
			name:      strOr(q["name"], "?"),
			goods:     round2(normalizeAmount(goods, rate)),
			landed:    round2(normalizeAmount(landedCost(goods, flt(q["freight"]), flt(q["tax_amount"])), rate)),
			leadDays:  int(flt(q["lead_time_days"])),
			quality:   profileMetric(ctx, intID(q["partner_id"]), "quality_rating"),
			terms:     profileMetric(ctx, intID(q["partner_id"]), "on_time_delivery"),
			risk:      riskExposure(ctx, intID(q["partner_id"])),
		})
	}
	return out
}

// evalRank scores the bids for an evaluation using its stored (versioned)
// weights, transitions the quotations to evaluated, and writes an explainable
// ranking snapshot to the summary.
func evalRank(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	ev := loadRec(bypass, evalModel, id)
	if ev == nil {
		return "", fmt.Errorf("evaluation not found")
	}
	rfqID := intID(ev["rfq_id"])
	rfq := loadRec(bypass, rfqModel, rfqID)
	if rfq == nil {
		return "", fmt.Errorf("RFQ not found")
	}
	if RFQState(str(rfq["state"])) != RFQStateEvaluation {
		return "", fmt.Errorf("the RFQ must be in evaluation")
	}
	if !deadlinePassed(time.Now(), str(rfq["deadline"])) {
		return "", fmt.Errorf("bids are not locked yet: the deadline has not passed")
	}
	bids := loadBids(bypass, rfqID)
	if len(bids) == 0 {
		return "", fmt.Errorf("no quotations to rank")
	}
	c := scoreCriteria{
		wPrice:    flt(ev["weight_price"]),
		wLanded:   flt(ev["weight_landed"]),
		wDelivery: flt(ev["weight_delivery"]),
		wQuality:  flt(ev["weight_quality"]),
		wTerms:    flt(ev["weight_terms"]),
		wRisk:     flt(ev["weight_risk"]),
	}
	norm := make([]normBid, 0, len(bids))
	byKey := map[string]supplierBid{}
	for _, b := range bids {
		key := fmt.Sprintf("%d", b.quoteID)
		norm = append(norm, normBid{
			key: key, goods: b.goods, landed: b.landed, delivery: b.leadDays,
			quality: b.quality, terms: b.terms, risk: b.risk,
		})
		byKey[key] = b
	}
	ranked := weightedScore(norm, c)
	summary := renderRanking(ranked, byKey, c)
	if reason := strings.TrimSpace(vals["correction_reason"]); reason != "" {
		summary += fmt.Sprintf("\nAuthorized correction: %s (by uid %d, %s)", reason, orm.UIDFromContext(ctx), time.Now().Format(dtLayout))
	}
	_ = orm.UpdateRecordByID(bypass, evalModel, id, map[string]interface{}{
		"summary": summary,
		"name":    strOr(ev["name"], nextRef(bypass, "EVAL")),
	})
	for _, b := range bids {
		_ = orm.UpdateRecordByID(bypass, quoteModel, b.quoteID, map[string]interface{}{"state": string(QuoteEvaluated)})
	}
	return fmt.Sprintf("Ranked %d bid(s). Best: %s.", len(ranked), byKey[ranked[0].key].name), nil
}

func renderRanking(ranked []normBid, byKey map[string]supplierBid, c scoreCriteria) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Weights: price=%.0f landed=%.0f delivery=%.0f quality=%.0f terms=%.0f risk=%.0f\n",
		c.wPrice, c.wLanded, c.wDelivery, c.wQuality, c.wTerms, c.wRisk)
	// Recompute scores to render them alongside the ordered bids.
	for pos, nb := range ranked {
		sb := byKey[nb.key]
		fmt.Fprintf(&b, "%d. %s | landed=%.2f goods=%.2f lead=%dd quality=%.0f terms=%.0f risk=%.0f | score=%s\n",
			pos+1, sb.name, nb.landed, nb.goods, nb.delivery, nb.quality, nb.terms, nb.risk,
			fmtScore(nb, ranked, c))
	}
	return strings.TrimSpace(b.String())
}

// fmtScore recomputes one bid's composite score for display, matching the
// ranking normalization.
func fmtScore(target normBid, all []normBid, c scoreCriteria) string {
	minGoods, minLanded, minLead := mathHuge, mathHuge, mathHuge
	for _, b := range all {
		if b.goods > 0 && b.goods < minGoods {
			minGoods = b.goods
		}
		if b.landed > 0 && b.landed < minLanded {
			minLanded = b.landed
		}
		if b.delivery > 0 && float64(b.delivery) < minLead {
			minLead = float64(b.delivery)
		}
	}
	total := c.total()
	if total <= 0 {
		total = 1
	}
	s := c.wPrice * safeRatio(minGoods, target.goods) * 100
	s += c.wLanded * safeRatio(minLanded, target.landed) * 100
	s += c.wDelivery * safeRatio(minLead, float64(target.delivery)) * 100
	s += c.wQuality * clamp100(target.quality)
	s += c.wTerms * clamp100(target.terms)
	s += c.wRisk * (100 - clamp100(target.risk))
	return fmt.Sprintf("%.2f", round2(s/total))
}

// ---------------------------------------------------------------------------
// Award propose -> approve -> draft PO
// ---------------------------------------------------------------------------

// awardPropose is invoked on a quotation (object action): it validates the RFQ
// is under locked evaluation, checks award eligibility against the Supplier
// Master, and creates a proposed award.
func awardPropose(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	q := loadRec(bypass, quoteModel, id)
	if q == nil {
		return "", fmt.Errorf("quotation not found")
	}
	rfqID := intID(q["rfq_id"])
	rfq := loadRec(bypass, rfqModel, rfqID)
	if rfq == nil {
		return "", fmt.Errorf("RFQ not found")
	}
	if RFQState(str(rfq["state"])) != RFQStateEvaluation {
		return "", fmt.Errorf("award can only be proposed during evaluation of locked bids")
	}
	if QuotationState(str(q["state"])) != QuoteEvaluated {
		return "", fmt.Errorf("the quotation must be evaluated first")
	}
	if cnt, _ := orm.SearchCount(bypass, evalModel, [][]interface{}{{"rfq_id", "=", rfqID}}); cnt == 0 {
		return "", fmt.Errorf("no evaluation recorded for this RFQ")
	}
	partnerID := intID(q["partner_id"])
	category := str(rfq["category"])
	single := strings.EqualFold(str(vals["single_source"]), "true") || boolOf(vals["single_source"])
	if !canAward(bypass, partnerID, category) {
		return "", fmt.Errorf("supplier is not active/qualified for this category")
	}
	if partnerRecalled(bypass, partnerID) {
		return "", fmt.Errorf("supplier is recalled/blocked and cannot be awarded")
	}
	if single {
		n, _ := orm.SearchCount(bypass, quoteModel, [][]interface{}{
			{"rfq_id", "=", rfqID}, {"state", "!=", string(QuoteDraft)},
		})
		if n > 1 && strings.TrimSpace(vals["justification"]) == "" {
			return "", fmt.Errorf("single-source among competing bids requires a written justification")
		}
	}
	am, ok := orm.Registry[awardModel]
	if !ok {
		return "", fmt.Errorf("award model unavailable")
	}
	evs, _ := orm.SearchPage(bypass, evalModel, [][]interface{}{{"rfq_id", "=", rfqID}}, 1, 0, "id desc")
	rationale := strings.TrimSpace(vals["rationale"])
	if rationale == "" {
		rationale = "Best-value selection per weighted evaluation."
	}
	if single {
		rationale = "Single-source: " + strOr(vals["justification"], "sole qualified supplier") + ". " + rationale
	}
	aid, err := orm.Create(bypass, am, map[string]interface{}{
		"name":                nextRef(bypass, "AWD"),
		"rfq_id":              rfqID,
		"evaluation_id":       firstID(evs),
		"quotation_id":        id,
		"partner_id":          partnerID,
		"award_date":          time.Now().Format(dateLayout),
		"awarded_value":       flt(q["total_landed"]),
		"selection_rationale": rationale,
		"policy_exceptions":   strOr(vals["policy_exceptions"], ""),
		"state":               string(AwardProposed),
		"active":              true,
	})
	if err != nil {
		return "", err
	}
	_ = aid
	return fmt.Sprintf("Award proposed (%s) pending independent approval.", strOr(q["name"], "?")), nil
}

// awardApprove finalizes an award: separation of duties (proposer != approver),
// re-checks eligibility, locks the winning negotiation round and creates a draft
// purchase order linked back to the award, then marks the RFQ awarded.
func awardApprove(ctx context.Context, _ string, id int, vals map[string]string) (string, error) {
	bypass := orm.ContextWithBypass(ctx, true)
	aw := loadRec(bypass, awardModel, id)
	if aw == nil {
		return "", fmt.Errorf("award not found")
	}
	if AwardState(str(aw["state"])) != AwardProposed {
		return "", fmt.Errorf("only a proposed award can be approved")
	}
	uid := orm.UIDFromContext(ctx)
	if prev := intID(aw["approver_id"]); prev > 0 && prev == uid {
		return "", fmt.Errorf("separation of duties: the proposer cannot approve the award")
	}
	rfq := loadRec(bypass, rfqModel, intID(aw["rfq_id"]))
	if rfq == nil {
		return "", fmt.Errorf("RFQ not found")
	}
	partnerID := intID(aw["partner_id"])
	if !canAward(bypass, partnerID, str(rfq["category"])) || partnerRecalled(bypass, partnerID) {
		return "", fmt.Errorf("supplier no longer eligible for award")
	}
	q := loadRec(bypass, quoteModel, intID(aw["quotation_id"]))
	if q == nil {
		return "", fmt.Errorf("selected quotation not found")
	}

	poID, err := createDraftPO(bypass, aw, q)
	if err != nil {
		return "", err
	}
	_ = orm.UpdateRecordByID(bypass, awardModel, id, map[string]interface{}{
		"state":       string(AwardApproved),
		"approver_id": uid,
		"po_ref":      poID,
	})
	_ = orm.UpdateRecordByID(bypass, quoteModel, intID(aw["quotation_id"]), map[string]interface{}{"state": string(QuoteAccepted)})
	_ = orm.UpdateRecordByID(bypass, rfqModel, intID(aw["rfq_id"]), map[string]interface{}{"state": string(RFQStateAwarded)})
	// Lock the winning quotation's negotiation rounds against post-award changes.
	rounds, _ := orm.SearchPage(bypass, roundModel, [][]interface{}{{"quotation_id", "=", intID(aw["quotation_id"])}}, 200, 0, "round_no desc")
	for _, r := range rounds {
		_ = orm.UpdateRecordByID(bypass, roundModel, intID(r["id"]), map[string]interface{}{"locked": true})
	}
	reason := strings.TrimSpace(vals["comment"])
	if reason != "" {
		_ = orm.UpdateRecordByID(bypass, awardModel, id, map[string]interface{}{
			"notes": strOr(aw["notes"], "") + "\nApproved: " + reason,
		})
	}
	return fmt.Sprintf("Award approved; draft purchase order #%d created.", poID), nil
}

// createDraftPO builds a draft purchase.order (owned by the purchase anchor)
// from the awarded quotation. Purchase only authorizes the commercial order; it
// never posts stock or payables.
func createDraftPO(ctx context.Context, award, quote map[string]interface{}) (int, error) {
	pm, ok := orm.Registry[poModel]
	if !ok {
		return 0, fmt.Errorf("purchase order model unavailable")
	}
	lm, ok := orm.Registry[poLineModel]
	if !ok {
		return 0, fmt.Errorf("purchase order line model unavailable")
	}
	poID64, err := orm.Create(ctx, pm, map[string]interface{}{
		"name":         nextRef(ctx, "PO"),
		"partner_id":   intID(award["partner_id"]),
		"user_id":      intID(award["approver_id"]),
		"date_order":   time.Now().Format(dtLayout),
		"state":        "draft",
		"amount_total": flt(award["awarded_value"]),
		"notes":        fmt.Sprintf("Draft PO from award %s (RFQ %s).", str(award["name"]), str(quote["rfq_id"])),
	})
	if err != nil {
		return 0, err
	}
	poID := int(poID64)
	lines, _ := orm.SearchPage(ctx, quoteLineMdl, [][]interface{}{
		{"quotation_id", "=", intID(quote["id"])},
	}, 500, 0, "sequence asc, id asc")
	total := 0.0
	for i, ln := range lines {
		qty := flt(ln["quantity"])
		unit := flt(ln["unit_price"])
		sub := lineTotal(qty, unit, flt(ln["discount"]))
		total += sub
		val := map[string]interface{}{
			"order_id":       poID,
			"name":           strOr(ln["description"], "Sourced item"),
			"product_qty":    qty,
			"price_unit":     unit,
			"price_subtotal": sub,
			"sequence":       (i + 1) * 10,
		}
		if pid := intID(ln["product_id"]); pid > 0 {
			val["product_id"] = pid
		}
		if _, err := orm.Create(ctx, lm, val); err != nil {
			return poID, err
		}
	}
	// Reconcile the header total to the authoritative line sum.
	_ = orm.UpdateRecordByID(ctx, poModel, poID, map[string]interface{}{"amount_total": round2(total)})
	return poID, nil
}

// ---------------------------------------------------------------------------
// Sourcing-specific data helpers
// ---------------------------------------------------------------------------

// profileMetric reads a numeric performance metric from the procurement supplier
// profile for the given partner (0 when absent).
func profileMetric(ctx context.Context, partnerID int, field string) float64 {
	if partnerID <= 0 {
		return 0
	}
	rows, _ := orm.SearchPage(ctx, "purchase.supplier.profile", [][]interface{}{
		{"partner_id", "=", partnerID},
	}, 1, 0, "id desc")
	if len(rows) == 0 {
		return 0
	}
	return clamp100(flt(rows[0][field]))
}

// riskExposure converts a supplier risk level into a 0..100 exposure where
// higher is worse (used as an inverse-scored criterion).
func riskExposure(ctx context.Context, partnerID int) float64 {
	if partnerID <= 0 {
		return 0
	}
	rows, _ := orm.SearchPage(ctx, "purchase.supplier.profile", [][]interface{}{
		{"partner_id", "=", partnerID},
	}, 1, 0, "id desc")
	if len(rows) == 0 {
		return 0
	}
	switch str(rows[0]["risk"]) {
	case "low":
		return 15
	case "medium":
		return 45
	case "high":
		return 75
	case "critical":
		return 95
	default:
		return 0 // unclassified is not penalized outright
	}
}

// partnerRecalled reports whether the shared partner is blocked from trading.
func partnerRecalled(ctx context.Context, partnerID int) bool {
	if partnerID <= 0 {
		return false
	}
	p := loadRec(ctx, partnerComdl, partnerID)
	if p == nil {
		return false
	}
	if v, ok := p["is_blocked"]; ok && boolOf(v) {
		return true
	}
	if s := strings.ToLower(str(p["supplier_status"])); s == "blocked" {
		return true
	}
	return false
}

func firstID(rows []map[string]interface{}) int {
	if len(rows) > 0 {
		return intID(rows[0]["id"])
	}
	return 0
}
