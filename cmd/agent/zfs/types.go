package zfs

import "time"

// ─── Pool Data Structures ────────────────────────────────────────────────────

// Pool represents a ZFS storage pool
type Pool struct {
	Hostname       string    `json:"hostname"`
	Name           string    `json:"name"`
	GUID           string    `json:"guid,omitempty"`
	Status         string    `json:"status"`          // ONLINE, DEGRADED, FAULTED, OFFLINE, REMOVED, UNAVAIL
	Health         string    `json:"health"`          // ONLINE, DEGRADED, FAULTED
	Size           int64     `json:"size_bytes"`      // Total size in bytes
	Allocated      int64     `json:"allocated_bytes"` // Used space in bytes
	Free           int64     `json:"free_bytes"`      // Free space in bytes
	Fragmentation  int       `json:"fragmentation"`   // Fragmentation percentage
	CapacityPct    int       `json:"capacity_pct"`    // Capacity percentage used
	DedupRatio     float64   `json:"dedup_ratio"`     // Deduplication ratio
	Altroot        string    `json:"altroot,omitempty"`
	ReadErrors     int64     `json:"read_errors"`
	WriteErrors    int64     `json:"write_errors"`
	ChecksumErrors int64     `json:"checksum_errors"`
	Scan           *ScanInfo `json:"scan,omitempty"`
	Devices        []Device  `json:"devices,omitempty"`
	LastSeen       time.Time `json:"last_seen"`
}

// ScanInfo represents scrub or resilver operation status
type ScanInfo struct {
	Function      string    `json:"function"` // scrub, resilver, none
	State         string    `json:"state"`    // scanning, finished, canceled, none
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time,omitempty"`
	Duration      int64     `json:"duration_secs,omitempty"`
	DataExamined  int64     `json:"data_examined"` // Bytes examined
	DataTotal     int64     `json:"data_total"`    // Total bytes to examine
	ErrorsFound   int64     `json:"errors_found"`
	BytesRepaired int64     `json:"bytes_repaired"`
	ProgressPct   float64   `json:"progress_pct"`
	Rate          int64     `json:"rate_bytes_sec"` // Scan rate in bytes/sec
	TimeRemaining int64     `json:"time_remaining"` // Estimated seconds remaining
}

// ─── Device Data Structures ──────────────────────────────────────────────────

// Device represents a device within a ZFS pool
type Device struct {
	Name           string   `json:"name"`           // Device name (e.g., sda, nvme0n1)
	Path           string   `json:"path,omitempty"` // Full path (e.g., /dev/sda)
	GUID           string   `json:"guid,omitempty"`
	SerialNumber   string   `json:"serial_number,omitempty"` // Linked from SMART
	VdevType       string   `json:"vdev_type"`               // disk, mirror, raidz1, raidz2, raidz3, spare, log, cache
	VdevParent     string   `json:"vdev_parent,omitempty"`   // Parent vdev name for nested structures
	VdevIndex      int      `json:"vdev_index"`              // Position in vdev
	State          string   `json:"state"`                   // ONLINE, DEGRADED, FAULTED, OFFLINE, REMOVED, UNAVAIL
	ReadErrors     int64    `json:"read_errors"`
	WriteErrors    int64    `json:"write_errors"`
	ChecksumErrors int64    `json:"checksum_errors"`
	Size           int64    `json:"size_bytes,omitempty"`
	Allocated      int64    `json:"allocated_bytes,omitempty"`
	IsSpare        bool     `json:"is_spare"`
	IsLog          bool     `json:"is_log"`
	IsCache        bool     `json:"is_cache"`
	IsReplacing    bool     `json:"is_replacing"`
	Children       []Device `json:"children,omitempty"` // For mirror/raidz vdevs
}

// ─── Report Structures ───────────────────────────────────────────────────────

// ZFSReport contains all ZFS data for a host
type ZFSReport struct {
	Hostname  string    `json:"hostname"`
	Timestamp time.Time `json:"timestamp"`
	Available bool      `json:"zfs_available"` // Whether ZFS is installed/available
	Pools     []Pool    `json:"pools"`
}

