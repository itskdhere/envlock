package ui

import (
	"strings"
	"testing"
)

func TestUIFormatting(t *testing.T) {
	succ := Success("Operation completed %s", "successfully")
	if !strings.Contains(succ, "Operation completed successfully") {
		t.Fatalf("unexpected success output: %s", succ)
	}

	warn := Warning("Warning: %s", "disk full")
	if !strings.Contains(warn, "disk full") {
		t.Fatalf("unexpected warning output: %s", warn)
	}

	errStr := Error("Error: %s", "connection failed")
	if !strings.Contains(errStr, "connection failed") {
		t.Fatalf("unexpected error output: %s", errStr)
	}

	info := Info("Info: %s", "version 1.0")
	if !strings.Contains(info, "version 1.0") {
		t.Fatalf("unexpected info output: %s", info)
	}
}

func TestBanner(t *testing.T) {
	lines := []string{"Line 1", "Line 2"}
	banner := Banner("My Title", lines)
	if !strings.Contains(banner, "My Title") || !strings.Contains(banner, "Line 1") || !strings.Contains(banner, "Line 2") {
		t.Fatalf("unexpected banner output: %s", banner)
	}
}

func TestErrorCard(t *testing.T) {
	card := ErrorCard("Auth Failed", "Invalid credentials provided", "Try running: el keygen")
	if !strings.Contains(card, "Auth Failed") || !strings.Contains(card, "Invalid credentials") || !strings.Contains(card, "Try running: el keygen") {
		t.Fatalf("unexpected error card: %s", card)
	}
}

func TestColorDisabled(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if !IsColorDisabled() {
		t.Fatal("expected IsColorDisabled() to return true when NO_COLOR=1")
	}

	succ := Success("Done")
	if !strings.HasPrefix(succ, "[OK]") {
		t.Fatalf("expected [OK] prefix when NO_COLOR=1, got: %s", succ)
	}
}
