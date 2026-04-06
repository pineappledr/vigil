package zfs

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// ─── Agent Report Types ──────────────────────────────────────────────────────

// ZFSAgentReport matches the agent's ZFS report structure
type ZFSAgentReport struct {
	Hostname  string            `json:"hostname"`
	Timestamp time.Time         `json:"timestamp"`
	Available bool              `json:"zfs_available"`
	Pools     []ZFSAgentPool    `json:"pools"`
	Datasets  []ZFSAgentDataset `json:"datasets,omitempty"`
}

// ZFSAgentPool represents a pool from the agent report
type ZFSAgentPool struct {
	Name           string           `json:"name"`
	GUID           string           `json:"guid"`
	Status         string           `json:"status"`
	Health         string           `json:"health"`
	Size           int64            `json:"size_bytes"`
	Allocated      int64            `json:"allocated_bytes"`
	Free           int64            `json:"free_bytes"`
	Fragmentation  int              `json:"fragmentation"`
	CapacityPct    int              `json:"capacity_pct"`
	DedupRatio     float64          `json:"dedup_ratio"`
	Altroot        string           `json:"altroot"`
	ReadErrors     int64            `json:"read_errors"`
	WriteErrors    int64            `json:"write_errors"`
	ChecksumErrors int64            `json:"checksum_errors"`
	CompressRatio  float64          `json:"compress_ratio"`
	Scan           *ZFSAgentScan    `json:"scan"`
	Devices        []ZFSAgentDevice `json:"devices"`
}

// ZFSAgentDataset represents a dataset from the agent report
type ZFSAgentDataset struct {
	Name            string  `json:"name"`
	PoolName        string  `json:"pool_name"`
	UsedBytes       int64   `json:"used_bytes"`
	AvailableBytes  int64   `json:"available_bytes"`
	ReferencedBytes int64   `json:"referenced_bytes"`
	Mountpoint      string  `json:"mountpoint"`
	CompressRatio   float64 `json:"compress_ratio"`
	QuotaBytes      int64   `json:"quota_bytes"`
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
// IMPORTANT: Children field contains nested disks for mirror/raidz vdevs
type ZFSAgentDevice struct {
	Name           string           `json:"name"`
	Path           string           `json:"path"`
	GUID           string           `json:"guid"`
	SerialNumber   string           `json:"serial_number"`
	VdevType       string           `json:"vdev_type"`
	VdevParent     string           `json:"vdev_parent"`
	VdevIndex      int              `json:"vdev_index"`
	State          string           `json:"state"`
	ReadErrors     int64            `json:"read_errors"`
	WriteErrors    int64            `json:"write_errors"`
	ChecksumErrors int64            `json:"checksum_errors"`
	Size           int64            `json:"size_bytes"`
	Allocated      int64            `json:"allocated_bytes"`
	IsSpare        bool             `json:"is_spare"`
	IsLog          bool             `json:"is_log"`
	IsCache        bool             `json:"is_cache"`
	IsReplacing    bool             `json:"is_replacing"`
	Children       []ZFSAgentDevice `json:"children,omitempty"` // Child disks in mirror/raidz
}

// ─── Report Processing ───────────────────────────────────────────────────────

// ProcessZFSReport handles incoming ZFS data from an agent report
func ProcessZFSReport(db *sql.DB, hostname string, zfsData json.RawMessage) error {
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

	poolIDs := make(map[string]int64) // pool name -> pool ID
	for _, pool := range report.Pools {
		poolID, err := processPool(db, hostname, pool)
		if err != nil {
			log.Printf("⚠️  Failed to process pool %s: %v", pool.Name, err)
			continue
		}
		poolIDs[pool.Name] = poolID
	}

	// Process datasets
	processDatasets(db, hostname, report.Datasets, poolIDs)

	return nil
}

// processPool handles a single pool from the agent report
func processPool(db *sql.DB, hostname string, pool ZFSAgentPool) (int64, error) {
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
		CompressRatio:  pool.CompressRatio,
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
		dbPool.ScanSpeed = pool.Scan.Rate
		dbPool.ScanErrors = pool.Scan.ErrorsFound
		dbPool.ScanTimeRemaining = pool.Scan.TimeRemaining
		if !pool.Scan.StartTime.IsZero() {
			dbPool.LastScanTime = pool.Scan.StartTime
		} else if !pool.Scan.EndTime.IsZero() {
			dbPool.LastScanTime = pool.Scan.EndTime
		}
	}

	// Upsert pool
	poolID, err := UpsertZFSPool(db, dbPool)
	if err != nil {
		return 0, fmt.Errorf("upsert pool: %w", err)
	}

	// Process devices - including children (disks inside mirrors/raidz)
	vdevIndex := 0
	for _, dev := range pool.Devices {
		processDeviceRecursive(db, poolID, hostname, pool.Name, dev, "", &vdevIndex)
	}

	// Record scrub history if applicable
	if pool.Scan != nil {
		processScrubHistory(db, poolID, hostname, pool.Name, pool.Scan)
	}

	return poolID, nil
}

