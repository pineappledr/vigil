package addons

import (
	"database/sql"
	"fmt"
	"log"
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
	}

	for _, s := range statements {
		if _, err := db.Exec(s.sql); err != nil {
			return fmt.Errorf("addon migration failed at [%s]: %w", s.label, err)
		}
		log.Printf("  ✓ %s", s.label)
	}

	log.Println("📦 Migration completed: Add-on registry ready")
	return nil
}
