package wearout

import (
	"testing"
	"time"
)

func TestPruneHistoryDownsamplesOldRows(t *testing.T) {
	db := setupTestDB(t)
	now := time.Now().UTC().Truncate(time.Hour)

	// 2 drives, one snapshot per minute for the last 3 hours and for 3 hours two
	// days ago, inserted oldest first (ids grow with time, as in production).
	for _, serial := range []string{"SN-A", "SN-B"} {
		for m := 179; m >= 0; m-- {
			recent := now.Add(-time.Duration(m) * time.Minute)
			old := now.Add(-48*time.Hour - time.Duration(m)*time.Minute)
			for _, ts := range []time.Time{recent, old} {
				if err := StoreSnapshot(db, testSnapshot("host1", serial, "HDD", 10, ts)); err != nil {
					t.Fatalf("StoreSnapshot: %v", err)
				}
			}
		}
	}

	deleted, err := PruneHistory(db, 0, 24*time.Hour)
	if err != nil {
		t.Fatalf("PruneHistory: %v", err)
	}

	var total, old int
	db.QueryRow(`SELECT COUNT(*) FROM wearout_history`).Scan(&total)
	cutoff := now.Add(-24 * time.Hour).Format(timeFormat)
	db.QueryRow(`SELECT COUNT(*) FROM wearout_history WHERE timestamp < ?`, cutoff).Scan(&old)

	// Old window spans 180 minutes ending on an hour boundary -> hour buckets
	// (t-51h, t-50h], ... = 4 distinct hours per drive.
	if old != 2*4 {
		t.Errorf("old rows after downsampling = %d, want %d", old, 2*4)
	}
	if total != 2*180+old {
		t.Errorf("total rows = %d, want %d (recent rows must be untouched)", total, 2*180+old)
	}
	if deleted != int64(2*180-old) {
		t.Errorf("deleted = %d, want %d", deleted, 2*180-old)
	}

	// The newest snapshot of each old hour is kept, so the covered span shrinks
	// by less than an hour.
	var oldest string
	db.QueryRow(`SELECT MIN(timestamp) FROM wearout_history WHERE serial_number = 'SN-A'`).Scan(&oldest)
	if want := now.Add(-48*time.Hour - 121*time.Minute).Format(timeFormat); oldest != want {
		t.Errorf("oldest kept = %s, want %s", oldest, want)
	}

	// Idempotent.
	if again, err := PruneHistory(db, 0, 24*time.Hour); err != nil || again != 0 {
		t.Errorf("second PruneHistory = %d, %v; want 0, nil", again, err)
	}
}

func TestPruneHistoryRetentionDays(t *testing.T) {
	db := setupTestDB(t)
	now := time.Now().UTC()
	for _, d := range []int{1, 10, 400} {
		if err := StoreSnapshot(db, testSnapshot("host1", "SN-A", "SSD", 5, now.AddDate(0, 0, -d))); err != nil {
			t.Fatalf("StoreSnapshot: %v", err)
		}
	}

	deleted, err := PruneHistory(db, 365, 0)
	if err != nil {
		t.Fatalf("PruneHistory: %v", err)
	}
	if deleted != 1 {
		t.Errorf("deleted = %d, want 1", deleted)
	}

	// 0 / 0 disables both policies.
	if n, err := PruneHistory(db, 0, 0); err != nil || n != 0 {
		t.Errorf("PruneHistory(0, 0) = %d, %v; want 0, nil", n, err)
	}
}
