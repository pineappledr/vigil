package db

import (
	"database/sql"
	"fmt"
	"math"
	"time"
)

// DashboardTemperatureData holds all temperature data for the dashboard
type DashboardTemperatureData struct {
	// Summary counts
	TotalDrives    int `json:"total_drives"`
	DrivesNormal   int `json:"drives_normal"`
	DrivesWarning  int `json:"drives_warning"`
	DrivesCritical int `json:"drives_critical"`

	// Temperature stats
	AvgTemperature float64 `json:"avg_temperature"`
	MinTemperature int     `json:"min_temperature"`
	MaxTemperature int     `json:"max_temperature"`

	// Notable drives
	HottestDrive *DashboardDrive `json:"hottest_drive,omitempty"`
	CoolestDrive *DashboardDrive `json:"coolest_drive,omitempty"`

	// Thresholds for frontend
	Thresholds TemperatureThresholds `json:"thresholds"`

	// Alert counts
	ActiveAlerts         int `json:"active_alerts"`
	UnacknowledgedSpikes int `json:"unacknowledged_spikes"`

	// Drives by status
	DrivesByStatus map[string][]DashboardDrive `json:"drives_by_status,omitempty"`

	// Recent activity
	RecentAlerts []DashboardAlert `json:"recent_alerts,omitempty"`
	RecentSpikes []DashboardSpike `json:"recent_spikes,omitempty"`

	// Timestamp
	GeneratedAt time.Time `json:"generated_at"`
}

// DashboardDrive holds drive info for dashboard display
type DashboardDrive struct {
	Hostname     string    `json:"hostname"`
	SerialNumber string    `json:"serial_number"`
	DeviceName   string    `json:"device_name,omitempty"`
	Model        string    `json:"model,omitempty"`
	Temperature  int       `json:"temperature"`
	Status       string    `json:"status"`
	LastUpdated  time.Time `json:"last_updated"`
}

// DashboardAlert holds alert info for dashboard display
type DashboardAlert struct {
	ID           int64     `json:"id"`
	Hostname     string    `json:"hostname"`
	SerialNumber string    `json:"serial_number"`
	AlertType    string    `json:"alert_type"`
	Temperature  int       `json:"temperature"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"created_at"`
}

// DashboardSpike holds spike info for dashboard display
type DashboardSpike struct {
	ID           int64     `json:"id"`
	Hostname     string    `json:"hostname"`
	SerialNumber string    `json:"serial_number"`
	StartTemp    int       `json:"start_temp"`
	EndTemp      int       `json:"end_temp"`
	Change       int       `json:"change"`
	Direction    string    `json:"direction"`
	CreatedAt    time.Time `json:"created_at"`
}

// DashboardOverview holds a quick overview for header/sidebar display
type DashboardOverview struct {
	TotalDrives      int     `json:"total_drives"`
	DrivesWithIssues int     `json:"drives_with_issues"`
	ActiveAlerts     int     `json:"active_alerts"`
	AvgTemperature   float64 `json:"avg_temperature"`
	MaxTemperature   int     `json:"max_temperature"`
	Status           string  `json:"status"` // "normal", "warning", "critical"
}

