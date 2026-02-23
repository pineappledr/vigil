package temperature

import (
	"database/sql"
	"fmt"
	"math"
	"time"

	"vigil/internal/settings"
)

// InitTemperatureSpikesTable creates the temperature_spikes table
func InitTemperatureSpikesTable(db *sql.DB) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS temperature_spikes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hostname TEXT NOT NULL,
		serial_number TEXT NOT NULL,
		start_time DATETIME NOT NULL,
		end_time DATETIME NOT NULL,
		start_temp INTEGER NOT NULL,
		end_temp INTEGER NOT NULL,
		change_degrees INTEGER NOT NULL,
		rate_per_minute REAL,
		direction TEXT NOT NULL,
		acknowledged INTEGER DEFAULT 0,
		acknowledged_by TEXT,
		acknowledged_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_spikes_host_serial
		ON temperature_spikes(hostname, serial_number);
	CREATE INDEX IF NOT EXISTS idx_spikes_time
		ON temperature_spikes(start_time);
	CREATE INDEX IF NOT EXISTS idx_spikes_acknowledged
		ON temperature_spikes(acknowledged);
	CREATE INDEX IF NOT EXISTS idx_spikes_created
		ON temperature_spikes(created_at);
	`

	_, err := db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create temperature_spikes table: %w", err)
	}

	return nil
}

// RecordSpike saves a detected temperature spike to the database
func RecordSpike(db *sql.DB, spike *TemperatureSpike) error {
	query := `
		INSERT INTO temperature_spikes (
			hostname, serial_number, start_time, end_time,
			start_temp, end_temp, change_degrees, rate_per_minute, direction
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	result, err := db.Exec(query,
		spike.Hostname,
		spike.SerialNumber,
		spike.StartTime,
		spike.EndTime,
		spike.StartTemp,
		spike.EndTemp,
		spike.Change,
		spike.RatePerMin,
		spike.Direction,
	)
	if err != nil {
		return fmt.Errorf("failed to record spike: %w", err)
	}

	id, err := result.LastInsertId()
	if err == nil {
		spike.ID = id
	}

	return nil
}

// GetRecentSpikes retrieves recent spikes for a drive
func GetRecentSpikes(db *sql.DB, hostname, serial string, limit int) ([]TemperatureSpike, error) {
	query := `
		SELECT id, hostname, serial_number, start_time, end_time,
			   start_temp, end_temp, change_degrees, rate_per_minute,
			   direction, acknowledged, COALESCE(acknowledged_by, ''),
			   acknowledged_at, created_at
		FROM temperature_spikes
		WHERE hostname = ? AND serial_number = ?
		ORDER BY start_time DESC
		LIMIT ?
	`

	return querySpikes(db, query, hostname, serial, limit)
}

// GetAllRecentSpikes retrieves recent spikes across all drives
func GetAllRecentSpikes(db *sql.DB, limit int) ([]TemperatureSpike, error) {
	query := `
		SELECT id, hostname, serial_number, start_time, end_time,
			   start_temp, end_temp, change_degrees, rate_per_minute,
			   direction, acknowledged, COALESCE(acknowledged_by, ''),
			   acknowledged_at, created_at
		FROM temperature_spikes
		ORDER BY start_time DESC
		LIMIT ?
	`

	return querySpikes(db, query, limit)
}

// GetUnacknowledgedSpikes retrieves all unacknowledged spikes
func GetUnacknowledgedSpikes(db *sql.DB) ([]TemperatureSpike, error) {
	query := `
		SELECT id, hostname, serial_number, start_time, end_time,
			   start_temp, end_temp, change_degrees, rate_per_minute,
			   direction, acknowledged, COALESCE(acknowledged_by, ''),
			   acknowledged_at, created_at
		FROM temperature_spikes
		WHERE acknowledged = 0
		ORDER BY start_time DESC
	`

	return querySpikes(db, query)
}

// GetSpikeByID retrieves a single spike by ID
func GetSpikeByID(db *sql.DB, id int64) (*TemperatureSpike, error) {
	query := `
		SELECT id, hostname, serial_number, start_time, end_time,
			   start_temp, end_temp, change_degrees, rate_per_minute,
			   direction, acknowledged, COALESCE(acknowledged_by, ''),
			   acknowledged_at, created_at
		FROM temperature_spikes
		WHERE id = ?
	`

	var spike TemperatureSpike
	var ackAt sql.NullTime

	err := db.QueryRow(query, id).Scan(
		&spike.ID, &spike.Hostname, &spike.SerialNumber,
		&spike.StartTime, &spike.EndTime, &spike.StartTemp, &spike.EndTemp,
		&spike.Change, &spike.RatePerMin, &spike.Direction,
		&spike.Acknowledged, &spike.AcknowledgedBy, &ackAt, &spike.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get spike: %w", err)
	}

	if ackAt.Valid {
		spike.AcknowledgedAt = ackAt.Time
	}

	// Get drive info
	driveInfo, _ := getDriveInfo(db, spike.Hostname, spike.SerialNumber)
	if driveInfo != nil {
		spike.DeviceName = driveInfo.DeviceName
		spike.Model = driveInfo.Model
	}

	return &spike, nil
}

