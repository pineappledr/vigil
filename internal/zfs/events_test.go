package zfs

import (
	"testing"

	"vigil/internal/events"
)

func TestPublishPoolEvents_Degraded(t *testing.T) {
	bus := events.NewBus()
	var received []events.Event
	bus.Subscribe(func(e events.Event) { received = append(received, e) })

	pool := ZFSAgentPool{
		Name:   "tank",
		GUID:   "abc123",
		Health: "DEGRADED",
	}

	publishPoolEvents(bus, "server1", pool)

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Type != events.ZFSPoolDegraded {
		t.Errorf("expected type %q, got %q", events.ZFSPoolDegraded, received[0].Type)
	}
	if received[0].Severity != events.SeverityWarning {
		t.Errorf("expected severity warning, got %v", received[0].Severity)
	}
	if received[0].Hostname != "server1" {
		t.Errorf("expected hostname server1, got %q", received[0].Hostname)
	}
}

func TestPublishPoolEvents_Faulted(t *testing.T) {
	bus := events.NewBus()
	var received []events.Event
	bus.Subscribe(func(e events.Event) { received = append(received, e) })

	pool := ZFSAgentPool{Name: "tank", Health: "FAULTED"}
	publishPoolEvents(bus, "server1", pool)

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Type != events.ZFSPoolFaulted {
		t.Errorf("expected type %q, got %q", events.ZFSPoolFaulted, received[0].Type)
	}
	if received[0].Severity != events.SeverityCritical {
		t.Errorf("expected severity critical, got %v", received[0].Severity)
	}
}

func TestPublishPoolEvents_Online(t *testing.T) {
	bus := events.NewBus()
	var received []events.Event
	bus.Subscribe(func(e events.Event) { received = append(received, e) })

	pool := ZFSAgentPool{Name: "tank", Health: "ONLINE"}
	publishPoolEvents(bus, "server1", pool)

	if len(received) != 0 {
		t.Errorf("expected 0 events for ONLINE pool, got %d", len(received))
	}
}

func TestPublishDeviceEvents_FailedDevice(t *testing.T) {
	bus := events.NewBus()
	var received []events.Event
	bus.Subscribe(func(e events.Event) { received = append(received, e) })

	pool := ZFSAgentPool{
		Name:   "tank",
		Health: "ONLINE",
		Devices: []ZFSAgentDevice{
			{Name: "sda", State: "ONLINE", SerialNumber: "S1"},
			{Name: "sdb", State: "FAULTED", SerialNumber: "S2"},
		},
	}

	publishDeviceEvents(bus, "server1", pool)

	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].Type != events.ZFSDeviceFailed {
		t.Errorf("expected type %q, got %q", events.ZFSDeviceFailed, received[0].Type)
	}
	if received[0].SerialNumber != "S2" {
		t.Errorf("expected serial S2, got %q", received[0].SerialNumber)
	}
}

func TestPublishDeviceEvents_NestedChildren(t *testing.T) {
	bus := events.NewBus()
	var received []events.Event
	bus.Subscribe(func(e events.Event) { received = append(received, e) })

	pool := ZFSAgentPool{
		Name: "tank",
		Devices: []ZFSAgentDevice{
			{
				Name:  "mirror-0",
				State: "DEGRADED",
				Children: []ZFSAgentDevice{
					{Name: "sda", State: "ONLINE", SerialNumber: "S1"},
					{Name: "sdb", State: "REMOVED", SerialNumber: "S2"},
				},
			},
		},
	}

	publishDeviceEvents(bus, "server1", pool)

	// Only the REMOVED child should fire (DEGRADED is not in the failed list)
	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].SerialNumber != "S2" {
		t.Errorf("expected serial S2, got %q", received[0].SerialNumber)
	}
}
