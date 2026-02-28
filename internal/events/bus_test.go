package events

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestPublishCallsMatchingSubscriber(t *testing.T) {
	bus := NewBus()
	var called atomic.Bool

	bus.Subscribe(func(e Event) {
		if e.Type != SmartWarning {
			t.Errorf("expected SmartWarning, got %s", e.Type)
		}
		called.Store(true)
	}, SmartWarning)

	bus.Publish(Event{Type: SmartWarning, Message: "test"})

	if !called.Load() {
		t.Error("subscriber was not called")
	}
}

func TestSubscriberIgnoresUnmatchedTypes(t *testing.T) {
	bus := NewBus()
	var called atomic.Bool

	bus.Subscribe(func(e Event) {
		called.Store(true)
	}, SmartWarning)

	bus.Publish(Event{Type: TempAlert, Message: "temp"})

	if called.Load() {
		t.Error("subscriber should not have been called for TempAlert")
	}
}

func TestWildcardSubscriberReceivesAll(t *testing.T) {
	bus := NewBus()
	var count atomic.Int32

	bus.Subscribe(func(e Event) {
		count.Add(1)
	})

	bus.Publish(Event{Type: SmartWarning, Message: "a"})
	bus.Publish(Event{Type: TempAlert, Message: "b"})
	bus.Publish(Event{Type: JobComplete, Message: "c"})

	if count.Load() != 3 {
		t.Errorf("expected 3 calls, got %d", count.Load())
	}
}

func TestPublishSetsTimestamp(t *testing.T) {
	bus := NewBus()
	var got time.Time

	bus.Subscribe(func(e Event) {
		got = e.Timestamp
	})

	bus.Publish(Event{Type: SmartWarning, Message: "ts"})

	if got.IsZero() {
		t.Error("timestamp was not set")
	}
}

func TestPublishPreservesExplicitTimestamp(t *testing.T) {
	bus := NewBus()
	explicit := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	var got time.Time

	bus.Subscribe(func(e Event) {
		got = e.Timestamp
	})

	bus.Publish(Event{Type: SmartWarning, Message: "ts", Timestamp: explicit})

	if !got.Equal(explicit) {
		t.Errorf("expected %v, got %v", explicit, got)
	}
}

func TestConcurrentPublishSubscribe(t *testing.T) {
	bus := NewBus()
	var count atomic.Int32
	var wg sync.WaitGroup

	// Subscribe concurrently
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Subscribe(func(e Event) {
				count.Add(1)
			}, SmartWarning)
		}()
	}
	wg.Wait()

	// Publish concurrently
	for range 100 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Publish(Event{Type: SmartWarning, Message: "concurrent"})
		}()
	}
	wg.Wait()

	expected := int32(10 * 100)
	if count.Load() != expected {
		t.Errorf("expected %d, got %d", expected, count.Load())
	}
}

func TestPanicInSubscriberDoesNotCrash(t *testing.T) {
	bus := NewBus()
	var secondCalled atomic.Bool

	bus.Subscribe(func(e Event) {
		panic("bad subscriber")
	}, SmartWarning)

	bus.Subscribe(func(e Event) {
		secondCalled.Store(true)
	}, SmartWarning)

	bus.Publish(Event{Type: SmartWarning, Message: "panic test"})

	if !secondCalled.Load() {
		t.Error("second subscriber should still be called after first panics")
	}
}

func TestSeverityString(t *testing.T) {
	tests := []struct {
		s    Severity
		want string
	}{
		{SeverityInfo, "info"},
		{SeverityWarning, "warning"},
		{SeverityCritical, "critical"},
		{Severity(99), "unknown"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Severity(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}
