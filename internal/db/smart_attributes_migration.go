package db

import (
	"database/sql"
	"fmt"
	"log"
)

// MigrateSmartAttributes creates all tables required for Phase 1.2 of the
// health monitoring roadmap: S.M.A.R.T. tracking, temperature history,
// and critical attribute definitions.
func MigrateSmartAttributes(db *sql.DB) error {
	log.Println("📊 Running migration: Phase 1.2 SMART attributes schema")

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
				drive_type        TEXT,    -- 'HDD', 'SSD', 'BOTH', 'NVMe'
				severity          TEXT,    -- 'CRITICAL', 'WARNING', 'INFO'
				failure_threshold INTEGER, -- raw_value that signals a problem (NULL = N/A)
				higher_is_better  INTEGER DEFAULT 0,
				created_at        DATETIME DEFAULT CURRENT_TIMESTAMP
			);`},
		{"critical_smart_attributes seed", `
			INSERT OR IGNORE INTO critical_smart_attributes
				(attribute_id, attribute_name, description, drive_type, severity, failure_threshold, higher_is_better)
			VALUES
				-- Critical Failure Indicators
				(5,   'Reallocated Sectors Count',          'Count of reallocated sectors. When bad, remapped to spare area.',        'BOTH', 'CRITICAL', 0, 0),
				(10,  'Spin Retry Count',                   'Count of retry attempts to spin up. Indicates motor/bearing issues.',    'HDD',  'CRITICAL', 0, 0),
				(196, 'Reallocation Event Count',           'Count of remap operations from bad to spare sectors.',                   'BOTH', 'CRITICAL', 0, 0),
				(197, 'Current Pending Sector Count',       'Count of unstable sectors waiting to be remapped.',                      'BOTH', 'CRITICAL', 0, 0),
				(198, 'Offline Uncorrectable Sector Count', 'Count of uncorrectable errors found offline. Surface damage indicator.', 'BOTH', 'CRITICAL', 0, 0),
				(187, 'Reported Uncorrectable Errors',      'Count of uncorrectable errors reported to the host.',                    'BOTH', 'CRITICAL', 0, 0),
				(188, 'Command Timeout',                    'Count of aborted operations due to timeout.',                            'BOTH', 'CRITICAL', 0, 0),
				
				-- SSD-Specific Critical
				(181, 'Program Fail Count',                 'Count of flash program (write) failures. NAND wear indicator.',          'SSD',  'CRITICAL', 0, 0),
				(182, 'Erase Fail Count',                   'Count of flash erase failures. NAND wear indicator.',                    'SSD',  'CRITICAL', 0, 0),
				(183, 'Runtime Bad Block',                  'Count of bad blocks detected during operation.',                         'SSD',  'CRITICAL', 0, 0),
				(184, 'End-to-End Error',                   'Count of parity errors in data path.',                                   'SSD',  'CRITICAL', 0, 0),
				(232, 'Available Reserved Space',           'Percentage of reserved space remaining for bad block replacement.',      'SSD',  'CRITICAL', 10, 1),
				
				-- Warning Indicators
				(1,   'Read Error Rate',                    'Rate of hardware read errors. Vendor-specific interpretation.',          'HDD',  'WARNING',  NULL, 0),
				(7,   'Seek Error Rate',                    'Rate of seek errors of the magnetic heads.',                             'HDD',  'WARNING',  NULL, 0),
				(11,  'Calibration Retry Count',            'Count of recalibration retries.',                                        'HDD',  'WARNING',  0, 0),
				(177, 'Wear Leveling Count',                'SSD wear leveling status (percentage remaining).',                       'SSD',  'WARNING',  10, 1),
				(179, 'Used Reserved Block Count',          'Count of used reserved blocks for bad block replacement.',               'SSD',  'WARNING',  0, 0),
				(199, 'UltraDMA CRC Error Count',           'Count of CRC errors during Ultra DMA transfers. Often cable issues.',   'BOTH', 'WARNING',  0, 0),
				(200, 'Multi-Zone Error Rate',              'Count of errors while writing sectors.',                                 'HDD',  'WARNING',  NULL, 0),
				(201, 'Soft Read Error Rate',               'Count of off-track read errors.',                                        'HDD',  'WARNING',  NULL, 0),
				(233, 'Media Wearout Indicator',            'SSD wear indicator (percentage used).',                                  'SSD',  'WARNING',  90, 0),
				
				-- Temperature
				(194, 'Temperature Celsius',                'Current internal temperature in Celsius.',                               'BOTH', 'WARNING',  60, 0),
				(190, 'Airflow Temperature',                'Temperature of air flowing across the drive.',                           'BOTH', 'WARNING',  60, 0),
				
				-- Informational
				(9,   'Power-On Hours',                     'Total hours the drive has been powered on.',                             'BOTH', 'INFO',     NULL, 0),
				(12,  'Power Cycle Count',                  'Count of full power on/off cycles.',                                     'BOTH', 'INFO',     NULL, 0),
				(193, 'Load Cycle Count',                   'Count of head load/unload cycles.',                                      'HDD',  'INFO',     NULL, 0),
				(195, 'Hardware ECC Recovered',             'Count of ECC error corrections.',                                        'SSD',  'WARNING',  NULL, 0),
				(202, 'Data Address Mark Errors',           'Count of data address mark errors.',                                     'SSD',  'WARNING',  NULL, 0),
				(241, 'Total LBAs Written',                 'Total count of logical block addresses written.',                        'SSD',  'INFO',     NULL, 0),
				(242, 'Total LBAs Read',                    'Total count of logical block addresses read.',                           'SSD',  'INFO',     NULL, 0);`},

		// ─── 2. temperature_history ──────────────────────────────────────
		{"temperature_history", `
			CREATE TABLE IF NOT EXISTS temperature_history (
				id            INTEGER  PRIMARY KEY AUTOINCREMENT,
				hostname      TEXT     NOT NULL,
				serial_number TEXT     NOT NULL,
				temperature   INTEGER  NOT NULL,
				timestamp     DATETIME DEFAULT CURRENT_TIMESTAMP,
				UNIQUE(hostname, serial_number, timestamp)
			);`},
		{"temperature_history indexes", `
			CREATE INDEX IF NOT EXISTS idx_temp_hostname  ON temperature_history(hostname);
			CREATE INDEX IF NOT EXISTS idx_temp_serial    ON temperature_history(serial_number);
			CREATE INDEX IF NOT EXISTS idx_temp_timestamp ON temperature_history(timestamp);`},

		// ─── 3. drive_health_snapshots (for tracking health over time) ───
		{"drive_health_snapshots", `
			CREATE TABLE IF NOT EXISTS drive_health_snapshots (
				id             INTEGER  PRIMARY KEY AUTOINCREMENT,
				hostname       TEXT     NOT NULL,
				serial_number  TEXT     NOT NULL,
				model_name     TEXT,
				drive_type     TEXT,
				overall_health TEXT     NOT NULL,
				smart_passed   INTEGER  DEFAULT 1,
				critical_count INTEGER  DEFAULT 0,
				warning_count  INTEGER  DEFAULT 0,
				issues_json    TEXT,
				timestamp      DATETIME DEFAULT CURRENT_TIMESTAMP
			);`},
		{"drive_health_snapshots indexes", `
			CREATE INDEX IF NOT EXISTS idx_health_hostname  ON drive_health_snapshots(hostname);
			CREATE INDEX IF NOT EXISTS idx_health_serial    ON drive_health_snapshots(serial_number);
			CREATE INDEX IF NOT EXISTS idx_health_timestamp ON drive_health_snapshots(timestamp);
			CREATE INDEX IF NOT EXISTS idx_health_status    ON drive_health_snapshots(overall_health);`},
	}

	for _, s := range statements {
		if _, err := db.Exec(s.sql); err != nil {
			return fmt.Errorf("migration failed at [%s]: %w", s.label, err)
		}
		log.Printf("  ✓ %s", s.label)
	}

	log.Println("📊 Migration completed: Phase 1.2 tables ready")
	return nil
}
