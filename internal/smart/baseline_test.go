package smart

import (
	"database/sql"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	agentsmart "vigil/cmd/agent/smart"
	vigildb "vigil/internal/db"
	"vigil/internal/events"
)

func baselineTestDB(t *testing.T) *sql.DB {
	t.Helper()
	d, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	d.SetMaxOpenConns(1)
	t.Cleanup(func() { d.Close() })
	if err := MigrateSmartAttributes(d); err != nil {
		t.Fatal(err)
	}
	if err := vigildb.MigrateSchemaExtensions(d); err != nil {
		t.Fatal(err)
	}
	return d
}

func storeCRC(t *testing.T, d *sql.DB, raw int64, at time.Time) {
	t.Helper()
	err := StoreSmartAttributes(d, &agentsmart.DriveSmartData{
		Hostname: "brain", SerialNumber: "4190A050FBEG", SmartPassed: true, Timestamp: at,
		Attributes: []agentsmart.SmartAttribute{
			{ID: 199, Name: "UDMA_CRC_Error_Count", RawValue: raw},
			{ID: 194, Name: "Temperature_Celsius", RawValue: 40},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func health(t *testing.T, d *sql.DB) string {
	t.Helper()
	a, err := GetDriveHealthSummary(d, "brain", "4190A050FBEG")
	if err != nil {
		t.Fatal(err)
	}
	return a.OverallHealth
}

func TestAcknowledgeCurrent_RoundTrip(t *testing.T) {
	d := baselineTestDB(t)
	t0 := time.Now().Add(-time.Hour).UTC().Truncate(time.Second)
	storeCRC(t, d, 1633, t0)

	if got := health(t, d); got != agentsmart.SeverityCritical {
		t.Fatalf("before acknowledging: got %s, want critical", got)
	}

	n, err := AcknowledgeCurrent(d, "brain", "4190A050FBEG", "Horus")
	if err != nil {
		t.Fatal(err)
	}
	// Only the error counter; temperature is not acknowledgeable.
	if n != 1 {
		t.Fatalf("acknowledged %d counters, want 1", n)
	}
	if b := LoadBaseline(d, "brain", "4190A050FBEG"); b[199] != 1633 || len(b) != 1 {
		t.Fatalf("baseline = %v, want {199:1633}", b)
	}
	list, err := ListBaselines(d, "brain")
	if err != nil || len(list) != 1 {
		t.Fatalf("ListBaselines = %v, %v", list, err)
	}
	if list[0].AcknowledgedBy != "Horus" || list[0].AcknowledgedAt.IsZero() {
		t.Fatalf("who and when must be recorded, got %+v", list[0])
	}
	if got := health(t, d); got != agentsmart.SeverityHealthy {
		t.Fatalf("acknowledged and frozen: got %s, want healthy", got)
	}

	// The cable fails again: one more CRC error is news.
	storeCRC(t, d, 1634, t0.Add(30*time.Minute))
	if got := health(t, d); got != agentsmart.SeverityCritical {
		t.Fatalf("counter grew: got %s, want critical", got)
	}

	if _, err := ClearBaseline(d, "brain", "4190A050FBEG"); err != nil {
		t.Fatal(err)
	}
	if b := LoadBaseline(d, "brain", "4190A050FBEG"); len(b) != 0 {
		t.Fatalf("after clearing, baseline = %v", b)
	}
}

func TestAcknowledgeCurrent_NothingToAcknowledgeIsAnError(t *testing.T) {
	d := baselineTestDB(t)
	storeCRC(t, d, 0, time.Now().UTC())
	if _, err := AcknowledgeCurrent(d, "brain", "4190A050FBEG", "Horus"); err == nil {
		t.Fatal("a drive with every counter at zero must not get an empty baseline silently")
	}
}

// The spam this exists to stop: an acknowledged, frozen counter publishes no
// event, so no Telegram message every 24 h forever.
func TestPublishSmartHealthEvents_AcknowledgedDrivePublishesNothing(t *testing.T) {
	bus := events.NewBus()
	var received []events.Event
	bus.Subscribe(func(e events.Event) { received = append(received, e) })

	drive := &agentsmart.DriveSmartData{
		Hostname: "brain", SerialNumber: "4190A050FBEG", SmartPassed: true,
		Attributes: []agentsmart.SmartAttribute{{ID: 199, RawValue: 1633}},
	}
	publishSmartHealthEvents(bus, drive, agentsmart.Baseline{199: 1633})
	if len(received) != 0 {
		t.Fatalf("got %d events, want 0", len(received))
	}
	drive.Attributes[0].RawValue = 1634
	publishSmartHealthEvents(bus, drive, agentsmart.Baseline{199: 1633})
	if len(received) != 1 || received[0].Type != events.SmartCritical {
		t.Fatalf("counter grew: got %+v, want one smart_critical", received)
	}
}
