package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

// Setting represents a configuration setting in the database
type Setting struct {
	ID          int64     `json:"id"`
	Category    string    `json:"category"`
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	ValueType   string    `json:"value_type"`
	Description string    `json:"description,omitempty"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SettingUpdate represents a request to update a setting
type SettingUpdate struct {
	Value string `json:"value"`
}

// DefaultSettings defines the default configuration values
var DefaultSettings = []Setting{
	// Temperature settings
	{Category: "temperature", Key: "warning_threshold", Value: "45", ValueType: "int", Description: "Temperature warning threshold in Celsius"},
	{Category: "temperature", Key: "critical_threshold", Value: "55", ValueType: "int", Description: "Temperature critical threshold in Celsius"},
	{Category: "temperature", Key: "spike_threshold", Value: "10", ValueType: "int", Description: "Temperature change considered a spike (degrees)"},
	{Category: "temperature", Key: "spike_window_minutes", Value: "30", ValueType: "int", Description: "Time window for spike detection in minutes"},
	{Category: "temperature", Key: "retention_days", Value: "90", ValueType: "int", Description: "Days to keep temperature history"},

	// Alert settings
	{Category: "alerts", Key: "enabled", Value: "true", ValueType: "bool", Description: "Enable temperature alerts"},
	{Category: "alerts", Key: "cooldown_minutes", Value: "60", ValueType: "int", Description: "Minutes between duplicate alerts for same drive"},
	{Category: "alerts", Key: "recovery_enabled", Value: "true", ValueType: "bool", Description: "Generate recovery alerts when temperature returns to normal"},

	// System settings
	{Category: "system", Key: "data_retention_days", Value: "365", ValueType: "int", Description: "Days to keep historical data"},
	{Category: "system", Key: "timezone", Value: "UTC", ValueType: "string", Description: "Display timezone for timestamps"},
}

// InitSettingsTable creates the settings table and populates defaults
func InitSettingsTable(db *sql.DB) error {
	// Create settings table
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		category TEXT NOT NULL,
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		value_type TEXT DEFAULT 'string',
		description TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(category, key)
	);

	CREATE INDEX IF NOT EXISTS idx_settings_category ON settings(category);
	CREATE INDEX IF NOT EXISTS idx_settings_category_key ON settings(category, key);
	`

	_, err := db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create settings table: %w", err)
	}

	// Insert default settings (ignore if already exist)
	insertSQL := `
	INSERT OR IGNORE INTO settings (category, key, value, value_type, description)
	VALUES (?, ?, ?, ?, ?)
	`

	for _, setting := range DefaultSettings {
		_, err := db.Exec(insertSQL,
			setting.Category,
			setting.Key,
			setting.Value,
			setting.ValueType,
			setting.Description,
		)
		if err != nil {
			return fmt.Errorf("failed to insert default setting %s.%s: %w",
				setting.Category, setting.Key, err)
		}
	}

	return nil
}

