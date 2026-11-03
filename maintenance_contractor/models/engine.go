package models

import (
	"context"
	"strconv"
	"time"

	"nexa/core/orm"
	"nexa/core/scheduler"
)

// Warranty & service-contract expiry scanner (blueprint §12). The
// "maintenance.warranty.scan.expiry" cron was declared in
// data/warranty_cron_data.xml but had no handler. It transitions service
// contracts past their end date to "expired", backfills a renewal reminder for
// contracts approaching expiry, and lapses warranties whose coverage has ended.

const (
	wDateLayout = "2006-01-02"

	cronWarrantyScan = "maintenance.warranty.scan.expiry"

	mServiceContract = "maintenance.service.contract"
	mWarranty        = "maintenance.warranty"

	renewalLeadDays = 30

	stateActive  = "active"
	stateExpired = "expired"
)

func init() {
	scheduler.RegisterCronHandler(cronWarrantyScan, runWarrantyScan)
}

func runWarrantyScan(ctx context.Context, _ map[string]interface{}) error {
	bypass := orm.ContextWithBypass(ctx, true)
	today := time.Now().UTC().Truncate(24 * time.Hour)
	scanContracts(bypass, today)
	scanWarranties(bypass, today)
	return nil
}

// scanContracts marks lapsed contracts expired and surfaces renewal reminders.
func scanContracts(ctx context.Context, today time.Time) {
	contracts := fetchWAll(ctx, mServiceContract, [][]interface{}{
		{"state", "=", stateActive},
	}, "id asc")
	for _, c := range contracts {
		id := atoiW(strW(c["id"]))
		end := parseWDate(strW(c["end_date"]))
		if end.IsZero() {
			continue
		}
		if !end.After(today) {
			_ = orm.UpdateRecordByID(ctx, mServiceContract, id, map[string]interface{}{
				"state": stateExpired,
			})
			continue
		}
		// Within the lead window and no reminder yet -> set one.
		daysToExpiry := int(end.Sub(today).Hours() / 24)
		if daysToExpiry <= renewalLeadDays && strW(c["renewal_reminder_date"]) == "" {
			reminder := end.AddDate(0, 0, -renewalLeadDays).Format(wDateLayout)
			_ = orm.UpdateRecordByID(ctx, mServiceContract, id, map[string]interface{}{
				"renewal_reminder_date": reminder,
			})
		}
	}
}

// scanWarranties lapses warranties whose coverage end date has passed.
func scanWarranties(ctx context.Context, today time.Time) {
	warranties := fetchWAll(ctx, mWarranty, [][]interface{}{
		{"active", "=", true},
	}, "id asc")
	for _, w := range warranties {
		end := parseWDate(strW(w["end_date"]))
		if end.IsZero() || end.After(today) {
			continue
		}
		_ = orm.UpdateRecordByID(ctx, mWarranty, atoiW(strW(w["id"])), map[string]interface{}{
			"active": false,
		})
	}
}

// ---------------------------------------------------------------------------
// local helpers
// ---------------------------------------------------------------------------

func fetchWAll(ctx context.Context, model string, domain [][]interface{}, order string) []map[string]interface{} {
	var out []map[string]interface{}
	offset := 0
	const page = 500
	for {
		rows, err := orm.SearchPage(ctx, model, domain, page, offset, order)
		if err != nil || len(rows) == 0 {
			break
		}
		out = append(out, rows...)
		if len(rows) < page {
			break
		}
		offset += page
	}
	return out
}

func parseWDate(s string) time.Time {
	if len(s) >= 10 {
		s = s[:10]
	}
	t, err := time.Parse(wDateLayout, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func strW(v interface{}) string { return trimW(orm.AsString(v)) }

func trimW(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 {
		c := s[len(s)-1]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' {
			s = s[:len(s)-1]
			continue
		}
		break
	}
	return s
}

func atoiW(s string) int {
	n, _ := strconv.Atoi(trimW(s))
	return n
}
