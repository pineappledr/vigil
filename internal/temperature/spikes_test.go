package temperature

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"vigil/internal/settings"
)

// setupSpikeTestDB creates an in-memory database for spike testing
func setupSpikeTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create temperature_history table
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

	// Initialize spikes table
	if err := InitTemperatureSpikesTable(db); err != nil {
		t.Fatalf("Failed to initialize spikes table: %v", err)
	}

	// Initialize settings table
	if err := settings.InitSettingsTable(db); err != nil {
		t.Fatalf("Failed to initialize settings table: %v", err)
	}

	return db
}

func TestInitTemperatureSpikesTable(t *testing.T) {
	db := setupSpikeTestDB(t)
	defer db.Close()

	// Verify table exists
	var name string
	err := db.QueryRow(`
		SELECT name FROM sqlite_master
		WHERE type='table' AND name='temperature_spikes'
	`).Scan(&name)

	if err != nil {
		t.Fatalf("Table not found: %v", err)
	}

	if name != "temperature_spikes" {
		t.Errorf("Expected table 'temperature_spikes', got '%s'", name)
	}
}

func TestRecordSpike(t *testing.T) {
	db := setupSpikeTestDB(t)
	defer db.Close()

	spike := &TemperatureSpike{
		Hostname:     "server1",
		SerialNumber: "SERIAL001",
		StartTime:    time.Now().Add(-10 * time.Minute),
		EndTime:      time.Now(),
		StartTemp:    35,
		EndTemp:      50,
		Change:       15,
		RatePerMin:   1.5,
		Direction:    "heating",
	}

	err := RecordSpike(db, spike)
	if err != nil {
		t.Fatalf("RecordSpike failed: %v", err)
	}

	if spike.ID == 0 {
		t.Error("Expected spike ID to be set")
	}

	// Verify it was saved
	var count int
	db.QueryRow("SELECT COUNT(*) FROM temperature_spikes").Scan(&count)
	if count != 1 {
		t.Errorf("Expected 1 spike, got %d", count)
	}
}

func TestGetRecentSpikes(t *testing.T) {
	db := setupSpikeTestDB(t)
	defer db.Close()

	// Insert test spikes
	for i := 0; i < 5; i++ {
		spike := &TemperatureSpike{
			Hostname:     "server1",
			SerialNumber: "SERIAL001",
			StartTime:    time.Now().Add(-time.Duration(i) * time.Hour),
			EndTime:      time.Now().Add(-time.Duration(i)*time.Hour + 10*time.Minute),
			StartTemp:    35 + i,
			EndTemp:      45 + i,
			Change:       10,
			RatePerMin:   1.0,
			Direction:    "heating",
		}
		RecordSpike(db, spike)
	}

	// Get recent spikes
	spikes, err := GetRecentSpikes(db, "server1", "SERIAL001", 3)
	if err != nil {
		t.Fatalf("GetRecentSpikes failed: %v", err)
	}

	if len(spikes) != 3 {
		t.Errorf("Expected 3 spikes, got %d", len(spikes))
	}
}

func TestGetAllRecentSpikes(t *testing.T) {
	db := setupSpikeTestDB(t)
	defer db.Close()

	// Insert spikes for different drives
	drives := []struct {
		hostname string
		serial   string
	}{
		{"server1", "SERIAL001"},
		{"server1", "SERIAL002"},
		{"server2", "SERIAL003"},
	}

	for _, d := range drives {
		spike := &TemperatureSpike{
			Hostname:     d.hostname,
			SerialNumber: d.serial,
			StartTime:    time.Now().Add(-10 * time.Minute),
			EndTime:      time.Now(),
			StartTemp:    35,
			EndTemp:      45,
			Change:       10,
			RatePerMin:   1.0,
			Direction:    "heating",
		}
		RecordSpike(db, spike)
	}

	spikes, err := GetAllRecentSpikes(db, 10)
	if err != nil {
		t.Fatalf("GetAllRecentSpikes failed: %v", err)
	}

	if len(spikes) != 3 {
		t.Errorf("Expected 3 spikes, got %d", len(spikes))
	}
}

func TestGetUnacknowledgedSpikes(t *testing.T) {
	db := setupSpikeTestDB(t)
	defer db.Close()

	// Insert 3 spikes
	for i := 0; i < 3; i++ {
		spike := &TemperatureSpike{
			Hostname:     "server1",
			SerialNumber: "SERIAL001",
			StartTime:    time.Now().Add(-time.Duration(i) * time.Hour),
			EndTime:      time.Now(),
			StartTemp:    35,
			EndTemp:      45,
			Change:       10,
			RatePerMin:   1.0,
			Direction:    "heating",
		}
		RecordSpike(db, spike)
	}

	// Acknowledge one
	AcknowledgeSpike(db, 1, "admin")

	// Get unacknowledged
	spikes, err := GetUnacknowledgedSpikes(db)
	if err != nil {
		t.Fatalf("GetUnacknowledgedSpikes failed: %v", err)
	}

	if len(spikes) != 2 {
		t.Errorf("Expected 2 unacknowledged spikes, got %d", len(spikes))
	}
}

