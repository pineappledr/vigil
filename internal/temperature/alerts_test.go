package temperature

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"vigil/internal/settings"
)

// setupAlertTestDB creates an in-memory database for alert testing
func setupAlertTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create required tables
	_, err = db.Exec(`
		CREATE TABLE temperature_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hostname TEXT NOT NULL,
			serial_number TEXT NOT NULL,
			temperature INTEGER NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		t.Fatalf("Failed to create temperature_history table: %v", err)
	}

	// Initialize tables
	if err := settings.InitSettingsTable(db); err != nil {
		t.Fatalf("Failed to initialize settings table: %v", err)
	}

	if err := InitTemperatureSpikesTable(db); err != nil {
		t.Fatalf("Failed to initialize spikes table: %v", err)
	}

	if err := InitTemperatureAlertsTable(db); err != nil {
		t.Fatalf("Failed to initialize alerts table: %v", err)
	}

	// Clear alert state cache for clean tests
	ClearAlertStateCache()

	return db
}

func TestInitTemperatureAlertsTable(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()

	// Verify table exists
	var name string
	err := db.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='temperature_alerts'
	`).Scan(&name)

	if err != nil {
		t.Fatalf("Table not found: %v", err)
	}

	if name != "temperature_alerts" {
		t.Errorf("Expected table 'temperature_alerts', got '%s'", name)
	}
}

func TestCreateAlert(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()

	alert := &TemperatureAlert{
		Hostname:     "server1",
		SerialNumber: "SERIAL001",
		AlertType:    AlertTypeWarning,
		Temperature:  48,
		Threshold:    45,
		Message:      "Temperature exceeds warning threshold",
	}

	err := CreateAlert(db, alert)
	if err != nil {
		t.Fatalf("CreateAlert failed: %v", err)
	}

	if alert.ID == 0 {
		t.Error("Expected alert ID to be set")
	}

	// Verify it was saved
	var count int
	db.QueryRow("SELECT COUNT(*) FROM temperature_alerts").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 alert, got %d", count)
	}
}

func TestGetAlerts(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()

	// Create multiple alerts
	alertTypes := []string{AlertTypeWarning, AlertTypeCritical, AlertTypeRecovery}
	for _, aType := range alertTypes {
		alert := &TemperatureAlert{
			Hostname:     "server1",
			SerialNumber: "SERIAL001",
			AlertType:    aType,
			Temperature:  50,
			Message:      "Test alert",
		}
		CreateAlert(db, alert)
	}

	// Get all alerts
	alerts, err := GetAlerts(db, AlertFilter{Limit: 10})
	if err != nil {
		t.Fatalf("GetAlerts failed: %v", err)
	}

	if len(alerts) != 3 {
		t.Errorf("Expected 3 alerts, got %d", len(alerts))
	}

	// Filter by type
	alerts, err = GetAlerts(db, AlertFilter{AlertType: AlertTypeCritical, Limit: 10})
	if err != nil {
		t.Fatalf("GetAlerts with filter failed: %v", err)
	}

	if len(alerts) != 1 {
		t.Errorf("Expected 1 critical alert, got %d", len(alerts))
	}
}

func TestGetActiveAlerts(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()

	// Create 3 alerts
	for i := 0; i < 3; i++ {
		alert := &TemperatureAlert{
			Hostname:     "server1",
			SerialNumber: "SERIAL001",
			AlertType:    AlertTypeWarning,
			Temperature:  50,
			Message:      "Test alert",
		}
		CreateAlert(db, alert)
	}

	// Acknowledge one
	AcknowledgeAlert(db, 1, "admin")

	// Get active (unacknowledged) alerts
	alerts, err := GetActiveAlerts(db)
	if err != nil {
		t.Fatalf("GetActiveAlerts failed: %v", err)
	}

	if len(alerts) != 2 {
		t.Errorf("Expected 2 active alerts, got %d", len(alerts))
	}
}

func TestAcknowledgeAlert(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()

	// Create alert
	alert := &TemperatureAlert{
		Hostname:     "server1",
		SerialNumber: "SERIAL001",
		AlertType:    AlertTypeWarning,
		Temperature:  50,
		Message:      "Test alert",
	}
	CreateAlert(db, alert)

	// Acknowledge it
	err := AcknowledgeAlert(db, alert.ID, "testuser")
	if err != nil {
		t.Fatalf("AcknowledgeAlert failed: %v", err)
	}

	// Verify
	updated, _ := GetAlertByID(db, alert.ID)
	if !updated.Acknowledged {
		t.Error("Expected alert to be acknowledged")
	}
	if updated.AcknowledgedBy != "testuser" {
		t.Errorf("Expected AcknowledgedBy 'testuser', got '%s'", updated.AcknowledgedBy)
	}
}

