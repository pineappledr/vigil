// internal/wearout/storage_test.go
package wearout

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

// ── Test DB setup ───────────────────────────────────────────────────────────

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}

	if err := MigrateWearoutTables(db); err != nil {
		t.Fatalf("migration failed: %v", err)
	}

	t.Cleanup(func() { db.Close() })
	return db
}

func testSnapshot(hostname, serial, driveType string, pct float64, ts time.Time) WearoutSnapshot {
	return WearoutSnapshot{
		Hostname:     hostname,
		SerialNumber: serial,
		DriveType:    driveType,
		Percentage:   pct,
		FactorsJSON:  fmt.Sprintf(`[{"name":"test","percentage":%.2f,"weight":1.0,"description":"test factor"}]`, pct),
		Timestamp:    ts,
	}
}

// ── StoreSnapshot + GetLatestSnapshot ───────────────────────────────────────

func TestStoreAndGetLatestSnapshot(t *testing.T) {
	db := setupTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	snap := testSnapshot("host1", "SN-001", "SSD", 25.5, now)
	if err := StoreSnapshot(db, snap); err != nil {
		t.Fatalf("StoreSnapshot failed: %v", err)
	}

	got, err := GetLatestSnapshot(db, "host1", "SN-001")
	if err != nil {
		t.Fatalf("GetLatestSnapshot failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected snapshot, got nil")
	}
	if got.Percentage != 25.5 {
		t.Errorf("Percentage = %.2f, want 25.50", got.Percentage)
	}
	if got.DriveType != "SSD" {
		t.Errorf("DriveType = %q, want %q", got.DriveType, "SSD")
	}
	if got.Hostname != "host1" {
		t.Errorf("Hostname = %q, want %q", got.Hostname, "host1")
	}
}

func TestGetLatestSnapshot_ReturnsNewest(t *testing.T) {
	db := setupTestDB(t)
	base := time.Now().UTC().Truncate(time.Second)

	// Insert older then newer
	StoreSnapshot(db, testSnapshot("h", "s", "SSD", 10, base.Add(-2*time.Hour)))
	StoreSnapshot(db, testSnapshot("h", "s", "SSD", 30, base.Add(-1*time.Hour)))
	StoreSnapshot(db, testSnapshot("h", "s", "SSD", 50, base))

	got, _ := GetLatestSnapshot(db, "h", "s")
	if got == nil {
		t.Fatal("expected snapshot")
	}
	if got.Percentage != 50 {
		t.Errorf("expected latest (50%%), got %.2f%%", got.Percentage)
	}
}

func TestGetLatestSnapshot_NotFound(t *testing.T) {
	db := setupTestDB(t)

	got, err := GetLatestSnapshot(db, "nonexistent", "none")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for missing drive, got %+v", got)
	}
}

// ── GetAllLatestSnapshots ───────────────────────────────────────────────────

func TestGetAllLatestSnapshots(t *testing.T) {
	db := setupTestDB(t)
	base := time.Now().UTC().Truncate(time.Second)

	// Two drives, each with two snapshots
	StoreSnapshot(db, testSnapshot("h", "drive-A", "SSD", 10, base.Add(-1*time.Hour)))
	StoreSnapshot(db, testSnapshot("h", "drive-A", "SSD", 20, base))
	StoreSnapshot(db, testSnapshot("h", "drive-B", "HDD", 40, base.Add(-1*time.Hour)))
	StoreSnapshot(db, testSnapshot("h", "drive-B", "HDD", 60, base))

	snapshots, err := GetAllLatestSnapshots(db)
	if err != nil {
		t.Fatalf("GetAllLatestSnapshots failed: %v", err)
	}

	if len(snapshots) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snapshots))
	}

	// Ordered by percentage DESC → drive-B (60%) first
	if snapshots[0].SerialNumber != "drive-B" || snapshots[0].Percentage != 60 {
		t.Errorf("first snapshot = %s %.0f%%, want drive-B 60%%", snapshots[0].SerialNumber, snapshots[0].Percentage)
	}
	if snapshots[1].SerialNumber != "drive-A" || snapshots[1].Percentage != 20 {
		t.Errorf("second snapshot = %s %.0f%%, want drive-A 20%%", snapshots[1].SerialNumber, snapshots[1].Percentage)
	}
}

// ── GetSnapshotHistory ──────────────────────────────────────────────────────

func TestGetSnapshotHistory(t *testing.T) {
	db := setupTestDB(t)
	now := time.Now().UTC().Truncate(time.Second)

	// Insert 10 days of data
	for i := 0; i < 10; i++ {
		ts := now.AddDate(0, 0, -9+i)
		StoreSnapshot(db, testSnapshot("h", "s", "SSD", float64(i*10), ts))
	}

	// Get last 5 days
	history, err := GetSnapshotHistory(db, "h", "s", 5)
	if err != nil {
		t.Fatalf("GetSnapshotHistory failed: %v", err)
	}

	if len(history) < 4 || len(history) > 6 {
		t.Errorf("expected ~5 snapshots for 5-day window, got %d", len(history))
	}

	// Should be ordered ASC by timestamp
	for i := 1; i < len(history); i++ {
		if history[i].Timestamp.Before(history[i-1].Timestamp) {
			t.Error("history not ordered by timestamp ASC")
			break
		}
	}
}

func TestGetSnapshotHistory_Empty(t *testing.T) {
	db := setupTestDB(t)

	history, err := GetSnapshotHistory(db, "h", "nonexistent", 30)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(history) != 0 {
		t.Errorf("expected empty history, got %d", len(history))
	}
}

// ── DriveSpec CRUD ──────────────────────────────────────────────────────────

