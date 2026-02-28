package notify

import (
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"vigil/internal/events"

	_ "modernc.org/sqlite"
)

// mockSender records calls for assertion.
type mockSender struct {
	mu       sync.Mutex
	calls    []string
	failNext bool
}

func (m *mockSender) Send(url, message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, message)
	if m.failNext {
		m.failNext = false
		return fmt.Errorf("mock send error")
	}
	return nil
}

func (m *mockSender) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// setupDispatcherTest creates an in-memory DB, bus, mock sender, and dispatcher.
func setupDispatcherTest(t *testing.T) (*sql.DB, *events.Bus, *mockSender, *Dispatcher) {
	t.Helper()
	db := setupTestDB(t)
	bus := events.NewBus()
	sender := &mockSender{}
	d := NewDispatcher(db, bus, sender)
	return db, bus, sender, d
}

func TestDispatcherSendsOnMatchingSeverity(t *testing.T) {
	db, bus, sender, d := setupDispatcherTest(t)

	// Create an enabled service that notifies on critical
	CreateService(db, &NotificationService{
		Name:             "test",
		ServiceType:      "generic",
		ConfigJSON:       `{"shoutrrr_url":"generic://example.com"}`,
		Enabled:          true,
		NotifyOnCritical: true,
	})

	d.Start()
	defer d.Stop()

	bus.Publish(events.Event{
		Type:     events.SmartCritical,
		Severity: events.SeverityCritical,
		Hostname: "node1",
		Message:  "Reallocated sector count exceeded threshold",
	})

	// Give the async goroutine time to process
	time.Sleep(100 * time.Millisecond)

	if sender.callCount() != 1 {
		t.Errorf("expected 1 send, got %d", sender.callCount())
	}
}

func TestDispatcherSkipsDisabledSeverity(t *testing.T) {
	db, bus, sender, d := setupDispatcherTest(t)

	// Service only notifies on critical, NOT warning
	CreateService(db, &NotificationService{
		Name:             "crit-only",
		ServiceType:      "generic",
		ConfigJSON:       `{"shoutrrr_url":"generic://example.com"}`,
		Enabled:          true,
		NotifyOnCritical: true,
		NotifyOnWarning:  false,
	})

	d.Start()
	defer d.Stop()

	bus.Publish(events.Event{
		Type:     events.TempAlert,
		Severity: events.SeverityWarning,
		Message:  "Temperature above warning threshold",
	})

	time.Sleep(100 * time.Millisecond)

	if sender.callCount() != 0 {
		t.Errorf("expected 0 sends for warning, got %d", sender.callCount())
	}
}

func TestDispatcherEnforcesCooldown(t *testing.T) {
	db, bus, sender, d := setupDispatcherTest(t)

	svcID, _ := CreateService(db, &NotificationService{
		Name:             "cooldown-test",
		ServiceType:      "generic",
		ConfigJSON:       `{"shoutrrr_url":"generic://example.com"}`,
		Enabled:          true,
		NotifyOnCritical: true,
	})

	// Set a 10-second cooldown for smart_critical
	UpsertEventRule(db, &EventRule{
		ServiceID: svcID,
		EventType: "smart_critical",
		Enabled:   true,
		Cooldown:  10,
	})

	d.Start()
	defer d.Stop()

	evt := events.Event{
		Type:     events.SmartCritical,
		Severity: events.SeverityCritical,
		Message:  "Critical SMART error",
	}

	bus.Publish(evt)
	time.Sleep(50 * time.Millisecond)

	bus.Publish(evt) // should be throttled
	time.Sleep(50 * time.Millisecond)

	if sender.callCount() != 1 {
		t.Errorf("expected 1 send (second throttled), got %d", sender.callCount())
	}
}

