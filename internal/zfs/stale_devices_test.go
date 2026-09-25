package zfs

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	vigildb "vigil/internal/db"
)

func zfsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	d.SetMaxOpenConns(1)
	t.Cleanup(func() { d.Close() })
	if err := vigildb.MigrateSchemaExtensions(d); err != nil {
		t.Fatal(err)
	}
	// The ALTERs in db.migrateSchema() run before zfs_pools exists; replay
	// the ones processPool needs (same as handlers/e2e_zfs_test.go).
	for _, col := range []string{"compress_ratio REAL DEFAULT 1.0", "scan_speed INTEGER DEFAULT 0",
		"scan_errors INTEGER DEFAULT 0", "scan_time_remaining INTEGER DEFAULT 0"} {
		d.Exec("ALTER TABLE zfs_pools ADD COLUMN " + col)
	}
	return d
}

func mirror(disks ...string) ZFSAgentPool {
	var children []ZFSAgentDevice
	for _, d := range disks {
		children = append(children, ZFSAgentDevice{Name: d, VdevType: "disk", State: "ONLINE"})
	}
	return ZFSAgentPool{Name: "Storage", Health: "ONLINE", Devices: []ZFSAgentDevice{
		{Name: "mirror-0", VdevType: "mirror", State: "ONLINE", Children: children},
	}}
}

func deviceNames(t *testing.T, d *sql.DB, poolID int64) map[string]bool {
	t.Helper()
	rows, err := d.Query("SELECT device_name FROM zfs_pool_devices WHERE pool_id = ?", poolID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var n string
		rows.Scan(&n)
		out[n] = true
	}
	return out
}

// Brain, 2026-09-24: a USB drive took `sda`, the mirror's sda2 became sdc2,
// and the pool showed sda2 + sdb2 + sdc2 — three disks in a 2-way mirror.
func TestProcessPool_RenamedDeviceDoesNotLinger(t *testing.T) {
	d := zfsTestDB(t)

	id, err := processPool(d, "Brain", mirror("sda2", "sdb2"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := processPool(d, "Brain", mirror("sdb2", "sdc2")); err != nil {
		t.Fatal(err)
	}
	got := deviceNames(t, d, id)
	want := map[string]bool{"mirror-0": true, "sdb2": true, "sdc2": true}
	if len(got) != len(want) || !got["sdb2"] || !got["sdc2"] || !got["mirror-0"] {
		t.Fatalf("devices = %v, want %v", got, want)
	}
}

// A report with no devices is a failed read, not an empty pool.
func TestProcessPool_EmptyReportKeepsDevices(t *testing.T) {
	d := zfsTestDB(t)
	id, _ := processPool(d, "Brain", mirror("sda2", "sdb2"))
	if _, err := processPool(d, "Brain", ZFSAgentPool{Name: "Storage", Health: "ONLINE"}); err != nil {
		t.Fatal(err)
	}
	if n := len(deviceNames(t, d, id)); n != 3 {
		t.Fatalf("an empty report deleted devices: %d left, want 3", n)
	}
}