// AcknowledgeSpike marks a spike as acknowledged
func AcknowledgeSpike(db *sql.DB, id int64, username string) error {
	query := `
		UPDATE temperature_spikes
		SET acknowledged = 1, acknowledged_by = ?, acknowledged_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	result, err := db.Exec(query, username, id)
	if err != nil {
		return fmt.Errorf("failed to acknowledge spike: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("spike not found")
	}

	return nil
}

// DeleteSpike removes a spike record
func DeleteSpike(db *sql.DB, id int64) error {
	result, err := db.Exec("DELETE FROM temperature_spikes WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete spike: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("spike not found")
	}

	return nil
}

// GetSpikeSummary returns counts of spikes by status
func GetSpikeSummary(db *sql.DB) (*SpikeSummary, error) {
	query := `
		SELECT
			COUNT(*) as total,
			SUM(CASE WHEN acknowledged = 0 THEN 1 ELSE 0 END) as unacknowledged,
			SUM(CASE WHEN acknowledged = 1 THEN 1 ELSE 0 END) as acknowledged,
			SUM(CASE WHEN start_time >= datetime('now', '-24 hours') THEN 1 ELSE 0 END) as last_24h,
			SUM(CASE WHEN start_time >= datetime('now', '-7 days') THEN 1 ELSE 0 END) as last_7d
		FROM temperature_spikes
	`

	var summary SpikeSummary
	err := db.QueryRow(query).Scan(
		&summary.Total,
		&summary.Unacknowledged,
		&summary.Acknowledged,
		&summary.Last24Hours,
		&summary.Last7Days,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get spike summary: %w", err)
	}

	return &summary, nil
}

// SpikeSummary holds spike count statistics
type SpikeSummary struct {
	Total          int `json:"total"`
	Unacknowledged int `json:"unacknowledged"`
	Acknowledged   int `json:"acknowledged"`
	Last24Hours    int `json:"last_24h"`
	Last7Days      int `json:"last_7d"`
}

// Helper function to query spikes
func querySpikes(db *sql.DB, query string, args ...interface{}) ([]TemperatureSpike, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query spikes: %w", err)
	}
	defer rows.Close()

	var spikes []TemperatureSpike
	for rows.Next() {
		var spike TemperatureSpike
		var ackAt sql.NullTime

		err := rows.Scan(
			&spike.ID, &spike.Hostname, &spike.SerialNumber,
			&spike.StartTime, &spike.EndTime, &spike.StartTemp, &spike.EndTemp,
			&spike.Change, &spike.RatePerMin, &spike.Direction,
			&spike.Acknowledged, &spike.AcknowledgedBy, &ackAt, &spike.CreatedAt,
		)
		if err != nil {
			continue
		}

		if ackAt.Valid {
			spike.AcknowledgedAt = ackAt.Time
		}

		// Get drive info
		driveInfo, _ := getDriveInfo(db, spike.Hostname, spike.SerialNumber)
		if driveInfo != nil {
			spike.DeviceName = driveInfo.DeviceName
			spike.Model = driveInfo.Model
		}

		spikes = append(spikes, spike)
	}

	return spikes, rows.Err()
}

// ============================================
// SPIKE DETECTION ALGORITHM
// ============================================

// DetectSpikes analyzes temperature history and detects rapid changes
func DetectSpikes(db *sql.DB, hostname, serial string, windowMinutes, thresholdDegrees int) ([]TemperatureSpike, error) {
	// Get temperature readings within the detection window
	cutoff := time.Now().Add(-time.Duration(windowMinutes*2) * time.Minute)

	query := `
		SELECT temperature, timestamp
		FROM temperature_history
		WHERE hostname = ? AND serial_number = ? AND timestamp >= ?
		ORDER BY timestamp ASC
	`

	rows, err := db.Query(query, hostname, serial, cutoff)
	if err != nil {
		return nil, fmt.Errorf("failed to query temperature history: %w", err)
	}
	defer rows.Close()

	// Collect readings
	type reading struct {
		temp int
		time time.Time
	}
	var readings []reading

	for rows.Next() {
		var r reading
		if err := rows.Scan(&r.temp, &r.time); err != nil {
			continue
		}
		readings = append(readings, r)
	}

	if len(readings) < 2 {
		return nil, nil // Not enough data
	}

	// Detect spikes using sliding window
	var spikes []TemperatureSpike
	windowDuration := time.Duration(windowMinutes) * time.Minute

	for i := 0; i < len(readings); i++ {
		// Look ahead within the window
		for j := i + 1; j < len(readings); j++ {
			timeDiff := readings[j].time.Sub(readings[i].time)

			// Stop if outside window
			if timeDiff > windowDuration {
				break
			}

			tempChange := readings[j].temp - readings[i].temp
			absChange := int(math.Abs(float64(tempChange)))

			// Check if this is a spike
			if absChange >= thresholdDegrees {
				direction := "heating"
				if tempChange < 0 {
					direction = "cooling"
				}

				ratePerMin := float64(absChange) / timeDiff.Minutes()

				spike := TemperatureSpike{
					Hostname:     hostname,
					SerialNumber: serial,
					StartTime:    readings[i].time,
					EndTime:      readings[j].time,
					StartTemp:    readings[i].temp,
					EndTemp:      readings[j].temp,
					Change:       absChange,
					RatePerMin:   math.Round(ratePerMin*100) / 100,
					Direction:    direction,
				}

				spikes = append(spikes, spike)

				// Skip to end of this spike to avoid duplicates
				i = j
				break
			}
		}
	}

	return spikes, nil
}

// DetectAndRecordSpikes detects spikes and saves new ones to database
func DetectAndRecordSpikes(db *sql.DB, hostname, serial string) ([]TemperatureSpike, error) {
	// Get detection settings
	windowMinutes := settings.GetIntSettingWithDefault(db, "temperature", "spike_window_minutes", 30)
	thresholdDegrees := settings.GetIntSettingWithDefault(db, "temperature", "spike_threshold", 10)

	// Detect spikes
	detected, err := DetectSpikes(db, hostname, serial, windowMinutes, thresholdDegrees)
	if err != nil {
		return nil, err
	}

	if len(detected) == 0 {
		return nil, nil
	}

	// Check which spikes are new (not already recorded)
	var newSpikes []TemperatureSpike

	for _, spike := range detected {
		// Check if a similar spike already exists
		exists, err := spikeExists(db, hostname, serial, spike.StartTime, spike.EndTime)
		if err != nil {
			continue
		}

		if !exists {
			// Record the new spike
			if err := RecordSpike(db, &spike); err != nil {
				continue
			}
			newSpikes = append(newSpikes, spike)
		}
	}

	return newSpikes, nil
}

// spikeExists checks if a spike with similar time range already exists
func spikeExists(db *sql.DB, hostname, serial string, startTime, endTime time.Time) (bool, error) {
	// Allow 1 minute tolerance for matching
	tolerance := time.Minute

	query := `
		SELECT COUNT(*)
		FROM temperature_spikes
		WHERE hostname = ? AND serial_number = ?
			AND start_time BETWEEN ? AND ?
			AND end_time BETWEEN ? AND ?
	`

	var count int
	err := db.QueryRow(query,
		hostname, serial,
		startTime.Add(-tolerance), startTime.Add(tolerance),
		endTime.Add(-tolerance), endTime.Add(tolerance),
	).Scan(&count)

	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// DetectAllDrivesSpikes runs spike detection on all drives
func DetectAllDrivesSpikes(db *sql.DB) ([]TemperatureSpike, error) {
	// Get unique drive combinations
	query := `
		SELECT DISTINCT hostname, serial_number
		FROM temperature_history
		WHERE timestamp >= datetime('now', '-1 hour')
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get drives: %w", err)
	}
	defer rows.Close()

	var allSpikes []TemperatureSpike

	for rows.Next() {
		var hostname, serial string
		if err := rows.Scan(&hostname, &serial); err != nil {
			continue
		}

		spikes, err := DetectAndRecordSpikes(db, hostname, serial)
		if err != nil {
			continue
		}

		allSpikes = append(allSpikes, spikes...)
	}

	return allSpikes, nil
}

// CleanupOldSpikes removes spike records older than retention period
func CleanupOldSpikes(db *sql.DB, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	result, err := db.Exec(`
		DELETE FROM temperature_spikes
		WHERE created_at < ?
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old spikes: %w", err)
	}

	return result.RowsAffected()
}
