package notify

import "time"

// NotificationService is a configured Shoutrrr destination.
// Stored in the existing notification_settings table.
type NotificationService struct {
	ID               int64     `json:"id"`
	Name             string    `json:"name"`
	ServiceType      string    `json:"service_type"`
	ConfigJSON       string    `json:"config_json"`
	Enabled          bool      `json:"enabled"`
	NotifyOnCritical bool      `json:"notify_on_critical"`
	NotifyOnWarning  bool      `json:"notify_on_warning"`
	NotifyOnHealthy  bool      `json:"notify_on_healthy"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

// EventRule controls per-event-type notification behaviour for a service.
type EventRule struct {
	ID        int64  `json:"id"`
	ServiceID int64  `json:"service_id"`
	EventType string `json:"event_type"`
	Enabled   bool   `json:"enabled"`
	Cooldown  int    `json:"cooldown_secs"` // minimum seconds between repeated alerts
}

// QuietHours defines a daily window during which non-critical
// notifications are suppressed.
type QuietHours struct {
	ID        int64  `json:"id"`
	ServiceID int64  `json:"service_id"`
	StartTime string `json:"start_time"` // "HH:MM" in UTC
	EndTime   string `json:"end_time"`   // "HH:MM" in UTC
	Enabled   bool   `json:"enabled"`
}

// DigestConfig controls the daily digest summary for a service.
type DigestConfig struct {
	ID        int64  `json:"id"`
	ServiceID int64  `json:"service_id"`
	Enabled   bool   `json:"enabled"`
	SendAt    string `json:"send_at"` // "HH:MM" in UTC
}

// NotificationRecord is a row from notification_history.
type NotificationRecord struct {
	ID           int64     `json:"id"`
	SettingID    int64     `json:"setting_id"`
	EventType    string    `json:"event_type"`
	Hostname     string    `json:"hostname"`
	SerialNumber string    `json:"serial_number"`
	Message      string    `json:"message"`
	Status       string    `json:"status"`
	ErrorMessage string    `json:"error_message,omitempty"`
	SentAt       time.Time `json:"sent_at,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}
