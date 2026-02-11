package db

import (
	"database/sql"
	"fmt"
	"time"
)

// ─── Pool CRUD Operations ────────────────────────────────────────────────────

// UpsertZFSPool inserts or updates a ZFS pool record
func UpsertZFSPool(pool *ZFSPool) (int64, error) {
	now := NowString()

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
		pool.ScanFunction, pool.ScanState, pool.ScanProgress, NullTimeString(pool.LastScanTime),
		now, now,
	)

	if err != nil {
		return 0, fmt.Errorf("upsert ZFS pool: %w", err)
	}

	// Get pool ID (either inserted or existing)
	id, err := result.LastInsertId()
	if err != nil || id == 0 {
		id, err = GetID("SELECT id FROM zfs_pools WHERE hostname = ? AND pool_name = ?",
			pool.Hostname, pool.PoolName)
		if err != nil {
			return 0, fmt.Errorf("get pool ID: %w", err)
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
		return nil, fmt.Errorf("get ZFS pool: %w", err)
	}

	pool.LastScanTime = ParseNullTime(lastScanTime)
	pool.LastSeen = ParseNullTime(lastSeen)
	pool.CreatedAt = ParseNullTime(createdAt)

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
		FROM zfs_pools WHERE id = ?
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
		return nil, fmt.Errorf("get ZFS pool by ID: %w", err)
	}

	pool.LastScanTime = ParseNullTime(lastScanTime)
	pool.LastSeen = ParseNullTime(lastSeen)
	pool.CreatedAt = ParseNullTime(createdAt)

	return pool, nil
}

// GetZFSPoolsByHostname retrieves all ZFS pools for a hostname
func GetZFSPoolsByHostname(hostname string) ([]ZFSPool, error) {
	return queryPools("SELECT * FROM zfs_pools WHERE hostname = ? ORDER BY pool_name", hostname)
}

// GetAllZFSPools retrieves all ZFS pools
func GetAllZFSPools() ([]ZFSPool, error) {
	return queryPools("SELECT * FROM zfs_pools ORDER BY hostname, pool_name")
}

// DeleteZFSPool removes a ZFS pool (cascades to devices/scrub history)
func DeleteZFSPool(hostname, poolName string) error {
	_, err := DB.Exec("DELETE FROM zfs_pools WHERE hostname = ? AND pool_name = ?", hostname, poolName)
	return err
}

// DeleteStaleZFSPools removes pools not seen since cutoff time
func DeleteStaleZFSPools(hostname string, cutoff time.Time) (int64, error) {
	result, err := DB.Exec(
		"DELETE FROM zfs_pools WHERE hostname = ? AND last_seen < ?",
		hostname, cutoff.Format(timeFormat),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ─── Helper Functions ────────────────────────────────────────────────────────

func queryPools(query string, args ...interface{}) ([]ZFSPool, error) {
	rows, err := DB.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query ZFS pools: %w", err)
	}
	defer rows.Close()

	return scanPools(rows)
}

func scanPools(rows *sql.Rows) ([]ZFSPool, error) {
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
			return nil, fmt.Errorf("scan ZFS pool row: %w", err)
		}

		pool.LastScanTime = ParseNullTime(lastScanTime)
		pool.LastSeen = ParseNullTime(lastSeen)
		pool.CreatedAt = ParseNullTime(createdAt)

		pools = append(pools, pool)
	}

	return pools, nil
}
