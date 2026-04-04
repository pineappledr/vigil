package drivegroups

import "time"

// DriveGroup is a named group of drives with a display color.
type DriveGroup struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	Color       string    `json:"color"`
	MemberCount int       `json:"member_count,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// DriveGroupMember links a drive (hostname + serial) to a group.
type DriveGroupMember struct {
	ID           int64  `json:"id"`
	GroupID      int64  `json:"group_id"`
	Hostname     string `json:"hostname"`
	SerialNumber string `json:"serial_number"`
}

// GroupEventRule overrides notification event rules for a specific group.
type GroupEventRule struct {
	ID        int64  `json:"id"`
	ServiceID int64  `json:"service_id"`
	GroupID   int64  `json:"group_id"`
	EventType string `json:"event_type"`
	Enabled   bool   `json:"enabled"`
	Cooldown  int    `json:"cooldown_secs"`
}
