package temperature

import (
	"database/sql"
	"fmt"
	"math"
	"time"

	"vigil/internal/settings"
)

// GetTemperatureStats retrieves temperature statistics for a specific drive
func GetTemperatureStats(db *sql.DB, hostname, serial string, period TemperaturePeriod) (*TemperatureStats, error) {
	// Build time filter using SQLite datetime function
	timeFilter := ""
	args := []interface{}{hostname, serial}

	if period != PeriodAllTime {
		// Use SQLite's datetime function for reliable comparison
		var interval string
		switch period {
		case Period24Hours:
			interval = "-24 hours"
		case Period7Days:
			interval = "-7 days"
		case Period30Days:
			interval = "-30 days"
		default:
			interval = "-365 days"
		}
		timeFilter = fmt.Sprintf("AND timestamp >= datetime('now', '%s')", interval)
	}

	// Query for basic stats
	query := fmt.Sprintf(`
		SELECT
			MIN(temperature) as min_temp,
			MAX(temperature) as max_temp,
			AVG(temperature) as avg_temp,
			COUNT(*) as data_points,
			MIN(timestamp) as first_reading,
			MAX(timestamp) as last_reading
		FROM temperature_history
		WHERE hostname = ? AND serial_number = ? %s
	`, timeFilter)

	var stats TemperatureStats
	var firstReading, lastReading sql.NullString
	var avgTemp sql.NullFloat64
	var minTemp, maxTemp sql.NullInt64

	err := db.QueryRow(query, args...).Scan(
		&minTemp,
		&maxTemp,
		&avgTemp,
		&stats.DataPoints,
		&firstReading,
		&lastReading,
	)
	if err == sql.ErrNoRows || stats.DataPoints == 0 {
		return nil, nil // No data
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get temperature stats: %w", err)
	}

	stats.Hostname = hostname
	stats.SerialNumber = serial
	stats.Period = string(period)

	if minTemp.Valid {
		stats.MinTemp = int(minTemp.Int64)
	}
	if maxTemp.Valid {
		stats.MaxTemp = int(maxTemp.Int64)
	}
	if avgTemp.Valid {
		stats.AvgTemp = math.Round(avgTemp.Float64*100) / 100
	}
	if firstReading.Valid {
		stats.FirstReading, _ = parseTimestamp(firstReading.String)
	}
	if lastReading.Valid {
		stats.LastReading, _ = parseTimestamp(lastReading.String)
	}

	// Get current temperature
	currentTemp, err := GetCurrentTemperature(db, hostname, serial)
	if err == nil && currentTemp != nil {
		stats.CurrentTemp = currentTemp.Temperature
	}

	// Calculate standard deviation
	stats.StdDev, stats.Variance = calculateStdDev(db, hostname, serial, period, stats.AvgTemp)

	// Calculate trend
	stats.TrendSlope, stats.TrendDesc = calculateTrend(db, hostname, serial, period)

	// Get drive info
	driveInfo, _ := getDriveInfo(db, hostname, serial)
	if driveInfo != nil {
		stats.DeviceName = driveInfo.DeviceName
		stats.Model = driveInfo.Model
	}

	return &stats, nil
}

// calculateStdDev calculates standard deviation for temperature readings
func calculateStdDev(db *sql.DB, hostname, serial string, period TemperaturePeriod, avg float64) (float64, float64) {
	timeFilter := ""
	args := []interface{}{hostname, serial, avg}

	if period != PeriodAllTime {
		duration := PeriodToDuration(period)
		cutoff := time.Now().Add(-duration)
		timeFilter = "AND timestamp >= ?"
		args = append(args, cutoff)
	}

	// Calculate variance using the formula: SUM((x - mean)^2) / n
	query := fmt.Sprintf(`
		SELECT AVG((temperature - ?) * (temperature - ?)) as variance
		FROM temperature_history
		WHERE hostname = ? AND serial_number = ? %s
	`, timeFilter)

	// Reorder args for the query
	queryArgs := []interface{}{avg, avg, hostname, serial}
	if period != PeriodAllTime {
		queryArgs = append(queryArgs, args[3])
	}

	var variance sql.NullFloat64
	err := db.QueryRow(query, queryArgs...).Scan(&variance)
	if err != nil || !variance.Valid {
		return 0, 0
	}

	stdDev := math.Sqrt(variance.Float64)
	return math.Round(stdDev*100) / 100, math.Round(variance.Float64*100) / 100
}

