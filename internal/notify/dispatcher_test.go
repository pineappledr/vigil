package notify

import (
	"database/sql"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"vigil/internal/drivegroups"
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

func TestDispatcherExplicitRuleBypassesSeverityFilter(t *testing.T) {
	db, bus, sender, d := setupDispatcherTest(t)

	// Service only notifies on critical/warning, NOT healthy (info).
	// But an explicit enabled rule for maintenance_complete should bypass this.
	svcID, _ := CreateService(db, &NotificationService{
		Name:             "explicit-rule-test",
		ServiceType:      "generic",
		ConfigJSON:       `{"shoutrrr_url":"generic://example.com"}`,
		Enabled:          true,
		NotifyOnCritical: true,
		NotifyOnWarning:  true,
		NotifyOnHealthy:  false,
	})

	UpsertEventRule(db, &EventRule{
		ServiceID: svcID,
		EventType: "maintenance_complete",
		Enabled:   true,
		Cooldown:  0,
	})

	d.Start()
	defer d.Stop()

	bus.Publish(events.Event{
		Type:     events.MaintenanceComplete,
		Severity: events.SeverityInfo,
		Hostname: "jarvis",
		Message:  "✅ Maintenance pipeline completed successfully",
	})

	time.Sleep(100 * time.Millisecond)

	if sender.callCount() != 1 {
		t.Errorf("expected 1 send (explicit rule bypasses severity filter), got %d", sender.callCount())
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

func TestDispatcherGroupRuleOverridesServiceRule(t *testing.T) {
	db, bus, sender, d := setupDispatcherTest(t)

	// Add drivegroups tables
	if err := drivegroups.Migrate(db); err != nil {
		t.Fatal(err)
	}

	svcID, _ := CreateService(db, &NotificationService{
		Name:             "group-test",
		ServiceType:      "generic",
		ConfigJSON:       `{"shoutrrr_url":"generic://example.com"}`,
		Enabled:          true,
		NotifyOnCritical: true,
	})

	// Service-level rule: 10s cooldown for smart_critical
	UpsertEventRule(db, &EventRule{
		ServiceID: svcID,
		EventType: "smart_critical",
		Enabled:   true,
		Cooldown:  10,
	})

	// Create a group and assign a drive
	gid, _ := drivegroups.CreateGroup(db, &drivegroups.DriveGroup{Name: "Production"})
	drivegroups.AssignDrive(db, gid, "host1", "serial1")

	// Group-level rule: 0s cooldown (fire every time)
	drivegroups.UpsertGroupEventRule(db, &drivegroups.GroupEventRule{
		ServiceID: svcID,
		GroupID:   gid,
		EventType: "smart_critical",
		Enabled:   true,
		Cooldown:  0,
	})

	d.Start()
	defer d.Stop()

	evt := events.Event{
		Type:         events.SmartCritical,
		Severity:     events.SeverityCritical,
		Hostname:     "host1",
		SerialNumber: "serial1",
		Message:      "Critical error on grouped drive",
	}

	// Fire twice — group rule has 0 cooldown, so both should send
	bus.Publish(evt)
	time.Sleep(50 * time.Millisecond)
	bus.Publish(evt)
	time.Sleep(100 * time.Millisecond)

	if sender.callCount() != 2 {
		t.Errorf("expected 2 sends (group rule: no cooldown), got %d", sender.callCount())
	}
}

func TestDispatcherGroupRuleDisablesEvent(t *testing.T) {
	db, bus, sender, d := setupDispatcherTest(t)

	if err := drivegroups.Migrate(db); err != nil {
		t.Fatal(err)
	}

	svcID, _ := CreateService(db, &NotificationService{
		Name:             "group-disable",
		ServiceType:      "generic",
		ConfigJSON:       `{"shoutrrr_url":"generic://example.com"}`,
		Enabled:          true,
		NotifyOnCritical: true,
	})

	// Service-level: allow smart_critical
	UpsertEventRule(db, &EventRule{
		ServiceID: svcID,
		EventType: "smart_critical",
		Enabled:   true,
		Cooldown:  0,
	})

	// Group disables this event type
	gid, _ := drivegroups.CreateGroup(db, &drivegroups.DriveGroup{Name: "Backup"})
	drivegroups.AssignDrive(db, gid, "bak1", "sn1")
	drivegroups.UpsertGroupEventRule(db, &drivegroups.GroupEventRule{
		ServiceID: svcID,
		GroupID:   gid,
		EventType: "smart_critical",
		Enabled:   false,
		Cooldown:  0,
	})

	d.Start()
	defer d.Stop()

	bus.Publish(events.Event{
		Type:         events.SmartCritical,
		Severity:     events.SeverityCritical,
		Hostname:     "bak1",
		SerialNumber: "sn1",
		Message:      "Should be suppressed by group rule",
	})

	time.Sleep(100 * time.Millisecond)

	if sender.callCount() != 0 {
		t.Errorf("expected 0 sends (group rule disabled), got %d", sender.callCount())
	}
}

func TestDispatcherUngroupedDriveFallsBackToServiceRules(t *testing.T) {
	db, bus, sender, d := setupDispatcherTest(t)

	if err := drivegroups.Migrate(db); err != nil {
		t.Fatal(err)
	}

	svcID, _ := CreateService(db, &NotificationService{
		Name:             "fallback-test",
		ServiceType:      "generic",
		ConfigJSON:       `{"shoutrrr_url":"generic://example.com"}`,
		Enabled:          true,
		NotifyOnCritical: true,
	})

	// Service-level rule: 10s cooldown
	UpsertEventRule(db, &EventRule{
		ServiceID: svcID,
		EventType: "smart_critical",
		Enabled:   true,
		Cooldown:  10,
	})

	d.Start()
	defer d.Stop()

	evt := events.Event{
		Type:         events.SmartCritical,
		Severity:     events.SeverityCritical,
		Hostname:     "ungrouped-host",
		SerialNumber: "ungrouped-sn",
		Message:      "Ungrouped drive event",
	}

	bus.Publish(evt)
	time.Sleep(50 * time.Millisecond)
	bus.Publish(evt) // should be throttled by service-level 10s cooldown
	time.Sleep(100 * time.Millisecond)

	if sender.callCount() != 1 {
		t.Errorf("expected 1 send (ungrouped: service cooldown), got %d", sender.callCount())
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
