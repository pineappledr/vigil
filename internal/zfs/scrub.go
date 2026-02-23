package zfs

import (
	"database/sql"
	"fmt"
	"time"
)

// ─── Scrub History Operations ────────────────────────────────────────────────

// InsertZFSScrubHistory adds a new scrub/resilver history record
func InsertZFSScrubHistory(db *sql.DB, record *ZFSScrubHistory) (int64, error) {
	// CRITICAL: Validate start_time - use current time if not set (NOT NULL constraint)
	startTime := record.StartTime
	if startTime.IsZero() {
		startTime = time.Now()
	}

	result, err := db.Exec(`
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
		startTime.Format(timeFormat), nullTimeString(record.EndTime), record.DurationSecs,
		record.DataExamined, record.DataTotal, record.ErrorsFound,
		record.BytesRepaired, record.BlocksRepaired,
		record.ProgressPct, record.RateBytesPerSec, record.TimeRemaining,
		nowString(),
	)

	if err != nil {
		return 0, fmt.Errorf("insert scrub history: %w", err)
	}

	return result.LastInsertId()
}

// GetZFSScrubHistory retrieves scrub history for a pool
func GetZFSScrubHistory(db *sql.DB, poolID int64, limit int) ([]ZFSScrubHistory, error) {
	if limit <= 0 {
		limit = 10
	}

	rows, err := db.Query(`
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
		return nil, fmt.Errorf("query scrub history: %w", err)
	}
	defer rows.Close()

	return scanScrubHistory(rows)
}

// GetLastScrub retrieves the most recent scrub for a pool
func GetLastScrub(db *sql.DB, poolID int64) (*ZFSScrubHistory, error) {
	history, err := GetZFSScrubHistory(db, poolID, 1)
	if err != nil {
		return nil, err
	}
	if len(history) == 0 {
		return nil, nil
	}
	return &history[0], nil
}

// GetScrubHistoryByHostname retrieves scrub history for all pools on a host
func GetScrubHistoryByHostname(db *sql.DB, hostname string, limit int) ([]ZFSScrubHistory, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := db.Query(`
		SELECT id, pool_id, hostname, pool_name, scan_type, state,
			start_time, end_time, duration_secs,
			data_examined, data_total, errors_found,
			bytes_repaired, blocks_repaired,
			progress_pct, rate_bytes_sec, time_remaining,
			created_at
		FROM zfs_scrub_history
		WHERE hostname = ?
		ORDER BY start_time DESC
		LIMIT ?
	`, hostname, limit)
	if err != nil {
		return nil, fmt.Errorf("query scrub history by hostname: %w", err)
	}
	defer rows.Close()

	return scanScrubHistory(rows)
}

// ScrubRecordExists checks if a scrub record already exists
func ScrubRecordExists(db *sql.DB, poolID int64, startTime time.Time) (bool, error) {
	if startTime.IsZero() {
		return false, nil
	}

	return existsQuery(db,
		"SELECT 1 FROM zfs_scrub_history WHERE pool_id = ? AND start_time = ?",
		poolID, startTime.Format(timeFormat),
	)
}

// DeleteOldScrubHistory removes scrub records older than retention period
func DeleteOldScrubHistory(db *sql.DB, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	result, err := db.Exec(
		"DELETE FROM zfs_scrub_history WHERE start_time < ?",
		cutoff.Format(timeFormat),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// ─── Helper Functions ────────────────────────────────────────────────────────

func scanScrubHistory(rows *sql.Rows) ([]ZFSScrubHistory, error) {
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
			return nil, fmt.Errorf("scan scrub history row: %w", err)
		}

		rec.StartTime = parseNullTime(startTime)
		rec.EndTime = parseNullTime(endTime)
		rec.CreatedAt = parseNullTime(createdAt)

		history = append(history, rec)
	}

	return history, nil
}
