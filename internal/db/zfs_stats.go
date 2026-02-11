package db

import (
	"fmt"
)

// ─── Statistics and Aggregation ──────────────────────────────────────────────

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
		return nil, fmt.Errorf("get ZFS pool summary: %w", err)
	}

	return summary, nil
}

// GetZFSGlobalStats returns system-wide ZFS statistics
func GetZFSGlobalStats() (*ZFSGlobalStats, error) {
	stats := &ZFSGlobalStats{}

	err := DB.QueryRow(`
		SELECT 
			COUNT(*) as total_pools,
			SUM(CASE WHEN health = 'ONLINE' THEN 1 ELSE 0 END) as healthy,
			SUM(CASE WHEN health = 'DEGRADED' THEN 1 ELSE 0 END) as degraded,
			SUM(CASE WHEN health = 'FAULTED' THEN 1 ELSE 0 END) as faulted,
			COALESCE(SUM(read_errors + write_errors + checksum_errors), 0) as total_errors,
			SUM(CASE WHEN scan_state = 'scanning' THEN 1 ELSE 0 END) as active_scrubs
		FROM zfs_pools
	`).Scan(
		&stats.TotalPools,
		&stats.HealthyPools,
		&stats.DegradedPools,
		&stats.FaultedPools,
		&stats.TotalErrors,
		&stats.ActiveScrubs,
	)

	if err != nil {
		return nil, fmt.Errorf("get ZFS global stats: %w", err)
	}

	// Get device count
	DB.QueryRow("SELECT COUNT(*) FROM zfs_pool_devices").Scan(&stats.TotalDevices)

	return stats, nil
}

// GetZFSPoolListItems returns lightweight pool data for list views
func GetZFSPoolListItems() ([]ZFSPoolListItem, error) {
	rows, err := DB.Query(`
		SELECT 
			p.id, p.hostname, p.pool_name, p.status, p.health,
			p.size_bytes, p.allocated_bytes, p.free_bytes, p.capacity_pct,
			p.read_errors, p.write_errors, p.checksum_errors,
			p.scan_state, p.scan_progress,
			(SELECT COUNT(*) FROM zfs_pool_devices d WHERE d.pool_id = p.id) as device_count,
			(SELECT MAX(start_time) FROM zfs_scrub_history s WHERE s.pool_id = p.id) as last_scrub
		FROM zfs_pools p
		ORDER BY p.hostname, p.pool_name
	`)
	if err != nil {
		return nil, fmt.Errorf("query ZFS pool list: %w", err)
	}
	defer rows.Close()

	var items []ZFSPoolListItem
	for rows.Next() {
		var item ZFSPoolListItem
		var lastScrub *string

		err := rows.Scan(
			&item.ID, &item.Hostname, &item.PoolName, &item.Status, &item.Health,
			&item.SizeBytes, &item.AllocatedBytes, &item.FreeBytes, &item.CapacityPct,
			&item.ReadErrors, &item.WriteErrors, &item.ChecksumErrors,
			&item.ScanState, &item.ScanProgress,
			&item.DeviceCount, &lastScrub,
		)
		if err != nil {
			return nil, fmt.Errorf("scan ZFS pool list item: %w", err)
		}

		if lastScrub != nil {
			item.LastScrub = *lastScrub
		}

		items = append(items, item)
	}

	return items, nil
}

// GetPoolsWithErrors returns pools that have errors
func GetPoolsWithErrors() ([]ZFSPool, error) {
	return queryPools(`
		SELECT * FROM zfs_pools 
		WHERE read_errors > 0 OR write_errors > 0 OR checksum_errors > 0
		ORDER BY hostname, pool_name
	`)
}

// GetDegradedPools returns pools with non-ONLINE status
func GetDegradedPools() ([]ZFSPool, error) {
	return queryPools(`
		SELECT * FROM zfs_pools 
		WHERE health != 'ONLINE'
		ORDER BY hostname, pool_name
	`)
}

// GetPoolsNeedingScrub returns pools that haven't been scrubbed recently
func GetPoolsNeedingScrub(daysSinceLastScrub int) ([]ZFSPoolListItem, error) {
	rows, err := DB.Query(`
		SELECT 
			p.id, p.hostname, p.pool_name, p.status, p.health,
			p.size_bytes, p.allocated_bytes, p.free_bytes, p.capacity_pct,
			p.read_errors, p.write_errors, p.checksum_errors,
			p.scan_state, p.scan_progress,
			(SELECT COUNT(*) FROM zfs_pool_devices d WHERE d.pool_id = p.id) as device_count,
			(SELECT MAX(start_time) FROM zfs_scrub_history s WHERE s.pool_id = p.id) as last_scrub
		FROM zfs_pools p
		WHERE NOT EXISTS (
			SELECT 1 FROM zfs_scrub_history s 
			WHERE s.pool_id = p.id 
			AND s.start_time > datetime('now', ?)
		)
		ORDER BY p.hostname, p.pool_name
	`, fmt.Sprintf("-%d days", daysSinceLastScrub))
	if err != nil {
		return nil, fmt.Errorf("query pools needing scrub: %w", err)
	}
	defer rows.Close()

	var items []ZFSPoolListItem
	for rows.Next() {
		var item ZFSPoolListItem
		var lastScrub *string

		err := rows.Scan(
			&item.ID, &item.Hostname, &item.PoolName, &item.Status, &item.Health,
			&item.SizeBytes, &item.AllocatedBytes, &item.FreeBytes, &item.CapacityPct,
			&item.ReadErrors, &item.WriteErrors, &item.ChecksumErrors,
			&item.ScanState, &item.ScanProgress,
			&item.DeviceCount, &lastScrub,
		)
		if err != nil {
			return nil, fmt.Errorf("scan pool needing scrub: %w", err)
		}

		if lastScrub != nil {
			item.LastScrub = *lastScrub
		}

		items = append(items, item)
	}

	return items, nil
}