// ─── Scrub History ───────────────────────────────────────────────────────────

// ScrubRecord represents a historical scrub/resilver operation
type ScrubRecord struct {
	ID             int64     `json:"id,omitempty"`
	PoolID         int64     `json:"pool_id"`
	Hostname       string    `json:"hostname"`
	PoolName       string    `json:"pool_name"`
	ScanType       string    `json:"scan_type"` // scrub, resilver
	State          string    `json:"state"`     // finished, canceled, in_progress
	StartTime      time.Time `json:"start_time"`
	EndTime        time.Time `json:"end_time,omitempty"`
	Duration       int64     `json:"duration_secs,omitempty"`
	DataExamined   int64     `json:"data_examined"`
	DataTotal      int64     `json:"data_total"`
	ErrorsFound    int64     `json:"errors_found"`
	BytesRepaired  int64     `json:"bytes_repaired"`
	BlocksRepaired int64     `json:"blocks_repaired"`
	ProgressPct    float64   `json:"progress_pct"`
	Rate           int64     `json:"rate_bytes_sec"`
	TimeRemaining  int64     `json:"time_remaining,omitempty"`
}

// ─── Health Status Constants ─────────────────────────────────────────────────

const (
	// Pool/Device States
	StateOnline   = "ONLINE"
	StateDegraded = "DEGRADED"
	StateFaulted  = "FAULTED"
	StateOffline  = "OFFLINE"
	StateRemoved  = "REMOVED"
	StateUnavail  = "UNAVAIL"

	// Scan Functions
	ScanNone     = "none"
	ScanScrub    = "scrub"
	ScanResilver = "resilver"

	// Scan States
	ScanStateNone     = "none"
	ScanStateScanning = "scanning"
	ScanStateFinished = "finished"
	ScanStateCanceled = "canceled"

	// Vdev Types
	VdevTypeDisk   = "disk"
	VdevTypeMirror = "mirror"
	VdevTypeRaidz1 = "raidz1"
	VdevTypeRaidz2 = "raidz2"
	VdevTypeRaidz3 = "raidz3"
	VdevTypeSpare  = "spare"
	VdevTypeLog    = "log"
	VdevTypeCache  = "cache"
)

// ─── Helper Functions ────────────────────────────────────────────────────────

// IsHealthy returns true if the pool is in a healthy state
func (p *Pool) IsHealthy() bool {
	return p.Health == StateOnline
}

// IsDegraded returns true if the pool is degraded
func (p *Pool) IsDegraded() bool {
	return p.Health == StateDegraded
}

// IsFaulted returns true if the pool is faulted
func (p *Pool) IsFaulted() bool {
	return p.Health == StateFaulted
}

// HasErrors returns true if the pool has any errors
func (p *Pool) HasErrors() bool {
	return p.ReadErrors > 0 || p.WriteErrors > 0 || p.ChecksumErrors > 0
}

// TotalErrors returns the sum of all error types
func (p *Pool) TotalErrors() int64 {
	return p.ReadErrors + p.WriteErrors + p.ChecksumErrors
}

// IsScanning returns true if a scrub or resilver is in progress
func (p *Pool) IsScanning() bool {
	return p.Scan != nil && p.Scan.State == ScanStateScanning
}

// DeviceCount returns the total number of data devices (excluding spares, logs, cache)
func (p *Pool) DeviceCount() int {
	count := 0
	for _, d := range p.Devices {
		if !d.IsSpare && !d.IsLog && !d.IsCache {
			count += countDataDevices(d)
		}
	}
	return count
}

// countDataDevices recursively counts leaf devices
func countDataDevices(d Device) int {
	if len(d.Children) == 0 {
		return 1
	}
	count := 0
	for _, child := range d.Children {
		count += countDataDevices(child)
	}
	return count
}

// HasErrors returns true if the device has any errors
func (d *Device) HasErrors() bool {
	return d.ReadErrors > 0 || d.WriteErrors > 0 || d.ChecksumErrors > 0
}

// TotalErrors returns the sum of all error types for the device
func (d *Device) TotalErrors() int64 {
	return d.ReadErrors + d.WriteErrors + d.ChecksumErrors
}