// GetDashboardTemperatureData retrieves comprehensive dashboard data
func GetDashboardTemperatureData(db *sql.DB, includeDetails bool) (*DashboardTemperatureData, error) {
	data := &DashboardTemperatureData{
		GeneratedAt:    time.Now(),
		DrivesByStatus: make(map[string][]DashboardDrive),
	}

	// Get thresholds
	data.Thresholds = getThresholdsFromSettings(db)

	// Get all current temperatures
	temps, err := GetAllCurrentTemperatures(db)
	if err != nil {
		return nil, fmt.Errorf("failed to get current temperatures: %w", err)
	}

	if len(temps) == 0 {
		return data, nil
	}

	// Process drives
	data.TotalDrives = len(temps)
	data.MinTemperature = temps[0].Temperature
	data.MaxTemperature = temps[0].Temperature

	var totalTemp int
	var hottest, coolest *CurrentTemperature

	for i := range temps {
		t := &temps[i]
		totalTemp += t.Temperature

		// Track min/max (use <= and >= to ensure first element is captured)
		if t.Temperature <= data.MinTemperature {
			data.MinTemperature = t.Temperature
			coolest = t
		}
		if t.Temperature >= data.MaxTemperature {
			data.MaxTemperature = t.Temperature
			hottest = t
		}

		// Count by status
		switch t.Status {
		case "normal":
			data.DrivesNormal++
		case "warning":
			data.DrivesWarning++
		case "critical":
			data.DrivesCritical++
		}

		// Add to status groups if details requested
		if includeDetails {
			drive := DashboardDrive{
				Hostname:     t.Hostname,
				SerialNumber: t.SerialNumber,
				DeviceName:   t.DeviceName,
				Model:        t.Model,
				Temperature:  t.Temperature,
				Status:       t.Status,
				LastUpdated:  t.Timestamp,
			}
			data.DrivesByStatus[t.Status] = append(data.DrivesByStatus[t.Status], drive)
		}
	}

	// Calculate average
	data.AvgTemperature = math.Round(float64(totalTemp)/float64(len(temps))*10) / 10

	// Set hottest/coolest drives
	if hottest != nil {
		data.HottestDrive = &DashboardDrive{
			Hostname:     hottest.Hostname,
			SerialNumber: hottest.SerialNumber,
			DeviceName:   hottest.DeviceName,
			Model:        hottest.Model,
			Temperature:  hottest.Temperature,
			Status:       hottest.Status,
			LastUpdated:  hottest.Timestamp,
		}
	}
	if coolest != nil {
		data.CoolestDrive = &DashboardDrive{
			Hostname:     coolest.Hostname,
			SerialNumber: coolest.SerialNumber,
			DeviceName:   coolest.DeviceName,
			Model:        coolest.Model,
			Temperature:  coolest.Temperature,
			Status:       coolest.Status,
			LastUpdated:  coolest.Timestamp,
		}
	}

	// Get alert counts
	alertSummary, err := GetAlertSummary(db)
	if err == nil {
		data.ActiveAlerts = alertSummary.Unacknowledged
	}

	// Get spike counts
	spikeSummary, err := GetSpikeSummary(db)
	if err == nil {
		data.UnacknowledgedSpikes = spikeSummary.Unacknowledged
	}

	// Get recent alerts and spikes if details requested
	if includeDetails {
		data.RecentAlerts = getRecentDashboardAlerts(db, 5)
		data.RecentSpikes = getRecentDashboardSpikes(db, 5)
	}

	return data, nil
}

// GetDashboardOverview retrieves quick overview data
func GetDashboardOverview(db *sql.DB) (*DashboardOverview, error) {
	overview := &DashboardOverview{}

	// Get current temperatures (status is already calculated using thresholds)
	temps, err := GetAllCurrentTemperatures(db)
	if err != nil {
		return nil, err
	}

	if len(temps) == 0 {
		overview.Status = "normal"
		return overview, nil
	}

	overview.TotalDrives = len(temps)

	var totalTemp int
	var maxStatus string = "normal"

	for _, t := range temps {
		totalTemp += t.Temperature

		if t.Temperature > overview.MaxTemperature {
			overview.MaxTemperature = t.Temperature
		}

		// Track worst status
		switch t.Status {
		case "critical":
			overview.DrivesWithIssues++
			maxStatus = "critical"
		case "warning":
			overview.DrivesWithIssues++
			if maxStatus != "critical" {
				maxStatus = "warning"
			}
		}
	}

	overview.AvgTemperature = math.Round(float64(totalTemp)/float64(len(temps))*10) / 10
	overview.Status = maxStatus

	// Get active alert count
	alertSummary, err := GetAlertSummary(db)
	if err == nil {
		overview.ActiveAlerts = alertSummary.Unacknowledged
	}

	// Adjust status based on alerts
	if overview.ActiveAlerts > 0 && overview.Status == "normal" {
		overview.Status = "warning"
	}

	return overview, nil
}

// GetTemperatureTrends retrieves temperature trends for multiple drives
func GetTemperatureTrends(db *sql.DB, period TemperaturePeriod, limit int) ([]DriveTrend, error) {
	// Get unique drives with recent data
	var query string
	var rows *sql.Rows
	var err error

	if period == PeriodAllTime {
		// No time filter for all-time
		query = `
			SELECT DISTINCT hostname, serial_number
			FROM temperature_history
			ORDER BY hostname, serial_number
			LIMIT ?
		`
		rows, err = db.Query(query, limit)
	} else {
		query = `
			SELECT DISTINCT hostname, serial_number
			FROM temperature_history
			WHERE timestamp >= datetime('now', ?)
			ORDER BY hostname, serial_number
			LIMIT ?
		`
		periodSQL := periodToSQLInterval(period)
		rows, err = db.Query(query, periodSQL, limit)
	}

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trends []DriveTrend
	for rows.Next() {
		var hostname, serial string
		if err := rows.Scan(&hostname, &serial); err != nil {
			continue
		}

		stats, err := GetTemperatureStats(db, hostname, serial, period)
		if err != nil || stats == nil {
			continue
		}

		trend := DriveTrend{
			Hostname:     hostname,
			SerialNumber: serial,
			DeviceName:   stats.DeviceName,
			Model:        stats.Model,
			CurrentTemp:  stats.CurrentTemp,
			AvgTemp:      stats.AvgTemp,
			MinTemp:      stats.MinTemp,
			MaxTemp:      stats.MaxTemp,
			TrendSlope:   stats.TrendSlope,
			TrendDesc:    stats.TrendDesc,
		}

		trends = append(trends, trend)
	}

	return trends, nil
}