func TestAcknowledgeSpike(t *testing.T) {
	db := setupSpikeTestDB(t)
	defer db.Close()

	// Insert spike
	spike := &TemperatureSpike{
		Hostname:     "server1",
		SerialNumber: "SERIAL001",
		StartTime:    time.Now().Add(-10 * time.Minute),
		EndTime:      time.Now(),
		StartTemp:    35,
		EndTemp:      45,
		Change:       10,
		RatePerMin:   1.0,
		Direction:    "heating",
	}
	RecordSpike(db, spike)

	// Acknowledge it
	err := AcknowledgeSpike(db, spike.ID, "testuser")
	if err != nil {
		t.Fatalf("AcknowledgeSpike failed: %v", err)
	}

	// Verify
	updated, _ := GetSpikeByID(db, spike.ID)
	if !updated.Acknowledged {
		t.Error("Expected spike to be acknowledged")
	}
	if updated.AcknowledgedBy != "testuser" {
		t.Errorf("Expected AcknowledgedBy 'testuser', got '%s'", updated.AcknowledgedBy)
	}
}

func TestDeleteSpike(t *testing.T) {
	db := setupSpikeTestDB(t)
	defer db.Close()

	// Insert spike
	spike := &TemperatureSpike{
		Hostname:     "server1",
		SerialNumber: "SERIAL001",
		StartTime:    time.Now().Add(-10 * time.Minute),
		EndTime:      time.Now(),
		StartTemp:    35,
		EndTemp:      45,
		Change:       10,
		RatePerMin:   1.0,
		Direction:    "heating",
	}
	RecordSpike(db, spike)

	// Delete it
	err := DeleteSpike(db, spike.ID)
	if err != nil {
		t.Fatalf("DeleteSpike failed: %v", err)
	}

	// Verify deletion
	deleted, _ := GetSpikeByID(db, spike.ID)
	if deleted != nil {
		t.Error("Expected spike to be deleted")
	}
}

func TestGetSpikeSummary(t *testing.T) {
	db := setupSpikeTestDB(t)
	defer db.Close()

	// Insert spikes with different timestamps
	now := time.Now()
	spikeTimes := []time.Duration{
		-30 * time.Minute,    // Last 24h
		-12 * time.Hour,      // Last 24h
		-3 * 24 * time.Hour,  // Last 7d
		-10 * 24 * time.Hour, // Older than 7d
	}

	for _, offset := range spikeTimes {
		spike := &TemperatureSpike{
			Hostname:     "server1",
			SerialNumber: "SERIAL001",
			StartTime:    now.Add(offset),
			EndTime:      now.Add(offset + 10*time.Minute),
			StartTemp:    35,
			EndTemp:      45,
			Change:       10,
			RatePerMin:   1.0,
			Direction:    "heating",
		}
		RecordSpike(db, spike)
	}

	// Acknowledge one
	AcknowledgeSpike(db, 1, "admin")

	summary, err := GetSpikeSummary(db)
	if err != nil {
		t.Fatalf("GetSpikeSummary failed: %v", err)
	}

	if summary.Total != 4 {
		t.Errorf("Total = %d, want 4", summary.Total)
	}
	if summary.Acknowledged != 1 {
		t.Errorf("Acknowledged = %d, want 1", summary.Acknowledged)
	}
	if summary.Unacknowledged != 3 {
		t.Errorf("Unacknowledged = %d, want 3", summary.Unacknowledged)
	}
}

func TestDetectSpikes(t *testing.T) {
	db := setupSpikeTestDB(t)
	defer db.Close()

	// Insert temperature data with a spike (35 -> 50 in 10 minutes)
	baseTime := time.Now().Add(-30 * time.Minute)
	temps := []struct {
		temp   int
		offset time.Duration
	}{
		{35, 0},
		{36, 2 * time.Minute},
		{38, 4 * time.Minute},
		{42, 6 * time.Minute},
		{46, 8 * time.Minute},
		{50, 10 * time.Minute}, // Spike! 15 degree change
		{49, 12 * time.Minute},
		{48, 14 * time.Minute},
	}

	for _, td := range temps {
		db.Exec(`
			INSERT INTO temperature_history (hostname, serial_number, temperature, timestamp)
			VALUES (?, ?, ?, ?)
		`, "server1", "SERIAL001", td.temp, baseTime.Add(td.offset))
	}

	// Detect spikes with 10 degree threshold
	spikes, err := DetectSpikes(db, "server1", "SERIAL001", 30, 10)
	if err != nil {
		t.Fatalf("DetectSpikes failed: %v", err)
	}

	if len(spikes) == 0 {
		t.Error("Expected at least one spike to be detected")
	}

	if len(spikes) > 0 {
		spike := spikes[0]
		if spike.Direction != "heating" {
			t.Errorf("Direction = %s, want 'heating'", spike.Direction)
		}
		if spike.Change < 10 {
			t.Errorf("Change = %d, want >= 10", spike.Change)
		}
	}
}

