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

// UpdateAgentLastAuth stamps last_auth_at to now (UTC).
func UpdateAgentLastAuth(db *sql.DB, agentID int64) error {
	_, err := db.Exec(
		"UPDATE agent_registry SET last_auth_at = ? WHERE id = ?",
		time.Now().UTC().Format(timeFormat), agentID,
	)
	return err
}

// UpdateAgentLastSeen stamps last_seen_at to now (UTC).
func UpdateAgentLastSeen(db *sql.DB, agentID int64) error {
	_, err := db.Exec(
		"UPDATE agent_registry SET last_seen_at = ? WHERE id = ?",
		time.Now().UTC().Format(timeFormat), agentID,
	)
	return err
}

// DeleteAgent removes an agent and all its sessions (ON DELETE CASCADE).
func DeleteAgent(db *sql.DB, id int64) error {
	_, err := db.Exec("DELETE FROM agent_registry WHERE id = ?", id)
	return err
}

// DeleteHostData removes all hostname-keyed data: reports, drive aliases,
// ZFS pools (cascades to devices/scrub history), wearout history, and
// SMART attributes.  Call this after DeleteAgent to fully clean up.
func DeleteHostData(db *sql.DB, hostname string) (deleted map[string]int64) {
	deleted = make(map[string]int64)

	tables := []struct {
		label string
		sql   string
	}{
		{"reports", "DELETE FROM reports WHERE hostname = ?"},
		{"drive_aliases", "DELETE FROM drive_aliases WHERE hostname = ?"},
		{"zfs_pools", "DELETE FROM zfs_pools WHERE hostname = ?"},
		{"wearout_history", "DELETE FROM wearout_history WHERE hostname = ?"},
		{"smart_attributes", "DELETE FROM smart_attributes WHERE hostname = ?"},
	}

	for _, t := range tables {
		result, err := db.Exec(t.sql, hostname)
		if err != nil {
			continue // table may not exist yet
		}
		n, _ := result.RowsAffected()
		if n > 0 {
			deleted[t.label] = n
		}
	}
	return deleted
}

// ─── Registration Tokens ─────────────────────────────────────────────────────

// CreateRegistrationToken generates and stores a one-time token.
// If expiresIn is nil the token never expires; otherwise it expires after the
// given duration.
func CreateRegistrationToken(db *sql.DB, name string, expiresIn *time.Duration) (*RegistrationToken, error) {
	raw := make([]byte, 32)
	rand.Read(raw)
	token := hex.EncodeToString(raw)

	now := time.Now().UTC()

	var expiresVal interface{} // nil → SQL NULL
	var expiresPtr *time.Time
	if expiresIn != nil {
		t := now.Add(*expiresIn)
		expiresPtr = &t
		expiresVal = t.Format(timeFormat)
	}

	result, err := db.Exec(`
		INSERT INTO agent_registration_tokens (token, name, expires_at)
		VALUES (?, ?, ?)
	`, token, name, expiresVal)
	if err != nil {
		return nil, fmt.Errorf("create registration token: %w", err)
	}

	id, _ := result.LastInsertId()
	return &RegistrationToken{
		ID:        id,
		Token:     token,
		Name:      name,
		CreatedAt: now,
		ExpiresAt: expiresPtr,
	}, nil
}

// GetRegistrationToken retrieves a valid token. Returns nil if not found or
// expired. Tokens with NULL expires_at never expire.
func GetRegistrationToken(db *sql.DB, token string) (*RegistrationToken, error) {
	var t RegistrationToken
	var usedAt sql.NullString
	var usedByAgentID sql.NullInt64
	var createdAt string
	var expiresAt sql.NullString

	err := db.QueryRow(`
		SELECT id, token, name, created_at, expires_at, used_at, used_by_agent_id
		FROM agent_registration_tokens
		WHERE token = ? AND (expires_at IS NULL OR expires_at > datetime('now'))
	`, token).Scan(&t.ID, &t.Token, &t.Name, &createdAt, &expiresAt, &usedAt, &usedByAgentID)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	t.CreatedAt, _ = time.Parse(timeFormat, createdAt)
	if expiresAt.Valid {
		ts, _ := time.Parse(timeFormat, expiresAt.String)
		t.ExpiresAt = &ts
	}

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
	`, time.Now().UTC().Format(timeFormat), agentID, token)
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
		var createdAt string
		var expiresAt sql.NullString

		if err := rows.Scan(&t.ID, &t.Token, &t.Name, &createdAt, &expiresAt, &usedAt, &usedByAgentID); err != nil {
			return nil, err
		}

		t.CreatedAt, _ = time.Parse(timeFormat, createdAt)
		if expiresAt.Valid {
			ts, _ := time.Parse(timeFormat, expiresAt.String)
			t.ExpiresAt = &ts
		}

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

	now := time.Now().UTC()
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