func TestAcknowledgeAllAlerts(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()

	// Create multiple alerts
	for i := 0; i < 5; i++ {
		alert := &TemperatureAlert{
			Hostname:     "server1",
			SerialNumber: "SERIAL001",
			AlertType:    AlertTypeWarning,
			Temperature:  50,
			Message:      "Test alert",
		}
		CreateAlert(db, alert)
	}

	// Acknowledge all
	count, err := AcknowledgeAllAlerts(db, "admin")
	if err != nil {
		t.Fatalf("AcknowledgeAllAlerts failed: %v", err)
	}

	if count != 5 {
		t.Errorf("Expected 5 acknowledged, got %d", count)
	}

	// Verify no active alerts remain
	active, _ := GetActiveAlerts(db)
	if len(active) != 0 {
		t.Errorf("Expected 0 active alerts, got %d", len(active))
	}
}

func TestDeleteAlert(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()

	// Create alert
	alert := &TemperatureAlert{
		Hostname:     "server1",
		SerialNumber: "SERIAL001",
		AlertType:    AlertTypeWarning,
		Temperature:  50,
		Message:      "Test alert",
	}
	CreateAlert(db, alert)

	// Delete it
	err := DeleteAlert(db, alert.ID)
	if err != nil {
		t.Fatalf("DeleteAlert failed: %v", err)
	}

	// Verify deletion
	deleted, _ := GetAlertByID(db, alert.ID)
	if deleted != nil {
		t.Error("Expected alert to be deleted")
	}
}

func TestGetAlertSummary(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()

	// Create alerts of different types
	alertTypes := []string{
		AlertTypeWarning, AlertTypeWarning,
		AlertTypeCritical,
		AlertTypeSpike, AlertTypeSpike,
		AlertTypeRecovery,
	}

	for _, aType := range alertTypes {
		alert := &TemperatureAlert{
			Hostname:     "server1",
			SerialNumber: "SERIAL001",
			AlertType:    aType,
			Temperature:  50,
			Message:      "Test alert",
		}
		CreateAlert(db, alert)
	}

	// Acknowledge some
	AcknowledgeAlert(db, 1, "admin")
	AcknowledgeAlert(db, 2, "admin")

	summary, err := GetAlertSummary(db)
	if err != nil {
		t.Fatalf("GetAlertSummary failed: %v", err)
	}

	if summary.Total != 6 {
		t.Errorf("Total = %d, want 6", summary.Total)
	}
	if summary.Warning != 2 {
		t.Errorf("Warning = %d, want 2", summary.Warning)
	}
	if summary.Critical != 1 {
		t.Errorf("Critical = %d, want 1", summary.Critical)
	}
	if summary.Spike != 2 {
		t.Errorf("Spike = %d, want 2", summary.Spike)
	}
	if summary.Recovery != 1 {
		t.Errorf("Recovery = %d, want 1", summary.Recovery)
	}
	if summary.Acknowledged != 2 {
		t.Errorf("Acknowledged = %d, want 2", summary.Acknowledged)
	}
	if summary.Unacknowledged != 4 {
		t.Errorf("Unacknowledged = %d, want 4", summary.Unacknowledged)
	}
}

func TestCheckTemperatureAndAlert_Warning(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()
	ClearAlertStateCache()

	// Temperature above warning but below critical
	alert, err := CheckTemperatureAndAlert(db, "server1", "SERIAL001", 48)
	if err != nil {
		t.Fatalf("CheckTemperatureAndAlert failed: %v", err)
	}

	if alert == nil {
		t.Fatal("Expected warning alert to be created")
	}

	if alert.AlertType != AlertTypeWarning {
		t.Errorf("AlertType = %s, want warning", alert.AlertType)
	}
}

func TestCheckTemperatureAndAlert_Critical(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()
	ClearAlertStateCache()

	// Temperature above critical
	alert, err := CheckTemperatureAndAlert(db, "server1", "SERIAL001", 60)
	if err != nil {
		t.Fatalf("CheckTemperatureAndAlert failed: %v", err)
	}

	if alert == nil {
		t.Fatal("Expected critical alert to be created")
	}

	if alert.AlertType != AlertTypeCritical {
		t.Errorf("AlertType = %s, want critical", alert.AlertType)
	}
}

func TestCheckTemperatureAndAlert_Normal(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()
	ClearAlertStateCache()

	// Normal temperature
	alert, err := CheckTemperatureAndAlert(db, "server1", "SERIAL001", 35)
	if err != nil {
		t.Fatalf("CheckTemperatureAndAlert failed: %v", err)
	}

	if alert != nil {
		t.Error("Expected no alert for normal temperature")
	}
}

func TestCheckTemperatureAndAlert_Recovery(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()
	ClearAlertStateCache()

	// First, trigger a warning alert
	CheckTemperatureAndAlert(db, "server1", "SERIAL001", 50)

	// Then recover
	alert, err := CheckTemperatureAndAlert(db, "server1", "SERIAL001", 35)
	if err != nil {
		t.Fatalf("CheckTemperatureAndAlert failed: %v", err)
	}

	if alert == nil {
		t.Fatal("Expected recovery alert to be created")
	}

	if alert.AlertType != AlertTypeRecovery {
		t.Errorf("AlertType = %s, want recovery", alert.AlertType)
	}
}

