package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// ActionToken is a single-use, time-limited, session-bound token that
// authorises a specific destructive action.
type ActionToken struct {
	Token     string `json:"token"`
	Action    string `json:"action"`
	SessionID string `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ActionTokenService manages action tokens backed by SQLite.
type ActionTokenService struct {
	db *sql.DB
}

// NewActionTokenService creates a new service and ensures the schema exists.
func NewActionTokenService(db *sql.DB) (*ActionTokenService, error) {
	svc := &ActionTokenService{db: db}
	if err := svc.migrate(); err != nil {
		return nil, err
	}
	return svc, nil
}

func (s *ActionTokenService) migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS action_tokens (
			token      TEXT PRIMARY KEY,
			action     TEXT NOT NULL,
			session_id TEXT NOT NULL,
			used       INTEGER DEFAULT 0,
			expires_at DATETIME NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_action_tokens_session ON action_tokens(session_id);
		CREATE INDEX IF NOT EXISTS idx_action_tokens_expires ON action_tokens(expires_at);
	`)
	return err
}

// Create generates a new action token bound to the given session.
// The token expires after ttl.
func (s *ActionTokenService) Create(sessionToken, action string, ttl time.Duration) (*ActionToken, error) {
	if sessionToken == "" {
		return nil, fmt.Errorf("session token is required")
	}
	if action == "" {
		return nil, fmt.Errorf("action is required")
	}

	token, err := generateActionToken()
	if err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}

	expiresAt := time.Now().UTC().Add(ttl)

	_, err = s.db.Exec(`
		INSERT INTO action_tokens (token, action, session_id, expires_at)
		VALUES (?, ?, ?, ?)`,
		token, action, sessionToken,
		expiresAt.Format("2006-01-02 15:04:05"))
	if err != nil {
		return nil, fmt.Errorf("store action token: %w", err)
	}

	return &ActionToken{
		Token:     token,
		Action:    action,
		SessionID: sessionToken,
		ExpiresAt: expiresAt,
	}, nil
}

// Validate checks that the token is valid, unused, not expired, bound to
// the correct session, and matches the requested action.  On success the
// token is consumed (single-use) and cannot be reused.
func (s *ActionTokenService) Validate(token, sessionToken, action string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var storedAction, storedSession string
	var used int
	var expired int

	err = tx.QueryRow(`
		SELECT action, session_id, used,
		       CASE WHEN expires_at < datetime('now') THEN 1 ELSE 0 END
		FROM action_tokens WHERE token = ?`, token).
		Scan(&storedAction, &storedSession, &used, &expired)
	if err == sql.ErrNoRows {
		return fmt.Errorf("invalid action token")
	}
	if err != nil {
		return fmt.Errorf("lookup token: %w", err)
	}

	if used != 0 {
		return fmt.Errorf("action token already consumed")
	}

	if expired != 0 {
		return fmt.Errorf("action token expired")
	}

	if storedSession != sessionToken {
		return fmt.Errorf("action token not bound to this session")
	}

	if storedAction != action {
		return fmt.Errorf("action token does not match requested action %q", action)
	}

	// Consume the token (single-use)
	_, err = tx.Exec(`UPDATE action_tokens SET used = 1 WHERE token = ?`, token)
	if err != nil {
		return fmt.Errorf("consume token: %w", err)
	}

	return tx.Commit()
}

// Revoke deletes an action token (e.g. user cancelled the operation).
func (s *ActionTokenService) Revoke(token string) error {
	_, err := s.db.Exec(`DELETE FROM action_tokens WHERE token = ?`, token)
	return err
}

// CleanupExpired removes tokens that have expired.
func (s *ActionTokenService) CleanupExpired() {
	s.db.Exec(`DELETE FROM action_tokens WHERE expires_at < datetime('now')`)
}

// generateActionToken produces a 32-byte hex-encoded random string.
func generateActionToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
