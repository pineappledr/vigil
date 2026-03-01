package addons

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"
)

const timeFormat = "2006-01-02 15:04:05"

// Register inserts a new add-on or updates the manifest if the name already exists.
// Returns the add-on ID.
func Register(db *sql.DB, name, version, description, manifestJSON string) (int64, error) {
	query := `
	INSERT INTO addons (name, version, description, manifest_json, status, last_seen, updated_at)
	VALUES (?, ?, ?, ?, 'online', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	ON CONFLICT(name) DO UPDATE SET
		version       = excluded.version,
		description   = excluded.description,
		manifest_json = excluded.manifest_json,
		status        = 'online',
		last_seen     = CURRENT_TIMESTAMP,
		updated_at    = CURRENT_TIMESTAMP
	`

	result, err := db.Exec(query, name, version, description, manifestJSON)
	if err != nil {
		return 0, fmt.Errorf("register addon %q: %w", name, err)
	}

	// ON CONFLICT UPDATE doesn't return LastInsertId reliably — query it.
	var id int64
	err = db.QueryRow("SELECT id FROM addons WHERE name = ?", name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("register addon %q: lookup id: %w", name, err)
	}
	_ = result // keep linter happy
	return id, nil
}

// RegisterWithURL creates a minimal add-on record from the admin UI flow.
// The manifest is set to a stub; the add-on will upsert its real manifest on first connect.
func RegisterWithURL(db *sql.DB, name, url string) (int64, error) {
	stubManifest := fmt.Sprintf(`{"name":%q,"version":"0.0.0","description":"Awaiting connection","pages":[{"id":"default","title":"Status","components":[]}]}`, name)

	result, err := db.Exec(`
		INSERT INTO addons (name, version, description, url, manifest_json, status, enabled)
		VALUES (?, '0.0.0', 'Awaiting first connection', ?, ?, 'offline', 1)
		ON CONFLICT(name) DO UPDATE SET
			url        = excluded.url,
			updated_at = CURRENT_TIMESTAMP
	`, name, url, stubManifest)
	if err != nil {
		return 0, fmt.Errorf("register addon with url %q: %w", name, err)
	}

	var id int64
	err = db.QueryRow("SELECT id FROM addons WHERE name = ?", name).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("register addon with url %q: lookup id: %w", name, err)
	}
	_ = result
	return id, nil
}

// Get retrieves a single add-on by ID.
func Get(db *sql.DB, id int64) (*Addon, error) {
	return scanOne(db.QueryRow(`
		SELECT id, name, version, COALESCE(description,''), COALESCE(url,''), manifest_json,
		       status, enabled, COALESCE(last_seen,''), created_at, updated_at
		FROM addons WHERE id = ?`, id))
}

// GetByName retrieves a single add-on by name.
func GetByName(db *sql.DB, name string) (*Addon, error) {
	return scanOne(db.QueryRow(`
		SELECT id, name, version, COALESCE(description,''), COALESCE(url,''), manifest_json,
		       status, enabled, COALESCE(last_seen,''), created_at, updated_at
		FROM addons WHERE name = ?`, name))
}

// List returns all registered add-ons ordered by name.
func List(db *sql.DB) ([]Addon, error) {
	rows, err := db.Query(`
		SELECT id, name, version, COALESCE(description,''), COALESCE(url,''), manifest_json,
		       status, enabled, COALESCE(last_seen,''), created_at, updated_at
		FROM addons ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list addons: %w", err)
	}
	defer rows.Close()

	var out []Addon
	for rows.Next() {
		a, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpdateStatus sets the status of an add-on.
func UpdateStatus(db *sql.DB, id int64, status Status) error {
	res, err := db.Exec(`
		UPDATE addons SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		string(status), id)
	if err != nil {
		return fmt.Errorf("update addon status: %w", err)
	}
	return expectOneRow(res, "update addon status")
}

// SetEnabled enables or disables an add-on.
func SetEnabled(db *sql.DB, id int64, enabled bool) error {
	v := 0
	if enabled {
		v = 1
	}
	res, err := db.Exec(`
		UPDATE addons SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		v, id)
	if err != nil {
		return fmt.Errorf("set addon enabled: %w", err)
	}
	return expectOneRow(res, "set addon enabled")
}

// UpdateManifest updates the manifest, version, description, and status for an existing add-on.
func UpdateManifest(db *sql.DB, id int64, version, description, manifestJSON string) error {
	res, err := db.Exec(`
		UPDATE addons SET
			version       = ?,
			description   = ?,
			manifest_json = ?,
			status        = 'online',
			last_seen     = CURRENT_TIMESTAMP,
			updated_at    = CURRENT_TIMESTAMP
		WHERE id = ?`, version, description, manifestJSON, id)
	if err != nil {
		return fmt.Errorf("update addon manifest: %w", err)
	}
	return expectOneRow(res, "update addon manifest")
}

// TouchHeartbeat updates last_seen for a given add-on.
func TouchHeartbeat(db *sql.DB, id int64) error {
	_, err := db.Exec(`UPDATE addons SET last_seen = CURRENT_TIMESTAMP WHERE id = ?`, id)
	return err
}

// Deregister removes an add-on by ID.
func Deregister(db *sql.DB, id int64) error {
	res, err := db.Exec(`DELETE FROM addons WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("deregister addon: %w", err)
	}
	return expectOneRow(res, "deregister addon")
}

