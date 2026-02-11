package db

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// ─── Agent Report Types ──────────────────────────────────────────────────────

// ZFSAgentReport matches the agent's ZFS report structure
type ZFSAgentReport struct {
	Hostname  string         `json:"hostname"`
	Timestamp time.Time      `json:"timestamp"`
	Available bool           `json:"zfs_available"`
	Pools     []ZFSAgentPool `json:"pools"`
}

// ZFSAgentPool represents a pool from the agent report
type ZFSAgentPool struct {
	Name           string            `json:"name"`
	GUID           string            `json:"guid"`
	Status         string            `json:"status"`
	Health         string            `json:"health"`
	Size           int64             `json:"size_bytes"`
	Allocated      int64             `json:"allocated_bytes"`
	Free           int64             `json:"free_bytes"`
	Fragmentation  int               `json:"fragmentation"`
	CapacityPct    int               `json:"capacity_pct"`
	DedupRatio     float64           `json:"dedup_ratio"`
	Altroot        string            `json:"altroot"`
	ReadErrors     int64             `json:"read_errors"`
	WriteErrors    int64             `json:"write_errors"`
	ChecksumErrors int64             `json:"checksum_errors"`
	Scan           *ZFSAgentScan     `json:"scan"`
	Devices        []ZFSAgentDevice  `json:"devices"`
}

