package settings

import (
	"encoding/json"
	"fmt"
	"strconv"
)

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

	// Retention settings
	{Category: "retention", Key: "notification_history_days", Value: "90", ValueType: "int", Description: "Days to keep notification history"},
	{Category: "retention", Key: "smart_data_days", Value: "90", ValueType: "int", Description: "Days to keep SMART attribute and temperature history"},
	{Category: "retention", Key: "host_history_limit", Value: "50", ValueType: "int", Description: "Maximum report history entries per host"},
	{Category: "retention", Key: "notification_display_limit", Value: "50", ValueType: "int", Description: "Default number of notification history entries to display"},

	// Backup settings
	{Category: "backup", Key: "enabled", Value: "true", ValueType: "bool", Description: "Enable scheduled database backups"},
	{Category: "backup", Key: "interval_hours", Value: "24", ValueType: "int", Description: "Hours between automatic backups"},
	{Category: "backup", Key: "max_backups", Value: "7", ValueType: "int", Description: "Maximum number of backup files to retain"},
}

// validateSettingValue validates a value against its expected type
func validateSettingValue(valueType, value string) error {
	switch valueType {
	case "int":
		if _, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("value must be an integer")
		}
	case "float":
		if _, err := strconv.ParseFloat(value, 64); err != nil {
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
	}
	return nil
}