// processDeviceRecursive processes a device and all its children
// This flattens the hierarchy while maintaining parent-child relationships via VdevParent
func processDeviceRecursive(db *sql.DB, poolID int64, hostname, poolName string, dev ZFSAgentDevice, parentName string, index *int) {
	// Determine parent
	vdevParent := dev.VdevParent
	if vdevParent == "" && parentName != "" {
		vdevParent = parentName
	}

	// Create device record
	dbDevice := &ZFSPoolDevice{
		PoolID:         poolID,
		Hostname:       hostname,
		PoolName:       poolName,
		DeviceName:     dev.Name,
		DevicePath:     dev.Path,
		DeviceGUID:     dev.GUID,
		SerialNumber:   dev.SerialNumber,
		VdevType:       dev.VdevType,
		VdevParent:     vdevParent,
		VdevIndex:      *index,
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

	if err := UpsertZFSPoolDevice(db, poolID, dbDevice); err != nil {
		log.Printf("⚠️  Failed to upsert ZFS device %s: %v", dev.Name, err)
	} else {
		log.Printf("✅ Processed ZFS device: %s (type=%s, parent=%s, serial=%s)",
			dev.Name, dev.VdevType, vdevParent, dev.SerialNumber)
	}

	*index++

	// Process children recursively (disks inside mirror/raidz)
	for childIdx, child := range dev.Children {
		// Child's parent is this device
		child.VdevParent = dev.Name
		child.VdevIndex = childIdx
		processDeviceRecursive(db, poolID, hostname, poolName, child, dev.Name, index)
	}
}

// processScrubHistory records scrub history if needed
func processScrubHistory(db *sql.DB, poolID int64, hostname, poolName string, scan *ZFSAgentScan) {
	if scan.Function == "" || scan.Function == "none" {
		return
	}

	// Check if we should record this scrub
	lastScrub, _ := GetLastScrub(db, poolID)
	shouldRecord := shouldRecordScrub(lastScrub, scan)

	if !shouldRecord {
		return
	}

	startTime := scan.StartTime
	if startTime.IsZero() && !scan.EndTime.IsZero() {
		startTime = scan.EndTime // Use end time as fallback
	}

	record := &ZFSScrubHistory{
		PoolID:          poolID,
		Hostname:        hostname,
		PoolName:        poolName,
		ScanType:        scan.Function,
		State:           scan.State,
		StartTime:       startTime,
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

	if _, err := InsertZFSScrubHistory(db, record); err != nil {
		log.Printf("⚠️  Failed to insert scrub history: %v", err)
	}
}

// shouldRecordScrub determines if a scrub should be recorded
func shouldRecordScrub(lastScrub *ZFSScrubHistory, scan *ZFSAgentScan) bool {
	// Skip scrubs with no useful timestamp at all
	if scan.StartTime.IsZero() && scan.EndTime.IsZero() {
		return false
	}

	// No previous scrub - record it
	if lastScrub == nil {
		return true
	}

	// State changed to finished - record completion
	if scan.State == "finished" && lastScrub.State != "finished" {
		return true
	}

	// New scrub started (different start time or end time)
	scanTime := scan.StartTime
	if scanTime.IsZero() {
		scanTime = scan.EndTime
	}
	lastTime := lastScrub.StartTime
	if lastTime.IsZero() {
		lastTime = lastScrub.EndTime
	}
	if !scanTime.IsZero() && !lastTime.IsZero() && scanTime.After(lastTime) {
		return true
	}

	return false
}

// processDatasets handles incoming dataset data from an agent report
func processDatasets(db *sql.DB, hostname string, datasets []ZFSAgentDataset, poolIDs map[string]int64) {
	for _, ds := range datasets {
		poolID, ok := poolIDs[ds.PoolName]
		if !ok {
			// Try to resolve pool name from dataset name (e.g. "tank/data" → "tank")
			parts := splitDatasetPool(ds.Name)
			if parts != "" {
				poolID, ok = poolIDs[parts]
			}
			if !ok {
				continue
			}
		}

		dbDS := &ZFSDataset{
			PoolID:          poolID,
			Hostname:        hostname,
			PoolName:        ds.PoolName,
			DatasetName:     ds.Name,
			UsedBytes:       ds.UsedBytes,
			AvailableBytes:  ds.AvailableBytes,
			ReferencedBytes: ds.ReferencedBytes,
			Mountpoint:      ds.Mountpoint,
			CompressRatio:   ds.CompressRatio,
			QuotaBytes:      ds.QuotaBytes,
		}

		if _, err := UpsertZFSDataset(db, dbDS); err != nil {
			log.Printf("⚠️  Failed to upsert dataset %s: %v", ds.Name, err)
		}
	}
}

// splitDatasetPool extracts the pool name from a dataset path (e.g. "tank/data" → "tank")
func splitDatasetPool(name string) string {
	for i, c := range name {
		if c == '/' {
			return name[:i]
		}
	}
	return name
}

// ─── Batch Operations ────────────────────────────────────────────────────────

// CleanupStaleZFSData removes old ZFS data not seen in the specified duration
func CleanupStaleZFSData(db *sql.DB, hostname string, staleDuration time.Duration) error {
	cutoff := time.Now().Add(-staleDuration)

	// Get pools to check for stale devices
	pools, err := GetZFSPoolsByHostname(db, hostname)
	if err != nil {
		return err
	}

	for _, pool := range pools {
		if deleted, err := DeleteStaleZFSDevices(db, pool.ID, cutoff); err != nil {
			log.Printf("⚠️  Failed to cleanup stale devices for pool %s: %v", pool.PoolName, err)
		} else if deleted > 0 {
			log.Printf("🧹 Removed %d stale devices from pool %s", deleted, pool.PoolName)
		}

		if deleted, err := DeleteStaleDatasets(db, pool.ID, cutoff); err != nil {
			log.Printf("⚠️  Failed to cleanup stale datasets for pool %s: %v", pool.PoolName, err)
		} else if deleted > 0 {
			log.Printf("🧹 Removed %d stale datasets from pool %s", deleted, pool.PoolName)
		}
	}

	// Remove stale pools
	if deleted, err := DeleteStaleZFSPools(db, hostname, cutoff); err != nil {
		return fmt.Errorf("cleanup stale pools: %w", err)
	} else if deleted > 0 {
		log.Printf("🧹 Removed %d stale pools from host %s", deleted, hostname)
	}

	return nil
}
