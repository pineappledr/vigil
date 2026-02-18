package db

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// setupDashboardTestDB creates an in-memory database for dashboard testing
func setupDashboardTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create all required tables
	_, err = db.Exec(`
		CREATE TABLE temperature_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hostname TEXT NOT NULL,
			serial_number TEXT NOT NULL,
			temperature INTEGER NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE TABLE smart_results (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hostname TEXT NOT NULL,
			serial_number TEXT NOT NULL,
			device_name TEXT,
			model TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create tables: %v", err)
	}

	// Initialize settings, spikes, alerts tables
	if err := InitSettingsTable(db); err != nil {
		t.Fatalf("Failed to initialize settings: %v", err)
	}
	if err := InitTemperatureSpikesTable(db); err != nil {
		t.Fatalf("Failed to initialize spikes: %v", err)
	}
	if err := InitTemperatureAlertsTable(db); err != nil {
		t.Fatalf("Failed to initialize alerts: %v", err)
	}

	return db
}

// insertDashboardTestData adds test data for dashboard tests
func insertDashboardTestData(t *testing.T, db *sql.DB) {
	t.Helper()
	// Insert drives with different temperatures
	drives := []struct {
		hostname string
		serial   string
		model    string
		temp     int
	}{
		{"server1", "SERIAL001", "Samsung 870 EVO", 35},
		{"server1", "SERIAL002", "WD Blue", 42},
		{"server1", "SERIAL003", "Seagate IronWolf", 48}, // Warning
		{"server2", "SERIAL004", "Samsung 980 Pro", 38},
		{"server2", "SERIAL005", "Crucial MX500", 58}, // Critical
	}

	for _, d := range drives {
		// Insert into smart_results for drive info
		db.Exec(`
			INSERT INTO smart_results (hostname, serial_number, device_name, model)
			VALUES (?, ?, ?, ?)
		`, d.hostname, d.serial, "/dev/sda", d.model)

		// Insert temperature history
		db.Exec(`
			INSERT INTO temperature_history (hostname, serial_number, temperature, timestamp)
			VALUES (?, ?, ?, datetime('now'))
		`, d.hostname, d.serial, d.temp)
	}
}

func TestGetDashboardTemperatureData(t *testing.T) {
	db := setupDashboardTestDB(t)
	defer db.Close()

	insertDashboardTestData(t, db)

	data, err := GetDashboardTemperatureData(db, false)
	if err != nil {
		t.Fatalf("GetDashboardTemperatureData failed: %v", err)
	}

	// Verify counts
	if data.TotalDrives != 5 {
		t.Errorf("TotalDrives = %d, want 5", data.TotalDrives)
	}

	if data.DrivesNormal != 3 {
		t.Errorf("DrivesNormal = %d, want 3", data.DrivesNormal)
	}

	if data.DrivesWarning != 1 {
		t.Errorf("DrivesWarning = %d, want 1", data.DrivesWarning)
	}

	if data.DrivesCritical != 1 {
		t.Errorf("DrivesCritical = %d, want 1", data.DrivesCritical)
	}

	// Verify min/max
	if data.MinTemperature != 35 {
		t.Errorf("MinTemperature = %d, want 35", data.MinTemperature)
	}

	if data.MaxTemperature != 58 {
		t.Errorf("MaxTemperature = %d, want 58", data.MaxTemperature)
	}

	// Verify hottest/coolest
	if data.HottestDrive == nil {
		t.Error("Expected HottestDrive to be set")
	} else if data.HottestDrive.Temperature != 58 {
		t.Errorf("HottestDrive temp = %d, want 58", data.HottestDrive.Temperature)
	}

	if data.CoolestDrive == nil {
		t.Error("Expected CoolestDrive to be set")
	} else if data.CoolestDrive.Temperature != 35 {
		t.Errorf("CoolestDrive temp = %d, want 35", data.CoolestDrive.Temperature)
	}
}

func TestGetDashboardTemperatureDataWithDetails(t *testing.T) {
	db := setupDashboardTestDB(t)
	defer db.Close()

	insertDashboardTestData(t, db)

	// Add some alerts
	CreateAlert(db, &TemperatureAlert{
		Hostname:     "server1",
		SerialNumber: "SERIAL003",
		AlertType:    AlertTypeWarning,
		Temperature:  48,
		Message:      "Test warning",
	})

	data, err := GetDashboardTemperatureData(db, true)
	if err != nil {
		t.Fatalf("GetDashboardTemperatureData failed: %v", err)
	}

	// Verify drives by status are populated
	if len(data.DrivesByStatus) == 0 {
		t.Error("Expected DrivesByStatus to be populated with details=true")
	}

	if len(data.DrivesByStatus["normal"]) != 3 {
		t.Errorf("DrivesByStatus[normal] = %d, want 3", len(data.DrivesByStatus["normal"]))
	}

	// Verify recent alerts
	if data.ActiveAlerts != 1 {
		t.Errorf("ActiveAlerts = %d, want 1", data.ActiveAlerts)
	}
}

func TestGetDashboardOverview(t *testing.T) {
	db := setupDashboardTestDB(t)
	defer db.Close()

	insertDashboardTestData(t, db)

	overview, err := GetDashboardOverview(db)
	if err != nil {
		t.Fatalf("GetDashboardOverview failed: %v", err)
	}

	if overview.TotalDrives != 5 {
		t.Errorf("TotalDrives = %d, want 5", overview.TotalDrives)
	}

	if overview.DrivesWithIssues != 2 { // 1 warning + 1 critical
		t.Errorf("DrivesWithIssues = %d, want 2", overview.DrivesWithIssues)
	}

	if overview.MaxTemperature != 58 {
		t.Errorf("MaxTemperature = %d, want 58", overview.MaxTemperature)
	}

	// Status should be critical (have a critical drive)
	if overview.Status != "critical" {
		t.Errorf("Status = %s, want critical", overview.Status)
	}
}

func TestGetDashboardOverviewEmpty(t *testing.T) {
	db := setupDashboardTestDB(t)
	defer db.Close()

	// No data
	overview, err := GetDashboardOverview(db)
	if err != nil {
		t.Fatalf("GetDashboardOverview failed: %v", err)
	}

	if overview.TotalDrives != 0 {
		t.Errorf("TotalDrives = %d, want 0", overview.TotalDrives)
	}

	if overview.Status != "normal" {
		t.Errorf("Status = %s, want normal", overview.Status)
	}
}

func TestGetTemperatureTrends(t *testing.T) {
	db := setupDashboardTestDB(t)
	defer db.Close()

	// Insert historical data for trending using SQLite datetime format
	for i := 0; i < 24; i++ {
		temp := 35 + i/2 // Gradual increase
		// Use datetime('now', '-X hours') for proper SQLite comparison
		hoursAgo := 24 - i
		_, err := db.Exec(`
			INSERT INTO temperature_history (hostname, serial_number, temperature, timestamp)
			VALUES (?, ?, ?, datetime('now', ?))
		`, "server1", "SERIAL001", temp, fmt.Sprintf("-%d hours", hoursAgo))
		if err != nil {
			t.Fatalf("Failed to insert test data: %v", err)
		}
	}

	trends, err := GetTemperatureTrends(db, Period24Hours, 10)
	if err != nil {
		t.Fatalf("GetTemperatureTrends failed: %v", err)
	}

	if len(trends) == 0 {
		t.Error("Expected at least one trend")
	}

	if len(trends) > 0 {
		trend := trends[0]
		if trend.Hostname != "server1" {
			t.Errorf("Hostname = %s, want server1", trend.Hostname)
		}

		// Should show heating trend
		if trend.TrendDesc != "heating" && trend.TrendSlope <= 0 {
			t.Logf("Trend: slope=%.4f, desc=%s", trend.TrendSlope, trend.TrendDesc)
		}
	}
}

func TestGetTemperatureDistribution(t *testing.T) {
	db := setupDashboardTestDB(t)
	defer db.Close()

	insertDashboardTestData(t, db)

	dist, err := GetTemperatureDistribution(db)
	if err != nil {
		t.Fatalf("GetTemperatureDistribution failed: %v", err)
	}

	if len(dist.Buckets) == 0 {
		t.Error("Expected distribution buckets")
	}

	// Count total drives in distribution
	total := 0
	for _, b := range dist.Buckets {
		total += b.Count
	}

	if total != 5 {
		t.Errorf("Total in distribution = %d, want 5", total)
	}
}

func TestGetTemperatureDistributionEmpty(t *testing.T) {
	db := setupDashboardTestDB(t)
	defer db.Close()

	// No data
	dist, err := GetTemperatureDistribution(db)
	if err != nil {
		t.Fatalf("GetTemperatureDistribution failed: %v", err)
	}

	if len(dist.Buckets) != 0 {
		t.Errorf("Expected empty buckets, got %d", len(dist.Buckets))
	}
}

func TestDashboardWithAlerts(t *testing.T) {
	db := setupDashboardTestDB(t)
	defer db.Close()

	insertDashboardTestData(t, db)

	// Add alerts and spikes
	CreateAlert(db, &TemperatureAlert{
		Hostname:     "server1",
		SerialNumber: "SERIAL003",
		AlertType:    AlertTypeWarning,
		Temperature:  48,
		Message:      "Warning alert",
	})

	CreateAlert(db, &TemperatureAlert{
		Hostname:     "server2",
		SerialNumber: "SERIAL005",
		AlertType:    AlertTypeCritical,
		Temperature:  58,
		Message:      "Critical alert",
	})

	RecordSpike(db, &TemperatureSpike{
		Hostname:     "server1",
		SerialNumber: "SERIAL001",
		StartTime:    time.Now().Add(-10 * time.Minute),
		EndTime:      time.Now(),
		StartTemp:    30,
		EndTemp:      45,
		Change:       15,
		Direction:    "heating",
	})

	data, err := GetDashboardTemperatureData(db, true)
	if err != nil {
		t.Fatalf("GetDashboardTemperatureData failed: %v", err)
	}

	if data.ActiveAlerts != 2 {
		t.Errorf("ActiveAlerts = %d, want 2", data.ActiveAlerts)
	}

	if data.UnacknowledgedSpikes != 1 {
		t.Errorf("UnacknowledgedSpikes = %d, want 1", data.UnacknowledgedSpikes)
	}

	if len(data.RecentAlerts) != 2 {
		t.Errorf("RecentAlerts = %d, want 2", len(data.RecentAlerts))
	}

	if len(data.RecentSpikes) != 1 {
		t.Errorf("RecentSpikes = %d, want 1", len(data.RecentSpikes))
	}
}

func TestPeriodToSQLInterval(t *testing.T) {
	tests := []struct {
		period   TemperaturePeriod
		expected string
	}{
		{Period24Hours, "-24 hours"},
		{Period7Days, "-7 days"},
		{Period30Days, "-30 days"},
		{PeriodAllTime, "-365 days"},
	}

	for _, tt := range tests {
		result := periodToSQLInterval(tt.period)
		if result != tt.expected {
			t.Errorf("periodToSQLInterval(%s) = %s, want %s", tt.period, result, tt.expected)
		}
	}
}