// ── helpers ──────────────────────────────────────────────────────────────

func scanOne(row *sql.Row) (*Addon, error) {
	var a Addon
	var enabled int
	var lastSeen, createdAt, updatedAt string

	err := row.Scan(&a.ID, &a.Name, &a.Version, &a.Description, &a.URL,
		&a.ManifestJSON, &a.Status, &enabled, &lastSeen, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan addon: %w", err)
	}
	a.Enabled = enabled == 1
	a.LastSeen = parseTime(lastSeen)
	a.CreatedAt = parseTime(createdAt)
	a.UpdatedAt = parseTime(updatedAt)
	return &a, nil
}

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanRow(s scannable) (Addon, error) {
	var a Addon
	var enabled int
	var lastSeen, createdAt, updatedAt string

	err := s.Scan(&a.ID, &a.Name, &a.Version, &a.Description, &a.URL,
		&a.ManifestJSON, &a.Status, &enabled, &lastSeen, &createdAt, &updatedAt)
	if err != nil {
		return a, fmt.Errorf("scan addon row: %w", err)
	}
	a.Enabled = enabled == 1
	a.LastSeen = parseTime(lastSeen)
	a.CreatedAt = parseTime(createdAt)
	a.UpdatedAt = parseTime(updatedAt)
	return a, nil
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(timeFormat, s)
	return t
}

func expectOneRow(res sql.Result, op string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: rows affected: %w", op, err)
	}
	if n == 0 {
		return fmt.Errorf("%s: not found", op)
	}
	return nil
}

// ── Token Operations ─────────────────────────────────────────────────────

// CreateRegistrationToken generates and stores a one-time token for add-on enrollment.
func CreateRegistrationToken(db *sql.DB, name string, expiresIn *time.Duration) (*RegistrationToken, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, fmt.Errorf("generate token: %w", err)
	}
	token := hex.EncodeToString(raw)
	now := time.Now().UTC()

	var expiresVal interface{}
	var expiresPtr *time.Time
	if expiresIn != nil {
		t := now.Add(*expiresIn)
		expiresPtr = &t
		expiresVal = t.Format(timeFormat)
	}

	result, err := db.Exec(`
		INSERT INTO addon_registration_tokens (token, name, expires_at)
		VALUES (?, ?, ?)
	`, token, name, expiresVal)
	if err != nil {
		return nil, fmt.Errorf("create addon registration token: %w", err)
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

// GetRegistrationToken retrieves a token by its value.
// Returns nil, nil if not found or expired.
func GetRegistrationToken(db *sql.DB, token string) (*RegistrationToken, error) {
	row := db.QueryRow(`
		SELECT id, token, COALESCE(name,''), created_at,
		       expires_at, used_at, used_by_addon_id
		FROM addon_registration_tokens
		WHERE token = ? AND (expires_at IS NULL OR expires_at > CURRENT_TIMESTAMP)
	`, token)

	var t RegistrationToken
	var createdAt string
	var expiresAt, usedAt sql.NullString
	var usedByID sql.NullInt64

	err := row.Scan(&t.ID, &t.Token, &t.Name, &createdAt, &expiresAt, &usedAt, &usedByID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get addon registration token: %w", err)
	}

	t.CreatedAt = parseTime(createdAt)
	if expiresAt.Valid {
		et := parseTime(expiresAt.String)
		t.ExpiresAt = &et
	}
	if usedAt.Valid {
		ut := parseTime(usedAt.String)
		t.UsedAt = &ut
	}
	if usedByID.Valid {
		t.UsedByAddonID = &usedByID.Int64
	}
	return &t, nil
}

// ListRegistrationTokens returns all add-on registration tokens.
func ListRegistrationTokens(db *sql.DB) ([]RegistrationToken, error) {
	rows, err := db.Query(`
		SELECT id, token, COALESCE(name,''), created_at,
		       expires_at, used_at, used_by_addon_id
		FROM addon_registration_tokens ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("list addon tokens: %w", err)
	}
	defer rows.Close()

	var out []RegistrationToken
	for rows.Next() {
		var t RegistrationToken
		var createdAt string
		var expiresAt, usedAt sql.NullString
		var usedByID sql.NullInt64

		if err := rows.Scan(&t.ID, &t.Token, &t.Name, &createdAt, &expiresAt, &usedAt, &usedByID); err != nil {
			return nil, fmt.Errorf("scan addon token: %w", err)
		}
		t.CreatedAt = parseTime(createdAt)
		if expiresAt.Valid {
			et := parseTime(expiresAt.String)
			t.ExpiresAt = &et
		}
		if usedAt.Valid {
			ut := parseTime(usedAt.String)
			t.UsedAt = &ut
		}
		if usedByID.Valid {
			t.UsedByAddonID = &usedByID.Int64
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// ConsumeRegistrationToken validates a token and marks it as used by addonID.
func ConsumeRegistrationToken(db *sql.DB, token string, addonID int64) error {
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
		UPDATE addon_registration_tokens
		SET used_at = ?, used_by_addon_id = ?
		WHERE token = ?
	`, time.Now().UTC().Format(timeFormat), addonID, token)
	return err
}

// DeleteRegistrationToken removes a token by ID.
func DeleteRegistrationToken(db *sql.DB, id int64) error {
	res, err := db.Exec(`DELETE FROM addon_registration_tokens WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete addon token: %w", err)
	}
	return expectOneRow(res, "delete addon token")
}
