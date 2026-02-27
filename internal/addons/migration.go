package addons

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
)

// Migrate creates the addons table and indexes.
func Migrate(db *sql.DB) error {
	log.Println("📦 Running migration: Add-on registry")

	statements := []struct {
		label string
		sql   string
	}{
		{"addons", `
			CREATE TABLE IF NOT EXISTS addons (
				id            INTEGER PRIMARY KEY AUTOINCREMENT,
				name          TEXT    NOT NULL UNIQUE,
				version       TEXT    NOT NULL,
				description   TEXT,
				url           TEXT,
				manifest_json TEXT    NOT NULL,
				status        TEXT    NOT NULL DEFAULT 'offline',
				enabled       INTEGER DEFAULT 1,
				last_seen     DATETIME,
				created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP
			);`},
		{"addons indexes", `
			CREATE INDEX IF NOT EXISTS idx_addons_status  ON addons(status);
			CREATE INDEX IF NOT EXISTS idx_addons_enabled ON addons(enabled);`},
		{"addon_registration_tokens", `
			CREATE TABLE IF NOT EXISTS addon_registration_tokens (
				id               INTEGER PRIMARY KEY AUTOINCREMENT,
				token            TEXT    NOT NULL UNIQUE,
				name             TEXT,
				created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
				expires_at       DATETIME,
				used_at          DATETIME,
				used_by_addon_id INTEGER,
				FOREIGN KEY (used_by_addon_id) REFERENCES addons(id) ON DELETE SET NULL
			);`},
		{"addon_registration_tokens indexes", `
			CREATE INDEX IF NOT EXISTS idx_addon_reg_tokens_token   ON addon_registration_tokens(token);
			CREATE INDEX IF NOT EXISTS idx_addon_reg_tokens_expires ON addon_registration_tokens(expires_at);`},
	}

	for _, s := range statements {
		if _, err := db.Exec(s.sql); err != nil {
			return fmt.Errorf("addon migration failed at [%s]: %w", s.label, err)
		}
		log.Printf("  ✓ %s", s.label)
	}

	// Backfill url column for databases created before v3.0.
	// ALTER TABLE ... ADD COLUMN fails with "duplicate column" if it already exists.
	if _, err := db.Exec(`ALTER TABLE addons ADD COLUMN url TEXT`); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("addon migration failed at [addons url column]: %w", err)
		}
	} else {
		log.Printf("  ✓ addons url column (backfill)")
	}

	log.Println("📦 Migration completed: Add-on registry ready")
	return nil
}
