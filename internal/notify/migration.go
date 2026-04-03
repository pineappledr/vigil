package notify

import (
	"database/sql"
	"fmt"
	"log"
)

// Migrate creates notification-specific tables that extend the base
// notification_settings / notification_history tables from migrations.go.
func Migrate(db *sql.DB) error {
	log.Println("🔔 Running migration: Notification extensions")

	statements := []struct {
		label string
		sql   string
	}{
		// Per-event-type rules for each notification service
		{"notification_event_rules", `
			CREATE TABLE IF NOT EXISTS notification_event_rules (
				id            INTEGER PRIMARY KEY AUTOINCREMENT,
				service_id    INTEGER NOT NULL,
				event_type    TEXT    NOT NULL,
				enabled       INTEGER DEFAULT 1,
				cooldown_secs INTEGER DEFAULT 300,
				UNIQUE(service_id, event_type),
				FOREIGN KEY (service_id) REFERENCES notification_settings(id) ON DELETE CASCADE
			);`},
		{"notification_event_rules indexes", `
			CREATE INDEX IF NOT EXISTS idx_notif_rules_service ON notification_event_rules(service_id);`},

		// Quiet hours per service
		{"notification_quiet_hours", `
			CREATE TABLE IF NOT EXISTS notification_quiet_hours (
				id         INTEGER PRIMARY KEY AUTOINCREMENT,
				service_id INTEGER NOT NULL UNIQUE,
				start_time TEXT    NOT NULL DEFAULT '22:00',
				end_time   TEXT    NOT NULL DEFAULT '07:00',
				enabled    INTEGER DEFAULT 0,
				FOREIGN KEY (service_id) REFERENCES notification_settings(id) ON DELETE CASCADE
			);`},

		// Daily digest configuration per service
		{"notification_digest_config", `
			CREATE TABLE IF NOT EXISTS notification_digest_config (
				id         INTEGER PRIMARY KEY AUTOINCREMENT,
				service_id INTEGER NOT NULL UNIQUE,
				enabled    INTEGER DEFAULT 0,
				send_at    TEXT    NOT NULL DEFAULT '08:00',
				FOREIGN KEY (service_id) REFERENCES notification_settings(id) ON DELETE CASCADE
			);`},
	}

	for _, s := range statements {
		if _, err := db.Exec(s.sql); err != nil {
			return fmt.Errorf("notification migration failed at [%s]: %w", s.label, err)
		}
		log.Printf("  ✓ %s", s.label)
	}

	// Fix existing rules that have cooldown=0 for monitoring events that
	// should have a non-zero cooldown. Without this, previously-created
	// services keep spamming because the cooldown was never set.
	cooldownFixes := map[string]int{
		"smart_warning":      86400,
		"smart_critical":     86400,
		"temp_critical":      3600,
		"zfs_pool_degraded":  86400,
		"zfs_pool_faulted":   86400,
		"zfs_device_failed":  86400,
		"reallocated_sectors": 86400,
		"wearout_warning":    86400,
		"wearout_critical":   86400,
		"wearout_predicted":  604800,
	}
	for eventType, cooldown := range cooldownFixes {
		_, err := db.Exec(`
			UPDATE notification_event_rules
			SET cooldown_secs = ?
			WHERE event_type = ? AND cooldown_secs = 0`,
			cooldown, eventType)
		if err != nil {
			log.Printf("  ⚠️  fix cooldown for %s: %v", eventType, err)
		}
	}
	log.Printf("  ✓ monitoring cooldowns backfilled")

	log.Println("🔔 Migration completed: Notification extensions ready")
	return nil
}