// GetAllSettings retrieves all settings from the database
func GetAllSettings(db *sql.DB) ([]Setting, error) {
	query := `
	SELECT id, category, key, value, value_type, COALESCE(description, ''), updated_at
	FROM settings
	ORDER BY category, key
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query settings: %w", err)
	}
	defer rows.Close()

	var settings []Setting
	for rows.Next() {
		var s Setting
		err := rows.Scan(&s.ID, &s.Category, &s.Key, &s.Value, &s.ValueType, &s.Description, &s.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan setting: %w", err)
		}
		settings = append(settings, s)
	}

	return settings, rows.Err()
}

// GetSettingsByCategory retrieves all settings for a specific category
func GetSettingsByCategory(db *sql.DB, category string) ([]Setting, error) {
	query := `
	SELECT id, category, key, value, value_type, COALESCE(description, ''), updated_at
	FROM settings
	WHERE category = ?
	ORDER BY key
	`

	rows, err := db.Query(query, category)
	if err != nil {
		return nil, fmt.Errorf("failed to query settings for category %s: %w", category, err)
	}
	defer rows.Close()

	var settings []Setting
	for rows.Next() {
		var s Setting
		err := rows.Scan(&s.ID, &s.Category, &s.Key, &s.Value, &s.ValueType, &s.Description, &s.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan setting: %w", err)
		}
		settings = append(settings, s)
	}

	return settings, rows.Err()
}

// GetSetting retrieves a specific setting by category and key
func GetSetting(db *sql.DB, category, key string) (*Setting, error) {
	query := `
	SELECT id, category, key, value, value_type, COALESCE(description, ''), updated_at
	FROM settings
	WHERE category = ? AND key = ?
	`

	var s Setting
	err := db.QueryRow(query, category, key).Scan(
		&s.ID, &s.Category, &s.Key, &s.Value, &s.ValueType, &s.Description, &s.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil // Setting not found
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get setting %s.%s: %w", category, key, err)
	}

	return &s, nil
}

// UpdateSetting updates the value of a specific setting
func UpdateSetting(db *sql.DB, category, key, value string) error {
	// First, get the setting to validate it exists
	existing, err := GetSetting(db, category, key)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("setting %s.%s not found", category, key)
	}

	// Validate the value based on type
	if err := validateSettingValue(existing.ValueType, value); err != nil {
		return fmt.Errorf("invalid value for %s.%s: %w", category, key, err)
	}

	// Update the setting
	query := `
	UPDATE settings
	SET value = ?, updated_at = CURRENT_TIMESTAMP
	WHERE category = ? AND key = ?
	`

	result, err := db.Exec(query, value, category, key)
	if err != nil {
		return fmt.Errorf("failed to update setting %s.%s: %w", category, key, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("setting %s.%s not found", category, key)
	}

	return nil
}

// ResetCategoryToDefaults resets all settings in a category to their default values
func ResetCategoryToDefaults(db *sql.DB, category string) error {
	// Find default settings for this category
	var defaults []Setting
	for _, s := range DefaultSettings {
		if s.Category == category {
			defaults = append(defaults, s)
		}
	}

	if len(defaults) == 0 {
		return fmt.Errorf("no default settings found for category: %s", category)
	}

	// Update each setting to its default value
	for _, def := range defaults {
		err := UpdateSetting(db, def.Category, def.Key, def.Value)
		if err != nil {
			return fmt.Errorf("failed to reset %s.%s: %w", def.Category, def.Key, err)
		}
	}

	return nil
}

// ResetAllToDefaults resets all settings to their default values
func ResetAllToDefaults(db *sql.DB) error {
	for _, def := range DefaultSettings {
		err := UpdateSetting(db, def.Category, def.Key, def.Value)
		if err != nil {
			return fmt.Errorf("failed to reset %s.%s: %w", def.Category, def.Key, err)
		}
	}
	return nil
}

// validateSettingValue validates a value against its expected type
func validateSettingValue(valueType, value string) error {
	switch valueType {
	case "int":
		_, err := strconv.Atoi(value)
		if err != nil {
			return fmt.Errorf("value must be an integer")
		}
	case "float":
		_, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return fmt.Errorf("value must be a number")
		}
	case "bool":
		if value != "true" && value != "false" {
			return fmt.Errorf("value must be 'true' or 'false'")
		}
	case "json":
		if !json.Valid([]byte(value)) {
			return fmt.Errorf("value must be valid JSON")
		}
	case "string":
		// Any string is valid
	default:
		// Unknown type, allow any value
	}
	return nil
}

// ============================================
// Helper functions for type-safe access
// ============================================

// GetIntSetting retrieves a setting as an integer
func GetIntSetting(db *sql.DB, category, key string) (int, error) {
	s, err := GetSetting(db, category, key)
	if err != nil {
		return 0, err
	}
	if s == nil {
		return 0, fmt.Errorf("setting %s.%s not found", category, key)
	}

	val, err := strconv.Atoi(s.Value)
	if err != nil {
		return 0, fmt.Errorf("setting %s.%s is not an integer: %w", category, key, err)
	}
	return val, nil
}

// GetFloatSetting retrieves a setting as a float64
func GetFloatSetting(db *sql.DB, category, key string) (float64, error) {
	s, err := GetSetting(db, category, key)
	if err != nil {
		return 0, err
	}
	if s == nil {
		return 0, fmt.Errorf("setting %s.%s not found", category, key)
	}

	val, err := strconv.ParseFloat(s.Value, 64)
	if err != nil {
		return 0, fmt.Errorf("setting %s.%s is not a number: %w", category, key, err)
	}
	return val, nil
}

// GetBoolSetting retrieves a setting as a boolean
func GetBoolSetting(db *sql.DB, category, key string) (bool, error) {
	s, err := GetSetting(db, category, key)
	if err != nil {
		return false, err
	}
	if s == nil {
		return false, fmt.Errorf("setting %s.%s not found", category, key)
	}

	return s.Value == "true", nil
}

// GetStringSetting retrieves a setting as a string
func GetStringSetting(db *sql.DB, category, key string) (string, error) {
	s, err := GetSetting(db, category, key)
	if err != nil {
		return "", err
	}
	if s == nil {
		return "", fmt.Errorf("setting %s.%s not found", category, key)
	}

	return s.Value, nil
}

// ============================================
// Settings with defaults (no error on missing)
// ============================================

// GetIntSettingWithDefault retrieves a setting as int, returning default if not found
func GetIntSettingWithDefault(db *sql.DB, category, key string, defaultVal int) int {
	val, err := GetIntSetting(db, category, key)
	if err != nil {
		return defaultVal
	}
	return val
}

// GetBoolSettingWithDefault retrieves a setting as bool, returning default if not found
func GetBoolSettingWithDefault(db *sql.DB, category, key string, defaultVal bool) bool {
	val, err := GetBoolSetting(db, category, key)
	if err != nil {
		return defaultVal
	}
	return val
}

// GetStringSettingWithDefault retrieves a setting as string, returning default if not found
func GetStringSettingWithDefault(db *sql.DB, category, key, defaultVal string) string {
	val, err := GetStringSetting(db, category, key)
	if err != nil {
		return defaultVal
	}
	return val
}

// ============================================
// Grouped settings response
// ============================================

// SettingsGrouped represents settings grouped by category
type SettingsGrouped map[string][]Setting

// GetSettingsGrouped retrieves all settings grouped by category
func GetSettingsGrouped(db *sql.DB) (SettingsGrouped, error) {
	settings, err := GetAllSettings(db)
	if err != nil {
		return nil, err
	}

	grouped := make(SettingsGrouped)
	for _, s := range settings {
		grouped[s.Category] = append(grouped[s.Category], s)
	}

	return grouped, nil
}

// GetCategories retrieves all unique setting categories
func GetCategories(db *sql.DB) ([]string, error) {
	query := `SELECT DISTINCT category FROM settings ORDER BY category`

	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query categories: %w", err)
	}
	defer rows.Close()

	var categories []string
	for rows.Next() {
		var cat string
		if err := rows.Scan(&cat); err != nil {
			return nil, fmt.Errorf("failed to scan category: %w", err)
		}
		categories = append(categories, cat)
	}

	return categories, rows.Err()
}