func TestUpsertAndGetDriveSpec(t *testing.T) {
	db := setupTestDB(t)
	tbw := 300.0
	mtbf := int64(1_000_000)

	spec := DriveSpec{
		ModelPattern:   "Samsung SSD 870%",
		RatedTBW:       &tbw,
		RatedMTBFHours: &mtbf,
	}

	if err := UpsertDriveSpec(db, spec); err != nil {
		t.Fatalf("UpsertDriveSpec failed: %v", err)
	}

	// Match against a model name
	got, err := GetDriveSpec(db, "Samsung SSD 870 EVO 1TB")
	if err != nil {
		t.Fatalf("GetDriveSpec failed: %v", err)
	}
	if got == nil {
		t.Fatal("expected spec, got nil")
	}
	if *got.RatedTBW != 300.0 {
		t.Errorf("RatedTBW = %.0f, want 300", *got.RatedTBW)
	}
	if *got.RatedMTBFHours != 1_000_000 {
		t.Errorf("RatedMTBFHours = %d, want 1000000", *got.RatedMTBFHours)
	}
}

func TestGetDriveSpec_NoMatch(t *testing.T) {
	db := setupTestDB(t)

	got, err := GetDriveSpec(db, "Unknown Model XYZ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for unmatched model, got %+v", got)
	}
}

func TestUpsertDriveSpec_Idempotent(t *testing.T) {
	db := setupTestDB(t)
	tbw1 := 150.0
	tbw2 := 300.0

	spec1 := DriveSpec{ModelPattern: "WDC WD40%", RatedTBW: &tbw1}
	spec2 := DriveSpec{ModelPattern: "WDC WD40%", RatedTBW: &tbw2}

	UpsertDriveSpec(db, spec1)
	UpsertDriveSpec(db, spec2) // should update, not duplicate

	specs, _ := ListDriveSpecs(db)
	if len(specs) != 1 {
		t.Fatalf("expected 1 spec after upsert, got %d", len(specs))
	}
	if *specs[0].RatedTBW != 300.0 {
		t.Errorf("RatedTBW after upsert = %.0f, want 300", *specs[0].RatedTBW)
	}
}

func TestDeleteDriveSpec(t *testing.T) {
	db := setupTestDB(t)
	tbw := 100.0
	UpsertDriveSpec(db, DriveSpec{ModelPattern: "Test%", RatedTBW: &tbw})

	specs, _ := ListDriveSpecs(db)
	if len(specs) != 1 {
		t.Fatal("expected 1 spec")
	}

	if err := DeleteDriveSpec(db, specs[0].ID); err != nil {
		t.Fatalf("DeleteDriveSpec failed: %v", err)
	}

	specs, _ = ListDriveSpecs(db)
	if len(specs) != 0 {
		t.Errorf("expected 0 specs after delete, got %d", len(specs))
	}
}

func TestDeleteDriveSpec_NotFound(t *testing.T) {
	db := setupTestDB(t)

	err := DeleteDriveSpec(db, 9999)
	if err == nil {
		t.Error("expected error deleting nonexistent spec")
	}
}

func TestListDriveSpecs(t *testing.T) {
	db := setupTestDB(t)
	tbw1 := 150.0
	tbw2 := 300.0

	UpsertDriveSpec(db, DriveSpec{ModelPattern: "B-Model%", RatedTBW: &tbw1})
	UpsertDriveSpec(db, DriveSpec{ModelPattern: "A-Model%", RatedTBW: &tbw2})

	specs, err := ListDriveSpecs(db)
	if err != nil {
		t.Fatalf("ListDriveSpecs failed: %v", err)
	}
	if len(specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(specs))
	}

	// Ordered by model_pattern
	if specs[0].ModelPattern != "A-Model%" {
		t.Errorf("first spec = %q, want A-Model%%", specs[0].ModelPattern)
	}
}

// ── StoreSnapshot upsert on conflict ────────────────────────────────────────

func TestStoreSnapshot_UpsertOnConflict(t *testing.T) {
	db := setupTestDB(t)
	ts := time.Now().UTC().Truncate(time.Second)

	snap1 := testSnapshot("h", "s", "SSD", 10, ts)
	snap2 := testSnapshot("h", "s", "SSD", 20, ts) // same timestamp

	StoreSnapshot(db, snap1)
	StoreSnapshot(db, snap2)

	got, _ := GetLatestSnapshot(db, "h", "s")
	if got == nil {
		t.Fatal("expected snapshot")
	}
	// Should have been updated to 20%
	if got.Percentage != 20 {
		t.Errorf("expected upserted value 20%%, got %.2f%%", got.Percentage)
	}
}

// ── marshalFactors ──────────────────────────────────────────────────────────

func TestMarshalFactors(t *testing.T) {
	factors := []ContributingFactor{
		{Name: "test", Percentage: 50, Weight: 1.0, Description: "desc"},
	}

	got := marshalFactors(factors)
	if got == "" || got == "[]" {
		t.Errorf("marshalFactors returned empty: %q", got)
	}

	// nil/empty slice
	got = marshalFactors(nil)
	if got != "null" && got != "[]" {
		// json.Marshal(nil slice) returns "null"
		t.Logf("marshalFactors(nil) = %q (acceptable)", got)
	}
}

// ── Migration idempotency ───────────────────────────────────────────────────

func TestMigrateWearoutTables_Idempotent(t *testing.T) {
	db := setupTestDB(t) // already migrated once

	// Running again should not fail
	if err := MigrateWearoutTables(db); err != nil {
		t.Errorf("second migration should be idempotent, got: %v", err)
	}
}
