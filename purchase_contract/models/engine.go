package models

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Contract engines (blueprint §10). Contracted price schedules are validated
// server-side and a value ceiling is tracked through an append-only consumption
// ledger. Exceeding the ceiling is a block/warn signal, never a silent approval,
// and holding a contract does not bypass approval or budget control.

const (
	contractModel    = "purchase.contract"
	versionModel     = "purchase.contract.version"
	priceModel       = "purchase.contract.price.schedule"
	obligationModel  = "purchase.contract.obligation"
	consumptionModel = "purchase.contract.consumption"

	contractCron = "purchase.contract.expiry.renewal"

	dateLayout = "2006-01-02"
	dtLayout   = "2006-01-02 15:04:05"

	warnPctDefault = 90.0
	eps            = 0.0005
)

func init() {
	scheduler.RegisterCronHandler(contractCron, runContractExpiry)
}

// ---------------------------------------------------------------------------
// Pure price + ceiling maths (unit tested)
// ---------------------------------------------------------------------------

// priceTier is one contracted quantity-break rate.
type priceTier struct {
	QtyFrom   float64
	QtyTo     float64
	UnitPrice float64
	IndexPct  float64
}

// resolveTierPrice selects the contracted rate for a quantity. A tier matches
// when the quantity is at or above its lower bound and (if an upper bound is
// set) at or below it. When tiers overlap the highest qualifying band wins, so
// larger orders pick up the better bulk rate. Indexation is applied on top.
func resolveTierPrice(tiers []priceTier, qty float64) (float64, bool) {
	best, price, found := math.Inf(-1), 0.0, false
	for _, t := range tiers {
		if qty < t.QtyFrom {
			continue
		}
		if t.QtyTo > 0 && qty > t.QtyTo {
			continue
		}
		if !found || t.QtyFrom > best {
			best = t.QtyFrom
			price = t.UnitPrice * (1 + t.IndexPct/100.0)
			found = true
		}
	}
	if !found {
		return 0, false
	}
	return round2(price), true
}

// priceWithinTolerance reports whether a proposed unit price is acceptable
// against the contracted rate. Paying at or below contract is always fine;
// exceeding it is allowed only within the percentage tolerance.
func priceWithinTolerance(contractPrice, proposedPrice, tolPct float64) bool {
	if contractPrice <= eps {
		return proposedPrice <= eps
	}
	limit := contractPrice * tolPct / 100.0
	return proposedPrice-contractPrice <= limit+eps
}

// contractUsage is the gross drawdown against the ceiling: open commitments plus
// invoiced value less credits. Commitments are released when they convert to an
// invoice, so the two never double count.
func contractUsage(committed, invoiced, credit float64) float64 {
	return round2(committed + invoiced - credit)
}

// ceilingStatus classifies usage against a value ceiling.
func ceilingStatus(used, ceiling, warnPct float64) string {
	if ceiling <= 0 {
		return "unlimited"
	}
	if warnPct <= 0 {
		warnPct = warnPctDefault
	}
	switch {
	case used > ceiling+eps:
		return "exceeded"
	case used >= ceiling*warnPct/100.0:
		return "warning"
	default:
		return "within"
	}
}

// availableCeiling is the remaining headroom under a ceiling (0 when unlimited
// or already exceeded).
func availableCeiling(used, ceiling float64) float64 {
	if ceiling <= 0 {
		return 0
	}
	r := ceiling - used
	if r < 0 {
		return 0
	}
	return round2(r)
}

// daysUntil counts whole days from a reference day to a target date (negative
// once the target has passed).
func daysUntil(from time.Time, target string) (int, bool) {
	t, ok := parseDay(target)
	if !ok {
		return 0, false
	}
	d := int(math.Round(t.Sub(from).Hours() / 24))
	return d, true
}

// withinRenewalWindow reports whether a contract sits inside its notice window
// before expiry (and has not yet expired).
func withinRenewalWindow(daysLeft, noticeDays int) bool {
	return noticeDays > 0 && daysLeft >= 0 && daysLeft <= noticeDays
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

func todayStr() string { return time.Now().Format(dateLayout) }

func parseDay(v interface{}) (time.Time, bool) {
	s := strings.TrimSpace(str(v))
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{dateLayout, dtLayout, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.Truncate(24 * time.Hour), true
		}
	}
	return time.Time{}, false
}

