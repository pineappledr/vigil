package settings

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open test database: %v", err)
	}

	if err := InitSettingsTable(db); err != nil {
		t.Fatalf("Failed to initialize settings table: %v", err)
	}

	return db
}

func TestInitSettingsTable(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM settings").Scan(&count); err != nil {
		t.Fatalf("Failed to query settings table: %v", err)
	}

	if count == 0 {
		t.Error("Expected default settings to be inserted")
	}

	var tempCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM settings WHERE category = 'temperature'").Scan(&tempCount); err != nil {
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

	settings, err := GetSettingsByCategory(db, "temperature")
	if err != nil {
		t.Fatalf("GetSettingsByCategory failed: %v", err)
	}

	if len(settings) == 0 {
		t.Error("Expected temperature settings")
	}

	for _, s := range settings {
		if s.Category != "temperature" {
			t.Errorf("Expected category 'temperature', got '%s'", s.Category)
		}
	}

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

	if err := UpdateSetting(db, "temperature", "warning_threshold", "50"); err != nil {
		t.Fatalf("UpdateSetting failed: %v", err)
	}

	setting, err := GetSetting(db, "temperature", "warning_threshold")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}

	if setting.Value != "50" {
		t.Errorf("Expected updated value '50', got '%s'", setting.Value)
	}

	if err := UpdateSetting(db, "nonexistent", "key", "value"); err == nil {
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
		{"temperature", "warning_threshold", "50", false},
		{"temperature", "warning_threshold", "invalid", true},
		{"alerts", "enabled", "true", false},
		{"alerts", "enabled", "false", false},
		{"alerts", "enabled", "yes", true},
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

	if err := UpdateSetting(db, "temperature", "warning_threshold", "99"); err != nil {
		t.Fatalf("UpdateSetting failed: %v", err)
	}

	if err := ResetCategoryToDefaults(db, "temperature"); err != nil {
		t.Fatalf("ResetCategoryToDefaults failed: %v", err)
	}

	setting, err := GetSetting(db, "temperature", "warning_threshold")
	if err != nil {
		t.Fatalf("GetSetting failed: %v", err)
	}

	if setting.Value != "45" {
		t.Errorf("Expected reset value '45', got '%s'", setting.Value)
	}

	if err := ResetCategoryToDefaults(db, "nonexistent"); err == nil {
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

	val := GetIntSettingWithDefault(db, "temperature", "warning_threshold", 999)
	if val != 45 {
		t.Errorf("Expected 45, got %d", val)
	}

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

	for _, cat := range []string{"temperature", "alerts", "system"} {
		if _, ok := grouped[cat]; !ok {
			t.Errorf("Expected category '%s' in grouped settings", cat)
		}
	}
}

func TestGetCategories(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	categories, err := GetCategories(db)
	if err != nil {
		t.Fatalf("GetCategories failed: %v", err)
	}

	if len(categories) < 3 {
		t.Errorf("Expected at least 3 categories, got %d", len(categories))
	}

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
