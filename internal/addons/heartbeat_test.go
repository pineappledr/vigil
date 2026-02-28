package addons

import (
	"database/sql"
	"testing"
	"time"

	"vigil/internal/events"

	_ "modernc.org/sqlite"
)

func setupHeartbeatDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func registerTestAddon(t *testing.T, db *sql.DB, name string) int64 {
	t.Helper()
	id, err := Register(db, name, "1.0", "test addon", `{"name":"`+name+`","version":"1.0","pages":[]}`)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestHeartbeatMonitor_DegradeOnMissedBeats(t *testing.T) {
	db := setupHeartbeatDB(t)
	bus := events.NewBus()

	id := registerTestAddon(t, db, "test-addon")

	// Set last_seen to 10 minutes ago
	_, err := db.Exec(`UPDATE addons SET last_seen = ? WHERE id = ?`,
		time.Now().UTC().Add(-10*time.Minute).Format(timeFormat), id)
	if err != nil {
		t.Fatal(err)
	}

	var received []events.Event
	bus.Subscribe(func(e events.Event) { received = append(received, e) })

	hm := NewHeartbeatMonitor(db, bus, 1*time.Minute, 3)
	hm.check()

	// Addon should now be degraded
	addon, _ := Get(db, id)
	if addon.Status != StatusDegraded {
		t.Errorf("expected status %q, got %q", StatusDegraded, addon.Status)
	}

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Type != events.AddonDegraded {
		t.Errorf("expected event type %q, got %q", events.AddonDegraded, received[0].Type)
	}
}

func TestHeartbeatMonitor_RecoverOnHeartbeat(t *testing.T) {
	db := setupHeartbeatDB(t)
	bus := events.NewBus()

	id := registerTestAddon(t, db, "test-addon")

	// Mark as degraded but with recent heartbeat
	if err := UpdateStatus(db, id, StatusDegraded); err != nil {
		t.Fatal(err)
	}
	if err := TouchHeartbeat(db, id); err != nil {
		t.Fatal(err)
	}

	var received []events.Event
	bus.Subscribe(func(e events.Event) { received = append(received, e) })

	hm := NewHeartbeatMonitor(db, bus, 1*time.Minute, 3)
	hm.check()

	addon, _ := Get(db, id)
	if addon.Status != StatusOnline {
		t.Errorf("expected status %q, got %q", StatusOnline, addon.Status)
	}

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Type != events.AddonOnline {
		t.Errorf("expected event type %q, got %q", events.AddonOnline, received[0].Type)
	}
}

func TestHeartbeatMonitor_NoDuplicateDegradeEvents(t *testing.T) {
	db := setupHeartbeatDB(t)
	bus := events.NewBus()

	id := registerTestAddon(t, db, "test-addon")

	// Set last_seen to 10 minutes ago AND already degraded
	_, err := db.Exec(`UPDATE addons SET last_seen = ?, status = ? WHERE id = ?`,
		time.Now().UTC().Add(-10*time.Minute).Format(timeFormat), string(StatusDegraded), id)
	if err != nil {
		t.Fatal(err)
	}

	var received []events.Event
	bus.Subscribe(func(e events.Event) { received = append(received, e) })

	hm := NewHeartbeatMonitor(db, bus, 1*time.Minute, 3)
	hm.check()

	// Already degraded — should NOT emit again
	if len(received) != 0 {
		t.Errorf("expected 0 events for already-degraded addon, got %d", len(received))
	}
}

func TestHeartbeatMonitor_SkipsDisabledAddons(t *testing.T) {
	db := setupHeartbeatDB(t)
	bus := events.NewBus()

	id := registerTestAddon(t, db, "disabled-addon")

	// Disable the addon
	if err := SetEnabled(db, id, false); err != nil {
		t.Fatal(err)
	}
	// Set last_seen far back
	_, _ = db.Exec(`UPDATE addons SET last_seen = ? WHERE id = ?`,
		time.Now().UTC().Add(-1*time.Hour).Format(timeFormat), id)

	var received []events.Event
	bus.Subscribe(func(e events.Event) { received = append(received, e) })

	hm := NewHeartbeatMonitor(db, bus, 1*time.Minute, 3)
	hm.check()

	// Disabled addon should not generate events
	if len(received) != 0 {
		t.Errorf("expected 0 events for disabled addon, got %d", len(received))
	}
}

func TestHeartbeatMonitor_SkipsNeverSeenAddons(t *testing.T) {
	db := setupHeartbeatDB(t)
	bus := events.NewBus()

	// Register but never touch heartbeat — last_seen is set to CURRENT_TIMESTAMP by Register
	// but for this test, set it to empty
	id := registerTestAddon(t, db, "new-addon")
	_, _ = db.Exec(`UPDATE addons SET last_seen = '' WHERE id = ?`, id)

	var received []events.Event
	bus.Subscribe(func(e events.Event) { received = append(received, e) })

	hm := NewHeartbeatMonitor(db, bus, 1*time.Minute, 3)
	hm.check()

	if len(received) != 0 {
		t.Errorf("expected 0 events for never-seen addon, got %d", len(received))
	}
}

func TestHeartbeatMonitor_StartStop(t *testing.T) {
	db := setupHeartbeatDB(t)
	bus := events.NewBus()

	hm := NewHeartbeatMonitor(db, bus, 100*time.Millisecond, 3)
	hm.Start()

	// Should not start twice
	hm.Start()

	time.Sleep(50 * time.Millisecond)
	hm.Stop()

	// Should not panic on double stop
	hm.Stop()
}
