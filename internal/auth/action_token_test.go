package auth

import (
	"database/sql"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func setupTokenTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestActionTokenCreateAndValidate(t *testing.T) {
	db := setupTokenTestDB(t)
	svc, err := NewActionTokenService(db)
	if err != nil {
		t.Fatal(err)
	}

	tok, err := svc.Create("session-abc", "format_drive", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if tok.Token == "" {
		t.Fatal("expected non-empty token")
	}
	if tok.Action != "format_drive" {
		t.Errorf("action = %q", tok.Action)
	}

	// Validate successfully
	err = svc.Validate(tok.Token, "session-abc", "format_drive")
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
}

func TestActionTokenSingleUse(t *testing.T) {
	db := setupTokenTestDB(t)
	svc, _ := NewActionTokenService(db)

	tok, _ := svc.Create("session-abc", "destroy", 5*time.Minute)

	// First use succeeds
	if err := svc.Validate(tok.Token, "session-abc", "destroy"); err != nil {
		t.Fatal(err)
	}

	// Second use fails
	err := svc.Validate(tok.Token, "session-abc", "destroy")
	if err == nil || !strings.Contains(err.Error(), "already consumed") {
		t.Errorf("expected consumed error, got: %v", err)
	}
}

func TestActionTokenExpired(t *testing.T) {
	db := setupTokenTestDB(t)
	svc, _ := NewActionTokenService(db)

	// Create with already-expired TTL
	tok, _ := svc.Create("session-abc", "wipe", -1*time.Second)

	err := svc.Validate(tok.Token, "session-abc", "wipe")
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Errorf("expected expired error, got: %v", err)
	}
}

func TestActionTokenWrongSession(t *testing.T) {
	db := setupTokenTestDB(t)
	svc, _ := NewActionTokenService(db)

	tok, _ := svc.Create("session-abc", "format", 5*time.Minute)

	err := svc.Validate(tok.Token, "session-other", "format")
	if err == nil || !strings.Contains(err.Error(), "not bound to this session") {
		t.Errorf("expected session mismatch error, got: %v", err)
	}
}

func TestActionTokenWrongAction(t *testing.T) {
	db := setupTokenTestDB(t)
	svc, _ := NewActionTokenService(db)

	tok, _ := svc.Create("session-abc", "format", 5*time.Minute)

	err := svc.Validate(tok.Token, "session-abc", "delete")
	if err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Errorf("expected action mismatch error, got: %v", err)
	}
}

func TestActionTokenInvalid(t *testing.T) {
	db := setupTokenTestDB(t)
	svc, _ := NewActionTokenService(db)

	err := svc.Validate("nonexistent", "session-abc", "format")
	if err == nil || !strings.Contains(err.Error(), "invalid action token") {
		t.Errorf("expected invalid token error, got: %v", err)
	}
}

func TestActionTokenRevoke(t *testing.T) {
	db := setupTokenTestDB(t)
	svc, _ := NewActionTokenService(db)

	tok, _ := svc.Create("session-abc", "format", 5*time.Minute)

	if err := svc.Revoke(tok.Token); err != nil {
		t.Fatal(err)
	}

	err := svc.Validate(tok.Token, "session-abc", "format")
	if err == nil {
		t.Error("expected error after revoke")
	}
}

func TestActionTokenMissingInputs(t *testing.T) {
	db := setupTokenTestDB(t)
	svc, _ := NewActionTokenService(db)

	if _, err := svc.Create("", "format", time.Minute); err == nil {
		t.Error("expected error for empty session")
	}
	if _, err := svc.Create("sess", "", time.Minute); err == nil {
		t.Error("expected error for empty action")
	}
}
