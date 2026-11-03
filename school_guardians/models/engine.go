package models

import (
	"context"

	"nexa/core/orm"
)

// Behaviour layer for guardians. The access predicates are pure and unit-tested:
// they decide, from a learner's set of guardian links, who may see finances and who
// is the current primary contact, honouring the active/revoked status and the
// valid_from / valid_to window. The compute projects a live "learners" count onto the
// guardian record so the registry shows each guardian's caseload at a glance.

const (
	guardianModel = "school.guardian"
	linkModel     = "school.student.guardian"
)

// guardianLink is the minimal projection of a school.student.guardian row the pure
// predicates need. Keeping it a plain struct (not a DB record) is what lets the
// logic be unit-tested without touching Postgres.
type guardianLink struct {
	GuardianID       int
	IsPrimary        bool
	CanViewFinancial bool
	Status           string
	ValidFrom        string // YYYY-MM-DD, "" treated as open-ended start
	ValidTo          string // YYYY-MM-DD, "" means still open (current)
}

// withinValidity reports whether `today` falls inside the [from, to] window. A blank
// start is unbounded in the past; a blank end is unbounded in the future. All dates
// are YYYY-MM-DD so a lexical compare is a correct date compare.
func withinValidity(from, to, today string) bool {
	if from != "" && from > today {
		return false
	}
	if to != "" && today > to {
		return false
	}
	return true
}

// linkActive reports whether a link's status grants live access (revoked never does).
func linkActive(status string) bool {
	return status == string(LinkActive)
}

// live returns whether a link is currently in force: active and within its window.
func (l guardianLink) live(today string) bool {
	return linkActive(l.Status) && withinValidity(l.ValidFrom, l.ValidTo, today)
}

// canViewFinancial reports whether `guardianID` may see a learner's financial
// information today: they need at least one live link that opts into financial view.
func canViewFinancial(rows []guardianLink, guardianID int, today string) bool {
	for _, r := range rows {
		if r.GuardianID == guardianID && r.CanViewFinancial && r.live(today) {
			return true
		}
	}
	return false
}

// primaryGuardianID returns the guardian id of the learner's current primary contact.
// Among live links flagged IsPrimary the one with the latest valid_from wins; ties
// break on the higher guardian id so the result is deterministic. Returns 0 when no
// live primary link exists.
func primaryGuardianID(rows []guardianLink, today string) int {
	best := 0
	bestFrom := ""
	for _, r := range rows {
		if !r.IsPrimary || !r.live(today) {
			continue
		}
		if best == 0 || r.ValidFrom > bestFrom || (r.ValidFrom == bestFrom && r.GuardianID > best) {
			best = r.GuardianID
			bestFrom = r.ValidFrom
		}
	}
	return best
}

func init() {
	orm.RegisterCompute(guardianModel, "children_count", []string{"id"}, computeChildren)
}

// computeChildren counts a guardian's live learner links. It reads only the status
// (cheap indexed filter) rather than re-deriving the date window per row; revoked
// links are excluded so the number reflects the guardian's current caseload.
func computeChildren(ctx context.Context, rec map[string]interface{}) (interface{}, error) {
	id := intOr(rec["id"])
	if id <= 0 {
		return 0, nil
	}
	n, err := orm.SearchCount(orm.ContextWithBypass(ctx, true), linkModel, [][]interface{}{
		{"guardian_id", "=", id},
		{"status", "=", string(LinkActive)},
	})
	if err != nil {
		return 0, nil
	}
	return n, nil
}

func intOr(v interface{}) int {
	n, ok := orm.CoerceInt64(v)
	if !ok {
		return 0
	}
	return int(n)
}
