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
	WearoutWarning     EventType = "wearout_warning"
	WearoutCritical    EventType = "wearout_critical"
	WearoutPredicted   EventType = "wearout_predicted"

	// Add-on / job events
	JobStarted    EventType = "job_started"
	PhaseComplete EventType = "phase_complete"
	BurninPassed  EventType = "burnin_passed"
	JobComplete   EventType = "job_complete"
	JobFailed     EventType = "job_failed"

	// Trigger-specific job events (manual vs scheduled)
	ManualJobStarted    EventType = "manual_job_started"
	ManualJobComplete   EventType = "manual_job_complete"
	ScheduledJobStarted EventType = "scheduled_job_started"
	ScheduledJobComplete EventType = "scheduled_job_complete"

	// SnapRAID-specific add-on events
	GateFailed          EventType = "gate_failed"
	MaintenanceStarted  EventType = "maintenance_started"
	MaintenanceComplete EventType = "maintenance_complete"
	AutoFix             EventType = "auto_fix"

	// System events
	AddonDegraded EventType = "addon_degraded"
	AddonOnline   EventType = "addon_online"
)

// Category groups related event types for the notification settings UI.
type Category string

const (
	CategoryMonitoring Category = "Monitoring"
	CategoryAddonJob   Category = "Add-on / Job"
	CategorySnapRAID   Category = "SnapRAID"
	CategorySystem     Category = "System"
)

// EventTypeMeta describes an event type with its category, default severity,
// and a suggested cooldown (in seconds) for notification rules.
type EventTypeMeta struct {
	Type            EventType `json:"type"`
	Category        Category  `json:"category"`
	Label           string    `json:"label"`
	DefaultSeverity Severity  `json:"default_severity"`
	DefaultCooldown int       `json:"default_cooldown"`
	DefaultEnabled  bool      `json:"default_enabled"`
}

// AllEventTypes enumerates every known EventType for use in UI dropdowns.
var AllEventTypes = []EventType{
	// Monitoring
	SmartWarning, SmartCritical, TempAlert, TempCritical,
	ZFSPoolDegraded, ZFSPoolFaulted, ZFSDeviceFailed,
	DriveAppeared, DriveDisappeared, ReallocatedSectors,
	WearoutWarning, WearoutCritical, WearoutPredicted,
	// Add-on / job
	JobStarted, PhaseComplete, BurninPassed, JobComplete, JobFailed,
	ManualJobStarted, ManualJobComplete, ScheduledJobStarted, ScheduledJobComplete,
	// SnapRAID
	GateFailed, MaintenanceStarted, MaintenanceComplete, AutoFix,
	// System
	AddonDegraded, AddonOnline,
}

// AllEventTypeMeta provides enriched metadata for every known event type.
var AllEventTypeMeta = []EventTypeMeta{
	// Monitoring
	{SmartWarning, CategoryMonitoring, "SMART Warning", SeverityWarning, 300, true},
	{SmartCritical, CategoryMonitoring, "SMART Critical", SeverityCritical, 0, true},
	{TempAlert, CategoryMonitoring, "Temperature Alert", SeverityWarning, 600, true},
	{TempCritical, CategoryMonitoring, "Temperature Critical", SeverityCritical, 0, true},
	{ZFSPoolDegraded, CategoryMonitoring, "ZFS Pool Degraded", SeverityWarning, 300, true},
	{ZFSPoolFaulted, CategoryMonitoring, "ZFS Pool Faulted", SeverityCritical, 0, true},
	{ZFSDeviceFailed, CategoryMonitoring, "ZFS Device Failed", SeverityCritical, 0, true},
	{DriveAppeared, CategoryMonitoring, "Drive Appeared", SeverityInfo, 0, true},
	{DriveDisappeared, CategoryMonitoring, "Drive Disappeared", SeverityWarning, 0, true},
	{ReallocatedSectors, CategoryMonitoring, "Reallocated Sectors", SeverityWarning, 300, true},
	{WearoutWarning, CategoryMonitoring, "Wearout Warning", SeverityWarning, 86400, true},
	{WearoutCritical, CategoryMonitoring, "Wearout Critical", SeverityCritical, 0, true},
	{WearoutPredicted, CategoryMonitoring, "Failure Predicted", SeverityWarning, 604800, true},
	// Add-on / Job
	{JobStarted, CategoryAddonJob, "Job Started", SeverityInfo, 0, true},
	{PhaseComplete, CategoryAddonJob, "Phase Complete", SeverityInfo, 60, true},
	{BurninPassed, CategoryAddonJob, "Burn-in Passed", SeverityInfo, 0, true},
	{JobComplete, CategoryAddonJob, "Job Complete", SeverityInfo, 0, true},
	{JobFailed, CategoryAddonJob, "Job Failed", SeverityCritical, 0, true},
	{ManualJobStarted, CategoryAddonJob, "Manual Job Started", SeverityInfo, 0, false},
	{ManualJobComplete, CategoryAddonJob, "Manual Job Complete", SeverityInfo, 0, false},
	{ScheduledJobStarted, CategoryAddonJob, "Scheduled Job Started", SeverityInfo, 0, false},
	{ScheduledJobComplete, CategoryAddonJob, "Scheduled Job Complete", SeverityInfo, 0, true},
	// SnapRAID
	{GateFailed, CategorySnapRAID, "Gate Failed", SeverityWarning, 300, true},
	{MaintenanceStarted, CategorySnapRAID, "Maintenance Started", SeverityInfo, 0, true},
	{MaintenanceComplete, CategorySnapRAID, "Maintenance Complete", SeverityInfo, 0, true},
	{AutoFix, CategorySnapRAID, "Auto Fix", SeverityWarning, 0, true},
	// System
	{AddonDegraded, CategorySystem, "Add-on Degraded", SeverityWarning, 300, true},
	{AddonOnline, CategorySystem, "Add-on Online", SeverityInfo, 0, true},
}

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
