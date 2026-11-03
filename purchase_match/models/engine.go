package models

import (
	"context"
	"fmt"
	"hash/fnv"
	"math"
	"strings"
	"time"

	"nexa/core/orm"
)

// Supplier invoice matching engines (blueprint §12). Two/three/four-way matching
// is computed server-side against the PO, the accepted receipt (from T4) and the
// supplier bill. Purchase owns only the VALIDATION: the authoritative bill and
// any payment stay with Accounting / AP, so nothing here writes to `account.move`
// or authorizes a payment. Money is scale=2 commercial, quantities scale=3.

const (
	caseModel     = "purchase.invoice.match.case"
	lineModel     = "purchase.invoice.match.line"
	excModel      = "purchase.invoice.match.exception"
	eventModel    = "purchase.invoice.match.event"
	poModel       = "purchase.order"
	poLineModel   = "purchase.order.line"
	rcptModel     = "purchase.receipt"
	rcptLineModel = "purchase.receipt.line"
	commitmentMdl = "purchase.budget.commitment"

	dateLayout = "2006-01-02"
	dtLayout   = "2006-01-02 15:04:05"

	// eps guards decimal comparisons so a variance that is only floating noise
	// is never treated as a breach (mirrors the receipt engine's scale-3 floor).
	eps = 0.0005
)

// ---------------------------------------------------------------------------
// Pure matching maths (unit tested)
// ---------------------------------------------------------------------------

// lineVar is the reconciliation outcome for one compared line.
type lineVar struct {
	QtyExc     bool
	PriceExc   bool
	QtyVar     float64
	PriceVar   float64
	AmountVar  float64
	Approvable float64
}

// reconcileMatchLine compares an invoiced quantity/price against the expected
// baseline (accepted quantity for three/four-way, ordered for two-way). Only an
// OVER-invoice beyond tolerance raises a quantity exception; billing less than
// received is a legitimate partial invoice. A price exception fires in either
// direction once the unit-price variance breaches the percentage tolerance. The
// approvable amount is the safe payable value: the lower of invoiced vs accepted
// priced at the contracted PO rate.
func reconcileMatchLine(expectedQty, invoicedQty, poPrice, invPrice, qtyTolPct, priceTolPct, rounding float64) lineVar {
	v := lineVar{
		QtyVar:     round3(invoicedQty - expectedQty),
		PriceVar:   round2(invPrice - poPrice),
		Approvable: round2(math.Min(invoicedQty, expectedQty) * poPrice),
	}
	v.AmountVar = round2(invoicedQty*invPrice - expectedQty*poPrice)

	qtyTol := math.Abs(expectedQty) * qtyTolPct / 100.0
	if v.QtyVar > qtyTol+rounding+eps {
		v.QtyExc = true
	}

	priceTol := math.Abs(poPrice) * priceTolPct / 100.0
	if math.Abs(poPrice) <= eps && math.Abs(invPrice) <= eps {
		// both zero: no price exception
	} else if math.Abs(v.PriceVar) > priceTol+eps {
		v.PriceExc = true
	}
	return v
}

// taxVarianceOk reports whether an invoiced tax sits inside a percentage
// tolerance of the expected tax.
func taxVarianceOk(expectedTax, actualTax, tolPct float64) bool {
	diff := math.Abs(actualTax - expectedTax)
	limit := math.Abs(expectedTax) * tolPct / 100.0
	return diff <= limit+eps
}

// approvableAmount is the total value eligible to hand to AP: the sum of the
// safe per-line payable amounts (never paying for goods not accepted).
func approvableAmount(lines []lineVar) float64 {
	total := 0.0
	for _, l := range lines {
		total += l.Approvable
	}
	return round2(total)
}

// matchOutcome returns the case result given the exception count and whether a
// duplicate was detected. A duplicate always forces an exception.
func matchOutcome(excCount int, duplicate bool) MatchResult {
	if duplicate || excCount > 0 {
		return ResultException
	}
	return ResultMatched
}

// duplicateFingerprint builds a stable identity for an inbound bill from the
// supplier, invoice number, amount, currency and date. Two bills with the same
// fingerprint are candidate duplicates; the value is deterministic so repeated
// runs never disagree.
func duplicateFingerprint(partnerID int, number string, amount float64, currency, billDate string) string {
	canonical := fmt.Sprintf("%d|%s|%.2f|%s|%s",
		partnerID,
		strings.ToUpper(strings.TrimSpace(number)),
		math.Round(amount*100)/100,
		strings.ToUpper(strings.TrimSpace(currency)),
		strings.TrimSpace(billDate),
	)
	h := fnv.New64a()
	_, _ = h.Write([]byte(canonical))
	return fmt.Sprintf("%016x", h.Sum64())
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
func round3(v float64) float64 { return math.Round(v*1000) / 1000 }

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

func createRec(ctx context.Context, model string, vals map[string]interface{}) (int, error) {
	inst, ok := orm.Registry[model]
	if !ok {
		return 0, fmt.Errorf("model %s unavailable", model)
	}
	return orm.Create(ctx, inst, vals)
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

func nextRef(ctx context.Context, prefix string) string {
	if name, err := orm.NextSequence(ctx, prefix); err == nil && name != "" {
		return name
	}
	return fmt.Sprintf("%s/%05d", prefix, time.Now().Unix()%100000)
}

func nowStr() string { return time.Now().Format(dtLayout) }

func timeNowDay() string { return time.Now().Format(dateLayout) }

// appendHistory keeps an append-only running explanation of what governed each
// match decision, so later tolerance changes never rewrite past reasoning.
func appendHistory(ctx context.Context, id int, note string) {
	cur := loadRec(ctx, caseModel, id)
	if cur == nil {
		return
	}
	hist := strings.TrimSpace(str(cur["history"]))
	stamp := time.Now().Format(dtLayout)
	line := fmt.Sprintf("[%s] %s", stamp, strings.TrimSpace(note))
	if hist == "" {
		hist = line
	} else {
		hist = hist + "\n" + line
	}
	_ = orm.UpdateRecordByID(ctx, caseModel, id, map[string]interface{}{"history": hist})
}

// publishMatchEvent writes a publish-only outbox row towards AP. It never posts
// a journal or authorizes payment; consumers mark it consumed downstream.
func publishMatchEvent(ctx context.Context, evType MatchEvent, caseID int, amount float64, fingerprint, message string) error {
	_, err := createRec(ctx, eventModel, map[string]interface{}{
		"name":        nextRef(ctx, "MTEVT"),
		"event_type":  string(evType),
		"match_id":    caseID,
		"amount":      round2(amount),
		"fingerprint": fingerprint,
		"message":     message,
		"consumed":    false,
		"date":        time.Now().Format(dtLayout),
	})
	return err
}

// isDuplicateFingerprint reports whether another active match case already
// carries this fingerprint (excluding the case currently being reconciled).
func isDuplicateFingerprint(ctx context.Context, fingerprint string, selfID int) bool {
	if strings.TrimSpace(fingerprint) == "" {
		return false
	}
	rows, err := orm.SearchPage(ctx, caseModel, [][]interface{}{
		{"fingerprint", "=", fingerprint},
		{"id", "!=", selfID},
	}, 1, 0, "id asc")
	if err != nil {
		return false
	}
	return len(rows) > 0
}
