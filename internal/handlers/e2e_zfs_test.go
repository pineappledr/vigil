package handlers

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"vigil/internal/db"
	"vigil/internal/events"
)

// TestZFSDegradedReachesEventBus reproduces the Brain incident end-to-end
// through the real ingestion entry point (ProcessZFSFromReport), which is what
// POST /api/report calls. It asserts the exact events that stayed silent for
// three days now reach a subscriber.
func TestZFSDegradedReachesEventBus(t *testing.T) {
	if err := db.Init(filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateSchemaExtensions(db.DB); err != nil {
		t.Fatal(err)
	}
	// zfs_pools is created by MigrateSchemaExtensions; the ALTERs in db.Init's
	// migrateSchema() ran before it existed, so replay the ones this path needs.
	db.DB.Exec("ALTER TABLE zfs_pools ADD COLUMN compress_ratio REAL DEFAULT 1.0")
	db.DB.Exec("ALTER TABLE zfs_pools ADD COLUMN scan_speed INTEGER DEFAULT 0")
	db.DB.Exec("ALTER TABLE zfs_pools ADD COLUMN scan_errors INTEGER DEFAULT 0")
	db.DB.Exec("ALTER TABLE zfs_pools ADD COLUMN scan_time_remaining INTEGER DEFAULT 0")

	bus := events.NewBus()
	EventBus = bus
	t.Cleanup(func() { EventBus = nil })

	var got []events.EventType
	bus.Subscribe(func(e events.Event) { got = append(got, e.Type) })

	// Exact shape of Brain's report on 2026-08-30.
	payload := map[string]interface{}{}
	raw := `{"zfs":{"zfs_available":true,"pools":[{"name":"Storage","guid":"635275192531615609",
	  "health":"DEGRADED","status":"DEGRADED","size_bytes":7988639170560,
	  "allocated_bytes":3806516764672,"free_bytes":4182122405888,"capacity_pct":47,
	  "read_errors":2,"write_errors":547,"checksum_errors":0,
	  "devices":[{"name":"79b85ece-9d17-4299-8dc9-97ba419af8ae","state":"FAULTED",
	    "serial_number":"41L0A06SFBEG","vdev_type":"disk","read_errors":2,"write_errors":547},
	   {"name":"d2507fec-2e50-48aa-8e85-e1cb9a8a5333","state":"ONLINE",
	    "serial_number":"4190A050FBEG","vdev_type":"disk"}]}]}}`
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatal(err)
	}

	ProcessZFSFromReport("brain", payload)

	want := map[events.EventType]bool{
		events.ZFSPoolDegraded: false,
		events.ZFSDeviceFailed: false,
	}
	for _, g := range got {
		if _, ok := want[g]; ok {
			want[g] = true
		}
	}
	for k, seen := range want {
		if !seen {
			t.Errorf("event %q never published — this is the Brain bug", k)
		}
	}
	t.Logf("events published: %v", got)
}
