package db

import (
	"database/sql"
	"fmt"
	"time"

	"vigil/cmd/agent/smart"
)

// StoreSmartAttributes saves SMART attributes to the database
func StoreSmartAttributes(driveData *smart.DriveSmartData) error {
	stmt, err := DB.Prepare(`
		INSERT INTO smart_attributes 
		(hostname, serial_number, device_name, attribute_id, attribute_name, value, worst, threshold, raw_value, flags, when_failed, timestamp)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare statement: %w", err)
	}
	defer stmt.Close()

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
			attr.WhenFailed,
			attr.Timestamp.Format("2006-01-02 15:04:05"),
		)
		if err != nil {
			// Log error but continue with other attributes
			fmt.Printf("Warning: Failed to store attribute %d for %s: %v\n", attr.ID, driveData.SerialNumber, err)
		}
	}

	return nil
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
func GetDriveHealthSummary(hostname, serialNumber string) (map[string]interface{}, error) {
	attributes, err := GetLatestSmartAttributes(hostname, serialNumber)
	if err != nil {
		return nil, err
	}

	summary := map[string]interface{}{
		"hostname":       hostname,
		"serial_number":  serialNumber,
		"overall_health": "HEALTHY",
		"critical_count": 0,
		"warning_count":  0,
		"issues":         make([]map[string]interface{}, 0),
	}

	criticalCount := 0
	warningCount := 0
	issues := make([]map[string]interface{}, 0)

	for _, attr := range attributes {
		severity := smart.GetAttributeSeverity(attr.ID, attr.RawValue, attr.Threshold)

		switch severity {
		case "CRITICAL":
			criticalCount++
			issues = append(issues, map[string]interface{}{
				"attribute_id":   attr.ID,
				"attribute_name": attr.Name,
				"severity":       "CRITICAL",
				"raw_value":      attr.RawValue,
				"threshold":      attr.Threshold,
				"message":        fmt.Sprintf("%s has critical value: %d", attr.Name, attr.RawValue),
			})
		case "WARNING":
			warningCount++
			issues = append(issues, map[string]interface{}{
				"attribute_id":   attr.ID,
				"attribute_name": attr.Name,
				"severity":       "WARNING",
				"raw_value":      attr.RawValue,
				"threshold":      attr.Threshold,
				"message":        fmt.Sprintf("%s requires attention: %d", attr.Name, attr.RawValue),
			})
		}
	}

	summary["critical_count"] = criticalCount
	summary["warning_count"] = warningCount
	summary["issues"] = issues

	// Set overall health based on issues
	if criticalCount > 0 {
		summary["overall_health"] = "CRITICAL"
	} else if warningCount > 0 {
		summary["overall_health"] = "WARNING"
	}

	return summary, nil
}

// CleanupOldSmartData removes SMART data older than specified days
func CleanupOldSmartData(daysToKeep int) (int64, error) {
	cutoffDate := time.Now().AddDate(0, 0, -daysToKeep).Format("2006-01-02 15:04:05")

	result, err := DB.Exec(`
		DELETE FROM smart_attributes
		WHERE timestamp < ?
	`, cutoffDate)

	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// GetAttributeTrend calculates trend for a specific attribute over time
func GetAttributeTrend(hostname, serialNumber string, attributeID int, days int) (map[string]interface{}, error) {
	since := time.Now().AddDate(0, 0, -days).Format("2006-01-02 15:04:05")

	query := `
		SELECT raw_value, timestamp
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

	dataPoints := make([]map[string]interface{}, 0)
	var firstValue, lastValue int64

	count := 0
	for rows.Next() {
		var rawValue int64
		var timestampStr string

		if err := rows.Scan(&rawValue, &timestampStr); err != nil {
			continue
		}

		if count == 0 {
			firstValue = rawValue
		}
		lastValue = rawValue

		timestamp, _ := time.Parse("2006-01-02 15:04:05", timestampStr)
		dataPoints = append(dataPoints, map[string]interface{}{
			"value":     rawValue,
			"timestamp": timestamp.Unix(),
		})
		count++
	}

	trend := "stable"
	change := lastValue - firstValue

	if count > 1 {
		// Determine trend direction
		if change > 0 {
			switch attributeID {
			case 5, 197, 198:
				// Sector errors — increasing is degradation
				trend = "degrading"
			case 9, 12, 241, 242:
				// Monotonic counters (power-on hours, cycles, LBAs) — increasing is normal
				trend = "increasing"
			}
		} else if change < 0 {
			trend = "improving"
		}
	}

	return map[string]interface{}{
		"attribute_id": attributeID,
		"data_points":  dataPoints,
		"first_value":  firstValue,
		"last_value":   lastValue,
		"change":       change,
		"trend":        trend,
		"point_count":  count,
	}, nil
}

// ProcessReportForSmartStorage extracts and stores SMART data from incoming report
func ProcessReportForSmartStorage(hostname string, reportData map[string]interface{}) error {
	drives, ok := reportData["drives"].([]interface{})
	if !ok {
		return nil // No drives in report
	}

	for _, driveInterface := range drives {
		driveMap, ok := driveInterface.(map[string]interface{})
		if !ok {
			continue
		}

		// Parse SMART data for this drive
		driveData, err := smart.ParseSmartAttributes(driveMap, hostname)
		if err != nil {
			fmt.Printf("Warning: Failed to parse SMART data for drive: %v\n", err)
			continue
		}

		// Store the SMART attributes
		if len(driveData.Attributes) > 0 {
			if err := StoreSmartAttributes(driveData); err != nil {
				fmt.Printf("Warning: Failed to store SMART attributes: %v\n", err)
			}
		}
	}

	return nil
}
