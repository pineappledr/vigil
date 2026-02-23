package settings

import (
	"database/sql"
	"fmt"
	"strconv"
	"time"
)

// InitSettingsTable creates the settings table and populates defaults
func InitSettingsTable(db *sql.DB) error {
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

	if _, err := db.Exec(createTableSQL); err != nil {
		return fmt.Errorf("failed to create settings table: %w", err)
	}

	insertSQL := `
	INSERT OR IGNORE INTO settings (category, key, value, value_type, description)
	VALUES (?, ?, ?, ?, ?)
	`

	for _, setting := range DefaultSettings {
		if _, err := db.Exec(insertSQL,
			setting.Category,
			setting.Key,
			setting.Value,
			setting.ValueType,
			setting.Description,
		); err != nil {
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
		var updatedAt string
		if err := rows.Scan(&s.ID, &s.Category, &s.Key, &s.Value, &s.ValueType, &s.Description, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan setting: %w", err)
		}
		s.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
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
		var updatedAt string
		if err := rows.Scan(&s.ID, &s.Category, &s.Key, &s.Value, &s.ValueType, &s.Description, &updatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan setting: %w", err)
		}
		s.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
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
	var updatedAt string
	err := db.QueryRow(query, category, key).Scan(
		&s.ID, &s.Category, &s.Key, &s.Value, &s.ValueType, &s.Description, &updatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get setting %s.%s: %w", category, key, err)
	}
	s.UpdatedAt, _ = time.Parse("2006-01-02 15:04:05", updatedAt)
	return &s, nil
}

// UpdateSetting updates the value of a specific setting
func UpdateSetting(db *sql.DB, category, key, value string) error {
	existing, err := GetSetting(db, category, key)
	if err != nil {
		return err
	}
	if existing == nil {
		return fmt.Errorf("setting %s.%s not found", category, key)
	}

	if err := validateSettingValue(existing.ValueType, value); err != nil {
		return fmt.Errorf("invalid value for %s.%s: %w", category, key, err)
	}

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
	var defaults []Setting
	for _, s := range DefaultSettings {
		if s.Category == category {
			defaults = append(defaults, s)
		}
	}

	if len(defaults) == 0 {
		return fmt.Errorf("no default settings found for category: %s", category)
	}

	for _, def := range defaults {
		if err := UpdateSetting(db, def.Category, def.Key, def.Value); err != nil {
			return fmt.Errorf("failed to reset %s.%s: %w", def.Category, def.Key, err)
		}
	}

	return nil
}

// ResetAllToDefaults resets all settings to their default values
func ResetAllToDefaults(db *sql.DB) error {
	for _, def := range DefaultSettings {
		if err := UpdateSetting(db, def.Category, def.Key, def.Value); err != nil {
			return fmt.Errorf("failed to reset %s.%s: %w", def.Category, def.Key, err)
		}
	}
	return nil
}

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