// calculateTrend calculates the temperature trend using linear regression
func calculateTrend(db *sql.DB, hostname, serial string, period TemperaturePeriod) (float64, string) {
	timeFilter := ""
	args := []interface{}{hostname, serial}

	if period != PeriodAllTime {
		duration := PeriodToDuration(period)
		cutoff := time.Now().Add(-duration)
		timeFilter = "AND timestamp >= ?"
		args = append(args, cutoff)
	}

	// Get data points for linear regression
	query := fmt.Sprintf(`
		SELECT temperature, timestamp
		FROM temperature_history
		WHERE hostname = ? AND serial_number = ? %s
		ORDER BY timestamp ASC
	`, timeFilter)

	rows, err := db.Query(query, args...)
	if err != nil {
		return 0, "unknown"
	}
	defer rows.Close()

	var temps []float64
	var times []float64
	var baseTime time.Time

	for rows.Next() {
		var temp int
		var ts time.Time
		if err := rows.Scan(&temp, &ts); err != nil {
			continue
		}

		if baseTime.IsZero() {
			baseTime = ts
		}

		temps = append(temps, float64(temp))
		// Convert time to hours since first reading
		times = append(times, ts.Sub(baseTime).Hours())
	}

	if len(temps) < 2 {
		return 0, "insufficient_data"
	}

	// Calculate linear regression slope
	slope := linearRegressionSlope(times, temps)

	// Round to 4 decimal places
	slope = math.Round(slope*10000) / 10000

	// Determine trend description
	var desc string
	if slope > 0.1 {
		desc = "heating"
	} else if slope < -0.1 {
		desc = "cooling"
	} else {
		desc = "stable"
	}

	return slope, desc
}

// linearRegressionSlope calculates the slope of a linear regression line
func linearRegressionSlope(x, y []float64) float64 {
	n := float64(len(x))
	if n < 2 {
		return 0
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i := range x {
		sumX += x[i]
		sumY += y[i]
		sumXY += x[i] * y[i]
		sumX2 += x[i] * x[i]
	}

	denominator := n*sumX2 - sumX*sumX
	if denominator == 0 {
		return 0
	}

	return (n*sumXY - sumX*sumY) / denominator
}

// GetAllDrivesTemperatureStats retrieves stats for all drives
func GetAllDrivesTemperatureStats(db *sql.DB, period TemperaturePeriod) ([]TemperatureStats, error) {
	// Get unique drive combinations
	query := `
		SELECT DISTINCT hostname, serial_number
		FROM temperature_history
		ORDER BY hostname, serial_number
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get drives: %w", err)
	}
	defer rows.Close()

	var stats []TemperatureStats
	for rows.Next() {
		var hostname, serial string
		if err := rows.Scan(&hostname, &serial); err != nil {
			continue
		}

		driveStats, err := GetTemperatureStats(db, hostname, serial, period)
		if err != nil {
			continue
		}
		if driveStats != nil {
			stats = append(stats, *driveStats)
		}
	}

	return stats, nil
}

// GetTemperatureTimeSeries retrieves time series data for charting
func GetTemperatureTimeSeries(db *sql.DB, hostname, serial string, period TemperaturePeriod, interval AggregationInterval) (*TimeSeriesData, error) {
	// Build time filter using SQLite datetime function
	timeFilter := ""
	args := []interface{}{hostname, serial}

	if period != PeriodAllTime {
		var sqlInterval string
		switch period {
		case Period24Hours:
			sqlInterval = "-24 hours"
		case Period7Days:
			sqlInterval = "-7 days"
		case Period30Days:
			sqlInterval = "-30 days"
		default:
			sqlInterval = "-365 days"
		}
		timeFilter = fmt.Sprintf("AND timestamp >= datetime('now', '%s')", sqlInterval)
	}

	// Build aggregation query
	timeFormat := IntervalToSQLite(interval)

	query := fmt.Sprintf(`
		SELECT
			strftime('%s', timestamp) as time_bucket,
			MIN(temperature) as min_temp,
			MAX(temperature) as max_temp,
			AVG(temperature) as avg_temp,
			COUNT(*) as data_points
		FROM temperature_history
		WHERE hostname = ? AND serial_number = ? %s
		GROUP BY time_bucket
		ORDER BY time_bucket ASC
	`, timeFormat, timeFilter)

	rows, err := db.Query(query, args...) // #nosec G701 -- query is built from hardcoded format strings, user values are parameterized
	if err != nil {
		return nil, fmt.Errorf("failed to get time series: %w", err)
	}
	defer rows.Close()

	var points []TimeSeriesPoint
	for rows.Next() {
		var timeBucket string
		var point TimeSeriesPoint

		err := rows.Scan(&timeBucket, &point.MinTemp, &point.MaxTemp, &point.AvgTemp, &point.DataPoints)
		if err != nil {
			continue
		}

		// Parse the time bucket
		point.Timestamp, _ = time.Parse("2006-01-02 15:04:05", timeBucket)
		point.Temperature = int(math.Round(point.AvgTemp))
		point.AvgTemp = math.Round(point.AvgTemp*100) / 100

		points = append(points, point)
	}

	result := &TimeSeriesData{
		Hostname:     hostname,
		SerialNumber: serial,
		Period:       string(period),
		Interval:     string(interval),
		Points:       points,
	}

	// Get drive info
	driveInfo, _ := getDriveInfo(db, hostname, serial)
	if driveInfo != nil {
		result.DeviceName = driveInfo.DeviceName
		result.Model = driveInfo.Model
	}

	return result, nil
}

// GetCurrentTemperature retrieves the most recent temperature for a drive
func GetCurrentTemperature(db *sql.DB, hostname, serial string) (*CurrentTemperature, error) {
	query := `
		SELECT temperature, timestamp
		FROM temperature_history
		WHERE hostname = ? AND serial_number = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var current CurrentTemperature
	current.Hostname = hostname
	current.SerialNumber = serial

	var timestampStr string
	err := db.QueryRow(query, hostname, serial).Scan(&current.Temperature, &timestampStr)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get current temperature: %w", err)
	}

	current.Timestamp, _ = parseTimestamp(timestampStr)

	// Get thresholds from settings
	thresholds := getThresholdsFromSettings(db)
	current.Status = thresholds.GetStatus(current.Temperature)

	// Get drive info
	driveInfo, _ := getDriveInfo(db, hostname, serial)
	if driveInfo != nil {
		current.DeviceName = driveInfo.DeviceName
		current.Model = driveInfo.Model
	}

	return &current, nil
}

