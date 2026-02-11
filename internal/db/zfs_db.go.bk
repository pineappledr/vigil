package db

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// ─── ZFS Pool Types (mirrors agent types for server-side) ────────────────────

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

// ─── ZFS Pool CRUD Operations ────────────────────────────────────────────────

// UpsertZFSPool inserts or updates a ZFS pool record
func UpsertZFSPool(pool *ZFSPool) (int64, error) {
	now := time.Now()

	result, err := DB.Exec(`
		INSERT INTO zfs_pools (
			hostname, pool_name, pool_guid, status, health,
			size_bytes, allocated_bytes, free_bytes,
			fragmentation, capacity_pct, dedup_ratio, altroot,
			read_errors, write_errors, checksum_errors,
			scan_function, scan_state, scan_progress, last_scan_time,
			last_seen, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(hostname, pool_name) DO UPDATE SET
			pool_guid = excluded.pool_guid,
			status = excluded.status,
			health = excluded.health,
			size_bytes = excluded.size_bytes,
			allocated_bytes = excluded.allocated_bytes,
			free_bytes = excluded.free_bytes,
			fragmentation = excluded.fragmentation,
			capacity_pct = excluded.capacity_pct,
			dedup_ratio = excluded.dedup_ratio,
			altroot = excluded.altroot,
			read_errors = excluded.read_errors,
			write_errors = excluded.write_errors,
			checksum_errors = excluded.checksum_errors,
			scan_function = excluded.scan_function,
			scan_state = excluded.scan_state,
			scan_progress = excluded.scan_progress,
			last_scan_time = excluded.last_scan_time,
			last_seen = excluded.last_seen
	`,
		pool.Hostname, pool.PoolName, pool.PoolGUID, pool.Status, pool.Health,
		pool.SizeBytes, pool.AllocatedBytes, pool.FreeBytes,
		pool.Fragmentation, pool.CapacityPct, pool.DedupRatio, pool.Altroot,
		pool.ReadErrors, pool.WriteErrors, pool.ChecksumErrors,
		pool.ScanFunction, pool.ScanState, pool.ScanProgress, nullTimeString(pool.LastScanTime),
		now, now,
	)

	if err != nil {
		return 0, fmt.Errorf("failed to upsert ZFS pool: %w", err)
	}

	// Get the pool ID (either inserted or existing)
	id, err := result.LastInsertId()
	if err != nil || id == 0 {
		// If LastInsertId fails, query for the ID
		err = DB.QueryRow(
			"SELECT id FROM zfs_pools WHERE hostname = ? AND pool_name = ?",
			pool.Hostname, pool.PoolName,
		).Scan(&id)
		if err != nil {
			return 0, fmt.Errorf("failed to get pool ID: %w", err)
		}
	}

	return id, nil
}

// GetZFSPool retrieves a single ZFS pool by hostname and name
func GetZFSPool(hostname, poolName string) (*ZFSPool, error) {
	pool := &ZFSPool{}
	var lastScanTime, lastSeen, createdAt sql.NullString

	err := DB.QueryRow(`
		SELECT id, hostname, pool_name, pool_guid, status, health,
			size_bytes, allocated_bytes, free_bytes,
			fragmentation, capacity_pct, dedup_ratio, altroot,
			read_errors, write_errors, checksum_errors,
			scan_function, scan_state, scan_progress, last_scan_time,
			last_seen, created_at
		FROM zfs_pools
		WHERE hostname = ? AND pool_name = ?
	`, hostname, poolName).Scan(
		&pool.ID, &pool.Hostname, &pool.PoolName, &pool.PoolGUID, &pool.Status, &pool.Health,
		&pool.SizeBytes, &pool.AllocatedBytes, &pool.FreeBytes,
		&pool.Fragmentation, &pool.CapacityPct, &pool.DedupRatio, &pool.Altroot,
		&pool.ReadErrors, &pool.WriteErrors, &pool.ChecksumErrors,
		&pool.ScanFunction, &pool.ScanState, &pool.ScanProgress, &lastScanTime,
		&lastSeen, &createdAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get ZFS pool: %w", err)
	}

	pool.LastScanTime = parseNullTime(lastScanTime)
	pool.LastSeen = parseNullTime(lastSeen)
	pool.CreatedAt = parseNullTime(createdAt)

	return pool, nil
}

