package zfs

import (
	"database/sql"
	"fmt"
	"time"
)

// ─── Dataset CRUD Operations ────────────────────────────────────────────────

// UpsertZFSDataset inserts or updates a ZFS dataset record
func UpsertZFSDataset(db *sql.DB, ds *ZFSDataset) (int64, error) {
	now := nowString()

	existingID, err := getID(db,
		"SELECT id FROM zfs_datasets WHERE pool_id = ? AND dataset_name = ?",
		ds.PoolID, ds.DatasetName,
	)
	if err != nil {
		return 0, fmt.Errorf("check dataset exists: %w", err)
	}

	if existingID == 0 {
		result, err := db.Exec(`
			INSERT INTO zfs_datasets (
				pool_id, hostname, pool_name, dataset_name,
				used_bytes, available_bytes, referenced_bytes,
				mountpoint, compress_ratio, quota_bytes,
				last_seen, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			ds.PoolID, ds.Hostname, ds.PoolName, ds.DatasetName,
			ds.UsedBytes, ds.AvailableBytes, ds.ReferencedBytes,
			ds.Mountpoint, ds.CompressRatio, ds.QuotaBytes,
			now, now,
		)
		if err != nil {
			return 0, fmt.Errorf("insert ZFS dataset: %w", err)
		}
		return result.LastInsertId()
	}

	_, err = db.Exec(`
		UPDATE zfs_datasets SET
			used_bytes = ?, available_bytes = ?, referenced_bytes = ?,
			mountpoint = ?, compress_ratio = ?, quota_bytes = ?,
			last_seen = ?
		WHERE id = ?
	`,
		ds.UsedBytes, ds.AvailableBytes, ds.ReferencedBytes,
		ds.Mountpoint, ds.CompressRatio, ds.QuotaBytes,
		now, existingID,
	)
	if err != nil {
		return 0, fmt.Errorf("update ZFS dataset: %w", err)
	}

	return existingID, nil
}

// GetDatasetsByPool retrieves all datasets for a pool
func GetDatasetsByPool(db *sql.DB, poolID int64) ([]ZFSDataset, error) {
	rows, err := db.Query(`
		SELECT id, pool_id, hostname, pool_name, dataset_name,
			used_bytes, available_bytes, referenced_bytes,
			mountpoint, compress_ratio, quota_bytes,
			last_seen, created_at
		FROM zfs_datasets
		WHERE pool_id = ?
		ORDER BY dataset_name
	`, poolID)
	if err != nil {
		return nil, fmt.Errorf("query datasets: %w", err)
	}
	defer rows.Close()

	return scanDatasets(rows)
}

// GetDatasetsByHostname retrieves all datasets for a hostname
func GetDatasetsByHostname(db *sql.DB, hostname string) ([]ZFSDataset, error) {
	rows, err := db.Query(`
		SELECT id, pool_id, hostname, pool_name, dataset_name,
			used_bytes, available_bytes, referenced_bytes,
			mountpoint, compress_ratio, quota_bytes,
			last_seen, created_at
		FROM zfs_datasets
		WHERE hostname = ?
		ORDER BY pool_name, dataset_name
	`, hostname)
	if err != nil {
		return nil, fmt.Errorf("query datasets by hostname: %w", err)
	}
	defer rows.Close()

	return scanDatasets(rows)
}

// GetAllDatasets retrieves datasets across every hostname
func GetAllDatasets(db *sql.DB) ([]ZFSDataset, error) {
	rows, err := db.Query(`
		SELECT id, pool_id, hostname, pool_name, dataset_name,
			used_bytes, available_bytes, referenced_bytes,
			mountpoint, compress_ratio, quota_bytes,
			last_seen, created_at
		FROM zfs_datasets
		ORDER BY hostname, pool_name, dataset_name
	`)
	if err != nil {
		return nil, fmt.Errorf("query all datasets: %w", err)
	}
	defer rows.Close()

	return scanDatasets(rows)
}

// DeleteStaleDatasets removes datasets not seen since cutoff
func DeleteStaleDatasets(db *sql.DB, poolID int64, cutoff time.Time) (int64, error) {
	result, err := db.Exec(
		"DELETE FROM zfs_datasets WHERE pool_id = ? AND last_seen < ?",
		poolID, cutoff.Format(timeFormat),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func scanDatasets(rows *sql.Rows) ([]ZFSDataset, error) {
	var datasets []ZFSDataset

	for rows.Next() {
		var ds ZFSDataset
		var lastSeen, createdAt sql.NullString

		err := rows.Scan(
			&ds.ID, &ds.PoolID, &ds.Hostname, &ds.PoolName, &ds.DatasetName,
			&ds.UsedBytes, &ds.AvailableBytes, &ds.ReferencedBytes,
			&ds.Mountpoint, &ds.CompressRatio, &ds.QuotaBytes,
			&lastSeen, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan dataset row: %w", err)
		}

		ds.LastSeen = parseNullTime(lastSeen)
		ds.CreatedAt = parseNullTime(createdAt)

		datasets = append(datasets, ds)
	}

	return datasets, nil
}
