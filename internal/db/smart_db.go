package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"vigil/cmd/agent/smart"
)

// StoreSmartAttributes saves SMART attributes to the database
func StoreSmartAttributes(driveData *smart.DriveSmartData) error {
	if driveData == nil || len(driveData.Attributes) == 0 {
		return nil
	}

	tx, err := DB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO smart_attributes 
		(hostname, serial_number, device_name, attribute_id, attribute_name, 
		 value, worst, threshold, raw_value, flags, when_failed, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(hostname, serial_number, attribute_id, timestamp) DO UPDATE SET
			value = excluded.value,
			worst = excluded.worst,
			raw_value = excluded.raw_value,
			flags = excluded.flags,
			when_failed = excluded.when_failed
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

	timestamp := driveData.Timestamp.Format("2006-01-02 15:04:05")

	for _, attr := range driveData.Attributes {
		_, err = stmt.Exec(
			driveData.Hostname,
			driveData.SerialNumber,
			driveData.DeviceName,
			attr.ID,
			attr.Name,
			attr.Value,
			attr.Worst,
			attr.Threshold,
			attr.RawValue,
			attr.Flags,
			nullableString(attr.WhenFailed),
			timestamp,
		)
		if err != nil {
			log.Printf("Warning: Failed to store attribute %d for %s: %v", attr.ID, driveData.SerialNumber, err)
		}
	}

	// Also store temperature history if temperature is available
	if driveData.Temperature > 0 {
		_, err = tx.Exec(`
			INSERT INTO temperature_history (hostname, serial_number, temperature, timestamp)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(hostname, serial_number, timestamp) DO UPDATE SET
				temperature = excluded.temperature
		`, driveData.Hostname, driveData.SerialNumber, driveData.Temperature, timestamp)
		if err != nil {
			log.Printf("Warning: Failed to store temperature for %s: %v", driveData.SerialNumber, err)
		}
	}

	return tx.Commit()
}

// GetSmartAttributeHistory retrieves historical data for a specific attribute
func GetSmartAttributeHistory(hostname, serialNumber string, attributeID int, limit int) ([]smart.SmartAttribute, error) {
	query := `
		SELECT attribute_id, attribute_name, value, worst, threshold, raw_value, flags, when_failed, timestamp
		FROM smart_attributes
		WHERE hostname = ? AND serial_number = ? AND attribute_id = ?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := DB.Query(query, hostname, serialNumber, attributeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSmartAttributes(rows)
}

// GetLatestSmartAttributes retrieves the most recent SMART attributes for a drive
func GetLatestSmartAttributes(hostname, serialNumber string) ([]smart.SmartAttribute, error) {
	query := `
		SELECT attribute_id, attribute_name, value, worst, threshold, raw_value, flags, when_failed, timestamp
		FROM smart_attributes
		WHERE hostname = ? AND serial_number = ?
		AND timestamp = (
			SELECT MAX(timestamp) FROM smart_attributes 
			WHERE hostname = ? AND serial_number = ?
		)
		ORDER BY attribute_id
	`

	rows, err := DB.Query(query, hostname, serialNumber, hostname, serialNumber)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanSmartAttributes(rows)
}

// GetAllLatestSmartAttributes retrieves the latest SMART data for all drives
func GetAllLatestSmartAttributes() (map[string][]smart.SmartAttribute, error) {
	query := `
		SELECT DISTINCT hostname, serial_number
		FROM smart_attributes
	`

	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]smart.SmartAttribute)
	for rows.Next() {
		var hostname, serial string
		if err := rows.Scan(&hostname, &serial); err != nil {
			continue
		}

		attrs, err := GetLatestSmartAttributes(hostname, serial)
		if err != nil {
			continue
		}

		key := fmt.Sprintf("%s:%s", hostname, serial)
		result[key] = attrs
	}

	return result, nil
}

// GetCriticalSmartAttributes retrieves all critical SMART attribute definitions
func GetCriticalSmartAttributes() ([]smart.CriticalAttribute, error) {
	query := `
		SELECT attribute_id, attribute_name, description, drive_type, severity, failure_threshold
		FROM critical_smart_attributes
		ORDER BY severity DESC, attribute_id
	`

	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var attributes []smart.CriticalAttribute
	for rows.Next() {
		var attr smart.CriticalAttribute
		var failureThreshold sql.NullInt64

		err := rows.Scan(
			&attr.ID,
			&attr.Name,
			&attr.Description,
			&attr.DriveType,
			&attr.Severity,
			&failureThreshold,
		)
		if err != nil {
			continue
		}

		if failureThreshold.Valid {
			threshold := int(failureThreshold.Int64)
			attr.FailureThreshold = &threshold
		}

		attributes = append(attributes, attr)
	}

	return attributes, nil
}

// GetDriveHealthSummary analyzes SMART attributes and returns health status
func GetDriveHealthSummary(hostname, serialNumber string) (*smart.DriveHealthAnalysis, error) {
	attributes, err := GetLatestSmartAttributes(hostname, serialNumber)
	if err != nil {
		return nil, err
	}

	// Build a DriveSmartData object for analysis
	driveData := &smart.DriveSmartData{
		Hostname:     hostname,
		SerialNumber: serialNumber,
		Attributes:   attributes,
		SmartPassed:  true, // Will be updated if we find a failed status
		Timestamp:    time.Now(),
	}

	// Get drive info from the latest report
	driveInfo, err := GetDriveInfo(hostname, serialNumber)
	if err == nil && driveInfo != nil {
		driveData.ModelName = driveInfo.ModelName
		driveData.DriveType = driveInfo.DriveType
		driveData.SmartPassed = driveInfo.SmartPassed
	}

	// Perform health analysis
	return smart.AnalyzeDriveHealth(driveData), nil
}

// GetAllDrivesHealthSummary returns health summaries for all monitored drives
func GetAllDrivesHealthSummary() ([]*smart.DriveHealthAnalysis, error) {
	// Get all unique drives
	query := `
		SELECT DISTINCT hostname, serial_number
		FROM smart_attributes
		ORDER BY hostname, serial_number
	`

	rows, err := DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []*smart.DriveHealthAnalysis
	for rows.Next() {
		var hostname, serial string
		if err := rows.Scan(&hostname, &serial); err != nil {
			continue
		}

		summary, err := GetDriveHealthSummary(hostname, serial)
		if err != nil {
			log.Printf("Warning: Failed to get health summary for %s/%s: %v", hostname, serial, err)
			continue
		}

		summaries = append(summaries, summary)
	}

	return summaries, nil
}

// GetAttributeTrend calculates trend for a specific attribute over time
func GetAttributeTrend(hostname, serialNumber string, attributeID int, days int) (*AttributeTrend, error) {
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")

	query := `
		SELECT raw_value, value, timestamp
		FROM smart_attributes
		WHERE hostname = ? AND serial_number = ? AND attribute_id = ?
		AND timestamp >= ?
		ORDER BY timestamp ASC
	`

	rows, err := DB.Query(query, hostname, serialNumber, attributeID, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	trend := &AttributeTrend{
		AttributeID: attributeID,
		DataPoints:  make([]TrendDataPoint, 0),
	}

	var firstRaw, lastRaw int64
	var firstVal, lastVal int
	count := 0

	for rows.Next() {
		var rawValue int64
		var value int
		var timestampStr string

		if err := rows.Scan(&rawValue, &value, &timestampStr); err != nil {
			continue
		}

		if count == 0 {
			firstRaw = rawValue
			firstVal = value
		}
		lastRaw = rawValue
		lastVal = value

		timestamp, _ := time.Parse("2006-01-02 15:04:05", timestampStr)
		trend.DataPoints = append(trend.DataPoints, TrendDataPoint{
			RawValue:  rawValue,
			Value:     value,
			Timestamp: timestamp.Unix(),
		})
		count++
	}

	if count == 0 {
		return trend, nil
	}

	trend.FirstRawValue = firstRaw
	trend.LastRawValue = lastRaw
	trend.FirstValue = firstVal
	trend.LastValue = lastVal
	trend.RawChange = lastRaw - firstRaw
	trend.ValueChange = lastVal - firstVal
	trend.PointCount = count

	// Determine trend direction
	trend.Trend = determineTrend(attributeID, trend.RawChange, trend.ValueChange)

	return trend, nil
}

// AttributeTrend represents trend analysis data
type AttributeTrend struct {
	AttributeID   int              `json:"attribute_id"`
	DataPoints    []TrendDataPoint `json:"data_points"`
	FirstRawValue int64            `json:"first_raw_value"`
	LastRawValue  int64            `json:"last_raw_value"`
	FirstValue    int              `json:"first_value"`
	LastValue     int              `json:"last_value"`
	RawChange     int64            `json:"raw_change"`
	ValueChange   int              `json:"value_change"`
	Trend         string           `json:"trend"`
	PointCount    int              `json:"point_count"`
}

// TrendDataPoint represents a single point in trend data
type TrendDataPoint struct {
	RawValue  int64 `json:"raw_value"`
	Value     int   `json:"value"`
	Timestamp int64 `json:"timestamp"`
}

// determineTrend determines the trend direction based on attribute type
func determineTrend(attributeID int, rawChange int64, valueChange int) string {
	if rawChange == 0 && valueChange == 0 {
		return "stable"
	}

	// For most error counters, increasing raw values means degradation
	degradingAttributes := map[int]bool{
		5: true, 10: true, 187: true, 188: true, 196: true, 197: true, 198: true,
		181: true, 182: true, 183: true, 184: true, 199: true, 233: true,
	}

	// For these, increasing is just normal usage
	normalIncreaseAttributes := map[int]bool{
		9: true, 12: true, 193: true, 241: true, 242: true,
	}

	// For these, decreasing value is bad (available space, wear leveling)
	higherIsBetterAttributes := map[int]bool{
		177: true, 232: true,
	}

	if degradingAttributes[attributeID] {
		if rawChange > 0 {
			return "degrading"
		}
		return "stable"
	}

	if normalIncreaseAttributes[attributeID] {
		if rawChange > 0 {
			return "increasing"
		}
		return "stable"
	}

	if higherIsBetterAttributes[attributeID] {
		if valueChange < 0 {
			return "degrading"
		}
		return "stable"
	}

	// For temperature
	if attributeID == 194 || attributeID == 190 {
		if rawChange > 5 {
			return "increasing"
		}
		if rawChange < -5 {
			return "decreasing"
		}
		return "stable"
	}

	return "stable"
}

// GetTemperatureHistory retrieves temperature history for a drive
func GetTemperatureHistory(hostname, serialNumber string, hours int) ([]TemperatureRecord, error) {
	since := time.Now().Add(-time.Duration(hours) * time.Hour).Format("2006-01-02 15:04:05")

	query := `
		SELECT temperature, timestamp
		FROM temperature_history
		WHERE hostname = ? AND serial_number = ?
		AND timestamp >= ?
		ORDER BY timestamp ASC
	`

	rows, err := DB.Query(query, hostname, serialNumber, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []TemperatureRecord
	for rows.Next() {
		var temp int
		var timestampStr string

		if err := rows.Scan(&temp, &timestampStr); err != nil {
			continue
		}

		timestamp, _ := time.Parse("2006-01-02 15:04:05", timestampStr)
		records = append(records, TemperatureRecord{
			Temperature: temp,
			Timestamp:   timestamp,
		})
	}

	return records, nil
}

// TemperatureRecord represents a temperature reading
type TemperatureRecord struct {
	Temperature int       `json:"temperature"`
	Timestamp   time.Time `json:"timestamp"`
}

// CleanupOldSmartData removes SMART data older than specified days
func CleanupOldSmartData(daysToKeep int) (int64, error) {
	cutoffDate := time.Now().AddDate(0, 0, -daysToKeep).Format("2006-01-02 15:04:05")

	// Clean up smart_attributes
	result, err := DB.Exec(`DELETE FROM smart_attributes WHERE timestamp < ?`, cutoffDate)
	if err != nil {
		return 0, err
	}
	smartDeleted, _ := result.RowsAffected()

	// Clean up temperature_history
	result, err = DB.Exec(`DELETE FROM temperature_history WHERE timestamp < ?`, cutoffDate)
	if err != nil {
		return smartDeleted, err
	}
	tempDeleted, _ := result.RowsAffected()

	return smartDeleted + tempDeleted, nil
}

// DriveInfo holds basic drive information
type DriveInfo struct {
	Hostname     string
	SerialNumber string
	ModelName    string
	DriveType    string
	SmartPassed  bool
}

// GetDriveInfo retrieves basic drive info from the latest report
func GetDriveInfo(hostname, serialNumber string) (*DriveInfo, error) {
	query := `
		SELECT data FROM reports
		WHERE hostname = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var dataJSON []byte
	err := DB.QueryRow(query, hostname).Scan(&dataJSON)
	if err != nil {
		return nil, err
	}

	var reportData map[string]interface{}
	if err := json.Unmarshal(dataJSON, &reportData); err != nil {
		return nil, err
	}

	drives, ok := reportData["drives"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("no drives in report")
	}

	for _, driveInterface := range drives {
		drive, ok := driveInterface.(map[string]interface{})
		if !ok {
			continue
		}

		serial, _ := drive["serial_number"].(string)
		if serial != serialNumber {
			continue
		}

		info := &DriveInfo{
			Hostname:     hostname,
			SerialNumber: serialNumber,
			SmartPassed:  true,
		}

		// Model name
		if model, ok := drive["model_name"].(string); ok {
			info.ModelName = model
		} else if model, ok := drive["model_family"].(string); ok {
			info.ModelName = model
		}

		// SMART status
		if smartStatus, ok := drive["smart_status"].(map[string]interface{}); ok {
			if passed, ok := smartStatus["passed"].(bool); ok {
				info.SmartPassed = passed
			}
		}

		// Drive type
		info.DriveType = determineDriveTypeFromReport(drive)

		return info, nil
	}

	return nil, fmt.Errorf("drive not found in report")
}

// determineDriveTypeFromReport determines drive type from report data
func determineDriveTypeFromReport(drive map[string]interface{}) string {
	// Check for NVMe
	if device, ok := drive["device"].(map[string]interface{}); ok {
		if protocol, ok := device["protocol"].(string); ok && protocol == "NVMe" {
			return smart.DriveTypeNVMe
		}
		if dtype, ok := device["type"].(string); ok && dtype == "nvme" {
			return smart.DriveTypeNVMe
		}
	}

	// Check rotation rate
	if rate, ok := drive["rotation_rate"].(float64); ok {
		if rate == 0 {
			return smart.DriveTypeSSD
		}
		return smart.DriveTypeHDD
	}

	return smart.DriveTypeHDD // Default
}

// ProcessReportForSmartStorage extracts and stores SMART data from incoming report
func ProcessReportForSmartStorage(hostname string, reportData map[string]interface{}) error {
	drives, ok := reportData["drives"].([]interface{})
	if !ok {
		return nil // No drives in report
	}

	var lastErr error
	for _, driveInterface := range drives {
		driveMap, ok := driveInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// Parse SMART data for this drive
		driveData, err := smart.ParseSmartAttributes(driveMap, hostname)
		if err != nil {
			log.Printf("Warning: Failed to parse SMART data for drive: %v", err)
			lastErr = err
			continue
		}

		// Skip if no serial number (can't identify the drive)
		if driveData.SerialNumber == "" {
			continue
		}

		// Store the SMART attributes
		if len(driveData.Attributes) > 0 {
			if err := StoreSmartAttributes(driveData); err != nil {
				log.Printf("Warning: Failed to store SMART attributes for %s: %v", driveData.SerialNumber, err)
				lastErr = err
			}
		}
	}

	return lastErr
}

// scanSmartAttributes scans rows into SmartAttribute slice
func scanSmartAttributes(rows *sql.Rows) ([]smart.SmartAttribute, error) {
	var attributes []smart.SmartAttribute

	for rows.Next() {
		var attr smart.SmartAttribute
		var timestampStr string
		var whenFailed sql.NullString

		err := rows.Scan(
			&attr.ID,
			&attr.Name,
			&attr.Value,
			&attr.Worst,
			&attr.Threshold,
			&attr.RawValue,
			&attr.Flags,
			&whenFailed,
			&timestampStr,
		)
		if err != nil {
			continue
		}

		if whenFailed.Valid {
			attr.WhenFailed = whenFailed.String
		}

		attr.Timestamp, _ = time.Parse("2006-01-02 15:04:05", timestampStr)
		attributes = append(attributes, attr)
	}

	return attributes, nil
}

// nullableString returns sql.NullString for empty strings
func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}
