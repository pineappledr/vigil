package db

import (
	"database/sql"
	"log"
)

// MigrateSmartAttributes adds tables for enhanced S.M.A.R.T. attribute tracking
func MigrateSmartAttributes(db *sql.DB) error {
	log.Println("📊 Running migration: Enhanced S.M.A.R.T. attributes tracking")

	schema := `
	-- S.M.A.R.T. Attributes Historical Tracking
	CREATE TABLE IF NOT EXISTS smart_attributes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hostname TEXT NOT NULL,
		serial_number TEXT NOT NULL,
		device_name TEXT NOT NULL,
		attribute_id INTEGER NOT NULL,
		attribute_name TEXT NOT NULL,
		value INTEGER,
		worst INTEGER,
		threshold INTEGER,
		raw_value INTEGER,
		flags TEXT,
		when_failed TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(hostname, serial_number, attribute_id, timestamp)
	);
	CREATE INDEX IF NOT EXISTS idx_smart_attrs_hostname ON smart_attributes(hostname);
	CREATE INDEX IF NOT EXISTS idx_smart_attrs_serial ON smart_attributes(serial_number);
	CREATE INDEX IF NOT EXISTS idx_smart_attrs_timestamp ON smart_attributes(timestamp);
	CREATE INDEX IF NOT EXISTS idx_smart_attrs_attr_id ON smart_attributes(attribute_id);
	
	-- Critical S.M.A.R.T. Attributes Reference (for quick lookups)
	-- This helps identify which attributes are most important to track
	CREATE TABLE IF NOT EXISTS critical_smart_attributes (
		attribute_id INTEGER PRIMARY KEY,
		attribute_name TEXT NOT NULL,
		description TEXT,
		drive_type TEXT, -- 'HDD', 'SSD', or 'BOTH'
		severity TEXT, -- 'CRITICAL', 'WARNING', 'INFO'
		failure_threshold INTEGER, -- Raw value that indicates failure
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);
	
	-- Pre-populate critical S.M.A.R.T. attributes
	INSERT OR IGNORE INTO critical_smart_attributes (attribute_id, attribute_name, description, drive_type, severity, failure_threshold) VALUES
		(1, 'Read Error Rate', 'Rate of hardware read errors', 'HDD', 'WARNING', 0),
		(5, 'Reallocated Sectors Count', 'Count of reallocated sectors', 'BOTH', 'CRITICAL', 0),
		(7, 'Seek Error Rate', 'Rate of seek errors', 'HDD', 'WARNING', 0),
		(9, 'Power-On Hours', 'Count of hours in power-on state', 'BOTH', 'INFO', NULL),
		(10, 'Spin Retry Count', 'Count of retry of spin start attempts', 'HDD', 'CRITICAL', 0),
		(11, 'Calibration Retry Count', 'Count of calibration retries', 'HDD', 'WARNING', 0),
		(12, 'Power Cycle Count', 'Count of full hard disk power on/off cycles', 'BOTH', 'INFO', NULL),
		(187, 'Reported Uncorrectable Errors', 'Count of uncorrectable errors', 'BOTH', 'CRITICAL', 0),
		(188, 'Command Timeout', 'Count of aborted operations due to timeout', 'BOTH', 'CRITICAL', 0),
		(193, 'Load Cycle Count', 'Count of head load/unload cycles', 'HDD', 'INFO', NULL),
		(194, 'Temperature Celsius', 'Current internal temperature', 'BOTH', 'WARNING', 60),
		(196, 'Reallocation Event Count', 'Count of remap operations', 'BOTH', 'CRITICAL', 0),
		(197, 'Current Pending Sector Count', 'Count of unstable sectors', 'BOTH', 'CRITICAL', 0),
		(198, 'Offline Uncorrectable Sector Count', 'Count of uncorrectable errors', 'BOTH', 'CRITICAL', 0),
		(199, 'UltraDMA CRC Error Count', 'Count of CRC errors', 'BOTH', 'WARNING', 0),
		(200, 'Multi-Zone Error Rate', 'Count of errors while writing sectors', 'HDD', 'WARNING', 0),
		(201, 'Soft Read Error Rate', 'Rate of off-track errors', 'HDD', 'WARNING', 0),
		
		-- SSD Specific Attributes
		(177, 'Wear Leveling Count', 'SSD wear leveling status', 'SSD', 'WARNING', 10),
		(179, 'Used Reserved Block Count', 'Count of used reserved blocks', 'SSD', 'WARNING', 0),
		(181, 'Program Fail Count', 'Total count of Flash program failures', 'SSD', 'CRITICAL', 0),
		(182, 'Erase Fail Count', 'Count of Flash erase failures', 'SSD', 'CRITICAL', 0),
		(183, 'Runtime Bad Block', 'Total count of bad blocks', 'SSD', 'CRITICAL', 0),
		(184, 'End-to-End Error', 'Count of parity errors', 'SSD', 'CRITICAL', 0),
		(195, 'Hardware ECC Recovered', 'ECC error correction count', 'SSD', 'WARNING', NULL),
		(202, 'Data Address Mark Errors', 'Count of data address mark errors', 'SSD', 'WARNING', 0),
		(232, 'Available Reserved Space', 'Available reserved space percentage', 'SSD', 'CRITICAL', 10),
		(233, 'Media Wearout Indicator', 'SSD wear indicator', 'SSD', 'WARNING', 10),
		(241, 'Total LBAs Written', 'Total count of LBAs written', 'SSD', 'INFO', NULL),
		(242, 'Total LBAs Read', 'Total count of LBAs read', 'SSD', 'INFO', NULL);
	`

	if _, err := db.Exec(schema); err != nil {
		return err
	}

	log.Println("✓ Migration completed: S.M.A.R.T. attributes tracking enabled")
	return nil
}
