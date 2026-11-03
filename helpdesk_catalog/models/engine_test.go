package models

import "testing"

// Unit tests for the service-request lifecycle and fulfilment decision logic.
// These cover the governed-process rules that the actions rely on, with no
// database dependency, mirroring the helpdesk_channel / helpdesk_sla tests.

func TestRequestAllowedTransition(t *testing.T) {
	cases := []struct {
		state   RequestState
		action  string
		apprReq bool
		want    RequestState
		ok      bool
	}{
		{ReqDraft, "submit", false, ReqFulfilling, true},
		{ReqDraft, "submit", true, ReqInApproval, true},
		{ReqSubmitted, "submit", false, "", false},
		{ReqInApproval, "approve", true, ReqFulfilling, true},
		{ReqInApproval, "reject", true, ReqRejected, true},
		{ReqFulfilling, "approve", false, "", false},
		{ReqFulfilling, "complete", false, ReqCompleted, true},
		{ReqCompleted, "complete", false, "", false},
		{ReqDraft, "cancel", false, ReqCancelled, true},
		{ReqFulfilling, "cancel", false, ReqCancelled, true},
		{ReqCompleted, "cancel", false, "", false},
		{ReqCancelled, "submit", false, "", false},
	}
	for _, c := range cases {
		got, ok := requestAllowedTransition(c.state, c.action, c.apprReq)
		if ok != c.ok || (c.ok && got != c.want) {
			t.Errorf("transition(%q,%q,appr=%v) = (%q,%v) want (%q,%v)",
				c.state, c.action, c.apprReq, got, ok, c.want, c.ok)
		}
	}
}

func TestTaskIsReady(t *testing.T) {
	if !taskIsReady("pending", "", false) {
		t.Error("pending task with no dependency is ready")
	}
	if !taskIsReady("pending", "done", true) {
		t.Error("pending task with done dependency is ready")
	}
	if taskIsReady("pending", "in_progress", true) {
		t.Error("dependency not finished means not ready")
	}
	if taskIsReady("in_progress", "", false) {
		t.Error("non-pending task is never 'ready' to start")
	}
	if taskIsReady("done", "", false) {
		t.Error("done task is not ready")
	}
}

func TestRequiredTasksSatisfied(t *testing.T) {
	if requiredTasksSatisfied(nil) {
		t.Error("no tasks means not satisfied")
	}
	ok := [][2]string{{"t", "done"}, {"f", "pending"}}
	if !requiredTasksSatisfied(ok) {
		t.Error("all required done (optional pending) should satisfy")
	}
	notYet := [][2]string{{"t", "in_progress"}}
	if requiredTasksSatisfied(notYet) {
		t.Error("required in-progress should not satisfy")
	}
	skipped := [][2]string{{"t", "skipped"}}
	if !requiredTasksSatisfied(skipped) {
		t.Error("required skipped counts as resolved")
	}
}

func TestApprovalsResolved(t *testing.T) {
	if a, r := approvalsResolved(nil); !a || r {
		t.Errorf("no approvals => approved, not rejected; got (%v,%v)", a, r)
	}
	if a, r := approvalsResolved([]string{"approved", "approved"}); !a || r {
		t.Errorf("all approved should resolve approved; got (%v,%v)", a, r)
	}
	if a, _ := approvalsResolved([]string{"approved", "pending"}); a {
		t.Error("a pending approval means not resolved")
	}
	if _, r := approvalsResolved([]string{"approved", "rejected"}); !r {
		t.Error("a rejected step means rejected")
	}
}

func TestValidateFieldValue(t *testing.T) {
	if msg := validateFieldValue("integer", "t", "", ""); msg == "" {
		t.Error("required empty should fail")
	}
	if msg := validateFieldValue("integer", "f", "", ""); msg != "" {
		t.Errorf("optional empty should pass, got %q", msg)
	}
	if msg := validateFieldValue("integer", "f", "12", ""); msg != "" {
		t.Errorf("valid integer should pass, got %q", msg)
	}
	if msg := validateFieldValue("integer", "f", "x", ""); msg == "" {
		t.Error("non-integer should fail")
	}
	if msg := validateFieldValue("float", "f", "3.14", ""); msg != "" {
		t.Errorf("valid float should pass, got %q", msg)
	}
	if msg := validateFieldValue("boolean", "f", "maybe", ""); msg == "" {
		t.Error("invalid boolean should fail")
	}
	if msg := validateFieldValue("select", "f", "red", "red, green , blue"); msg != "" {
		t.Errorf("allowed option should pass, got %q", msg)
	}
	if msg := validateFieldValue("select", "f", "pink", "red, green, blue"); msg == "" {
		t.Error("disallowed option should fail")
	}
	if msg := validateFieldValue("text", "f", "anything", ""); msg != "" {
		t.Errorf("free text should pass, got %q", msg)
	}
}

func TestSplitOptionsAndContainsFold(t *testing.T) {
	opts := splitOptions(" a , ,b, c ")
	if len(opts) != 3 || opts[0] != "a" || opts[2] != "c" {
		t.Errorf("splitOptions wrong: %v", opts)
	}
	if !containsFold(opts, "B") {
		t.Error("containsFold should be case-insensitive")
	}
	if containsFold(opts, "z") {
		t.Error("absent option should not match")
	}
}

func TestAtoiEngine(t *testing.T) {
	if atoi("42") != 42 || atoi("") != 0 || atoi("abc") != 0 {
		t.Error("atoi engine helper mis-parses")
	}
}
