package addons

import (
	"crypto/ed25519"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"vigil/internal/auth"
	"vigil/internal/crypto"

	_ "modernc.org/sqlite"
)

func setupExecutorTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	db.Exec("PRAGMA foreign_keys = ON")

	// Create required tables
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	// agents table (simplified)
	db.Exec(`CREATE TABLE IF NOT EXISTS agents (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hostname TEXT,
		enabled INTEGER DEFAULT 1
	)`)

	t.Cleanup(func() { db.Close() })
	return db
}

func setupExecutor(t *testing.T) (*sql.DB, *Executor) {
	t.Helper()
	db := setupExecutorTestDB(t)

	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	keys := &crypto.ServerKeys{PrivateKey: priv, PublicKey: pub}

	tokenSvc, err := auth.NewActionTokenService(db)
	if err != nil {
		t.Fatal(err)
	}

	exec := NewExecutor(db, keys, tokenSvc)
	return db, exec
}

// registerTestAddonWithAction creates an addon whose manifest declares a form
// with a security_gate field matching the given action name.
func registerTestAddonWithAction(t *testing.T, db *sql.DB, action string) int64 {
	t.Helper()
	manifest := map[string]interface{}{
		"name":    "test-addon",
		"version": "1.0.0",
		"pages": []map[string]interface{}{{
			"id":    "main",
			"title": "Main",
			"components": []map[string]interface{}{{
				"type": "form",
				"id":   "cmd-form",
				"config": map[string]interface{}{
					"action": action,
					"fields": []map[string]interface{}{{
						"name":  "target",
						"label": "Target",
						"type":  "text",
					}},
				},
			}},
		}},
	}
	manifestJSON, _ := json.Marshal(manifest)
	id, err := Register(db, "test-addon", "1.0.0", "test", string(manifestJSON))
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestExecutor_FullPipeline(t *testing.T) {
	db, exec := setupExecutor(t)

	// Setup: addon, agent, action token
	addonID := registerTestAddonWithAction(t, db, "format_drive")
	db.Exec("INSERT INTO agents (id, hostname, enabled) VALUES (1, 'node1', 1)")

	tok, err := exec.tokenService.Create("session-abc", "format_drive", 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}

	cmd, err := exec.Execute(CommandRequest{
		AddonID:      addonID,
		AgentID:      1,
		Action:       "format_drive",
		ActionToken:  tok.Token,
		SessionToken: "session-abc",
		Params:       json.RawMessage(`{"device":"/dev/sda"}`),
	})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	if cmd.Signature == "" {
		t.Error("expected non-empty signature")
	}
	if cmd.AgentID != 1 {
		t.Errorf("agent_id = %d, want 1", cmd.AgentID)
	}

	// Verify the signature
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	verifyKeys := &crypto.ServerKeys{PrivateKey: priv, PublicKey: pub}
	// Should NOT verify with wrong keys
	if VerifyCommandSignature(verifyKeys, cmd) {
		t.Error("should not verify with different keys")
	}
}

func TestExecutor_Step1_AddonNotFound(t *testing.T) {
	_, exec := setupExecutor(t)

	_, err := exec.Execute(CommandRequest{
		AddonID:      9999,
		AgentID:      1,
		Action:       "test",
		ActionToken:  "tok",
		SessionToken: "sess",
	})
	if err == nil || !strings.Contains(err.Error(), "step 1") {
		t.Errorf("expected step 1 error, got: %v", err)
	}
}

func TestExecutor_Step1_AddonDisabled(t *testing.T) {
	db, exec := setupExecutor(t)
	addonID := registerTestAddonWithAction(t, db, "test")
	SetEnabled(db, addonID, false)

	_, err := exec.Execute(CommandRequest{
		AddonID:      addonID,
		AgentID:      1,
		Action:       "test",
		ActionToken:  "tok",
		SessionToken: "sess",
	})
	if err == nil || !strings.Contains(err.Error(), "disabled") {
		t.Errorf("expected disabled error, got: %v", err)
	}
}

func TestExecutor_Step2_InvalidToken(t *testing.T) {
	db, exec := setupExecutor(t)
	addonID := registerTestAddonWithAction(t, db, "test")

	_, err := exec.Execute(CommandRequest{
		AddonID:      addonID,
		AgentID:      1,
		Action:       "test",
		ActionToken:  "invalid-token",
		SessionToken: "sess",
	})
	if err == nil || !strings.Contains(err.Error(), "step 2") {
		t.Errorf("expected step 2 error, got: %v", err)
	}
}

func TestExecutor_Step3_AgentNotFound(t *testing.T) {
	db, exec := setupExecutor(t)
	addonID := registerTestAddonWithAction(t, db, "format_drive")

	tok, _ := exec.tokenService.Create("sess", "format_drive", 5*time.Minute)

	_, err := exec.Execute(CommandRequest{
		AddonID:      addonID,
		AgentID:      9999,
		Action:       "format_drive",
		ActionToken:  tok.Token,
		SessionToken: "sess",
	})
	if err == nil || !strings.Contains(err.Error(), "step 3") {
		t.Errorf("expected step 3 error, got: %v", err)
	}
}

func TestExecutor_Step3_AgentDisabled(t *testing.T) {
	db, exec := setupExecutor(t)
	addonID := registerTestAddonWithAction(t, db, "format_drive")
	db.Exec("INSERT INTO agents (id, hostname, enabled) VALUES (2, 'node2', 0)")

	tok, _ := exec.tokenService.Create("sess", "format_drive", 5*time.Minute)

	_, err := exec.Execute(CommandRequest{
		AddonID:      addonID,
		AgentID:      2,
		Action:       "format_drive",
		ActionToken:  tok.Token,
		SessionToken: "sess",
	})
	if err == nil || !strings.Contains(err.Error(), "step 3") {
		t.Errorf("expected step 3 error, got: %v", err)
	}
}

func TestExecutor_Step4_ActionNotInManifest(t *testing.T) {
	db, exec := setupExecutor(t)
	addonID := registerTestAddonWithAction(t, db, "format_drive")
	db.Exec("INSERT INTO agents (id, hostname, enabled) VALUES (1, 'node1', 1)")

	tok, _ := exec.tokenService.Create("sess", "delete_everything", 5*time.Minute)

	_, err := exec.Execute(CommandRequest{
		AddonID:      addonID,
		AgentID:      1,
		Action:       "delete_everything",
		ActionToken:  tok.Token,
		SessionToken: "sess",
	})
	if err == nil || !strings.Contains(err.Error(), "step 4") {
		t.Errorf("expected step 4 error, got: %v", err)
	}
}

func TestExecutor_Step5_BadParams(t *testing.T) {
	db, exec := setupExecutor(t)
	addonID := registerTestAddonWithAction(t, db, "format_drive")
	db.Exec("INSERT INTO agents (id, hostname, enabled) VALUES (1, 'node1', 1)")

	tok, _ := exec.tokenService.Create("sess", "format_drive", 5*time.Minute)

	_, err := exec.Execute(CommandRequest{
		AddonID:      addonID,
		AgentID:      1,
		Action:       "format_drive",
		ActionToken:  tok.Token,
		SessionToken: "sess",
		Params:       json.RawMessage(`{invalid}`),
	})
	if err == nil || !strings.Contains(err.Error(), "step 5") {
		t.Errorf("expected step 5 error, got: %v", err)
	}
}

func TestExecutor_TokenSingleUse(t *testing.T) {
	db, exec := setupExecutor(t)
	addonID := registerTestAddonWithAction(t, db, "format_drive")
	db.Exec("INSERT INTO agents (id, hostname, enabled) VALUES (1, 'node1', 1)")

	tok, _ := exec.tokenService.Create("sess", "format_drive", 5*time.Minute)

	// First execution succeeds
	_, err := exec.Execute(CommandRequest{
		AddonID:      addonID,
		AgentID:      1,
		Action:       "format_drive",
		ActionToken:  tok.Token,
		SessionToken: "sess",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Create a new token for same action (old one consumed) — but try to reuse old one
	_, err = exec.Execute(CommandRequest{
		AddonID:      addonID,
		AgentID:      1,
		Action:       "format_drive",
		ActionToken:  tok.Token,
		SessionToken: "sess",
	})
	if err == nil || !strings.Contains(err.Error(), "consumed") {
		t.Errorf("expected consumed error on reuse, got: %v", err)
	}
}
