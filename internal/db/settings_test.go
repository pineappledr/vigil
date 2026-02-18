package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

// setupTestDB creates an in-memory SQLite database for testing
func setupTestDB(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	// Initialize settings table
	if err := InitSettingsTable(db); err != nil {
		t.Fatalf("Failed to initialize settings table: %v", err)
	}

	return db
}

func TestInitSettingsTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Verify table exists
	var count int
	err := db.QueryRow("SELECT COUNT(*) FROM settings").Scan(&count)
	if err != nil {
		t.Fatalf("Failed to query settings table: %v", err)
	}

	// Should have default settings
	if count == 0 {
		t.Error("Expected default settings to be inserted")
	}

	// Verify we have at least the temperature settings
	var tempCount int
	err = db.QueryRow("SELECT COUNT(*) FROM settings WHERE category = 'temperature'").Scan(&tempCount)
	if err != nil {
		t.Fatalf("Failed to query temperature settings: %v", err)
	}

	if tempCount < 5 {
		t.Errorf("Expected at least 5 temperature settings, got %d", tempCount)
	}
}

func TestGetAllSettings(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	settings, err := GetAllSettings(db)
	if err != nil {
		t.Fatalf("GetAllSettings failed: %v", err)
	}

	if len(settings) == 0 {
		t.Error("Expected settings to be returned")
	}

	// Verify settings have required fields
	for _, s := range settings {
		if s.Category == "" {
			t.Error("Setting missing category")
		}
		if s.Key == "" {
			t.Error("Setting missing key")
		}
		if s.Value == "" {
			t.Error("Setting missing value")
		}
	}
}

func TestGetSettingsByCategory(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Get temperature settings
	settings, err := GetSettingsByCategory(db, "temperature")
	if err != nil {
		t.Fatalf("GetSettingsByCategory failed: %v", err)
	}

	if len(settings) == 0 {
		t.Error("Expected temperature settings")
	}

	// All returned settings should be in temperature category
	for _, s := range settings {
		if s.Category != "temperature" {
			t.Errorf("Expected category 'temperature', got '%s'", s.Category)
		}
	}

	// Non-existent category should return empty
	empty, err := GetSettingsByCategory(db, "nonexistent")
	if err != nil {
		t.Fatalf("GetSettingsByCategory failed for nonexistent: %v", err)
	}
	if len(empty) != 0 {
		t.Errorf("Expected empty result for nonexistent category, got %d", len(empty))
	}
}

func TestGetSetting(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Get existing setting
	setting, err := GetSetting(db, "temperature", "warning_threshold")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}

	if setting == nil {
		t.Fatal("Expected setting to be returned")
	}

	if setting.Value != "45" {
		t.Errorf("Expected default value '45', got '%s'", setting.Value)
	}

	if setting.ValueType != "int" {
		t.Errorf("Expected value type 'int', got '%s'", setting.ValueType)
	}

	// Non-existent setting should return nil
	notFound, err := GetSetting(db, "nonexistent", "key")
	if err != nil {
		t.Fatalf("GetSetting failed for nonexistent: %v", err)
	}
	if notFound != nil {
		t.Error("Expected nil for nonexistent setting")
	}
}

func TestUpdateSetting(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Update a setting
	err := UpdateSetting(db, "temperature", "warning_threshold", "50")
	if err != nil {
		t.Fatalf("UpdateSetting failed: %v", err)
	}

	// Verify the update
	setting, err := GetSetting(db, "temperature", "warning_threshold")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}

	if setting.Value != "50" {
		t.Errorf("Expected updated value '50', got '%s'", setting.Value)
	}

	// Try to update nonexistent setting
	err = UpdateSetting(db, "nonexistent", "key", "value")
	if err == nil {
		t.Error("Expected error for nonexistent setting")
	}
}

func TestUpdateSettingValidation(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tests := []struct {
		category string
		key      string
		value    string
		wantErr  bool
	}{
		{"temperature", "warning_threshold", "50", false},     // Valid int
		{"temperature", "warning_threshold", "invalid", true}, // Invalid int
		{"alerts", "enabled", "true", false},                  // Valid bool
		{"alerts", "enabled", "false", false},                 // Valid bool
		{"alerts", "enabled", "yes", true},                    // Invalid bool
	}

	for _, tt := range tests {
		err := UpdateSetting(db, tt.category, tt.key, tt.value)
		if (err != nil) != tt.wantErr {
			t.Errorf("UpdateSetting(%s, %s, %s) error = %v, wantErr %v",
				tt.category, tt.key, tt.value, err, tt.wantErr)
		}
	}
}

func TestResetCategoryToDefaults(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Modify a setting
	err := UpdateSetting(db, "temperature", "warning_threshold", "99")
	if err != nil {
		t.Fatalf("UpdateSetting failed: %v", err)
	}

	// Reset category
	err = ResetCategoryToDefaults(db, "temperature")
	if err != nil {
		t.Fatalf("ResetCategoryToDefaults failed: %v", err)
	}

	// Verify reset to default
	setting, err := GetSetting(db, "temperature", "warning_threshold")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}

	if setting.Value != "45" {
		t.Errorf("Expected reset value '45', got '%s'", setting.Value)
	}

	// Reset nonexistent category should fail
	err = ResetCategoryToDefaults(db, "nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent category")
	}
}

func TestGetIntSetting(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	val, err := GetIntSetting(db, "temperature", "warning_threshold")
	if err != nil {
		t.Fatalf("GetIntSetting failed: %v", err)
	}

	if val != 45 {
		t.Errorf("Expected 45, got %d", val)
	}
}

func TestGetBoolSetting(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	val, err := GetBoolSetting(db, "alerts", "enabled")
	if err != nil {
		t.Fatalf("GetBoolSetting failed: %v", err)
	}

	if !val {
		t.Error("Expected true, got false")
	}
}

func TestGetIntSettingWithDefault(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Existing setting
	val := GetIntSettingWithDefault(db, "temperature", "warning_threshold", 999)
	if val != 45 {
		t.Errorf("Expected 45, got %d", val)
	}

	// Non-existent setting should return default
	val = GetIntSettingWithDefault(db, "nonexistent", "key", 999)
	if val != 999 {
		t.Errorf("Expected default 999, got %d", val)
	}
}

func TestGetSettingsGrouped(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	grouped, err := GetSettingsGrouped(db)
	if err != nil {
		t.Fatalf("GetSettingsGrouped failed: %v", err)
	}

	// Should have temperature, alerts, and system categories
	if _, ok := grouped["temperature"]; !ok {
		t.Error("Expected temperature category in grouped settings")
	}

	if _, ok := grouped["alerts"]; !ok {
		t.Error("Expected alerts category in grouped settings")
	}

	if _, ok := grouped["system"]; !ok {
		t.Error("Expected system category in grouped settings")
	}
}

func TestGetCategories(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	categories, err := GetCategories(db)
	if err != nil {
		t.Fatalf("GetCategories failed: %v", err)
	}

	// Should have at least 3 categories
	if len(categories) < 3 {
		t.Errorf("Expected at least 3 categories, got %d", len(categories))
	}

	// Check for expected categories
	expected := map[string]bool{"temperature": false, "alerts": false, "system": false}
	for _, cat := range categories {
		if _, ok := expected[cat]; ok {
			expected[cat] = true
		}
	}

	for cat, found := range expected {
		if !found {
			t.Errorf("Expected category '%s' not found", cat)
		}
	}
}