// GetZFSPoolByID retrieves a ZFS pool by ID
func GetZFSPoolByID(id int64) (*ZFSPool, error) {
	pool := &ZFSPool{}
	var lastScanTime, lastSeen, createdAt sql.NullString

	err := DB.QueryRow(`
		SELECT id, hostname, pool_name, pool_guid, status, health,
			size_bytes, allocated_bytes, free_bytes,
			fragmentation, capacity_pct, dedup_ratio, altroot,
			read_errors, write_errors, checksum_errors,
			scan_function, scan_state, scan_progress, last_scan_time,
			last_seen, created_at
		FROM zfs_pools
		WHERE id = ?
	`, id).Scan(
		&pool.ID, &pool.Hostname, &pool.PoolName, &pool.PoolGUID, &pool.Status, &pool.Health,
		&pool.SizeBytes, &pool.AllocatedBytes, &pool.FreeBytes,
		&pool.Fragmentation, &pool.CapacityPct, &pool.DedupRatio, &pool.Altroot,
		&pool.ReadErrors, &pool.WriteErrors, &pool.ChecksumErrors,
		&pool.ScanFunction, &pool.ScanState, &pool.ScanProgress, &lastScanTime,
		&lastSeen, &createdAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get ZFS pool by ID: %w", err)
	}

	pool.LastScanTime = parseNullTime(lastScanTime)
	pool.LastSeen = parseNullTime(lastSeen)
	pool.CreatedAt = parseNullTime(createdAt)

	return pool, nil
}

// GetZFSPoolsByHostname retrieves all ZFS pools for a hostname
func GetZFSPoolsByHostname(hostname string) ([]ZFSPool, error) {
	rows, err := DB.Query(`
		SELECT id, hostname, pool_name, pool_guid, status, health,
			size_bytes, allocated_bytes, free_bytes,
			fragmentation, capacity_pct, dedup_ratio, altroot,
			read_errors, write_errors, checksum_errors,
			scan_function, scan_state, scan_progress, last_scan_time,
			last_seen, created_at
		FROM zfs_pools
		WHERE hostname = ?
		ORDER BY pool_name
	`, hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to query ZFS pools: %w", err)
	}
	defer rows.Close()

	var pools []ZFSPool
	for rows.Next() {
		var pool ZFSPool
		var lastScanTime, lastSeen, createdAt sql.NullString

		err := rows.Scan(
			&pool.ID, &pool.Hostname, &pool.PoolName, &pool.PoolGUID, &pool.Status, &pool.Health,
			&pool.SizeBytes, &pool.AllocatedBytes, &pool.FreeBytes,
			&pool.Fragmentation, &pool.CapacityPct, &pool.DedupRatio, &pool.Altroot,
			&pool.ReadErrors, &pool.WriteErrors, &pool.ChecksumErrors,
			&pool.ScanFunction, &pool.ScanState, &pool.ScanProgress, &lastScanTime,
			&lastSeen, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ZFS pool row: %w", err)
		}

		pool.LastScanTime = parseNullTime(lastScanTime)
		pool.LastSeen = parseNullTime(lastSeen)
		pool.CreatedAt = parseNullTime(createdAt)

		pools = append(pools, pool)
	}

	return pools, nil
}

