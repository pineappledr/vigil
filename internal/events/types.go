package events

import "time"

// EventType identifies the kind of event being published.
type EventType string

const (
	// Monitoring events
	SmartWarning       EventType = "smart_warning"
	SmartCritical      EventType = "smart_critical"
	TempAlert          EventType = "temp_alert"
	TempCritical       EventType = "temp_critical"
	ZFSPoolDegraded    EventType = "zfs_pool_degraded"
	ZFSPoolFaulted     EventType = "zfs_pool_faulted"
	ZFSDeviceFailed    EventType = "zfs_device_failed"
	DriveAppeared      EventType = "drive_appeared"
	DriveDisappeared   EventType = "drive_disappeared"
	ReallocatedSectors EventType = "reallocated_sectors"

	// Add-on / job events
	JobStarted    EventType = "job_started"
	PhaseComplete EventType = "phase_complete"
	BurninPassed  EventType = "burnin_passed"
	JobComplete   EventType = "job_complete"
	JobFailed     EventType = "job_failed"

	// System events
	AddonDegraded EventType = "addon_degraded"
	AddonOnline   EventType = "addon_online"
)

// Severity indicates the urgency of an event.
type Severity int

const (
	SeverityInfo     Severity = 0
	SeverityWarning  Severity = 1
	SeverityCritical Severity = 2
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Event is the payload published through the bus.
type Event struct {
	Type         EventType         `json:"type"`
	Severity     Severity          `json:"severity"`
	Hostname     string            `json:"hostname,omitempty"`
	SerialNumber string            `json:"serial_number,omitempty"`
	Message      string            `json:"message"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	Timestamp    time.Time         `json:"timestamp"`
}
