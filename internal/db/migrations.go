package db

import (
	"database/sql"
	"fmt"
	"log"
)

// MigrateSchemaExtensions creates tables for extended features:
// - Notification settings (Shoutrrr integration)
// - Audit logging
// - ZFS pool monitoring
// - API token management
func MigrateSchemaExtensions(db *sql.DB) error {
	log.Println("📊 Running migration: Extended schema (notifications, audit, ZFS, API tokens)")

	statements := []struct {
		label string
		sql   string
	}{
		// ─── notification_settings ───────────────────────────────────────
		{"notification_settings", `
			CREATE TABLE IF NOT EXISTS notification_settings (
				id              INTEGER PRIMARY KEY AUTOINCREMENT,
				name            TEXT    NOT NULL,
				service_type    TEXT    NOT NULL,
				config_json     TEXT    NOT NULL,
				enabled         INTEGER DEFAULT 1,
				notify_on_critical INTEGER DEFAULT 1,
				notify_on_warning  INTEGER DEFAULT 0,
				notify_on_healthy  INTEGER DEFAULT 0,
				created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
			);`},
		{"notification_settings indexes", `
			CREATE INDEX IF NOT EXISTS idx_notif_enabled ON notification_settings(enabled);
			CREATE INDEX IF NOT EXISTS idx_notif_service ON notification_settings(service_type);`},

		// ─── audit_log ───────────────────────────────────────────────────
		{"audit_log", `
			CREATE TABLE IF NOT EXISTS audit_log (
				id          INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id     INTEGER,
				username    TEXT,
				action      TEXT    NOT NULL,
				resource    TEXT    NOT NULL,
				resource_id TEXT,
				details     TEXT,
				ip_address  TEXT,
				user_agent  TEXT,
				status      TEXT    DEFAULT 'success',
				created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
			);`},
		{"audit_log indexes", `
			CREATE INDEX IF NOT EXISTS idx_audit_user     ON audit_log(user_id);
			CREATE INDEX IF NOT EXISTS idx_audit_action   ON audit_log(action);
			CREATE INDEX IF NOT EXISTS idx_audit_resource ON audit_log(resource);
			CREATE INDEX IF NOT EXISTS idx_audit_created  ON audit_log(created_at);`},

		// ─── zfs_pools ───────────────────────────────────────────────────
		{"zfs_pools", `
			CREATE TABLE IF NOT EXISTS zfs_pools (
				id              INTEGER PRIMARY KEY AUTOINCREMENT,
				hostname        TEXT    NOT NULL,
				pool_name       TEXT    NOT NULL,
				pool_guid       TEXT,
				status          TEXT    NOT NULL DEFAULT 'UNKNOWN',
				health          TEXT    NOT NULL DEFAULT 'UNKNOWN',
				size_bytes      INTEGER DEFAULT 0,
				allocated_bytes INTEGER DEFAULT 0,
				free_bytes      INTEGER DEFAULT 0,
				fragmentation   INTEGER DEFAULT 0,
				capacity_pct    INTEGER DEFAULT 0,
				dedup_ratio     REAL    DEFAULT 1.0,
				altroot         TEXT,
				read_errors     INTEGER DEFAULT 0,
				write_errors    INTEGER DEFAULT 0,
				checksum_errors INTEGER DEFAULT 0,
				scan_function   TEXT,
				scan_state      TEXT,
				scan_progress   REAL    DEFAULT 0,
				last_scan_time  DATETIME,
				last_seen       DATETIME DEFAULT CURRENT_TIMESTAMP,
				created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(hostname, pool_name)
			);`},
		{"zfs_pools indexes", `
			CREATE INDEX IF NOT EXISTS idx_zfs_pools_hostname ON zfs_pools(hostname);
			CREATE INDEX IF NOT EXISTS idx_zfs_pools_status   ON zfs_pools(status);
			CREATE INDEX IF NOT EXISTS idx_zfs_pools_health   ON zfs_pools(health);`},

		// ─── zfs_scrub_history ────────────────────────────────────────────
		{"zfs_scrub_history", `
			CREATE TABLE IF NOT EXISTS zfs_scrub_history (
				id              INTEGER PRIMARY KEY AUTOINCREMENT,
				pool_id         INTEGER NOT NULL,
				hostname        TEXT    NOT NULL,
				pool_name       TEXT    NOT NULL,
				scan_type       TEXT    NOT NULL DEFAULT 'scrub',
				state           TEXT    NOT NULL,
				start_time      DATETIME NOT NULL,
				end_time        DATETIME,
				duration_secs   INTEGER,
				data_examined   INTEGER DEFAULT 0,
				data_total      INTEGER DEFAULT 0,
				errors_found    INTEGER DEFAULT 0,
				bytes_repaired  INTEGER DEFAULT 0,
				blocks_repaired INTEGER DEFAULT 0,
				progress_pct    REAL    DEFAULT 0,
				rate_bytes_sec  INTEGER DEFAULT 0,
				time_remaining  INTEGER,
				created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (pool_id) REFERENCES zfs_pools(id) ON DELETE CASCADE
			);`},
		{"zfs_scrub_history indexes", `
			CREATE INDEX IF NOT EXISTS idx_zfs_scrub_pool      ON zfs_scrub_history(pool_id);
			CREATE INDEX IF NOT EXISTS idx_zfs_scrub_hostname  ON zfs_scrub_history(hostname);
			CREATE INDEX IF NOT EXISTS idx_zfs_scrub_start     ON zfs_scrub_history(start_time);
			CREATE INDEX IF NOT EXISTS idx_zfs_scrub_state     ON zfs_scrub_history(state);`},

		// ─── zfs_pool_devices ────────────────────────────────────────────
		{"zfs_pool_devices", `
			CREATE TABLE IF NOT EXISTS zfs_pool_devices (
				id              INTEGER PRIMARY KEY AUTOINCREMENT,
				pool_id         INTEGER NOT NULL,
				hostname        TEXT    NOT NULL,
				pool_name       TEXT    NOT NULL,
				device_name     TEXT    NOT NULL,
				device_path     TEXT,
				device_guid     TEXT,
				serial_number   TEXT,
				vdev_type       TEXT    NOT NULL DEFAULT 'disk',
				vdev_parent     TEXT,
				vdev_index      INTEGER DEFAULT 0,
				state           TEXT    NOT NULL DEFAULT 'UNKNOWN',
				read_errors     INTEGER DEFAULT 0,
				write_errors    INTEGER DEFAULT 0,
				checksum_errors INTEGER DEFAULT 0,
				size_bytes      INTEGER DEFAULT 0,
				allocated_bytes INTEGER DEFAULT 0,
				is_spare        INTEGER DEFAULT 0,
				is_log          INTEGER DEFAULT 0,
				is_cache        INTEGER DEFAULT 0,
				is_replacing    INTEGER DEFAULT 0,
				last_seen       DATETIME DEFAULT CURRENT_TIMESTAMP,
				created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(pool_id, device_name),
				FOREIGN KEY (pool_id) REFERENCES zfs_pools(id) ON DELETE CASCADE
			);`},
		{"zfs_pool_devices indexes", `
			CREATE INDEX IF NOT EXISTS idx_zfs_dev_pool    ON zfs_pool_devices(pool_id);
			CREATE INDEX IF NOT EXISTS idx_zfs_dev_host    ON zfs_pool_devices(hostname);
			CREATE INDEX IF NOT EXISTS idx_zfs_dev_serial  ON zfs_pool_devices(serial_number);
			CREATE INDEX IF NOT EXISTS idx_zfs_dev_state   ON zfs_pool_devices(state);`},

		// ─── api_tokens ──────────────────────────────────────────────────
		{"api_tokens", `
			CREATE TABLE IF NOT EXISTS api_tokens (
				id              INTEGER PRIMARY KEY AUTOINCREMENT,
				user_id         INTEGER NOT NULL,
				token_hash      TEXT    NOT NULL UNIQUE,
				name            TEXT    NOT NULL,
				description     TEXT,
				permissions     TEXT    DEFAULT 'read',
				expiry_days     INTEGER DEFAULT 0,
				expires_at      DATETIME,
				enabled         INTEGER DEFAULT 1,
				last_used_at    DATETIME,
				last_used_ip    TEXT,
				use_count       INTEGER DEFAULT 0,
				created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
			);`},
		{"api_tokens indexes", `
			CREATE INDEX IF NOT EXISTS idx_api_tokens_user    ON api_tokens(user_id);
			CREATE INDEX IF NOT EXISTS idx_api_tokens_hash    ON api_tokens(token_hash);
			CREATE INDEX IF NOT EXISTS idx_api_tokens_enabled ON api_tokens(enabled);
			CREATE INDEX IF NOT EXISTS idx_api_tokens_expiry  ON api_tokens(expires_at);`},

		// ─── notification_history ────────────────────────────────────────
		{"notification_history", `
			CREATE TABLE IF NOT EXISTS notification_history (
				id              INTEGER PRIMARY KEY AUTOINCREMENT,
				setting_id      INTEGER,
				event_type      TEXT    NOT NULL,
				hostname        TEXT,
				serial_number   TEXT,
				message         TEXT    NOT NULL,
				status          TEXT    NOT NULL DEFAULT 'pending',
				error_message   TEXT,
				sent_at         DATETIME,
				created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (setting_id) REFERENCES notification_settings(id) ON DELETE SET NULL
			);`},
		{"notification_history indexes", `
			CREATE INDEX IF NOT EXISTS idx_notif_hist_setting ON notification_history(setting_id);
			CREATE INDEX IF NOT EXISTS idx_notif_hist_status  ON notification_history(status);
			CREATE INDEX IF NOT EXISTS idx_notif_hist_created ON notification_history(created_at);`},
	}

	for _, s := range statements {
		if _, err := db.Exec(s.sql); err != nil {
			return fmt.Errorf("migration failed at [%s]: %w", s.label, err)
		}
		log.Printf("  ✓ %s", s.label)
	}

	log.Println("📊 Migration completed: Extended schema ready")
	return nil
}