func TestDetectSpikesNoSpike(t *testing.T) {
	db := setupSpikeTestDB(t)
	defer db.Close()

	// Insert stable temperature data
	baseTime := time.Now().Add(-30 * time.Minute)
	for i := 0; i < 10; i++ {
		db.Exec(`
			INSERT INTO temperature_history (hostname, serial_number, temperature, timestamp)
			VALUES (?, ?, ?, ?)
		`, "server1", "SERIAL001", 38+i%3, baseTime.Add(time.Duration(i)*2*time.Minute))
	}

	// Should not detect spikes with 10 degree threshold
	spikes, err := DetectSpikes(db, "server1", "SERIAL001", 30, 10)
	if err != nil {
		t.Fatalf("DetectSpikes failed: %v", err)
	}

	if len(spikes) != 0 {
		t.Errorf("Expected no spikes, got %d", len(spikes))
	}
}

func TestDetectCoolingSpike(t *testing.T) {
	db := setupSpikeTestDB(t)
	defer db.Close()

	// Insert temperature data with a cooling spike (50 -> 35)
	baseTime := time.Now().Add(-30 * time.Minute)
	temps := []struct {
		temp   int
		offset time.Duration
	}{
		{50, 0},
		{48, 2 * time.Minute},
		{44, 4 * time.Minute},
		{40, 6 * time.Minute},
		{35, 8 * time.Minute}, // Cooling spike!
	}

	for _, td := range temps {
		db.Exec(`
			INSERT INTO temperature_history (hostname, serial_number, temperature, timestamp)
			VALUES (?, ?, ?, ?)
		`, "server1", "SERIAL001", td.temp, baseTime.Add(td.offset))
	}

	spikes, err := DetectSpikes(db, "server1", "SERIAL001", 30, 10)
	if err != nil {
		t.Fatalf("DetectSpikes failed: %v", err)
	}

	if len(spikes) == 0 {
		t.Error("Expected cooling spike to be detected")
	}

	if len(spikes) > 0 && spikes[0].Direction != "cooling" {
		t.Errorf("Direction = %s, want 'cooling'", spikes[0].Direction)
	}
}

func TestSpikeExists(t *testing.T) {
	db := setupSpikeTestDB(t)
	defer db.Close()

	startTime := time.Now().Add(-10 * time.Minute)
	endTime := time.Now()

	// Insert a spike
	spike := &TemperatureSpike{
		Hostname:     "server1",
		SerialNumber: "SERIAL001",
		StartTime:    startTime,
		EndTime:      endTime,
		StartTemp:    35,
		EndTemp:      50,
		Change:       15,
		RatePerMin:   1.5,
		Direction:    "heating",
	}
	RecordSpike(db, spike)

	// Check existence - exact match
	exists, err := spikeExists(db, "server1", "SERIAL001", startTime, endTime)
	if err != nil {
		t.Fatalf("spikeExists failed: %v", err)
	}
	if !exists {
		t.Error("Expected spike to exist")
	}

	// Check with different time - should not exist
	exists, _ = spikeExists(db, "server1", "SERIAL001",
		time.Now().Add(-2*time.Hour), time.Now().Add(-1*time.Hour))
	if exists {
		t.Error("Expected spike to not exist")
	}
}

func TestCleanupOldSpikes(t *testing.T) {
	db := setupSpikeTestDB(t)
	defer db.Close()

	// Insert old spike
	db.Exec(`
		INSERT INTO temperature_spikes (
			hostname, serial_number, start_time, end_time,
			start_temp, end_temp, change_degrees, rate_per_minute, direction, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, "server1", "SERIAL001",
		time.Now().Add(-100*24*time.Hour),
		time.Now().Add(-100*24*time.Hour),
		35, 45, 10, 1.0, "heating",
		time.Now().Add(-100*24*time.Hour))

	// Insert recent spike
	RecordSpike(db, &TemperatureSpike{
		Hostname:     "server1",
		SerialNumber: "SERIAL001",
		StartTime:    time.Now().Add(-1 * time.Hour),
		EndTime:      time.Now(),
		StartTemp:    35,
		EndTemp:      45,
		Change:       10,
		RatePerMin:   1.0,
		Direction:    "heating",
	})

	// Cleanup spikes older than 90 days
	deleted, err := CleanupOldSpikes(db, 90)
	if err != nil {
		t.Fatalf("CleanupOldSpikes failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("Deleted = %d, want 1", deleted)
	}

	// Verify only recent spike remains
	var count int
	db.QueryRow("SELECT COUNT(*) FROM temperature_spikes").Scan(&count)
	if count != 1 {
		t.Errorf("Remaining spikes = %d, want 1", count)
	}
}
