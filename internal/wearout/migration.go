package wearout

import (
	"database/sql"
	"fmt"
	"log"
)

// MigrateWearoutTables creates the wearout_history and drive_specs tables.
func MigrateWearoutTables(db *sql.DB) error {
	log.Println("Running migration: Wearout tables")

	statements := []struct {
		label string
		sql   string
	}{
		{"wearout_history", `
			CREATE TABLE IF NOT EXISTS wearout_history (
				id            INTEGER PRIMARY KEY AUTOINCREMENT,
				hostname      TEXT    NOT NULL,
				serial_number TEXT    NOT NULL,
				drive_type    TEXT    NOT NULL,
				percentage    REAL    NOT NULL,
				factors_json  TEXT,
				timestamp     DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(hostname, serial_number, timestamp)
			);`},
		{"wearout_history indexes", `
			CREATE INDEX IF NOT EXISTS idx_wearout_host_serial ON wearout_history(hostname, serial_number);
			CREATE INDEX IF NOT EXISTS idx_wearout_timestamp   ON wearout_history(timestamp);`},
		{"drive_specs", `
			CREATE TABLE IF NOT EXISTS drive_specs (
				id                INTEGER PRIMARY KEY AUTOINCREMENT,
				model_pattern     TEXT NOT NULL UNIQUE,
				rated_tbw         REAL,
				rated_mtbf_hours  INTEGER,
				rated_load_cycles INTEGER,
				created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP
			);`},
	}

	for _, s := range statements {
		if _, err := db.Exec(s.sql); err != nil {
			return fmt.Errorf("wearout migration failed at [%s]: %w", s.label, err)
		}
		log.Printf("  > %s", s.label)
	}

	log.Println("Migration completed: Wearout tables ready")
	return nil
}