func TestCheckTemperatureAndAlert_Cooldown(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()
	ClearAlertStateCache()

	// Set a very long cooldown for testing
	settings.UpdateSetting(db, "alerts", "cooldown_minutes", "60")

	// First warning alert
	alert1, _ := CheckTemperatureAndAlert(db, "server1", "SERIAL001", 50)
	if alert1 == nil {
		t.Fatal("Expected first alert")
	}

	// Second warning alert should be suppressed (within cooldown)
	alert2, _ := CheckTemperatureAndAlert(db, "server1", "SERIAL001", 50)
	if alert2 != nil {
		t.Error("Expected second alert to be suppressed due to cooldown")
	}
}

func TestCheckTemperatureAndAlert_Disabled(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()
	ClearAlertStateCache()

	// Disable alerts
	settings.UpdateSetting(db, "alerts", "enabled", "false")

	alert, _ := CheckTemperatureAndAlert(db, "server1", "SERIAL001", 60)
	if alert != nil {
		t.Error("Expected no alert when alerts are disabled")
	}
}

func TestCreateSpikeAlert(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()

	spike := &TemperatureSpike{
		Hostname:     "server1",
		SerialNumber: "SERIAL001",
		StartTime:    time.Now().Add(-10 * time.Minute),
		EndTime:      time.Now(),
		StartTemp:    35,
		EndTemp:      50,
		Change:       15,
		Direction:    "heating",
	}

	alert, err := CreateSpikeAlert(db, spike)
	if err != nil {
		t.Fatalf("CreateSpikeAlert failed: %v", err)
	}

	if alert == nil {
		t.Fatal("Expected spike alert to be created")
	}

	if alert.AlertType != AlertTypeSpike {
		t.Errorf("AlertType = %s, want spike", alert.AlertType)
	}
}

func TestGetDriveAlertStatus(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()

	// No alerts - should be normal
	status, _ := GetDriveAlertStatus(db, "server1", "SERIAL001")
	if status != "normal" {
		t.Errorf("Status = %s, want normal", status)
	}

	// Add a warning alert
	CreateAlert(db, &TemperatureAlert{
		Hostname:     "server1",
		SerialNumber: "SERIAL001",
		AlertType:    AlertTypeWarning,
		Temperature:  50,
		Message:      "Test",
	})

	status, _ = GetDriveAlertStatus(db, "server1", "SERIAL001")
	if status != AlertTypeWarning {
		t.Errorf("Status = %s, want warning", status)
	}

	// Add a critical alert (should take priority)
	CreateAlert(db, &TemperatureAlert{
		Hostname:     "server1",
		SerialNumber: "SERIAL001",
		AlertType:    AlertTypeCritical,
		Temperature:  60,
		Message:      "Test",
	})

	status, _ = GetDriveAlertStatus(db, "server1", "SERIAL001")
	if status != AlertTypeCritical {
		t.Errorf("Status = %s, want critical", status)
	}
}

func TestCleanupOldAlerts(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()

	// Insert old alert (100 days ago)
	db.Exec(`
		INSERT INTO temperature_alerts (
			hostname, serial_number, alert_type, temperature, message, created_at
		) VALUES (?, ?, ?, ?, ?, ?)
	`, "server1", "SERIAL001", "warning", 50, "Old alert",
		time.Now().Add(-100*24*time.Hour))

	// Insert recent alert
	CreateAlert(db, &TemperatureAlert{
		Hostname:     "server1",
		SerialNumber: "SERIAL001",
		AlertType:    AlertTypeWarning,
		Temperature:  50,
		Message:      "Recent alert",
	})

	// Cleanup alerts older than 90 days
	deleted, err := CleanupOldAlerts(db, 90)
	if err != nil {
		t.Fatalf("CleanupOldAlerts failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("Deleted = %d, want 1", deleted)
	}

	// Verify only recent alert remains
	var count int
	db.QueryRow("SELECT COUNT(*) FROM temperature_alerts").Scan(&count)
	if count != 1 {
		t.Errorf("Remaining alerts = %d, want 1", count)
	}
}

func TestAlertFilter(t *testing.T) {
	db := setupAlertTestDB(t)
	defer db.Close()

	// Create alerts for different drives
	drives := []struct {
		hostname string
		serial   string
	}{
		{"server1", "SERIAL001"},
		{"server1", "SERIAL002"},
		{"server2", "SERIAL003"},
	}

	for _, d := range drives {
		CreateAlert(db, &TemperatureAlert{
			Hostname:     d.hostname,
			SerialNumber: d.serial,
			AlertType:    AlertTypeWarning,
			Temperature:  50,
			Message:      "Test",
		})
	}

	// Filter by hostname
	alerts, _ := GetAlerts(db, AlertFilter{Hostname: "server1", Limit: 10})
	if len(alerts) != 2 {
		t.Errorf("Expected 2 alerts for server1, got %d", len(alerts))
	}

	// Filter by hostname and serial
	alerts, _ = GetAlerts(db, AlertFilter{
		Hostname:     "server1",
		SerialNumber: "SERIAL001",
		Limit:        10,
	})
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert for server1/SERIAL001, got %d", len(alerts))
	}
}
