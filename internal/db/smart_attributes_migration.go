package db

import (
	"database/sql"
	"fmt"
	"log"
)

// MigrateSmartAttributes creates all tables required for Phase 1.1 of the
// health monitoring roadmap: S.M.A.R.T. tracking, temperature history,
// performance metrics, test scheduling/results, notifications, audit logging,
// ZFS pool monitoring, and API token management.
func MigrateSmartAttributes(db *sql.DB) error {
	log.Println("📊 Running migration: Phase 1.1 database schema")

	// Each statement is executed individually so that a failure in one block
	// does not prevent the others from running, and the log pinpoints exactly
	// which table caused a problem.
	statements := []struct {
		label string
		sql   string
	}{
		// ─── 1. smart_attributes ─────────────────────────────────────────
		{"smart_attributes", `
			CREATE TABLE IF NOT EXISTS smart_attributes (
				id             INTEGER PRIMARY KEY AUTOINCREMENT,
				hostname       TEXT    NOT NULL,
				serial_number  TEXT    NOT NULL,
				device_name    TEXT    NOT NULL,
				attribute_id   INTEGER NOT NULL,
				attribute_name TEXT    NOT NULL,
				value          INTEGER,
				worst          INTEGER,
				threshold      INTEGER,
				raw_value      INTEGER,
				flags          TEXT,
				when_failed    TEXT,
				timestamp      DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(hostname, serial_number, attribute_id, timestamp)
			);`},
		{"smart_attributes indexes", `
			CREATE INDEX IF NOT EXISTS idx_smart_attrs_hostname  ON smart_attributes(hostname);
			CREATE INDEX IF NOT EXISTS idx_smart_attrs_serial    ON smart_attributes(serial_number);
			CREATE INDEX IF NOT EXISTS idx_smart_attrs_timestamp ON smart_attributes(timestamp);
			CREATE INDEX IF NOT EXISTS idx_smart_attrs_attr_id   ON smart_attributes(attribute_id);`},

		// ─── critical_smart_attributes (reference / seed data) ───────────
		{"critical_smart_attributes", `
			CREATE TABLE IF NOT EXISTS critical_smart_attributes (
				attribute_id      INTEGER PRIMARY KEY,
				attribute_name    TEXT    NOT NULL,
				description       TEXT,
				drive_type        TEXT,    -- 'HDD', 'SSD', or 'BOTH'
				severity          TEXT,    -- 'CRITICAL', 'WARNING', 'INFO'
				failure_threshold INTEGER, -- raw_value that signals a problem (NULL = N/A)
				created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
			);`},
		{"critical_smart_attributes seed", `
			INSERT OR IGNORE INTO critical_smart_attributes
				(attribute_id, attribute_name, description, drive_type, severity, failure_threshold)
			VALUES
				(1,   'Read Error Rate',                    'Rate of hardware read errors',                          'HDD',  'WARNING',  0),
				(5,   'Reallocated Sectors Count',          'Count of reallocated sectors',                          'BOTH', 'CRITICAL', 0),
				(7,   'Seek Error Rate',                    'Rate of seek errors',                                   'HDD',  'WARNING',  0),
				(9,   'Power-On Hours',                     'Count of hours in power-on state',                      'BOTH', 'INFO',     NULL),
				(10,  'Spin Retry Count',                   'Count of retry of spin start attempts',                 'HDD',  'CRITICAL', 0),
				(11,  'Calibration Retry Count',            'Count of calibration retries',                          'HDD',  'WARNING',  0),
				(12,  'Power Cycle Count',                  'Count of full hard disk power on/off cycles',          'BOTH', 'INFO',     NULL),
				(177, 'Wear Leveling Count',                'SSD wear leveling status',                              'SSD',  'WARNING',  10),
				(179, 'Used Reserved Block Count',          'Count of used reserved blocks',                         'SSD',  'WARNING',  0),
				(181, 'Program Fail Count',                 'Total count of Flash program failures',                 'SSD',  'CRITICAL', 0),
				(182, 'Erase Fail Count',                   'Count of Flash erase failures',                         'SSD',  'CRITICAL', 0),
				(183, 'Runtime Bad Block',                  'Total count of bad blocks',                             'SSD',  'CRITICAL', 0),
				(184, 'End-to-End Error',                   'Count of parity errors',                                'SSD',  'CRITICAL', 0),
				(187, 'Reported Uncorrectable Errors',      'Count of uncorrectable errors',                         'BOTH', 'CRITICAL', 0),
				(188, 'Command Timeout',                    'Count of aborted operations due to timeout',           'BOTH', 'CRITICAL', 0),
				(193, 'Load Cycle Count',                   'Count of head load/unload cycles',                     'HDD',  'INFO',     NULL),
				(194, 'Temperature Celsius',                'Current internal temperature',                          'BOTH', 'WARNING',  60),
				(195, 'Hardware ECC Recovered',             'ECC error correction count',                            'SSD',  'WARNING',  NULL),
				(196, 'Reallocation Event Count',           'Count of remap operations',                             'BOTH', 'CRITICAL', 0),
				(197, 'Current Pending Sector Count',       'Count of unstable sectors',                             'BOTH', 'CRITICAL', 0),
				(198, 'Offline Uncorrectable Sector Count', 'Count of uncorrectable errors detected offline',       'BOTH', 'CRITICAL', 0),
				(199, 'UltraDMA CRC Error Count',           'Count of CRC errors during transfer',                  'BOTH', 'WARNING',  0),
				(200, 'Multi-Zone Error Rate',              'Count of errors while writing sectors',                 'HDD',  'WARNING',  0),
				(201, 'Soft Read Error Rate',               'Rate of off-track errors',                              'HDD',  'WARNING',  0),
				(202, 'Data Address Mark Errors',           'Count of data address mark errors',                    'SSD',  'WARNING',  0),
				(232, 'Available Reserved Space',           'Available reserved space percentage',                   'SSD',  'CRITICAL', 10),
				(233, 'Media Wearout Indicator',            'SSD wear indicator (percentage used)',                  'SSD',  'WARNING',  10),
				(241, 'Total LBAs Written',                 'Total count of LBAs written',                           'SSD',  'INFO',     NULL),
				(242, 'Total LBAs Read',                    'Total count of LBAs read',                              'SSD',  'INFO',     NULL);`},

		// ─── 2. temperature_history ──────────────────────────────────────
		{"temperature_history", `
			CREATE TABLE IF NOT EXISTS temperature_history (
				id            INTEGER  PRIMARY KEY AUTOINCREMENT,
				hostname      TEXT     NOT NULL,
				serial_number TEXT     NOT NULL,
				temperature   INTEGER  NOT NULL,  -- degrees Celsius
				timestamp     DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(hostname, serial_number, timestamp)
			);`},
		{"temperature_history indexes", `
			CREATE INDEX IF NOT EXISTS idx_temp_hostname  ON temperature_history(hostname);
			CREATE INDEX IF NOT EXISTS idx_temp_serial    ON temperature_history(serial_number);
			CREATE INDEX IF NOT EXISTS idx_temp_timestamp ON temperature_history(timestamp);`},

		// ─── 3. performance_metrics ──────────────────────────────────────
		{"performance_metrics", `
			CREATE TABLE IF NOT EXISTS performance_metrics (
				id                INTEGER  PRIMARY KEY AUTOINCREMENT,
				hostname          TEXT     NOT NULL,
				serial_number     TEXT     NOT NULL,
				device_name       TEXT     NOT NULL,
				read_ios          INTEGER  DEFAULT 0,   -- I/O operations (reads)
				write_ios         INTEGER  DEFAULT 0,   -- I/O operations (writes)
				read_bytes        INTEGER  DEFAULT 0,   -- bytes read
				write_bytes       INTEGER  DEFAULT 0,   -- bytes written
				read_latency_ms   REAL     DEFAULT 0.0, -- average read latency in ms
				write_latency_ms  REAL     DEFAULT 0.0, -- average write latency in ms
				timestamp         DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(hostname, serial_number, timestamp)
			);`},
		{"performance_metrics indexes", `
			CREATE INDEX IF NOT EXISTS idx_perf_hostname  ON performance_metrics(hostname);
			CREATE INDEX IF NOT EXISTS idx_perf_serial    ON performance_metrics(serial_number);
			CREATE INDEX IF NOT EXISTS idx_perf_timestamp ON performance_metrics(timestamp);`},

		// ─── 4. test_schedules ───────────────────────────────────────────
		{"test_schedules", `
			CREATE TABLE IF NOT EXISTS test_schedules (
				id            INTEGER  PRIMARY KEY AUTOINCREMENT,
				hostname      TEXT     NOT NULL,
				serial_number TEXT     NOT NULL,
				test_type     TEXT     NOT NULL,  -- 'short', 'extended', 'conveyance'
				schedule      TEXT     NOT NULL,  -- cron expression (e.g. '0 2 * * 0')
				enabled       INTEGER  NOT NULL DEFAULT 1,
				created_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at    DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(hostname, serial_number, test_type)
			);`},
		{"test_schedules indexes", `
			CREATE INDEX IF NOT EXISTS idx_sched_hostname  ON test_schedules(hostname);
			CREATE INDEX IF NOT EXISTS idx_sched_serial    ON test_schedules(serial_number);`},

		// ─── 5. test_results ─────────────────────────────────────────────
		{"test_results", `
			CREATE TABLE IF NOT EXISTS test_results (
				id            INTEGER  PRIMARY KEY AUTOINCREMENT,
				hostname      TEXT     NOT NULL,
				serial_number TEXT     NOT NULL,
				test_type     TEXT     NOT NULL,  -- 'short', 'extended', 'conveyance'
				status        TEXT     NOT NULL,  -- 'pending', 'running', 'passed', 'failed', 'aborted'
				started_at    DATETIME,
				finished_at   DATETIME,
				result_details TEXT,              -- JSON: smartctl output, error messages, etc.
				created_at    DATETIME DEFAULT CURRENT_TIMESTAMP
			);`},
		{"test_results indexes", `
			CREATE INDEX IF NOT EXISTS idx_tresult_hostname  ON test_results(hostname);
			CREATE INDEX IF NOT EXISTS idx_tresult_serial    ON test_results(serial_number);
			CREATE INDEX IF NOT EXISTS idx_tresult_status    ON test_results(status);
			CREATE INDEX IF NOT EXISTS idx_tresult_created   ON test_results(created_at);`},

		// ─── 6. notification_settings ────────────────────────────────────
		{"notification_settings", `
			CREATE TABLE IF NOT EXISTS notification_settings (
				id           INTEGER  PRIMARY KEY AUTOINCREMENT,
				service_type TEXT     NOT NULL,  -- 'email', 'webhook', 'shoutrrr'
				config       TEXT     NOT NULL,  -- JSON: service-specific configuration
				enabled      INTEGER  NOT NULL DEFAULT 1,
				created_at   DATETIME DEFAULT CURRENT_TIMESTAMP,
				updated_at   DATETIME DEFAULT CURRENT_TIMESTAMP
			);`},

		// ─── 7. audit_log ────────────────────────────────────────────────
		{"audit_log", `
			CREATE TABLE IF NOT EXISTS audit_log (
				id         INTEGER  PRIMARY KEY AUTOINCREMENT,
				user_id    INTEGER  NOT NULL,
				action     TEXT     NOT NULL,  -- e.g. 'login', 'logout', 'alias_update', 'setting_change'
				details    TEXT,               -- JSON: contextual information about the action
				ip_address TEXT,
				timestamp  DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
			);`},
		{"audit_log indexes", `
			CREATE INDEX IF NOT EXISTS idx_audit_user      ON audit_log(user_id);
			CREATE INDEX IF NOT EXISTS idx_audit_action    ON audit_log(action);
			CREATE INDEX IF NOT EXISTS idx_audit_timestamp ON audit_log(timestamp);`},

		// ─── 8. zfs_pools ────────────────────────────────────────────────
		{"zfs_pools", `
			CREATE TABLE IF NOT EXISTS zfs_pools (
				id          INTEGER  PRIMARY KEY AUTOINCREMENT,
				hostname    TEXT     NOT NULL,
				pool_name   TEXT     NOT NULL,
				status      TEXT     NOT NULL,  -- 'online', 'degraded', 'faulted', 'offline', 'removed'
				health      TEXT     NOT NULL,  -- 'ONLINE', 'DEGRADED', 'FAULTED', 'OFFLINE', 'REMOVED'
				total_bytes INTEGER  DEFAULT 0,
				used_bytes  INTEGER  DEFAULT 0,
				free_bytes  INTEGER  DEFAULT 0,
				timestamp   DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(hostname, pool_name)
			);`},
		{"zfs_pools indexes", `
			CREATE INDEX IF NOT EXISTS idx_zpool_hostname ON zfs_pools(hostname);
			CREATE INDEX IF NOT EXISTS idx_zpool_name     ON zfs_pools(pool_name);`},

		// ─── 9. zfs_scrub_history ────────────────────────────────────────
		{"zfs_scrub_history", `
			CREATE TABLE IF NOT EXISTS zfs_scrub_history (
				id                  INTEGER  PRIMARY KEY AUTOINCREMENT,
				hostname            TEXT     NOT NULL,
				pool_name           TEXT     NOT NULL,
				status              TEXT     NOT NULL,  -- 'running', 'completed', 'interrupted', 'failed'
				started_at          DATETIME,
				finished_at         DATETIME,
				errors_found        INTEGER  DEFAULT 0,
				data_examined_bytes INTEGER  DEFAULT 0,
				created_at          DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (hostname, pool_name) REFERENCES zfs_pools(hostname, pool_name)
			);`},
		{"zfs_scrub_history indexes", `
			CREATE INDEX IF NOT EXISTS idx_zscrub_hostname  ON zfs_scrub_history(hostname);
			CREATE INDEX IF NOT EXISTS idx_zscrub_pool      ON zfs_scrub_history(pool_name);
			CREATE INDEX IF NOT EXISTS idx_zscrub_created   ON zfs_scrub_history(created_at);`},

		// ─── 10. zfs_pool_devices ────────────────────────────────────────
		{"zfs_pool_devices", `
			CREATE TABLE IF NOT EXISTS zfs_pool_devices (
				id            INTEGER  PRIMARY KEY AUTOINCREMENT,
				hostname      TEXT     NOT NULL,
				pool_name     TEXT     NOT NULL,
				vdev_name     TEXT     NOT NULL,  -- logical vdev label
				vdev_type     TEXT     NOT NULL,  -- 'mirror', 'raidz1', 'raidz2', 'raidz3', 'spare', 'log', 'cache', 'single'
				device_name   TEXT     NOT NULL,  -- e.g. '/dev/sda1'
				serial_number TEXT,               -- cross-reference to smart_attributes
				state         TEXT     NOT NULL,  -- 'online', 'degraded', 'offline', 'removed', 'faulted'
				timestamp     DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (hostname, pool_name) REFERENCES zfs_pools(hostname, pool_name)
			);`},
		{"zfs_pool_devices indexes", `
			CREATE INDEX IF NOT EXISTS idx_zpdev_hostname  ON zfs_pool_devices(hostname);
			CREATE INDEX IF NOT EXISTS idx_zpdev_pool      ON zfs_pool_devices(pool_name);
			CREATE INDEX IF NOT EXISTS idx_zpdev_serial    ON zfs_pool_devices(serial_number);`},

		// ─── 11. api_tokens ──────────────────────────────────────────────
		{"api_tokens", `
			CREATE TABLE IF NOT EXISTS api_tokens (
				id               INTEGER  PRIMARY KEY AUTOINCREMENT,
				user_id          INTEGER  NOT NULL,
				token_hash       TEXT     NOT NULL UNIQUE,  -- SHA-256 of the raw token
				name             TEXT     NOT NULL,          -- human-readable label
				description      TEXT,                       -- optional notes
				expiry_days      INTEGER  NOT NULL DEFAULT 0, -- 0 = never, 30, 60, 90
				created_at       DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
				last_used_at     DATETIME,
				enabled          INTEGER  NOT NULL DEFAULT 1,
				permissions_scope TEXT    NOT NULL DEFAULT 'read', -- 'read', 'read,write', 'all'
				FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
			);`},
		{"api_tokens indexes", `
			CREATE INDEX IF NOT EXISTS idx_apitoken_user    ON api_tokens(user_id);
			CREATE INDEX IF NOT EXISTS idx_apitoken_hash    ON api_tokens(token_hash);
			CREATE INDEX IF NOT EXISTS idx_apitoken_enabled ON api_tokens(enabled);`},
	}

	for _, s := range statements {
		if _, err := db.Exec(s.sql); err != nil {
			return fmt.Errorf("migration failed at [%s]: %w", s.label, err)
		}
		log.Printf("  ✓ %s", s.label)
	}

	log.Println("📊 Migration completed: all Phase 1.1 tables ready")
	return nil
}