// ZFSAgentScan represents scan info from the agent report
type ZFSAgentScan struct {
	Function      string    `json:"function"`
	State         string    `json:"state"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	Duration      int64     `json:"duration_secs"`
	DataExamined  int64     `json:"data_examined"`
	DataTotal     int64     `json:"data_total"`
	ErrorsFound   int64     `json:"errors_found"`
	BytesRepaired int64     `json:"bytes_repaired"`
	ProgressPct   float64   `json:"progress_pct"`
	Rate          int64     `json:"rate_bytes_sec"`
	TimeRemaining int64     `json:"time_remaining"`
}

// ZFSAgentDevice represents a device from the agent report
type ZFSAgentDevice struct {
	Name           string `json:"name"`
	Path           string `json:"path"`
	GUID           string `json:"guid"`
	SerialNumber   string `json:"serial_number"`
	VdevType       string `json:"vdev_type"`
	VdevParent     string `json:"vdev_parent"`
	VdevIndex      int    `json:"vdev_index"`
	State          string `json:"state"`
	ReadErrors     int64  `json:"read_errors"`
	WriteErrors    int64  `json:"write_errors"`
	ChecksumErrors int64  `json:"checksum_errors"`
	Size           int64  `json:"size_bytes"`
	Allocated      int64  `json:"allocated_bytes"`
	IsSpare        bool   `json:"is_spare"`
	IsLog          bool   `json:"is_log"`
	IsCache        bool   `json:"is_cache"`
	IsReplacing    bool   `json:"is_replacing"`
}

// ─── Report Processing ───────────────────────────────────────────────────────

// ProcessZFSReport handles incoming ZFS data from an agent report
func ProcessZFSReport(hostname string, zfsData json.RawMessage) error {
	if len(zfsData) == 0 || string(zfsData) == "null" {
		return nil
	}

	var report ZFSAgentReport
	if err := json.Unmarshal(zfsData, &report); err != nil {
		return fmt.Errorf("parse ZFS report: %w", err)
	}

	if !report.Available || len(report.Pools) == 0 {
		return nil
	}

	for _, pool := range report.Pools {
		if err := processPool(hostname, pool); err != nil {
			log.Printf("⚠️  Failed to process pool %s: %v", pool.Name, err)
		}
	}

	return nil
}

// processPool handles a single pool from the agent report
func processPool(hostname string, pool ZFSAgentPool) error {
	// Build pool record
	dbPool := &ZFSPool{
		Hostname:       hostname,
		PoolName:       pool.Name,
		PoolGUID:       pool.GUID,
		Status:         pool.Status,
		Health:         pool.Health,
		SizeBytes:      pool.Size,
		AllocatedBytes: pool.Allocated,
		FreeBytes:      pool.Free,
		Fragmentation:  pool.Fragmentation,
		CapacityPct:    pool.CapacityPct,
		DedupRatio:     pool.DedupRatio,
		Altroot:        pool.Altroot,
		ReadErrors:     pool.ReadErrors,
		WriteErrors:    pool.WriteErrors,
		ChecksumErrors: pool.ChecksumErrors,
	}

	// Add scan info if available
	if pool.Scan != nil {
		dbPool.ScanFunction = pool.Scan.Function
		dbPool.ScanState = pool.Scan.State
		dbPool.ScanProgress = pool.Scan.ProgressPct
		if !pool.Scan.StartTime.IsZero() {
			dbPool.LastScanTime = pool.Scan.StartTime
		}
	}

	// Upsert pool
	poolID, err := UpsertZFSPool(dbPool)
	if err != nil {
		return fmt.Errorf("upsert pool: %w", err)
	}

	// Process devices
	for _, dev := range pool.Devices {
		if err := processDevice(poolID, hostname, pool.Name, dev); err != nil {
			log.Printf("⚠️  Failed to process device %s: %v", dev.Name, err)
		}
	}

	// Record scrub history if applicable
	if pool.Scan != nil {
		processScrubHistory(poolID, hostname, pool.Name, pool.Scan)
	}

	return nil
}

// processDevice handles a single device from the pool
func processDevice(poolID int64, hostname, poolName string, dev ZFSAgentDevice) error {
	dbDevice := &ZFSPoolDevice{
		PoolID:         poolID,
		Hostname:       hostname,
		PoolName:       poolName,
		DeviceName:     dev.Name,
		DevicePath:     dev.Path,
		DeviceGUID:     dev.GUID,
		SerialNumber:   dev.SerialNumber,
		VdevType:       dev.VdevType,
		VdevParent:     dev.VdevParent,
		VdevIndex:      dev.VdevIndex,
		State:          dev.State,
		ReadErrors:     dev.ReadErrors,
		WriteErrors:    dev.WriteErrors,
		ChecksumErrors: dev.ChecksumErrors,
		SizeBytes:      dev.Size,
		AllocatedBytes: dev.Allocated,
		IsSpare:        dev.IsSpare,
		IsLog:          dev.IsLog,
		IsCache:        dev.IsCache,
		IsReplacing:    dev.IsReplacing,
	}

	return UpsertZFSPoolDevice(poolID, dbDevice)
}

// processScrubHistory records scrub history if needed
func processScrubHistory(poolID int64, hostname, poolName string, scan *ZFSAgentScan) {
	if scan.Function == "" || scan.Function == "none" {
		return
	}

	// Check if we should record this scrub
	lastScrub, _ := GetLastScrub(poolID)
	shouldRecord := shouldRecordScrub(lastScrub, scan)

	if !shouldRecord {
		return
	}

	record := &ZFSScrubHistory{
		PoolID:          poolID,
		Hostname:        hostname,
		PoolName:        poolName,
		ScanType:        scan.Function,
		State:           scan.State,
		StartTime:       scan.StartTime,
		EndTime:         scan.EndTime,
		DurationSecs:    scan.Duration,
		DataExamined:    scan.DataExamined,
		DataTotal:       scan.DataTotal,
		ErrorsFound:     scan.ErrorsFound,
		BytesRepaired:   scan.BytesRepaired,
		ProgressPct:     scan.ProgressPct,
		RateBytesPerSec: scan.Rate,
		TimeRemaining:   scan.TimeRemaining,
	}

	if _, err := InsertZFSScrubHistory(record); err != nil {
		log.Printf("⚠️  Failed to insert scrub history: %v", err)
	}
}

// shouldRecordScrub determines if a scrub should be recorded
func shouldRecordScrub(lastScrub *ZFSScrubHistory, scan *ZFSAgentScan) bool {
	// No previous scrub - record it
	if lastScrub == nil {
		return true
	}

	// State changed to finished - record completion
	if scan.State == "finished" && lastScrub.State != "finished" {
		return true
	}

	// New scrub started (different start time)
	if !scan.StartTime.IsZero() && scan.StartTime.After(lastScrub.StartTime) {
		return true
	}

	return false
}

// ─── Batch Operations ────────────────────────────────────────────────────────

// CleanupStaleZFSData removes old ZFS data not seen in the specified duration
func CleanupStaleZFSData(hostname string, staleDuration time.Duration) error {
	cutoff := time.Now().Add(-staleDuration)

	// Get pools to check for stale devices
	pools, err := GetZFSPoolsByHostname(hostname)
	if err != nil {
		return err
	}

	for _, pool := range pools {
		if deleted, err := DeleteStaleZFSDevices(pool.ID, cutoff); err != nil {
			log.Printf("⚠️  Failed to cleanup stale devices for pool %s: %v", pool.PoolName, err)
		} else if deleted > 0 {
			log.Printf("🧹 Removed %d stale devices from pool %s", deleted, pool.PoolName)
		}
	}

	// Remove stale pools
	if deleted, err := DeleteStaleZFSPools(hostname, cutoff); err != nil {
		return fmt.Errorf("cleanup stale pools: %w", err)
	} else if deleted > 0 {
		log.Printf("🧹 Removed %d stale pools from host %s", deleted, hostname)
	}

	return nil
}
