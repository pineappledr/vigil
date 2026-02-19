package temperature

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"vigil/internal/db"
)

// setupTestDB creates an in-memory database for testing
func setupTestDB(t *testing.T) *sql.DB {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Create temperature_history table
	_, err = database.Exec(`
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

	// Initialize all required tables
	if err := InitializeTables(database); err != nil {
		t.Fatalf("Failed to initialize tables: %v", err)
	}

	// Clear alert state cache
	db.ClearAlertStateCache()

	return database
}

func TestInitializeTables(t *testing.T) {
	database, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Create temperature_history first (prerequisite)
	database.Exec(`
		CREATE TABLE temperature_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			hostname TEXT NOT NULL,
			serial_number TEXT NOT NULL,
			temperature INTEGER NOT NULL,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)

	err = InitializeTables(database)
	if err != nil {
		t.Fatalf("InitializeTables failed: %v", err)
	}

	// Verify tables exist
	tables := []string{"settings", "temperature_spikes", "temperature_alerts"}
	for _, table := range tables {
		var name string
		err := database.QueryRow(`
			SELECT name FROM sqlite_master 
			WHERE type='table' AND name=?
		`, table).Scan(&name)

		if err != nil {
			t.Errorf("Table %s not found: %v", table, err)
		}
	}
}

func TestExtractTemperatureFromSMART(t *testing.T) {
	tests := []struct {
		name     string
		attrs    map[string]interface{}
		expected int
	}{
		{
			name:     "Direct temperature value",
			attrs:    map[string]interface{}{"Temperature_Celsius": 42},
			expected: 42,
		},
		{
			name:     "Temperature as float",
			attrs:    map[string]interface{}{"Temperature_Celsius": 42.5},
			expected: 42,
		},
		{
			name:     "NVMe temperature",
			attrs:    map[string]interface{}{"temperature": 38},
			expected: 38,
		},
		{
			name: "Nested raw value",
			attrs: map[string]interface{}{
				"Temperature_Celsius": map[string]interface{}{
					"raw": map[string]interface{}{
						"value": 45,
					},
				},
			},
			expected: 45,
		},
		{
			name: "Nested value only",
			attrs: map[string]interface{}{
				"Temperature_Celsius": map[string]interface{}{
					"value": 40,
				},
			},
			expected: 40,
		},
		{
			name: "NVMe temperature sensors",
			attrs: map[string]interface{}{
				"temperature_sensors": []interface{}{float64(35), float64(38)},
			},
			expected: 35,
		},
		{
			name:     "No temperature",
			attrs:    map[string]interface{}{"other_attr": 123},
			expected: 0,
		},
		{
			name:     "Empty attrs",
			attrs:    map[string]interface{}{},
			expected: 0,
		},
		{
			name:     "Nil attrs",
			attrs:    nil,
			expected: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractTemperatureFromSMART(tt.attrs)
			if result != tt.expected {
				t.Errorf("ExtractTemperatureFromSMART() = %d, want %d", result, tt.expected)
			}
		})
	}
}

func TestValidateTemperature(t *testing.T) {
	tests := []struct {
		temp     int
		expected bool
	}{
		{35, true},
		{0, true},
		{-40, true},
		{100, true},
		{-41, false},
		{101, false},
		{-100, false},
		{200, false},
	}

	for _, tt := range tests {
		result := ValidateTemperature(tt.temp)
		if result != tt.expected {
			t.Errorf("ValidateTemperature(%d) = %v, want %v", tt.temp, result, tt.expected)
		}
	}
}

func TestProcessorStartStop(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	processor := NewProcessor(database)

	// Start processor
	processor.Start()

	status := processor.GetStatus()
	if !status["running"].(bool) {
		t.Error("Processor should be running after Start()")
	}

	// Starting again should be safe
	processor.Start()

	// Stop processor
	processor.Stop()

	// Give it time to stop
	time.Sleep(100 * time.Millisecond)

	// Stopping again should be safe
	processor.Stop()
}

func TestProcessorProcessReading(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	processor := NewProcessor(database)
	processor.Start()
	defer processor.Stop()

	// Process a reading
	processor.ProcessReading("server1", "SERIAL001", 50)

	// Give it time to process
	time.Sleep(100 * time.Millisecond)

	// Check if alert was created (50°C > 45°C warning threshold)
	alerts, err := db.GetActiveAlerts(database)
	if err != nil {
		t.Fatalf("Failed to get alerts: %v", err)
	}

	if len(alerts) == 0 {
		t.Error("Expected alert to be created for temperature above warning threshold")
	}
}

func TestProcessorQueueFull(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Create processor with small queue
	processor := &Processor{
		DB:              database,
		stopChan:        make(chan struct{}),
		processingQueue: make(chan processingRequest, 1),
	}

	// Fill the queue
	processor.processingQueue <- processingRequest{
		Hostname:     "test",
		SerialNumber: "test",
		Temperature:  35,
	}

	// This should process synchronously (queue full)
	processor.ProcessReading("server1", "SERIAL001", 60)

	// Check if alert was created (processed synchronously)
	alerts, _ := db.GetActiveAlerts(database)
	if len(alerts) == 0 {
		t.Error("Expected alert from synchronous processing when queue is full")
	}
}

func TestProcessDriveTemperature(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Process a high temperature
	err := ProcessDriveTemperature(database, "server1", "SERIAL001", 60)
	if err != nil {
		t.Fatalf("ProcessDriveTemperature failed: %v", err)
	}

	// Check for alert
	alerts, _ := db.GetActiveAlerts(database)
	if len(alerts) == 0 {
		t.Error("Expected critical alert for 60°C temperature")
	}

	// Verify alert type
	if len(alerts) > 0 && alerts[0].AlertType != "critical" {
		t.Errorf("Expected critical alert, got %s", alerts[0].AlertType)
	}
}

func TestProcessDriveTemperatureInvalid(t *testing.T) {
	database := setupTestDB(t)
	defer database.Close()

	// Process invalid temperature (should be skipped)
	err := ProcessDriveTemperature(database, "server1", "SERIAL001", 150)
	if err != nil {
		t.Errorf("Expected no error for invalid temperature, got: %v", err)
	}

	// No alert should be created
	alerts, _ := db.GetActiveAlerts(database)
	if len(alerts) != 0 {
		t.Error("Expected no alerts for invalid temperature")
	}
}