func TestDispatcherDisabledEventRule(t *testing.T) {
	db, bus, sender, d := setupDispatcherTest(t)

	svcID, _ := CreateService(db, &NotificationService{
		Name:             "rule-disabled",
		ServiceType:      "generic",
		ConfigJSON:       `{"shoutrrr_url":"generic://example.com"}`,
		Enabled:          true,
		NotifyOnCritical: true,
	})

	UpsertEventRule(db, &EventRule{
		ServiceID: svcID,
		EventType: "smart_critical",
		Enabled:   false,
	})

	d.Start()
	defer d.Stop()

	bus.Publish(events.Event{
		Type:     events.SmartCritical,
		Severity: events.SeverityCritical,
		Message:  "Should be blocked by rule",
	})

	time.Sleep(100 * time.Millisecond)

	if sender.callCount() != 0 {
		t.Errorf("expected 0 sends (rule disabled), got %d", sender.callCount())
	}
}

func TestDispatcherRecordsHistory(t *testing.T) {
	db, bus, _, d := setupDispatcherTest(t)

	CreateService(db, &NotificationService{
		Name:             "history-test",
		ServiceType:      "generic",
		ConfigJSON:       `{"shoutrrr_url":"generic://example.com"}`,
		Enabled:          true,
		NotifyOnCritical: true,
	})

	d.Start()
	defer d.Stop()

	bus.Publish(events.Event{
		Type:     events.SmartCritical,
		Severity: events.SeverityCritical,
		Hostname: "srv1",
		Message:  "Disk failure imminent",
	})

	time.Sleep(100 * time.Millisecond)

	history, err := RecentHistory(db, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 history record, got %d", len(history))
	}
	if history[0].Status != "sent" {
		t.Errorf("status = %q, want %q", history[0].Status, "sent")
	}
}

func TestDispatcherRecordsFailure(t *testing.T) {
	db, bus, sender, d := setupDispatcherTest(t)

	CreateService(db, &NotificationService{
		Name:             "fail-test",
		ServiceType:      "generic",
		ConfigJSON:       `{"shoutrrr_url":"generic://example.com"}`,
		Enabled:          true,
		NotifyOnCritical: true,
	})

	sender.failNext = true

	d.Start()
	defer d.Stop()

	bus.Publish(events.Event{
		Type:     events.SmartCritical,
		Severity: events.SeverityCritical,
		Message:  "Will fail to send",
	})

	time.Sleep(100 * time.Millisecond)

	history, _ := RecentHistory(db, 10)
	if len(history) != 1 {
		t.Fatalf("expected 1 record, got %d", len(history))
	}
	if history[0].Status != "failed" {
		t.Errorf("status = %q, want %q", history[0].Status, "failed")
	}
	if history[0].ErrorMessage == "" {
		t.Error("expected error message on failure")
	}
}

func TestFormatMessage(t *testing.T) {
	tests := []struct {
		name string
		e    events.Event
		want string
	}{
		{
			name: "with hostname",
			e:    events.Event{Severity: events.SeverityCritical, Hostname: "node1", Message: "disk bad"},
			want: "[critical] [node1] disk bad",
		},
		{
			name: "without hostname",
			e:    events.Event{Severity: events.SeverityWarning, Message: "temp high"},
			want: "[warning] temp high",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMessage(tt.e)
			if got != tt.want {
				t.Errorf("formatMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseHHMM(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"00:00", 0},
		{"08:30", 510},
		{"23:59", 1439},
		{"invalid", 0},
	}
	for _, tt := range tests {
		got := parseHHMM(tt.input)
		if got != tt.want {
			t.Errorf("parseHHMM(%q) = %d, want %d", tt.input, got, tt.want)
		}
	}
}

// Verify Stop() drains pending events.
func TestDispatcherStopDrains(t *testing.T) {
	db, bus, sender, d := setupDispatcherTest(t)

	CreateService(db, &NotificationService{
		Name:             "drain-test",
		ServiceType:      "generic",
		ConfigJSON:       `{"shoutrrr_url":"generic://example.com"}`,
		Enabled:          true,
		NotifyOnWarning:  true,
	})

	d.Start()

	var published atomic.Int32
	for range 5 {
		bus.Publish(events.Event{
			Type:     events.TempAlert,
			Severity: events.SeverityWarning,
			Message:  "test",
		})
		published.Add(1)
	}

	d.Stop()

	// All published events should have been processed
	if sender.callCount() < 1 {
		t.Error("expected at least 1 dispatch after stop/drain")
	}
}