// GetAllZFSPools retrieves all ZFS pools across all hosts
func GetAllZFSPools() ([]ZFSPool, error) {
	rows, err := DB.Query(`
		SELECT id, hostname, pool_name, pool_guid, status, health,
			size_bytes, allocated_bytes, free_bytes,
			fragmentation, capacity_pct, dedup_ratio, altroot,
			read_errors, write_errors, checksum_errors,
			scan_function, scan_state, scan_progress, last_scan_time,
			last_seen, created_at
		FROM zfs_pools
		ORDER BY hostname, pool_name
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query all ZFS pools: %w", err)
	}
	defer rows.Close()

	var pools []ZFSPool
	for rows.Next() {
		var pool ZFSPool
		var lastScanTime, lastSeen, createdAt sql.NullString

		err := rows.Scan(
			&pool.ID, &pool.Hostname, &pool.PoolName, &pool.PoolGUID, &pool.Status, &pool.Health,
			&pool.SizeBytes, &pool.AllocatedBytes, &pool.FreeBytes,
			&pool.Fragmentation, &pool.CapacityPct, &pool.DedupRatio, &pool.Altroot,
			&pool.ReadErrors, &pool.WriteErrors, &pool.ChecksumErrors,
			&pool.ScanFunction, &pool.ScanState, &pool.ScanProgress, &lastScanTime,
			&lastSeen, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ZFS pool row: %w", err)
		}

		pool.LastScanTime = parseNullTime(lastScanTime)
		pool.LastSeen = parseNullTime(lastSeen)
		pool.CreatedAt = parseNullTime(createdAt)

		pools = append(pools, pool)
	}

	return pools, nil
}

// DeleteZFSPool removes a ZFS pool (and cascades to devices/scrub history)
func DeleteZFSPool(hostname, poolName string) error {
	_, err := DB.Exec("DELETE FROM zfs_pools WHERE hostname = ? AND pool_name = ?", hostname, poolName)
	return err
}

// ─── ZFS Pool Device Operations ──────────────────────────────────────────────

// UpsertZFSPoolDevice inserts or updates a pool device
func UpsertZFSPoolDevice(poolID int64, device *ZFSPoolDevice) error {
	now := time.Now()

	_, err := DB.Exec(`
		INSERT INTO zfs_pool_devices (
			pool_id, hostname, pool_name, device_name, device_path, device_guid,
			serial_number, vdev_type, vdev_parent, vdev_index, state,
			read_errors, write_errors, checksum_errors,
			size_bytes, allocated_bytes,
			is_spare, is_log, is_cache, is_replacing,
			last_seen, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(pool_id, device_name) DO UPDATE SET
			device_path = excluded.device_path,
			device_guid = excluded.device_guid,
			serial_number = excluded.serial_number,
			vdev_type = excluded.vdev_type,
			vdev_parent = excluded.vdev_parent,
			vdev_index = excluded.vdev_index,
			state = excluded.state,
			read_errors = excluded.read_errors,
			write_errors = excluded.write_errors,
			checksum_errors = excluded.checksum_errors,
			size_bytes = excluded.size_bytes,
			allocated_bytes = excluded.allocated_bytes,
			is_spare = excluded.is_spare,
			is_log = excluded.is_log,
			is_cache = excluded.is_cache,
			is_replacing = excluded.is_replacing,
			last_seen = excluded.last_seen
	`,
		poolID, device.Hostname, device.PoolName, device.DeviceName, device.DevicePath, device.DeviceGUID,
		device.SerialNumber, device.VdevType, device.VdevParent, device.VdevIndex, device.State,
		device.ReadErrors, device.WriteErrors, device.ChecksumErrors,
		device.SizeBytes, device.AllocatedBytes,
		boolToInt(device.IsSpare), boolToInt(device.IsLog), boolToInt(device.IsCache), boolToInt(device.IsReplacing),
		now, now,
	)

	if err != nil {
		return fmt.Errorf("failed to upsert ZFS pool device: %w", err)
	}

	return nil
}

// GetZFSPoolDevices retrieves all devices for a pool
func GetZFSPoolDevices(poolID int64) ([]ZFSPoolDevice, error) {
	rows, err := DB.Query(`
		SELECT id, pool_id, hostname, pool_name, device_name, device_path, device_guid,
			serial_number, vdev_type, vdev_parent, vdev_index, state,
			read_errors, write_errors, checksum_errors,
			size_bytes, allocated_bytes,
			is_spare, is_log, is_cache, is_replacing,
			last_seen, created_at
		FROM zfs_pool_devices
		WHERE pool_id = ?
		ORDER BY vdev_parent, vdev_index
	`, poolID)
	if err != nil {
		return nil, fmt.Errorf("failed to query ZFS pool devices: %w", err)
	}
	defer rows.Close()

	var devices []ZFSPoolDevice
	for rows.Next() {
		var dev ZFSPoolDevice
		var isSpare, isLog, isCache, isReplacing int
		var lastSeen, createdAt sql.NullString

		err := rows.Scan(
			&dev.ID, &dev.PoolID, &dev.Hostname, &dev.PoolName, &dev.DeviceName, &dev.DevicePath, &dev.DeviceGUID,
			&dev.SerialNumber, &dev.VdevType, &dev.VdevParent, &dev.VdevIndex, &dev.State,
			&dev.ReadErrors, &dev.WriteErrors, &dev.ChecksumErrors,
			&dev.SizeBytes, &dev.AllocatedBytes,
			&isSpare, &isLog, &isCache, &isReplacing,
			&lastSeen, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan ZFS device row: %w", err)
		}

		dev.IsSpare = isSpare == 1
		dev.IsLog = isLog == 1
		dev.IsCache = isCache == 1
		dev.IsReplacing = isReplacing == 1
		dev.LastSeen = parseNullTime(lastSeen)
		dev.CreatedAt = parseNullTime(createdAt)

		devices = append(devices, dev)
	}

	return devices, nil
}

// GetZFSDeviceBySerial finds a ZFS device by serial number
func GetZFSDeviceBySerial(hostname, serialNumber string) (*ZFSPoolDevice, error) {
	var dev ZFSPoolDevice
	var isSpare, isLog, isCache, isReplacing int
	var lastSeen, createdAt sql.NullString

	err := DB.QueryRow(`
		SELECT id, pool_id, hostname, pool_name, device_name, device_path, device_guid,
			serial_number, vdev_type, vdev_parent, vdev_index, state,
			read_errors, write_errors, checksum_errors,
			size_bytes, allocated_bytes,
			is_spare, is_log, is_cache, is_replacing,
			last_seen, created_at
		FROM zfs_pool_devices
		WHERE hostname = ? AND serial_number = ?
		LIMIT 1
	`, hostname, serialNumber).Scan(
		&dev.ID, &dev.PoolID, &dev.Hostname, &dev.PoolName, &dev.DeviceName, &dev.DevicePath, &dev.DeviceGUID,
		&dev.SerialNumber, &dev.VdevType, &dev.VdevParent, &dev.VdevIndex, &dev.State,
		&dev.ReadErrors, &dev.WriteErrors, &dev.ChecksumErrors,
		&dev.SizeBytes, &dev.AllocatedBytes,
		&isSpare, &isLog, &isCache, &isReplacing,
		&lastSeen, &createdAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get ZFS device by serial: %w", err)
	}

	dev.IsSpare = isSpare == 1
	dev.IsLog = isLog == 1
	dev.IsCache = isCache == 1
	dev.IsReplacing = isReplacing == 1
	dev.LastSeen = parseNullTime(lastSeen)
	dev.CreatedAt = parseNullTime(createdAt)

	return &dev, nil
}

// DeleteStaleZFSDevices removes devices not seen recently for a pool
func DeleteStaleZFSDevices(poolID int64, cutoff time.Time) (int64, error) {
	result, err := DB.Exec(
		"DELETE FROM zfs_pool_devices WHERE pool_id = ? AND last_seen < ?",
		poolID, cutoff.Format("2006-01-02 15:04:05"),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ─── ZFS Scrub History Operations ────────────────────────────────────────────

// InsertZFSScrubHistory adds a new scrub/resilver history record
func InsertZFSScrubHistory(record *ZFSScrubHistory) (int64, error) {
	result, err := DB.Exec(`
		INSERT INTO zfs_scrub_history (
			pool_id, hostname, pool_name, scan_type, state,
			start_time, end_time, duration_secs,
			data_examined, data_total, errors_found,
			bytes_repaired, blocks_repaired,
			progress_pct, rate_bytes_sec, time_remaining,
			created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		record.PoolID, record.Hostname, record.PoolName, record.ScanType, record.State,
		nullTimeString(record.StartTime), nullTimeString(record.EndTime), record.DurationSecs,
		record.DataExamined, record.DataTotal, record.ErrorsFound,
		record.BytesRepaired, record.BlocksRepaired,
		record.ProgressPct, record.RateBytesPerSec, record.TimeRemaining,
		time.Now(),
	)

	if err != nil {
		return 0, fmt.Errorf("failed to insert scrub history: %w", err)
	}

	return result.LastInsertId()
}

// GetZFSScrubHistory retrieves scrub history for a pool
func GetZFSScrubHistory(poolID int64, limit int) ([]ZFSScrubHistory, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := DB.Query(`
		SELECT id, pool_id, hostname, pool_name, scan_type, state,
			start_time, end_time, duration_secs,
			data_examined, data_total, errors_found,
			bytes_repaired, blocks_repaired,
			progress_pct, rate_bytes_sec, time_remaining,
			created_at
		FROM zfs_scrub_history
		WHERE pool_id = ?
		ORDER BY start_time DESC
		LIMIT ?
	`, poolID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query scrub history: %w", err)
	}
	defer rows.Close()

	var history []ZFSScrubHistory
	for rows.Next() {
		var rec ZFSScrubHistory
		var startTime, endTime, createdAt sql.NullString

		err := rows.Scan(
			&rec.ID, &rec.PoolID, &rec.Hostname, &rec.PoolName, &rec.ScanType, &rec.State,
			&startTime, &endTime, &rec.DurationSecs,
			&rec.DataExamined, &rec.DataTotal, &rec.ErrorsFound,
			&rec.BytesRepaired, &rec.BlocksRepaired,
			&rec.ProgressPct, &rec.RateBytesPerSec, &rec.TimeRemaining,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan scrub history row: %w", err)
		}

		rec.StartTime = parseNullTime(startTime)
		rec.EndTime = parseNullTime(endTime)
		rec.CreatedAt = parseNullTime(createdAt)

		history = append(history, rec)
	}

	return history, nil
}

// GetLastScrub retrieves the most recent scrub for a pool
func GetLastScrub(poolID int64) (*ZFSScrubHistory, error) {
	history, err := GetZFSScrubHistory(poolID, 1)
	if err != nil {
		return nil, err
	}
	if len(history) == 0 {
		return nil, nil
	}
	return &history[0], nil
}

// ─── ZFS Summary and Statistics ──────────────────────────────────────────────

// GetZFSPoolSummary returns aggregate stats for a hostname
func GetZFSPoolSummary(hostname string) (*ZFSPoolSummary, error) {
	summary := &ZFSPoolSummary{Hostname: hostname}

	err := DB.QueryRow(`
		SELECT 
			COUNT(*) as total_pools,
			SUM(CASE WHEN health = 'ONLINE' THEN 1 ELSE 0 END) as healthy,
			SUM(CASE WHEN health = 'DEGRADED' THEN 1 ELSE 0 END) as degraded,
			SUM(CASE WHEN health = 'FAULTED' THEN 1 ELSE 0 END) as faulted,
			COALESCE(SUM(size_bytes), 0) as total_size,
			COALESCE(SUM(allocated_bytes), 0) as total_used,
			COALESCE(SUM(free_bytes), 0) as total_free,
			COALESCE(SUM(read_errors + write_errors + checksum_errors), 0) as total_errors,
			SUM(CASE WHEN scan_state = 'scanning' THEN 1 ELSE 0 END) as active_scrubs
		FROM zfs_pools
		WHERE hostname = ?
	`, hostname).Scan(
		&summary.TotalPools,
		&summary.HealthyPools,
		&summary.DegradedPools,
		&summary.FaultedPools,
		&summary.TotalSizeBytes,
		&summary.TotalUsedBytes,
		&summary.TotalFreeBytes,
		&summary.TotalErrors,
		&summary.ActiveScrubs,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get ZFS pool summary: %w", err)
	}

	return summary, nil
}

// GetGlobalZFSSummary returns aggregate stats across all hosts
func GetGlobalZFSSummary() (*ZFSPoolSummary, error) {
	summary := &ZFSPoolSummary{Hostname: "*"}

	err := DB.QueryRow(`
		SELECT 
			COUNT(*) as total_pools,
			SUM(CASE WHEN health = 'ONLINE' THEN 1 ELSE 0 END) as healthy,
			SUM(CASE WHEN health = 'DEGRADED' THEN 1 ELSE 0 END) as degraded,
			SUM(CASE WHEN health = 'FAULTED' THEN 1 ELSE 0 END) as faulted,
			COALESCE(SUM(size_bytes), 0) as total_size,
			COALESCE(SUM(allocated_bytes), 0) as total_used,
			COALESCE(SUM(free_bytes), 0) as total_free,
			COALESCE(SUM(read_errors + write_errors + checksum_errors), 0) as total_errors,
			SUM(CASE WHEN scan_state = 'scanning' THEN 1 ELSE 0 END) as active_scrubs
		FROM zfs_pools
	`).Scan(
		&summary.TotalPools,
		&summary.HealthyPools,
		&summary.DegradedPools,
		&summary.FaultedPools,
		&summary.TotalSizeBytes,
		&summary.TotalUsedBytes,
		&summary.TotalFreeBytes,
		&summary.TotalErrors,
		&summary.ActiveScrubs,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get global ZFS summary: %w", err)
	}

	return summary, nil
}

// ─── ZFS Report Processing ───────────────────────────────────────────────────

// ZFSReportData matches the agent's ZFSReport structure
type ZFSReportData struct {
	Hostname  string          `json:"hostname"`
	Timestamp time.Time       `json:"timestamp"`
	Available bool            `json:"zfs_available"`
	Pools     json.RawMessage `json:"pools"`
}

// ProcessZFSReport handles incoming ZFS data from an agent report
func ProcessZFSReport(hostname string, zfsData json.RawMessage) error {
	if len(zfsData) == 0 || string(zfsData) == "null" {
		return nil
	}

	var report struct {
		Hostname  string `json:"hostname"`
		Available bool   `json:"zfs_available"`
		Pools     []struct {
			Name           string  `json:"name"`
			GUID           string  `json:"guid"`
			Status         string  `json:"status"`
			Health         string  `json:"health"`
			Size           int64   `json:"size_bytes"`
			Allocated      int64   `json:"allocated_bytes"`
			Free           int64   `json:"free_bytes"`
			Fragmentation  int     `json:"fragmentation"`
			CapacityPct    int     `json:"capacity_pct"`
			DedupRatio     float64 `json:"dedup_ratio"`
			Altroot        string  `json:"altroot"`
			ReadErrors     int64   `json:"read_errors"`
			WriteErrors    int64   `json:"write_errors"`
			ChecksumErrors int64   `json:"checksum_errors"`
			Scan           *struct {
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
			} `json:"scan"`
			Devices []struct {
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
			} `json:"devices"`
		} `json:"pools"`
	}

	if err := json.Unmarshal(zfsData, &report); err != nil {
		return fmt.Errorf("failed to parse ZFS report: %w", err)
	}

	if !report.Available || len(report.Pools) == 0 {
		return nil
	}

	for _, pool := range report.Pools {
		// Upsert pool
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

		if pool.Scan != nil {
			dbPool.ScanFunction = pool.Scan.Function
			dbPool.ScanState = pool.Scan.State
			dbPool.ScanProgress = pool.Scan.ProgressPct
			if !pool.Scan.StartTime.IsZero() {
				dbPool.LastScanTime = pool.Scan.StartTime
			}
		}

		poolID, err := UpsertZFSPool(dbPool)
		if err != nil {
			log.Printf("⚠️  Failed to upsert ZFS pool %s: %v", pool.Name, err)
			continue
		}

		// Process devices
		for _, dev := range pool.Devices {
			dbDevice := &ZFSPoolDevice{
				PoolID:         poolID,
				Hostname:       hostname,
				PoolName:       pool.Name,
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

			if err := UpsertZFSPoolDevice(poolID, dbDevice); err != nil {
				log.Printf("⚠️  Failed to upsert ZFS device %s: %v", dev.Name, err)
			}
		}

		// Record scrub history if applicable
		if pool.Scan != nil && pool.Scan.Function != "" && pool.Scan.Function != "none" {
			// Check if we need to record this scrub
			lastScrub, _ := GetLastScrub(poolID)
			shouldRecord := lastScrub == nil ||
				pool.Scan.State == "finished" && lastScrub.State != "finished" ||
				!pool.Scan.StartTime.IsZero() && pool.Scan.StartTime.After(lastScrub.StartTime)

			if shouldRecord {
				scrubRecord := &ZFSScrubHistory{
					PoolID:          poolID,
					Hostname:        hostname,
					PoolName:        pool.Name,
					ScanType:        pool.Scan.Function,
					State:           pool.Scan.State,
					StartTime:       pool.Scan.StartTime,
					EndTime:         pool.Scan.EndTime,
					DurationSecs:    pool.Scan.Duration,
					DataExamined:    pool.Scan.DataExamined,
					DataTotal:       pool.Scan.DataTotal,
					ErrorsFound:     pool.Scan.ErrorsFound,
					BytesRepaired:   pool.Scan.BytesRepaired,
					ProgressPct:     pool.Scan.ProgressPct,
					RateBytesPerSec: pool.Scan.Rate,
					TimeRemaining:   pool.Scan.TimeRemaining,
				}
				if _, err := InsertZFSScrubHistory(scrubRecord); err != nil {
					log.Printf("⚠️  Failed to insert scrub history: %v", err)
				}
			}
		}
	}

	return nil
}

// ─── Helper Functions ────────────────────────────────────────────────────────

// parseNullTime parses a nullable time string
func parseNullTime(ns sql.NullString) time.Time {
	if !ns.Valid || ns.String == "" {
		return time.Time{}
	}
	t, _ := time.Parse("2006-01-02 15:04:05", ns.String)
	return t
}

// nullTimeString converts a time to a nullable string for DB storage
func nullTimeString(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t.Format("2006-01-02 15:04:05")
}

// boolToInt converts a bool to int for SQLite storage
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