// DriveTrend holds trend data for a drive
type DriveTrend struct {
	Hostname     string  `json:"hostname"`
	SerialNumber string  `json:"serial_number"`
	DeviceName   string  `json:"device_name,omitempty"`
	Model        string  `json:"model,omitempty"`
	CurrentTemp  int     `json:"current_temp"`
	AvgTemp      float64 `json:"avg_temp"`
	MinTemp      int     `json:"min_temp"`
	MaxTemp      int     `json:"max_temp"`
	TrendSlope   float64 `json:"trend_slope"`
	TrendDesc    string  `json:"trend_desc"`
}

// GetTemperatureDistribution returns temperature distribution for histogram
func GetTemperatureDistribution(db *sql.DB) (*TemperatureDistribution, error) {
	temps, err := GetAllCurrentTemperatures(db)
	if err != nil {
		return nil, err
	}

	dist := &TemperatureDistribution{
		Buckets: make([]DistributionBucket, 0),
	}

	if len(temps) == 0 {
		return dist, nil
	}

	// Create 5-degree buckets
	buckets := make(map[int]int)
	for _, t := range temps {
		bucket := (t.Temperature / 5) * 5 // Round down to nearest 5
		buckets[bucket]++
	}

	// Convert to slice
	for temp, count := range buckets {
		dist.Buckets = append(dist.Buckets, DistributionBucket{
			RangeStart: temp,
			RangeEnd:   temp + 4,
			Count:      count,
		})
	}

	// Sort buckets by temperature
	for i := 0; i < len(dist.Buckets)-1; i++ {
		for j := i + 1; j < len(dist.Buckets); j++ {
			if dist.Buckets[i].RangeStart > dist.Buckets[j].RangeStart {
				dist.Buckets[i], dist.Buckets[j] = dist.Buckets[j], dist.Buckets[i]
			}
		}
	}

	return dist, nil
}

// TemperatureDistribution holds histogram data
type TemperatureDistribution struct {
	Buckets []DistributionBucket `json:"buckets"`
}

// DistributionBucket represents a histogram bucket
type DistributionBucket struct {
	RangeStart int `json:"range_start"`
	RangeEnd   int `json:"range_end"`
	Count      int `json:"count"`
}

// Helper: get recent alerts for dashboard
func getRecentDashboardAlerts(db *sql.DB, limit int) []DashboardAlert {
	query := `
		SELECT id, hostname, serial_number, alert_type, temperature, message, created_at
		FROM temperature_alerts
		WHERE acknowledged = 0
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var alerts []DashboardAlert
	for rows.Next() {
		var a DashboardAlert
		err := rows.Scan(&a.ID, &a.Hostname, &a.SerialNumber, &a.AlertType,
			&a.Temperature, &a.Message, &a.CreatedAt)
		if err != nil {
			continue
		}
		alerts = append(alerts, a)
	}

	return alerts
}

// Helper: get recent spikes for dashboard
func getRecentDashboardSpikes(db *sql.DB, limit int) []DashboardSpike {
	query := `
		SELECT id, hostname, serial_number, start_temp, end_temp, 
			   change_degrees, direction, created_at
		FROM temperature_spikes
		WHERE acknowledged = 0
		ORDER BY created_at DESC
		LIMIT ?
	`

	rows, err := db.Query(query, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var spikes []DashboardSpike
	for rows.Next() {
		var s DashboardSpike
		err := rows.Scan(&s.ID, &s.Hostname, &s.SerialNumber, &s.StartTemp,
			&s.EndTemp, &s.Change, &s.Direction, &s.CreatedAt)
		if err != nil {
			continue
		}
		spikes = append(spikes, s)
	}

	return spikes
}

// Helper: convert period to SQL interval
func periodToSQLInterval(p TemperaturePeriod) string {
	switch p {
	case Period24Hours:
		return "-24 hours"
	case Period7Days:
		return "-7 days"
	case Period30Days:
		return "-30 days"
	default:
		return "-365 days"
	}
}