// GetAllCurrentTemperatures retrieves current temperatures for all drives
func GetAllCurrentTemperatures(db *sql.DB) ([]CurrentTemperature, error) {
	// Get the most recent temperature for each drive
	query := `
		SELECT th.hostname, th.serial_number, th.temperature, th.timestamp
		FROM temperature_history th
		INNER JOIN (
			SELECT hostname, serial_number, MAX(timestamp) as max_ts
			FROM temperature_history
			GROUP BY hostname, serial_number
		) latest ON th.hostname = latest.hostname
			AND th.serial_number = latest.serial_number
			AND th.timestamp = latest.max_ts
		ORDER BY th.hostname, th.serial_number
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to get current temperatures: %w", err)
	}
	defer rows.Close()

	thresholds := getThresholdsFromSettings(db)
	var temps []CurrentTemperature

	for rows.Next() {
		var ct CurrentTemperature
		var timestampStr string
		err := rows.Scan(&ct.Hostname, &ct.SerialNumber, &ct.Temperature, &timestampStr)
		if err != nil {
			continue
		}

		ct.Timestamp, _ = parseTimestamp(timestampStr)
		ct.Status = thresholds.GetStatus(ct.Temperature)

		// Get drive info
		driveInfo, _ := getDriveInfo(db, ct.Hostname, ct.SerialNumber)
		if driveInfo != nil {
			ct.DeviceName = driveInfo.DeviceName
			ct.Model = driveInfo.Model
		}

		temps = append(temps, ct)
	}

	return temps, nil
}

// GetTemperatureSummary provides an overview of all drive temperatures
func GetTemperatureSummary(db *sql.DB) (*TemperatureSummary, error) {
	temps, err := GetAllCurrentTemperatures(db)
	if err != nil {
		return nil, err
	}

	if len(temps) == 0 {
		return &TemperatureSummary{}, nil
	}

	summary := &TemperatureSummary{
		TotalDrives:    len(temps),
		MinTemperature: temps[0].Temperature,
		MaxTemperature: temps[0].Temperature,
		Drives:         temps,
	}

	var totalTemp int
	var hottest, coolest *CurrentTemperature

	for i := range temps {
		t := &temps[i]
		totalTemp += t.Temperature

		// Track min/max (use <= and >= to ensure first element is captured)
		if t.Temperature <= summary.MinTemperature {
			summary.MinTemperature = t.Temperature
			coolest = t
		}
		if t.Temperature >= summary.MaxTemperature {
			summary.MaxTemperature = t.Temperature
			hottest = t
		}

		// Count by status
		switch t.Status {
		case "normal":
			summary.DrivesNormal++
		case "warning":
			summary.DrivesWarning++
		case "critical":
			summary.DrivesCritical++
		}
	}

	summary.AvgTemperature = math.Round(float64(totalTemp)/float64(len(temps))*100) / 100
	summary.HottestDrive = hottest
	summary.CoolestDrive = coolest

	return summary, nil
}

// GetTemperatureRange retrieves temperature records within a time range
func GetTemperatureRange(db *sql.DB, hostname, serial string, from, to time.Time) ([]TempReading, error) {
	query := `
		SELECT id, hostname, serial_number, temperature, timestamp
		FROM temperature_history
		WHERE hostname = ? AND serial_number = ? AND timestamp BETWEEN ? AND ?
		ORDER BY timestamp ASC
	`

	rows, err := db.Query(query, hostname, serial,
		from.UTC().Format("2006-01-02 15:04:05"),
		to.UTC().Format("2006-01-02 15:04:05"))
	if err != nil {
		return nil, fmt.Errorf("failed to get temperature range: %w", err)
	}
	defer rows.Close()

	var records []TempReading
	for rows.Next() {
		var r TempReading
		var timestampStr string
		err := rows.Scan(&r.ID, &r.Hostname, &r.SerialNumber, &r.Temperature, &timestampStr)
		if err != nil {
			continue
		}
		r.Timestamp, _ = parseTimestamp(timestampStr)
		records = append(records, r)
	}

	return records, nil
}

// GetHeatmapData retrieves data for temperature heatmap visualization
func GetHeatmapData(db *sql.DB, period TemperaturePeriod, interval AggregationInterval) (*HeatmapData, error) {
	// Get all unique drives
	drivesQuery := `
		SELECT DISTINCT hostname, serial_number
		FROM temperature_history
		ORDER BY hostname, serial_number
	`

	rows, err := db.Query(drivesQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get drives: %w", err)
	}
	defer rows.Close()

	thresholds := getThresholdsFromSettings(db)
	heatmap := &HeatmapData{
		Period:   string(period),
		Interval: string(interval),
	}

	for rows.Next() {
		var hostname, serial string
		if err := rows.Scan(&hostname, &serial); err != nil {
			continue
		}

		timeSeries, err := GetTemperatureTimeSeries(db, hostname, serial, period, interval)
		if err != nil || timeSeries == nil {
			continue
		}

		drive := HeatmapDrive{
			Hostname:     hostname,
			SerialNumber: serial,
			DeviceName:   timeSeries.DeviceName,
			Model:        timeSeries.Model,
		}

		for _, pt := range timeSeries.Points {
			reading := HeatmapReading{
				Timestamp:   pt.Timestamp,
				Temperature: pt.Temperature,
				Status:      thresholds.GetStatus(pt.Temperature),
			}
			drive.Readings = append(drive.Readings, reading)
		}

		heatmap.Drives = append(heatmap.Drives, drive)
	}

	return heatmap, nil
}

// Helper: get thresholds from settings
func getThresholdsFromSettings(db *sql.DB) TemperatureThresholds {
	thresholds := DefaultThresholds()

	warning, err := settings.GetIntSetting(db, "temperature", "warning_threshold")
	if err == nil {
		thresholds.Warning = warning
	}

	critical, err := settings.GetIntSetting(db, "temperature", "critical_threshold")
	if err == nil {
		thresholds.Critical = critical
	}

	return thresholds
}

// driveInfo holds basic drive info
type driveInfo struct {
	DeviceName string
	Model      string
}

// Helper: get drive info from reports or drives table
func getDriveInfo(db *sql.DB, hostname, serial string) (*driveInfo, error) {
	// Try to get from smart_results first (has latest info)
	query := `
		SELECT device_name, model
		FROM smart_results
		WHERE hostname = ? AND serial_number = ?
		ORDER BY timestamp DESC
		LIMIT 1
	`

	var info driveInfo
	err := db.QueryRow(query, hostname, serial).Scan(&info.DeviceName, &info.Model)
	if err == nil {
		return &info, nil
	}

	// Fallback: try drives table
	query = `
		SELECT device, model
		FROM drives
		WHERE hostname = ? AND serial_number = ?
		LIMIT 1
	`

	err = db.QueryRow(query, hostname, serial).Scan(&info.DeviceName, &info.Model)
	if err == nil {
		return &info, nil
	}

	return nil, err
}

// CleanupOldTemperatureData removes temperature data older than retention period
func CleanupOldTemperatureData(db *sql.DB, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	result, err := db.Exec(`
		DELETE FROM temperature_history
		WHERE timestamp < ?
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old temperature data: %w", err)
	}

	return result.RowsAffected()
}

// parseTimestamp parses various SQLite timestamp formats
func parseTimestamp(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		time.RFC3339,
		time.RFC3339Nano,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", s)
}
