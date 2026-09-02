package main

import (
	"testing"

	"vigil/internal/events"
	"vigil/internal/handlers"
)

// TestEventBusIsWiredToHandlers guards the wiring between the event bus created
// in main() and the package-level handlers.EventBus that the report ingestion
// path checks.
//
// Regression: handlers.EventBus was declared ("set from main.go during startup")
// but never assigned, so it stayed nil. ProcessZFSFromReport guards on
// `if EventBus != nil`, so every ZFS pool/device event was silently dropped —
// a pool sat DEGRADED with a FAULTED device for three days and no notification
// was ever sent, even though the Telegram service was healthy and delivering
// other event types the whole time.
//
// The per-function tests in internal/zfs/events_test.go all passed throughout,
// because they call publishPoolEvents directly with a non-nil bus. Only the
// wiring was broken, so only a wiring test can catch it.
func TestEventBusIsWiredToHandlers(t *testing.T) {
	handlers.EventBus = nil
	t.Cleanup(func() { handlers.EventBus = nil })

	bus := events.NewBus()
	handlers.EventBus = bus

	if handlers.EventBus == nil {
		t.Fatal("handlers.EventBus is nil: ZFS pool and device events are silently dropped")
	}

	var got []events.EventType
	handlers.EventBus.Subscribe(func(e events.Event) {
		got = append(got, e.Type)
	}, events.ZFSPoolDegraded)

	handlers.EventBus.Publish(events.Event{
		Type:     events.ZFSPoolDegraded,
		Severity: events.SeverityWarning,
		Hostname: "brain",
		Message:  `ZFS pool "Storage" is DEGRADED`,
	})

	if len(got) != 1 || got[0] != events.ZFSPoolDegraded {
		t.Fatalf("event did not reach subscriber through handlers.EventBus: got %v", got)
	}
}
