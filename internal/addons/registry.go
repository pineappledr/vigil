package addons

import (
	"database/sql"
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

// Get retrieves a single add-on by ID.
func Get(db *sql.DB, id int64) (*Addon, error) {
	return scanOne(db.QueryRow(`
		SELECT id, name, version, COALESCE(description,''), manifest_json,
		       status, enabled, COALESCE(last_seen,''), created_at, updated_at
		FROM addons WHERE id = ?`, id))
}

// GetByName retrieves a single add-on by name.
func GetByName(db *sql.DB, name string) (*Addon, error) {
	return scanOne(db.QueryRow(`
		SELECT id, name, version, COALESCE(description,''), manifest_json,
		       status, enabled, COALESCE(last_seen,''), created_at, updated_at
		FROM addons WHERE name = ?`, name))
}

// List returns all registered add-ons ordered by name.
func List(db *sql.DB) ([]Addon, error) {
	rows, err := db.Query(`
		SELECT id, name, version, COALESCE(description,''), manifest_json,
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

	err := row.Scan(&a.ID, &a.Name, &a.Version, &a.Description,
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

	err := s.Scan(&a.ID, &a.Name, &a.Version, &a.Description,
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
