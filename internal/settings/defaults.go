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

	// Retention settings.
	// For *_days keys: 0 means "keep forever" (no time-based pruning).
	{Category: "retention", Key: "notification_history_days", Value: "90", ValueType: "int", Description: "Days to keep notification history (0 = forever)"},
	{Category: "retention", Key: "smart_data_days", Value: "90", ValueType: "int", Description: "Days to keep SMART attribute and temperature history (0 = forever)"},
	{Category: "retention", Key: "report_history_days", Value: "90", ValueType: "int", Description: "Days to keep agent report history (0 = forever)"},
	{Category: "retention", Key: "audit_log_days", Value: "90", ValueType: "int", Description: "Days to keep audit / activity log entries (0 = forever)"},
	{Category: "retention", Key: "addon_data_days", Value: "0", ValueType: "int", Description: "Auto-remove add-ons that have been offline this many days, and their notification history (0 = forever)"},
	{Category: "retention", Key: "host_history_limit", Value: "50", ValueType: "int", Description: "Maximum report history entries per host"},
	{Category: "retention", Key: "notification_display_limit", Value: "50", ValueType: "int", Description: "Default number of notification history entries to display"},

	// Agent settings
	{Category: "agents", Key: "report_interval_seconds", Value: "3600", ValueType: "int", Description: "How often agents send reports (seconds). Presets: 60 / 900 / 1800 / 3600 / 43200 / 86400. The online/offline threshold is derived from this."},

	// ZFS settings
	{Category: "zfs", Key: "capacity_warning_pct", Value: "80", ValueType: "int", Description: "ZFS pool capacity warning threshold (%)"},
	{Category: "zfs", Key: "capacity_critical_pct", Value: "90", ValueType: "int", Description: "ZFS pool capacity critical threshold (%)"},
	{Category: "zfs", Key: "fragmentation_warning_pct", Value: "75", ValueType: "int", Description: "ZFS pool fragmentation warning threshold (%)"},
	{Category: "zfs", Key: "vdev_error_threshold", Value: "1", ValueType: "int", Description: "Minimum vdev error count to trigger notification"},
	{Category: "zfs", Key: "scrub_overdue_days", Value: "14", ValueType: "int", Description: "Days since last scrub before triggering overdue alert"},
	{Category: "zfs", Key: "dataset_quota_warning_pct", Value: "85", ValueType: "int", Description: "Dataset quota usage percentage to trigger warning"},

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
