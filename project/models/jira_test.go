package models

import (
	"context"
	"strings"
	"testing"
)

func TestSanitizeKeyPrefix(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"Project Alpha", "PROJECTA"},
		{"ERP-Core", "ERPCORE"},
		{"IT", "IT"},
		{"a very long project name that exceeds limits", "AVERYLON"},
		{"$$$Special---Chars!!!", "SPECIALC"},
	}

	for _, c := range cases {
		got := sanitizeKeyPrefix(c.input)
		if got != c.expected {
			t.Errorf("sanitizeKeyPrefix(%q) = %q; want %q", c.input, got, c.expected)
		}
	}
}

func TestTrailingNumber(t *testing.T) {
	cases := []struct {
		input    string
		expected int
	}{
		{"PRJ-1", 1},
		{"PRJ-42", 42},
		{"ERP_105", 105},
		{"TSK-999", 999},
		{"NoNumberHere", 0},
		{"", 0},
	}

	for _, c := range cases {
		got := trailingNumber(c.input)
		if got != c.expected {
			t.Errorf("trailingNumber(%q) = %d; want %d", c.input, got, c.expected)
		}
	}
}

func TestSpaceBoardRedirectLogic(t *testing.T) {
	ctx := context.Background()

	// When no active sprint exists, projSpaceBoard directs to Kanban view
	// excluding cancelled tasks.
	url, err := projSpaceBoard(ctx, "", 99999, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(url, "model=project.task") {
		t.Errorf("expected URL to target project.task, got %s", url)
	}
	if !strings.Contains(url, "view_type=kanban") {
		t.Errorf("expected URL to use kanban view_type, got %s", url)
	}
	if !strings.Contains(url, "project_id") {
		t.Errorf("expected URL domain to filter by project_id, got %s", url)
	}
}
