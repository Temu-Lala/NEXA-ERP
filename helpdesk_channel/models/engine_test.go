package models

import "testing"

// Unit tests for the pure routing / delivery decision functions. These cover the
// non-trivial logic that the scheduled engines and actions rely on, without any
// database dependency, mirroring the helpdesk_sla test approach.

func ticketRec(pri, chan_, typ, cat, name string) map[string]string {
	return map[string]string{
		"priority": pri, "channel": chan_, "ticket_type": typ,
		"category_id": cat, "name": name,
	}
}

func TestIsOpenTicket(t *testing.T) {
	for _, s := range []string{"new", "open", "in_progress", "pending"} {
		if !isOpenTicket(s) {
			t.Errorf("state %q should be open", s)
		}
	}
	for _, s := range []string{"resolved", "closed", "cancelled", ""} {
		if isOpenTicket(s) {
			t.Errorf("state %q should not be open", s)
		}
	}
}

func TestIsUnassigned(t *testing.T) {
	if !isUnassigned("", "") {
		t.Error("empty assignee and team should be unassigned")
	}
	if !isUnassigned("  ", "") {
		t.Error("whitespace assignee should count as unassigned")
	}
	if isUnassigned("5", "") {
		t.Error("assignee set means not unassigned")
	}
	if isUnassigned("", "2") {
		t.Error("team set means not unassigned")
	}
}

func TestRuleMatchesWildcards(t *testing.T) {
	tk := ticketRec("high", "email", "incident", "7", "Payment gateway down")
	rule := map[string]string{"active": "t", "match_priority": "any", "match_channel": "any",
		"match_ticket_type": "any", "category_id": "", "keyword": ""}
	if !ruleMatches(tk, rule) {
		t.Error("all-wildcard rule must match any ticket")
	}
}

func TestRuleMatchesConstraints(t *testing.T) {
	tk := ticketRec("high", "email", "incident", "7", "Payment gateway down")
	if ruleMatches(tk, map[string]string{"match_priority": "urgent"}) {
		t.Error("priority mismatch should not match")
	}
	if ruleMatches(tk, map[string]string{"match_channel": "chat"}) {
		t.Error("channel mismatch should not match")
	}
	if ruleMatches(tk, map[string]string{"category_id": "8"}) {
		t.Error("category mismatch should not match")
	}
	if !ruleMatches(tk, map[string]string{"category_id": "7"}) {
		t.Error("category match should hold")
	}
	if !ruleMatches(tk, map[string]string{"keyword": "GATEWAY"}) {
		t.Error("keyword match should be case-insensitive substring")
	}
	if ruleMatches(tk, map[string]string{"keyword": "printer"}) {
		t.Error("absent keyword should not match")
	}
}

func TestFirstMatchingRuleOrderAndActivation(t *testing.T) {
	tk := ticketRec("high", "email", "incident", "7", "x")
	rules := []map[string]string{
		{"name": "r1", "active": "f", "match_priority": "high"},
		{"name": "r2", "active": "t", "match_priority": "any"},
	}
	got := firstMatchingRule(tk, rules)
	if got == nil || got["name"] != "r2" {
		t.Fatalf("inactive rule should be skipped, got %+v", got)
	}
	if firstMatchingRule(tk, []map[string]string{{"name": "off", "active": "f"}}) != nil {
		t.Error("no active matching rule should return nil")
	}
}

func TestPickLeastLoaded(t *testing.T) {
	cands := []map[string]string{
		{"user_id": "1", "max_open": "10", "current_open": "9", "available": "t"},
		{"user_id": "2", "max_open": "10", "current_open": "1", "available": "t"},
		{"user_id": "3", "max_open": "50", "current_open": "0", "available": "f"},
	}
	best := pickLeastLoaded(cands)
	if best == nil || best["user_id"] != "2" {
		t.Fatalf("expected least-loaded available agent #2, got %+v", best)
	}
	if pickLeastLoaded([]map[string]string{{"available": "f"}}) != nil {
		t.Error("all-unavailable should yield nil")
	}
}

func TestDecideRouting(t *testing.T) {
	tk := ticketRec("high", "email", "incident", "7", "x")
	loads := []map[string]string{
		{"user_id": "42", "max_open": "10", "current_open": "2", "available": "t"},
	}
	// Team routing.
	d := decideRouting(tk, map[string]string{"team_id": "5", "method": "team"}, nil)
	if d.TeamID != "5" || !d.ShouldMove {
		t.Errorf("team routing wrong: %+v", d)
	}
	// Least-loaded routing fills user.
	d = decideRouting(tk, map[string]string{"method": "least_loaded", "team_id": "5"}, loads)
	if d.UserID != "42" {
		t.Errorf("least_loaded should pick agent 42, got %+v", d)
	}
	// Priority raise.
	d = decideRouting(tk, map[string]string{"team_id": "5", "raise_priority": "t", "new_priority": "urgent"}, nil)
	if d.Priority != "urgent" {
		t.Errorf("raise_priority should set urgent, got %+v", d)
	}
	// Nil rule is a no-op.
	if decideRouting(tk, nil, nil).ShouldMove {
		t.Error("nil rule must not move")
	}
}

func TestCanRetryDelivery(t *testing.T) {
	if !canRetryDelivery("failed", 1, 3) {
		t.Error("failed under budget should retry")
	}
	if canRetryDelivery("failed", 3, 3) {
		t.Error("failed at budget must not retry")
	}
	if canRetryDelivery("sent", 0, 3) {
		t.Error("sent message must not retry")
	}
	if !canRetryDelivery("pending", 0, 0) {
		t.Error("max<=0 should fall back to default 3")
	}
}

func TestLooksDuplicate(t *testing.T) {
	seen := [][]string{{"msg-1", "thread-a"}, {"msg-2", ""}}
	if !looksDuplicate("msg-2", "", seen) {
		t.Error("external id match should be duplicate")
	}
	if !looksDuplicate("", "thread-a", seen) {
		t.Error("correlation key match should be duplicate")
	}
	if looksDuplicate("msg-9", "thread-z", seen) {
		t.Error("unrelated message is not a duplicate")
	}
	if looksDuplicate("", "", seen) {
		t.Error("empty ids must not be flagged duplicate")
	}
}

func TestAtoi(t *testing.T) {
	cases := map[string]int{"": 0, "12": 12, "3abc": 3, "  42 ": 42, "x": 0}
	for in, want := range cases {
		if got := atoi(in); got != want {
			t.Errorf("atoi(%q)=%d want %d", in, got, want)
		}
	}
}
