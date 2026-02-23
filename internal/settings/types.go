package settings

import "time"

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

// SettingsGrouped represents settings grouped by category
type SettingsGrouped map[string][]Setting
