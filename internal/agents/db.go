package agents

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

// ─── Agent Registry ───────────────────────────────────────────────────────────

// RegisterAgent inserts a new agent record and returns it.
func RegisterAgent(db *sql.DB, hostname, name, fingerprint, publicKey string) (*Agent, error) {
	result, err := db.Exec(`
		INSERT INTO agent_registry (hostname, name, fingerprint, public_key)
		VALUES (?, ?, ?, ?)
	`, hostname, name, fingerprint, publicKey)
	if err != nil {
		return nil, fmt.Errorf("insert agent: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	return GetAgentByID(db, id)
}

// GetAgentByID retrieves an agent by primary key.
func GetAgentByID(db *sql.DB, id int64) (*Agent, error) {
	row := db.QueryRow(`
		SELECT id, hostname, name, fingerprint, public_key,
		       registered_at, last_auth_at, last_seen_at, enabled
		FROM agent_registry WHERE id = ?
	`, id)
	return scanAgentRow(row)
}

// GetAgentByFingerprint retrieves an enabled agent by its machine fingerprint.
func GetAgentByFingerprint(db *sql.DB, fingerprint string) (*Agent, error) {
	row := db.QueryRow(`
		SELECT id, hostname, name, fingerprint, public_key,
		       registered_at, last_auth_at, last_seen_at, enabled
		FROM agent_registry WHERE fingerprint = ? AND enabled = 1
	`, fingerprint)
	return scanAgentRow(row)
}

// ListAgents returns all registered agents ordered by hostname.
func ListAgents(db *sql.DB) ([]Agent, error) {
	rows, err := db.Query(`
		SELECT id, hostname, name, fingerprint, public_key,
		       registered_at, last_auth_at, last_seen_at, enabled
		FROM agent_registry ORDER BY hostname
	`)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var out []Agent
	for rows.Next() {
		a, err := scanAgentRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, nil
}

// UpdateAgentLastAuth stamps last_auth_at to now.
func UpdateAgentLastAuth(db *sql.DB, agentID int64) error {
	_, err := db.Exec(
		"UPDATE agent_registry SET last_auth_at = ? WHERE id = ?",
		time.Now().Format(timeFormat), agentID,
	)
	return err
}

// UpdateAgentLastSeen stamps last_seen_at to now.
func UpdateAgentLastSeen(db *sql.DB, agentID int64) error {
	_, err := db.Exec(
		"UPDATE agent_registry SET last_seen_at = ? WHERE id = ?",
		time.Now().Format(timeFormat), agentID,
	)
	return err
}

// DeleteAgent removes an agent and all its sessions (ON DELETE CASCADE).
func DeleteAgent(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM agent_registry WHERE id = ?", id)
	return err
}

// ─── Registration Tokens ─────────────────────────────────────────────────────

// CreateRegistrationToken generates and stores a new 24-hour one-time token.
func CreateRegistrationToken(db *sql.DB, name string) (*RegistrationToken, error) {
	raw := make([]byte, 32)
	rand.Read(raw)
	token := hex.EncodeToString(raw)

	now := time.Now()
	expiresAt := now.Add(24 * time.Hour)

	result, err := db.Exec(`
		INSERT INTO agent_registration_tokens (token, name, expires_at)
		VALUES (?, ?, ?)
	`, token, name, expiresAt.Format(timeFormat))
	if err != nil {
		return nil, fmt.Errorf("create registration token: %w", err)
	}

	id, _ := result.LastInsertId()
	return &RegistrationToken{
		ID:        id,
		Token:     token,
		Name:      name,
		CreatedAt: now,
		ExpiresAt: expiresAt,
	}, nil
}

// GetRegistrationToken retrieves an unexpired token. Returns nil if not found
// or already expired.
func GetRegistrationToken(db *sql.DB, token string) (*RegistrationToken, error) {
	var t RegistrationToken
	var usedAt sql.NullString
	var usedByAgentID sql.NullInt64
	var createdAt, expiresAt string

	err := db.QueryRow(`
		SELECT id, token, name, created_at, expires_at, used_at, used_by_agent_id
		FROM agent_registration_tokens
		WHERE token = ? AND expires_at > datetime('now')
	`, token).Scan(&t.ID, &t.Token, &t.Name, &createdAt, &expiresAt, &usedAt, &usedByAgentID)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	t.CreatedAt, _ = time.Parse(timeFormat, createdAt)
	t.ExpiresAt, _ = time.Parse(timeFormat, expiresAt)

	if usedAt.Valid {
		ts, _ := time.Parse(timeFormat, usedAt.String)
		t.UsedAt = &ts
	}
	if usedByAgentID.Valid {
		id := usedByAgentID.Int64
		t.UsedByAgentID = &id
	}

	return &t, nil
}

// ConsumeRegistrationToken validates a token and marks it as used by agentID.
// Returns an error if the token is not found, already expired, or already used.
func ConsumeRegistrationToken(db *sql.DB, token string, agentID int64) error {
	t, err := GetRegistrationToken(db, token)
	if err != nil {
		return err
	}
	if t == nil {
		return fmt.Errorf("token not found or expired")
	}
	if t.UsedAt != nil {
		return fmt.Errorf("token already used")
	}

	_, err = db.Exec(`
		UPDATE agent_registration_tokens
		SET used_at = ?, used_by_agent_id = ?
		WHERE token = ?
	`, time.Now().Format(timeFormat), agentID, token)
	return err
}

// ListRegistrationTokens returns all tokens (including used/expired) for admin UI.
func ListRegistrationTokens(db *sql.DB) ([]RegistrationToken, error) {
	rows, err := db.Query(`
		SELECT id, token, name, created_at, expires_at, used_at, used_by_agent_id
		FROM agent_registration_tokens ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []RegistrationToken
	for rows.Next() {
		var t RegistrationToken
		var usedAt sql.NullString
		var usedByAgentID sql.NullInt64
		var createdAt, expiresAt string

		if err := rows.Scan(&t.ID, &t.Token, &t.Name, &createdAt, &expiresAt, &usedAt, &usedByAgentID); err != nil {
			return nil, err
		}

		t.CreatedAt, _ = time.Parse(timeFormat, createdAt)
		t.ExpiresAt, _ = time.Parse(timeFormat, expiresAt)

		if usedAt.Valid {
			ts, _ := time.Parse(timeFormat, usedAt.String)
			t.UsedAt = &ts
		}
		if usedByAgentID.Valid {
			id := usedByAgentID.Int64
			t.UsedByAgentID = &id
		}

		out = append(out, t)
	}
	return out, nil
}

// DeleteRegistrationToken removes a token by ID.
func DeleteRegistrationToken(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM agent_registration_tokens WHERE id = ?", id)
	return err
}

// ─── Agent Sessions ───────────────────────────────────────────────────────────

// CreateAgentSession invalidates any existing sessions for the agent and
// issues a fresh 1-hour session token.
func CreateAgentSession(db *sql.DB, agentID int64) (*AgentSession, error) {
	db.Exec("DELETE FROM agent_sessions WHERE agent_id = ?", agentID)

	raw := make([]byte, 32)
	rand.Read(raw)
	token := hex.EncodeToString(raw)

	now := time.Now()
	expiresAt := now.Add(time.Hour)

	_, err := db.Exec(`
		INSERT INTO agent_sessions (token, agent_id, expires_at, created_at)
		VALUES (?, ?, ?, ?)
	`, token, agentID, expiresAt.Format(timeFormat), now.Format(timeFormat))
	if err != nil {
		return nil, fmt.Errorf("create agent session: %w", err)
	}

	return &AgentSession{
		Token:     token,
		AgentID:   agentID,
		ExpiresAt: expiresAt,
		CreatedAt: now,
	}, nil
}

// GetAgentSession retrieves a valid (unexpired) session by token.
// Returns nil if the session does not exist or has expired.
func GetAgentSession(db *sql.DB, token string) (*AgentSession, error) {
	var s AgentSession
	var expiresAt, createdAt string

	err := db.QueryRow(`
		SELECT token, agent_id, expires_at, created_at
		FROM agent_sessions
		WHERE token = ? AND expires_at > datetime('now')
	`, token).Scan(&s.Token, &s.AgentID, &expiresAt, &createdAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	s.ExpiresAt, _ = time.Parse(timeFormat, expiresAt)
	s.CreatedAt, _ = time.Parse(timeFormat, createdAt)
	return &s, nil
}

// CleanupExpiredAgentSessions deletes all expired sessions.
func CleanupExpiredAgentSessions(db *sql.DB) {
	db.Exec("DELETE FROM agent_sessions WHERE expires_at < datetime('now')")
}

// ─── Scan helpers ─────────────────────────────────────────────────────────────

func scanAgentRow(row *sql.Row) (*Agent, error) {
	var a Agent
	var name sql.NullString
	var registeredAt, lastAuthAt, lastSeenAt sql.NullString
	var enabled int

	err := row.Scan(
		&a.ID, &a.Hostname, &name, &a.Fingerprint, &a.PublicKey,
		&registeredAt, &lastAuthAt, &lastSeenAt, &enabled,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	applyAgentFields(&a, name, registeredAt, lastAuthAt, lastSeenAt, enabled)
	return &a, nil
}

func scanAgentRows(rows *sql.Rows) (*Agent, error) {
	var a Agent
	var name sql.NullString
	var registeredAt, lastAuthAt, lastSeenAt sql.NullString
	var enabled int

	if err := rows.Scan(
		&a.ID, &a.Hostname, &name, &a.Fingerprint, &a.PublicKey,
		&registeredAt, &lastAuthAt, &lastSeenAt, &enabled,
	); err != nil {
		return nil, err
	}

	applyAgentFields(&a, name, registeredAt, lastAuthAt, lastSeenAt, enabled)
	return &a, nil
}

func applyAgentFields(a *Agent, name sql.NullString, registeredAt, lastAuthAt, lastSeenAt sql.NullString, enabled int) {
	if name.Valid {
		a.Name = name.String
	}
	a.Enabled = enabled == 1
	if registeredAt.Valid {
		a.RegisteredAt, _ = time.Parse(timeFormat, registeredAt.String)
	}
	if lastAuthAt.Valid {
		t, _ := time.Parse(timeFormat, lastAuthAt.String)
		a.LastAuthAt = &t
	}
	if lastSeenAt.Valid {
		t, _ := time.Parse(timeFormat, lastSeenAt.String)
		a.LastSeenAt = &t
	}
}
