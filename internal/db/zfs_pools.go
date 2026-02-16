package db

import (
	"database/sql"
	"fmt"
	"time"
)

// ─── Pool CRUD Operations ────────────────────────────────────────────────────

// UpsertZFSPool inserts or updates a ZFS pool record
// Uses SELECT + INSERT/UPDATE pattern for SQLite compatibility
func UpsertZFSPool(pool *ZFSPool) (int64, error) {
	now := NowString()

	// Check if pool exists
	existingID, err := GetID(
		"SELECT id FROM zfs_pools WHERE hostname = ? AND pool_name = ?",
		pool.Hostname, pool.PoolName,
	)
	if err != nil {
		return 0, fmt.Errorf("check pool exists: %w", err)
	}

	if existingID == 0 {
		// Insert new pool
		result, err := DB.Exec(`
			INSERT INTO zfs_pools (
				hostname, pool_name, pool_guid, status, health,
				size_bytes, allocated_bytes, free_bytes,
				fragmentation, capacity_pct, dedup_ratio, altroot,
				read_errors, write_errors, checksum_errors,
				scan_function, scan_state, scan_progress, last_scan_time,
				last_seen, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			pool.Hostname, pool.PoolName, pool.PoolGUID, pool.Status, pool.Health,
			pool.SizeBytes, pool.AllocatedBytes, pool.FreeBytes,
			pool.Fragmentation, pool.CapacityPct, pool.DedupRatio, pool.Altroot,
			pool.ReadErrors, pool.WriteErrors, pool.ChecksumErrors,
			pool.ScanFunction, pool.ScanState, pool.ScanProgress, NullTimeString(pool.LastScanTime),
			now, now,
		)
		if err != nil {
			return 0, fmt.Errorf("insert ZFS pool: %w", err)
		}
		return result.LastInsertId()
	}

	// Update existing pool
	_, err = DB.Exec(`
		UPDATE zfs_pools SET
			pool_guid = ?, status = ?, health = ?,
			size_bytes = ?, allocated_bytes = ?, free_bytes = ?,
			fragmentation = ?, capacity_pct = ?, dedup_ratio = ?, altroot = ?,
			read_errors = ?, write_errors = ?, checksum_errors = ?,
			scan_function = ?, scan_state = ?, scan_progress = ?, last_scan_time = ?,
			last_seen = ?
		WHERE id = ?
	`,
		pool.PoolGUID, pool.Status, pool.Health,
		pool.SizeBytes, pool.AllocatedBytes, pool.FreeBytes,
		pool.Fragmentation, pool.CapacityPct, pool.DedupRatio, pool.Altroot,
		pool.ReadErrors, pool.WriteErrors, pool.ChecksumErrors,
		pool.ScanFunction, pool.ScanState, pool.ScanProgress, NullTimeString(pool.LastScanTime),
		now, existingID,
	)
	if err != nil {
		return 0, fmt.Errorf("update ZFS pool: %w", err)
	}

	return existingID, nil
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