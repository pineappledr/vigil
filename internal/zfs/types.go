package zfs

import "time"

// ─── ZFS Pool Types ──────────────────────────────────────────────────────────

// ZFSPool represents a ZFS pool in the database
type ZFSPool struct {
	ID             int64     `json:"id"`
	Hostname       string    `json:"hostname"`
	PoolName       string    `json:"pool_name"`
	PoolGUID       string    `json:"pool_guid,omitempty"`
	Status         string    `json:"status"`
	Health         string    `json:"health"`
	SizeBytes      int64     `json:"size_bytes"`
	AllocatedBytes int64     `json:"allocated_bytes"`
	FreeBytes      int64     `json:"free_bytes"`
	Fragmentation  int       `json:"fragmentation"`
	CapacityPct    int       `json:"capacity_pct"`
	DedupRatio     float64   `json:"dedup_ratio"`
	Altroot        string    `json:"altroot,omitempty"`
	ReadErrors     int64     `json:"read_errors"`
	WriteErrors    int64     `json:"write_errors"`
	ChecksumErrors int64     `json:"checksum_errors"`
	ScanFunction   string    `json:"scan_function,omitempty"`
	ScanState      string    `json:"scan_state,omitempty"`
	ScanProgress   float64   `json:"scan_progress"`
	LastScanTime   time.Time `json:"last_scan_time,omitempty"`
	LastSeen       time.Time `json:"last_seen"`
	CreatedAt      time.Time `json:"created_at"`
}

// ─── ZFS Device Types ────────────────────────────────────────────────────────

// ZFSPoolDevice represents a device within a ZFS pool
type ZFSPoolDevice struct {
	ID             int64     `json:"id"`
	PoolID         int64     `json:"pool_id"`
	Hostname       string    `json:"hostname"`
	PoolName       string    `json:"pool_name"`
	DeviceName     string    `json:"device_name"`
	DevicePath     string    `json:"device_path,omitempty"`
	DeviceGUID     string    `json:"device_guid,omitempty"`
	SerialNumber   string    `json:"serial_number,omitempty"`
	VdevType       string    `json:"vdev_type"`
	VdevParent     string    `json:"vdev_parent,omitempty"`
	VdevIndex      int       `json:"vdev_index"`
	State          string    `json:"state"`
	ReadErrors     int64     `json:"read_errors"`
	WriteErrors    int64     `json:"write_errors"`
	ChecksumErrors int64     `json:"checksum_errors"`
	SizeBytes      int64     `json:"size_bytes"`
	AllocatedBytes int64     `json:"allocated_bytes"`
	IsSpare        bool      `json:"is_spare"`
	IsLog          bool      `json:"is_log"`
	IsCache        bool      `json:"is_cache"`
	IsReplacing    bool      `json:"is_replacing"`
	LastSeen       time.Time `json:"last_seen"`
	CreatedAt      time.Time `json:"created_at"`
}

// ─── ZFS Scrub Types ─────────────────────────────────────────────────────────

// ZFSScrubHistory represents a historical scrub/resilver record
type ZFSScrubHistory struct {
	ID              int64     `json:"id"`
	PoolID          int64     `json:"pool_id"`
	Hostname        string    `json:"hostname"`
	PoolName        string    `json:"pool_name"`
	ScanType        string    `json:"scan_type"`
	State           string    `json:"state"`
	StartTime       time.Time `json:"start_time"`
	EndTime         time.Time `json:"end_time,omitempty"`
	DurationSecs    int64     `json:"duration_secs"`
	DataExamined    int64     `json:"data_examined"`
	DataTotal       int64     `json:"data_total"`
	ErrorsFound     int64     `json:"errors_found"`
	BytesRepaired   int64     `json:"bytes_repaired"`
	BlocksRepaired  int64     `json:"blocks_repaired"`
	ProgressPct     float64   `json:"progress_pct"`
	RateBytesPerSec int64     `json:"rate_bytes_sec"`
	TimeRemaining   int64     `json:"time_remaining,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// ─── ZFS Summary Types ───────────────────────────────────────────────────────

// ZFSPoolSummary provides aggregate ZFS stats for a hostname
type ZFSPoolSummary struct {
	Hostname       string `json:"hostname"`
	TotalPools     int    `json:"total_pools"`
	HealthyPools   int    `json:"healthy_pools"`
	DegradedPools  int    `json:"degraded_pools"`
	FaultedPools   int    `json:"faulted_pools"`
	TotalSizeBytes int64  `json:"total_size_bytes"`
	TotalUsedBytes int64  `json:"total_used_bytes"`
	TotalFreeBytes int64  `json:"total_free_bytes"`
	TotalErrors    int64  `json:"total_errors"`
	ActiveScrubs   int    `json:"active_scrubs"`
}

// ZFSGlobalStats provides system-wide ZFS statistics
type ZFSGlobalStats struct {
	TotalPools    int   `json:"total_pools"`
	HealthyPools  int   `json:"healthy_pools"`
	DegradedPools int   `json:"degraded_pools"`
	FaultedPools  int   `json:"faulted_pools"`
	TotalDevices  int   `json:"total_devices"`
	TotalErrors   int64 `json:"total_errors"`
	ActiveScrubs  int   `json:"active_scrubs"`
}

// ─── ZFS API Response Types ──────────────────────────────────────────────────

// ZFSPoolWithDevices combines a pool with its devices for API responses
type ZFSPoolWithDevices struct {
	ZFSPool
	Devices []ZFSPoolDevice `json:"devices,omitempty"`
}

// ZFSPoolListItem is a lightweight pool representation for list views
type ZFSPoolListItem struct {
	ID             int64   `json:"id"`
	Hostname       string  `json:"hostname"`
	PoolName       string  `json:"name"`
	Status         string  `json:"status"`
	Health         string  `json:"health"`
	SizeBytes      int64   `json:"size_bytes"`
	AllocatedBytes int64   `json:"allocated_bytes"`
	FreeBytes      int64   `json:"free_bytes"`
	CapacityPct    int     `json:"capacity_pct"`
	ReadErrors     int64   `json:"read_errors"`
	WriteErrors    int64   `json:"write_errors"`
	ChecksumErrors int64   `json:"checksum_errors"`
	ScanState      string  `json:"scan_state,omitempty"`
	ScanProgress   float64 `json:"scan_progress,omitempty"`
	DeviceCount    int     `json:"device_count"`
	LastScrub      string  `json:"last_scrub,omitempty"`
}
