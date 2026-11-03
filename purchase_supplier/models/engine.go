package models

import (
	"context"
	"strings"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Governance engines (blueprint §3, §15). Document-expiry monitoring is the
// authoritative periodic revalidation signal; qualification records gate
// sourcing eligibility; risk assessments roll up into an explainable level.
// None of these mutate Inventory or Accounting data.

const (
	docExpiryCron = "purchase.supplier.doc.expiry"
	expiryWarnDay = 30

	suppModel  = "purchase.supplier"
	siteModel  = "purchase.supplier.site"
	contactMo  = "purchase.supplier.contact"
	itemModel  = "purchase.supplier.item"
	docModel   = "purchase.supplier.document"
	bankModel  = "purchase.supplier.bank.account"
	qualModel  = "purchase.supplier.qualification"
	chgModel   = "purchase.supplier.change.request"
	riskModel  = "purchase.supplier.risk.assessment"
	dateLayout = "2006-01-02"
	dtLayout   = "2006-01-02 15:04:05"
)

func init() {
	scheduler.RegisterCronHandler(docExpiryCron, runSupplierDocExpiry)
}

// ---------------------------------------------------------------------------
// Pure maths (unit tested)
// ---------------------------------------------------------------------------

// normalizeLegalName lowercases and strips non-alphanumerics so duplicate
// detection compares "Acme, Ltd." and "acme ltd" identically.
func normalizeLegalName(s string) string {
	var b strings.Builder
	prevSpace := true
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevSpace = false
		case !prevSpace:
			b.WriteRune(' ')
			prevSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

// bankFingerprint keeps only digits and the last six, so partially masked
// accounts still fingerprint consistently for duplicate detection.
func bankFingerprint(ref string) string {
	digits := make([]rune, 0, len(ref))
	for _, r := range ref {
		if r >= '0' && r <= '9' {
			digits = append(digits, r)
		}
	}
	if len(digits) <= 6 {
		return string(digits)
	}
	return string(digits[len(digits)-6:])
}

// supplierIdentity is a candidate supplier's normalized identity used for
// duplicate detection.
type supplierIdentity struct {
	name string // already normalized legal name
	reg  string // registration number
	tax  string // tax id
}

// isDuplicateSupplier decides whether a candidate matches the incoming record
// on the configured identity rules (normalized legal name, registration
// number, tax id). Any exact strong identifier match is a duplicate.
func isDuplicateSupplier(normName, regNo, taxID string, cand supplierIdentity) bool {
	if normName != "" && cand.name != "" && strings.EqualFold(normName, cand.name) {
		return true
	}
	if strings.TrimSpace(regNo) != "" && strings.EqualFold(strings.TrimSpace(regNo), strings.TrimSpace(cand.reg)) {
		return true
	}
	if strings.TrimSpace(taxID) != "" && strings.EqualFold(strings.TrimSpace(taxID), strings.TrimSpace(cand.tax)) {
		return true
	}
	return false
}

// riskOverall averages the ten dimension scores (higher = riskier).
func riskOverall(dims []int) float64 {
	if len(dims) == 0 {
		return 0
	}
	sum := 0
	for _, d := range dims {
		if d < 0 {
			d = 0
		}
		if d > 100 {
			d = 100
		}
		sum += d
	}
	return float64(sum) / float64(len(dims))
}

// riskLevel maps an overall score to a level with explainable boundaries.
// A score of zero (no assessment) is unclassified, not "safe".
func riskLevel(overall float64) SupplierRisk {
	switch {
	case overall <= 0:
		return RiskUnclassified
	case overall < 20:
		return RiskLow
	case overall < 45:
		return RiskMedium
	case overall < 70:
		return RiskHigh
	default:
		return RiskCritical
	}
}

// docExpiryState classifies a document by days remaining until expiry.
func docExpiryState(daysLeft int, warnDays int) DocState {
	if daysLeft < 0 {
		return DocExpired
	}
	if daysLeft <= warnDays {
		return DocExpiring
	}
	return DocAccepted
}

// ---------------------------------------------------------------------------
// Document expiry monitor
// ---------------------------------------------------------------------------

func runSupplierDocExpiry(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	now := time.Now()

	docs, err := orm.SearchPage(bypass, docModel, [][]interface{}{
		{"expiry_date", "!=", ""},
		{"review_state", "in", []interface{}{"accepted", "pending", "expiring", "expired"}},
		{"active", "=", true},
	}, 2000, 0, "id asc")
	if err != nil {
		return err
	}

	touched := map[int]bool{}
	for _, d := range docs {
		exp, ok := parseDate(d["expiry_date"])
		if !ok {
			continue
		}
		daysLeft := int(exp.Sub(now.Truncate(24*time.Hour)).Hours() / 24)
		want := docExpiryState(daysLeft, expiryWarnDay)
		state := str(d["review_state"])
		if state == "rejected" {
			continue
		}
		if want != DocAccepted && state != string(want) {
			id, _ := orm.CoerceInt64(d["id"])
			_ = orm.UpdateRecordByID(bypass, docModel, int(id), map[string]interface{}{
				"review_state": string(want),
			})
		}
		if sid := intID(d["supplier_id"]); sid > 0 {
			touched[sid] = true
		}
	}

	// Suppliers whose mandatory documents have expired are held for
	// revalidation (state → expired) so downstream controls must re-check.
	for sid := range touched {
		supp := loadRec(bypass, suppModel, sid)
		if supp == nil || str(supp["state"]) != "active" {
			continue
		}
		mine, _ := orm.SearchPage(bypass, docModel, [][]interface{}{
			{"supplier_id", "=", sid},
			{"mandatory", "=", true},
			{"review_state", "=", "expired"},
			{"active", "=", true},
		}, 1, 0, "id asc")
		if len(mine) > 0 {
			_ = orm.UpdateRecordByID(bypass, suppModel, sid, map[string]interface{}{
				"state":            "expired",
				"last_expiry_scan": now.Format(dtLayout),
			})
		} else {
			_ = orm.UpdateRecordByID(bypass, suppModel, sid, map[string]interface{}{
				"last_expiry_scan": now.Format(dtLayout),
			})
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Qualification gate (consumed by Sourcing in T3)
// ---------------------------------------------------------------------------

// IsSupplierQualified reports whether the partner behind a supplier is active
// and holds an un-expired qualification for the given category (blank category
// matches any active qualification).
func IsSupplierQualified(ctx context.Context, partnerID int, category string) bool {
	if partnerID <= 0 {
		return false
	}
	bypass := orm.ContextWithBypass(ctx, true)
	supps, err := orm.SearchPage(bypass, suppModel, [][]interface{}{
		{"partner_id", "=", partnerID},
		{"state", "=", "active"},
	}, 1, 0, "id asc")
	if err != nil || len(supps) == 0 {
		return false
	}
	sid := intID(supps[0]["id"])
	domain := [][]interface{}{
		{"supplier_id", "=", sid},
		{"state", "=", "active"},
		{"active", "=", true},
	}
	if strings.TrimSpace(category) != "" {
		domain = append(domain, []interface{}{"category", "=ilike", category})
	}
	quals, err := orm.SearchPage(bypass, qualModel, domain, 1, 0, "id asc")
	if err != nil || len(quals) == 0 {
		return false
	}
	exp := quals[0]["expiry_date"]
	if s := str(exp); s != "" {
		if d, ok := parseDate(exp); ok && d.Before(time.Now()) {
			return false
		}
	}
	return true
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

func parseDate(v interface{}) (time.Time, bool) {
	s := strings.TrimSpace(str(v))
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{dateLayout, dtLayout, time.RFC3339} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
