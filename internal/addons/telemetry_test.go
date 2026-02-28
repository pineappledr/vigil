package addons

import (
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"
)

func TestTelemetryBroker_SubscribeAndPublish(t *testing.T) {
	b := NewTelemetryBroker()

	ch := b.Subscribe(1)
	defer b.Unsubscribe(1, ch)

	evt := TelemetryEvent{
		AddonID: 1,
		Type:    "progress",
		Payload: json.RawMessage(`{"pct":50}`),
	}
	b.Publish(evt)

	select {
	case got := <-ch:
		if got.Type != "progress" {
			t.Errorf("type = %q, want %q", got.Type, "progress")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for event")
	}
}

func TestTelemetryBroker_MultipleSubscribers(t *testing.T) {
	b := NewTelemetryBroker()

	ch1 := b.Subscribe(1)
	ch2 := b.Subscribe(1)
	defer b.Unsubscribe(1, ch1)
	defer b.Unsubscribe(1, ch2)

	b.Publish(TelemetryEvent{AddonID: 1, Type: "log"})

	var count atomic.Int32
	for _, ch := range []chan TelemetryEvent{ch1, ch2} {
		select {
		case <-ch:
			count.Add(1)
		case <-time.After(time.Second):
			t.Fatal("timeout")
		}
	}

	if count.Load() != 2 {
		t.Errorf("expected 2 receives, got %d", count.Load())
	}
}

func TestTelemetryBroker_IsolatedByAddonID(t *testing.T) {
	b := NewTelemetryBroker()

	ch1 := b.Subscribe(1)
	ch2 := b.Subscribe(2)
	defer b.Unsubscribe(1, ch1)
	defer b.Unsubscribe(2, ch2)

	b.Publish(TelemetryEvent{AddonID: 1, Type: "progress"})

	select {
	case <-ch1:
		// ok
	case <-time.After(time.Second):
		t.Fatal("ch1 should have received")
	}

	select {
	case <-ch2:
		t.Fatal("ch2 should NOT have received event for addon 1")
	case <-time.After(50 * time.Millisecond):
		// ok
	}
}

func TestTelemetryBroker_Unsubscribe(t *testing.T) {
	b := NewTelemetryBroker()

	ch := b.Subscribe(1)
	b.Unsubscribe(1, ch)

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Error("channel should be closed after unsubscribe")
	}
}
