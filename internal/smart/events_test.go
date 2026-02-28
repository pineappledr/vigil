package smart

import (
	"testing"

	agentsmart "vigil/cmd/agent/smart"
	"vigil/internal/events"
)

func TestPublishSmartHealthEvents_Healthy(t *testing.T) {
	bus := events.NewBus()
	var received []events.Event
	bus.Subscribe(func(e events.Event) { received = append(received, e) })

	driveData := &agentsmart.DriveSmartData{
		Hostname:     "server1",
		SerialNumber: "ABC123",
		ModelName:    "TestModel",
		SmartPassed:  true,
		Attributes:   []agentsmart.SmartAttribute{},
	}

	publishSmartHealthEvents(bus, driveData)

	if len(received) != 0 {
		t.Errorf("expected 0 events for healthy drive, got %d", len(received))
	}
}

func TestPublishSmartHealthEvents_Critical(t *testing.T) {
	bus := events.NewBus()
	var received []events.Event
	bus.Subscribe(func(e events.Event) { received = append(received, e) })

	driveData := &agentsmart.DriveSmartData{
		Hostname:     "server1",
		SerialNumber: "ABC123",
		ModelName:    "TestModel",
		SmartPassed:  false, // SMART failed → critical
		Attributes:   []agentsmart.SmartAttribute{},
	}

	publishSmartHealthEvents(bus, driveData)

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Type != events.SmartCritical {
		t.Errorf("expected type %q, got %q", events.SmartCritical, received[0].Type)
	}
	if received[0].Severity != events.SeverityCritical {
		t.Errorf("expected severity critical, got %v", received[0].Severity)
	}
	if received[0].Hostname != "server1" {
		t.Errorf("expected hostname server1, got %q", received[0].Hostname)
	}
}

func TestPublishSmartHealthEvents_ReallocatedSectors(t *testing.T) {
	bus := events.NewBus()
	var received []events.Event
	bus.Subscribe(func(e events.Event) { received = append(received, e) })

	driveData := &agentsmart.DriveSmartData{
		Hostname:     "server1",
		SerialNumber: "ABC123",
		ModelName:    "TestModel",
		SmartPassed:  true,
		DriveType:    "HDD",
		Attributes: []agentsmart.SmartAttribute{
			{
				ID:       5, // Reallocated Sector Count
				Name:     "Reallocated_Sector_Ct",
				Value:    100,
				Worst:    100,
				RawValue: 50, // Non-zero triggers event
			},
		},
	}

	publishSmartHealthEvents(bus, driveData)

	// Should get both a ReallocatedSectors event and a SmartWarning/Critical event
	hasRealloc := false
	hasHealth := false
	for _, e := range received {
		if e.Type == events.ReallocatedSectors {
			hasRealloc = true
		}
		if e.Type == events.SmartWarning || e.Type == events.SmartCritical {
			hasHealth = true
		}
	}

	if !hasRealloc {
		t.Error("expected ReallocatedSectors event")
	}
	if !hasHealth {
		t.Error("expected SmartWarning or SmartCritical event")
	}
}

func TestMapSeverity(t *testing.T) {
	tests := []struct {
		input    string
		expected events.Severity
	}{
		{agentsmart.SeverityCritical, events.SeverityCritical},
		{agentsmart.SeverityWarning, events.SeverityWarning},
		{agentsmart.SeverityHealthy, events.SeverityInfo},
		{"UNKNOWN", events.SeverityInfo},
	}

	for _, tt := range tests {
		got := mapSeverity(tt.input)
		if got != tt.expected {
			t.Errorf("mapSeverity(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}
