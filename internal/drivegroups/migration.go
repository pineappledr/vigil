package drivegroups

import (
	"database/sql"
	"fmt"
)

// Migrate creates the drive groups tables if they don't exist.
func Migrate(db *sql.DB) error {
	stmts := []struct {
		name string
		sql  string
	}{
		{"drive_groups", `
			CREATE TABLE IF NOT EXISTS drive_groups (
				id         INTEGER PRIMARY KEY AUTOINCREMENT,
				name       TEXT NOT NULL UNIQUE,
				color      TEXT NOT NULL DEFAULT '#6366f1',
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)`},
		{"drive_group_members", `
			CREATE TABLE IF NOT EXISTS drive_group_members (
				id            INTEGER PRIMARY KEY AUTOINCREMENT,
				group_id      INTEGER NOT NULL,
				hostname      TEXT    NOT NULL,
				serial_number TEXT    NOT NULL,
				created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(hostname, serial_number),
				FOREIGN KEY (group_id) REFERENCES drive_groups(id) ON DELETE CASCADE
			)`},
		{"drive_group_members indexes", `
			CREATE INDEX IF NOT EXISTS idx_dgm_group ON drive_group_members(group_id);
			CREATE INDEX IF NOT EXISTS idx_dgm_drive ON drive_group_members(hostname, serial_number);`},
		{"notification_group_event_rules", `
			CREATE TABLE IF NOT EXISTS notification_group_event_rules (
				id            INTEGER PRIMARY KEY AUTOINCREMENT,
				service_id    INTEGER NOT NULL,
				group_id      INTEGER NOT NULL,
				event_type    TEXT    NOT NULL,
				enabled       INTEGER DEFAULT 1,
				cooldown_secs INTEGER DEFAULT 300,
				UNIQUE(service_id, group_id, event_type),
				FOREIGN KEY (service_id) REFERENCES notification_settings(id) ON DELETE CASCADE,
				FOREIGN KEY (group_id) REFERENCES drive_groups(id) ON DELETE CASCADE
			)`},
		{"notification_group_event_rules index", `
			CREATE INDEX IF NOT EXISTS idx_nger_service_group ON notification_group_event_rules(service_id, group_id);`},
	}

	for _, s := range stmts {
		if _, err := db.Exec(s.sql); err != nil {
			return fmt.Errorf("drivegroups migration %s: %w", s.name, err)
		}
	}
	return nil
}
