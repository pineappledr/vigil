package wearout

import (
	"database/sql"
	"time"
)

// PruneHistory bounds the size of wearout_history.
//
// A snapshot is stored for every drive on every agent report, so with a 60 s
// report interval the table grows by ~1,440 rows per drive per day and is never
// pruned. Every report also reads the drive's last 365 days for the trend
// regression, so an unbounded table turns into steady CPU load.
//
//   - keepDays > 0 deletes snapshots older than keepDays (0 = keep forever).
//   - fullResolution > 0 keeps every snapshot newer than fullResolution and,
//     for older ones, only the newest snapshot per drive per hour (ids grow
//     with time). The covered date range is preserved to within an hour, so
//     the trend slope and confidence are effectively unchanged
//     (0 = no downsampling).
//
// Returns the number of rows deleted.
func PruneHistory(db *sql.DB, keepDays int, fullResolution time.Duration) (int64, error) {
	var deleted int64

	if keepDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -keepDays).UTC().Format(timeFormat)
		res, err := db.Exec(`DELETE FROM wearout_history WHERE timestamp < ?`, cutoff)
		if err != nil {
			return deleted, err
		}
		n, _ := res.RowsAffected()
		deleted += n
	}

	if fullResolution > 0 {
		cutoff := time.Now().Add(-fullResolution).UTC().Format(timeFormat)
		res, err := db.Exec(`
			DELETE FROM wearout_history
			WHERE timestamp < ?
			  AND id NOT IN (
				SELECT MAX(id) FROM wearout_history
				WHERE timestamp < ?
				GROUP BY hostname, serial_number, strftime('%Y-%m-%d %H', timestamp)
			  )`, cutoff, cutoff)
		if err != nil {
			return deleted, err
		}
		n, _ := res.RowsAffected()
		deleted += n
	}

	return deleted, nil
}
