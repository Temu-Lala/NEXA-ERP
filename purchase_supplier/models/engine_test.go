package models

import "testing"

func TestNormalizeLegalName(t *testing.T) {
	cases := map[string]string{
		"Acme, Ltd.":   "acme ltd",
		"  acme ltd ":  "acme ltd",
		"ACME-LTD":     "acme ltd",
		"O'Brien & Co": "o brien co",
		"":             "",
	}
	for in, want := range cases {
		if got := normalizeLegalName(in); got != want {
			t.Errorf("normalizeLegalName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBankFingerprint(t *testing.T) {
	if got := bankFingerprint("GB29 NWBK 6016 1331 9268 19"); got != "926819" {
		t.Errorf("bankFingerprint long = %q, want 926819", got)
	}
	if got := bankFingerprint("12-3456"); got != "123456" {
		t.Errorf("bankFingerprint short = %q, want 123456", got)
	}
	if got := bankFingerprint("****"); got != "" {
		t.Errorf("bankFingerprint no digits = %q, want empty", got)
	}
}

func TestIsDuplicateSupplier(t *testing.T) {
	cand := supplierIdentity{name: "acme ltd", reg: "REG-1", tax: "TIN-9"}
	if !isDuplicateSupplier("acme ltd", "", "", cand) {
		t.Error("normalized name match should be duplicate")
	}
	if !isDuplicateSupplier("", "reg-1", "", cand) {
		t.Error("case-insensitive reg match should be duplicate")
	}
	if !isDuplicateSupplier("", "", "TIN-9", cand) {
		t.Error("tax id match should be duplicate")
	}
	if isDuplicateSupplier("globex", "reg-2", "tin-1", cand) {
		t.Error("no shared identifier should not be duplicate")
	}
	// Empty incoming identifiers must not match on their own.
	if isDuplicateSupplier("", "", "", supplierIdentity{name: "acme ltd"}) {
		t.Error("blank identifiers should not be duplicate")
	}
}

func TestRiskOverallAndLevel(t *testing.T) {
	if got := riskOverall(nil); got != 0 {
		t.Errorf("riskOverall(nil) = %v, want 0", got)
	}
	if got := riskOverall([]int{10, 20, 30}); got != 20 {
		t.Errorf("riskOverall = %v, want 20", got)
	}
	if got := riskOverall([]int{-5, 200}); got != 50 {
		t.Errorf("clamped mean = %v, want 50", got)
	}
	if riskLevel(0) != RiskUnclassified {
		t.Error("zero should be unclassified")
	}
	if riskLevel(10) != RiskLow || riskLevel(30) != RiskMedium ||
		riskLevel(60) != RiskHigh || riskLevel(80) != RiskCritical {
		t.Error("risk level boundaries incorrect")
	}
}

func TestDocExpiryState(t *testing.T) {
	if docExpiryState(-1, 30) != DocExpired {
		t.Error("negative days should be expired")
	}
	if docExpiryState(15, 30) != DocExpiring {
		t.Error("within warning window should be expiring")
	}
	if docExpiryState(90, 30) != DocAccepted {
		t.Error("far from expiry should stay accepted")
	}
}