func appendHistory(ctx context.Context, id int, note string) {
	cur := loadRec(ctx, contractModel, id)
	if cur == nil {
		return
	}
	hist := strings.TrimSpace(str(cur["history"]))
	line := fmt.Sprintf("[%s] %s", time.Now().Format(dtLayout), strings.TrimSpace(note))
	if hist == "" {
		hist = line
	} else {
		hist = hist + "\n" + line
	}
	_ = orm.UpdateRecordByID(ctx, contractModel, id, map[string]interface{}{"history": hist})
}

// loadContractTiers reads a contract's active price schedules as tiers.
func loadContractTiers(ctx context.Context, contractID int) []priceTier {
	if contractID <= 0 {
		return nil
	}
	rows, _ := orm.SearchPage(ctx, priceModel, [][]interface{}{{"contract_id", "=", contractID}}, 500, 0, "quantity_from asc")
	out := make([]priceTier, 0, len(rows))
	for _, r := range rows {
		out = append(out, priceTier{
			QtyFrom:   flt(r["quantity_from"]),
			QtyTo:     flt(r["quantity_to"]),
			UnitPrice: flt(r["unit_price"]),
			IndexPct:  flt(r["indexation_pct"]),
		})
	}
	return out
}

// recomputeTotals refreshes the contract's committed/invoiced/credited/used
// aggregates from the consumption ledger (idempotent, derived never stored
// independently).
func recomputeTotals(ctx context.Context, contractID int) (committed, invoiced, credit, used float64) {
	rows, _ := orm.SearchPage(ctx, consumptionModel, [][]interface{}{
		{"contract_id", "=", contractID}, {"active", "=", true},
	}, 5000, 0, "id asc")
	for _, r := range rows {
		amt := flt(r["amount"])
		switch ConsumptionKind(str(r["kind"])) {
		case ConsCommit, ConsEstimate:
			committed += amt
		case ConsInvoice:
			invoiced += amt
		case ConsCredit:
			credit += amt
		}
	}
	used = contractUsage(committed, invoiced, credit)
	_ = orm.UpdateRecordByID(ctx, contractModel, contractID, map[string]interface{}{
		"value_committed": round2(committed),
		"value_invoiced":  round2(invoiced),
		"value_credit":    round2(credit),
		"value_used":      used,
	})
	return round2(committed), round2(invoiced), round2(credit), used
}

// ---------------------------------------------------------------------------
// Expiry / renewal alert cron
// ---------------------------------------------------------------------------

// runContractExpiry moves active contracts into an "expiring" notice window and
// expires those past their end date. It is idempotent: a contract already marked
// is only touched when its state would genuinely change, so repeated runs never
// duplicate history notes or renewals.
func runContractExpiry(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	today := time.Now().Truncate(24 * time.Hour)
	rows, err := orm.SearchPage(bypass, contractModel, [][]interface{}{
		{"state", "in", []interface{}{string(CStateActive), string(CStateExpiring), string(CStateRenewed)}},
		{"date_end", "!=", ""},
	}, 5000, 0, "date_end asc")
	if err != nil {
		return err
	}
	for _, r := range rows {
		days, ok := daysUntil(today, str(r["date_end"]))
		if !ok {
			continue
		}
		notice := int(flt(r["renewal_notice"]))
		id := intID(r["id"])
		state := ContractState(str(r["state"]))
		if days < 0 {
			if state != CStateExpired {
				_ = orm.UpdateRecordByID(bypass, contractModel, id, map[string]interface{}{"state": string(CStateExpired)})
				note := fmt.Sprintf("Contract expired (%d day(s) past end date).", -days)
				if boolOf(r["auto_renew"]) {
					note += " Auto-renew flagged: renewal decision required."
				}
				appendHistory(bypass, id, note)
			}
			continue
		}
		if withinRenewalWindow(days, notice) && state == CStateActive {
			_ = orm.UpdateRecordByID(bypass, contractModel, id, map[string]interface{}{"state": string(CStateExpiring)})
			appendHistory(bypass, id, fmt.Sprintf("Renewal notice: %d day(s) to expiry.", days))
		}
	}
	return nil
}
