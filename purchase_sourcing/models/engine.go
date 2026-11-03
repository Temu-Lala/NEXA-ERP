package models

import (
	"context"
	"fmt"
	"strings"
	"time"

	"nexa/core/orm"
	suppliermodels "nexa_addons/purchase_supplier/models"
)

// Competitive sourcing engines (blueprint §7, §8). All award economics are
// computed server-side on a precise decimal (scale=2): landed-cost
// normalization makes unlike bids comparable, weighted evaluation turns a
// criteria matrix into an explainable ranking, and currency normalization
// records the rate used. Award eligibility is delegated to the Supplier Master
// governance gate (T1). Nothing here writes to Inventory or Accounting; the
// award creates a draft purchase.order the buyer must still approve.

const (
	rfqModel      = "purchase.rfq"
	rfqLineModel  = "purchase.rfq.line"
	rfqSuppModel  = "purchase.rfq.supplier"
	quoteModel    = "purchase.quotation"
	quoteLineMdl  = "purchase.quotation.line"
	evalModel     = "purchase.evaluation"
	awardModel    = "purchase.award"
	roundModel    = "purchase.quotation.round"
	poModel       = "purchase.order"
	poLineModel   = "purchase.order.line"
	partnerComdl  = "core.partner"
	ratePrecision = 6

	dateLayout = "2006-01-02"
	dtLayout   = "2006-01-02 15:04:05"
)

// ---------------------------------------------------------------------------
// Pure economics (unit tested)
// ---------------------------------------------------------------------------

// normBid is one supplier bid reduced to comparable base-currency components.
// Amounts carry no supplier identity so ranking stays purely arithmetic.
type normBid struct {
	key      string
	goods    float64 // normalized merchandise subtotal in base currency
	landed   float64 // normalized landed cost (goods + freight + tax)
	delivery int     // lead time in days (lower is better)
	quality  float64
	terms    float64
	risk     float64 // risk exposure (higher is worse)
}

// landedCost sums merchandise, freight and tax. All three must already be in a
// common currency (see normalizeAmount) so bids are directly comparable —
// freight and non-recoverable tax belong to the decision, not just the sticker
// price (blueprint §7.3).
func landedCost(goods, freight, tax float64) float64 {
	return goods + freight + tax
}

// normalizeAmount converts an amount expressed in a source currency to the base
// currency using the recorded rate (source -> base). A rate <= 0 is treated as
// 1.0 so same-currency bids pass through unchanged.
func normalizeAmount(amount, rate float64) float64 {
	if rate <= 0 {
		rate = 1
	}
	return amount * rate
}

// bestValueIndex is a 0..100 index where higher is better. Goods and lead time
// are inverse indicators (normalized against the best bid), while quality and
// terms are already positive scales. It is a reviewable signal, not a verdict.
func bestValueIndex(goods, minGoods, leadDays, minLead, quality, terms float64) float64 {
	cost, speed := 1.0, 1.0
	if minGoods > 0 && goods > 0 {
		cost = minGoods / goods
	}
	if leadDays > 0 && minLead > 0 {
		speed = minLead / leadDays
	}
	if cost > 1 {
		cost = 1
	}
	if speed > 1 {
		speed = 1
	}
	idx := (0.5*cost + 0.2*speed + 0.2*quality/100 + 0.1*terms/100) * 100
	if idx < 0 {
		idx = 0
	}
	if idx > 100 {
		idx = 100
	}
	return round2(idx)
}

// scoreCriteria is a criteria weighting where every weight is non-negative and
// at least one is set, so scores are meaningful.
type scoreCriteria struct {
	wPrice, wLanded, wDelivery, wQuality, wTerms, wRisk float64
}

func (c scoreCriteria) total() float64 {
	return c.wPrice + c.wLanded + c.wDelivery + c.wQuality + c.wTerms + c.wRisk
}

// weightedScore ranks bids into an explainable 0..100 composite. Price and
// landed cost are scored relative to the lowest bid; lead time relative to the
// fastest; quality and terms are taken as given 0..100 scales; risk is scored
// as (100 - risk). Weights are normalized by their sum. Ties on the dominant
// commercial dimension are deliberately left to the secondary dimensions.
func weightedScore(bids []normBid, c scoreCriteria) []normBid {
	if len(bids) == 0 {
		return nil
	}
	minGoods, minLanded, minLead := mathHuge, mathHuge, mathHuge
	for _, b := range bids {
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
	out := make([]normBid, len(bids))
	copy(out, bids)
	ranks := make([]rankedBid, len(out))
	for i, b := range out {
		s := c.wPrice * safeRatio(minGoods, b.goods) * 100
		s += c.wLanded * safeRatio(minLanded, b.landed) * 100
		s += c.wDelivery * safeRatio(minLead, float64(b.delivery)) * 100
		s += c.wQuality * clamp100(b.quality)
		s += c.wTerms * clamp100(b.terms)
		s += c.wRisk * (100 - clamp100(b.risk))
		ranks[i] = rankedBid{bid: b, score: round2(s / total)}
	}
	// Sort descending by score, then by landed cost, then by key for stability.
	sortRank(ranks)
	for i := range ranks {
		out[i] = ranks[i].bid
	}
	return out
}

func safeRatio(best, val float64) float64 {
	if val <= 0 {
		return 1 // absent component is not penalized
	}
	if best <= 0 {
		return 0
	}
	r := best / val
	if r > 1 {
		r = 1
	}
	return r
}

func clamp100(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

const mathHuge = 1e18

// sortRank orders ranked bids by score desc, then landed asc, then key asc so
// the output is fully deterministic for a given input set.
func sortRank(rs []rankedBid) {
	for i := 1; i < len(rs); i++ {
		for j := i; j > 0 && lessBid(rs[j], rs[j-1]); j-- {
			rs[j], rs[j-1] = rs[j-1], rs[j]
		}
	}
}

type rankedBid struct {
	bid   normBid
	score float64
}

func lessBid(a, b rankedBid) bool {
	if a.score != b.score {
		return a.score > b.score
	}
	if a.bid.landed != b.bid.landed {
		return a.bid.landed < b.bid.landed
	}
	return a.bid.key < b.bid.key
}

// deadlinePassed reports whether a bid deadline (date or datetime) is strictly
// in the past relative to now. Bids are locked after the deadline.
func deadlinePassed(now time.Time, deadline string) bool {
	s := strings.TrimSpace(deadline)
	if s == "" {
		return false
	}
	for _, layout := range []string{dateLayout, dtLayout} {
		if t, err := time.Parse(layout, s); err == nil {
			return now.After(t)
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Eligibility gate (delegates to Supplier Master governance in T1)
// ---------------------------------------------------------------------------

// canAward reports whether a partner may be awarded: the supplier must be active
// and qualified for the RFQ category (the T1 gate also excludes blocked/suspended
// suppliers since it requires state == "active").
func canAward(ctx context.Context, partnerID int, category string) bool {
	if partnerID <= 0 {
		return false
	}
	return suppliermodels.IsSupplierQualified(ctx, partnerID, category)
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

func round2(v float64) float64 {
	return float64(int64(v*100+0.5)) / 100
}

func nextRef(ctx context.Context, prefix string) string {
	if name, err := orm.NextSequence(ctx, prefix); err == nil && name != "" {
		return name
	}
	return fmt.Sprintf("%s/%05d", prefix, time.Now().Unix()%100000)
}

// lineTotal returns qty * unit - discount for a quotation line, so merchandise
// subtotals are recomputed server-side rather than trusted from the client.
func lineTotal(qty, unit, discount float64) float64 {
	return round2(qty*unit - discount)
}
