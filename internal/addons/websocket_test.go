package addons

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"vigil/internal/events"

	_ "modernc.org/sqlite"
)

func setupWSTestDB(t *testing.T) *sql.DB {
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

func setupWSServer(t *testing.T, db *sql.DB, bus *events.Bus, broker *TelemetryBroker) (*httptest.Server, string) {
	t.Helper()
	hub := NewWebSocketHub(db, bus, broker)
	srv := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	t.Cleanup(func() {
		hub.CloseAll()
		srv.Close()
	})
	// Convert http:// to ws://
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")
	return srv, wsURL
}

func TestWebSocket_ConnectDisconnect(t *testing.T) {
	db := setupWSTestDB(t)
	bus := events.NewBus()
	broker := NewTelemetryBroker()

	id := registerTestAddon(t, db, "ws-addon")
	_, wsURL := setupWSServer(t, db, bus, broker)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"?addon_id="+itoa(id), nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}

	// Give server time to process the connection
	time.Sleep(50 * time.Millisecond)

	conn.Close()
	time.Sleep(50 * time.Millisecond)
}

func TestWebSocket_RejectsDisabledAddon(t *testing.T) {
	db := setupWSTestDB(t)
	bus := events.NewBus()
	broker := NewTelemetryBroker()

	id := registerTestAddon(t, db, "disabled-addon")
	SetEnabled(db, id, false)

	_, wsURL := setupWSServer(t, db, bus, broker)

	_, resp, err := websocket.DefaultDialer.Dial(wsURL+"?addon_id="+itoa(id), nil)
	if err == nil {
		t.Fatal("expected connection to be rejected")
	}
	if resp != nil && resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestWebSocket_RejectsUnknownAddon(t *testing.T) {
	db := setupWSTestDB(t)
	bus := events.NewBus()
	broker := NewTelemetryBroker()

	_, wsURL := setupWSServer(t, db, bus, broker)

	_, resp, err := websocket.DefaultDialer.Dial(wsURL+"?addon_id=9999", nil)
	if err == nil {
		t.Fatal("expected connection to be rejected")
	}
	if resp != nil && resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestWebSocket_RejectsMissingAddonID(t *testing.T) {
	db := setupWSTestDB(t)
	bus := events.NewBus()
	broker := NewTelemetryBroker()

	_, wsURL := setupWSServer(t, db, bus, broker)

	_, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err == nil {
		t.Fatal("expected connection to be rejected")
	}
	if resp != nil && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestWebSocket_HeartbeatFrame(t *testing.T) {
	db := setupWSTestDB(t)
	bus := events.NewBus()
	broker := NewTelemetryBroker()

	id := registerTestAddon(t, db, "hb-addon")
	_, wsURL := setupWSServer(t, db, bus, broker)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"?addon_id="+itoa(id), nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// Send heartbeat frame
	frame := TelemetryFrame{Type: "heartbeat"}
	data, _ := json.Marshal(frame)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// Verify heartbeat was touched (addon should be online)
	addon, _ := Get(db, id)
	if addon.Status != StatusOnline {
		t.Errorf("expected status online, got %q", addon.Status)
	}
}

func TestWebSocket_ProgressFrameBridgesToSSE(t *testing.T) {
	db := setupWSTestDB(t)
	bus := events.NewBus()
	broker := NewTelemetryBroker()

	id := registerTestAddon(t, db, "progress-addon")
	_, wsURL := setupWSServer(t, db, bus, broker)

	// Subscribe to SSE broker before connecting
	sseCh := broker.Subscribe(id)
	defer broker.Unsubscribe(id, sseCh)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"?addon_id="+itoa(id), nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// Send progress frame
	payload, _ := json.Marshal(ProgressPayload{
		JobID:   "job-1",
		Phase:   "format",
		Percent: 50.0,
		Message: "Formatting disk",
	})
	frame := TelemetryFrame{Type: "progress", Payload: payload}
	data, _ := json.Marshal(frame)
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// Verify SSE broker received it
	select {
	case evt := <-sseCh:
		if evt.Type != "progress" {
			t.Errorf("expected type progress, got %q", evt.Type)
		}
		if evt.AddonID != id {
			t.Errorf("expected addon_id %d, got %d", id, evt.AddonID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for SSE event")
	}
}

func TestWebSocket_ProgressPhaseComplete(t *testing.T) {
	db := setupWSTestDB(t)
	bus := events.NewBus()
	broker := NewTelemetryBroker()

	id := registerTestAddon(t, db, "phase-addon")
	_, wsURL := setupWSServer(t, db, bus, broker)

	var busEvents []events.Event
	bus.Subscribe(func(e events.Event) { busEvents = append(busEvents, e) })

	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"?addon_id="+itoa(id), nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// Send 100% progress
	payload, _ := json.Marshal(ProgressPayload{
		JobID:   "job-1",
		Phase:   "verify",
		Percent: 100.0,
	})
	frame := TelemetryFrame{Type: "progress", Payload: payload}
	data, _ := json.Marshal(frame)
	conn.WriteMessage(websocket.TextMessage, data)

	time.Sleep(100 * time.Millisecond)

	found := false
	for _, e := range busEvents {
		if e.Type == events.PhaseComplete {
			found = true
			if e.Metadata["phase"] != "verify" {
				t.Errorf("expected phase verify, got %q", e.Metadata["phase"])
			}
		}
	}
	if !found {
		t.Error("expected PhaseComplete event")
	}
}

func TestWebSocket_NotificationPublishesToBus(t *testing.T) {
	db := setupWSTestDB(t)
	bus := events.NewBus()
	broker := NewTelemetryBroker()

	id := registerTestAddon(t, db, "notify-addon")
	_, wsURL := setupWSServer(t, db, bus, broker)

	var busEvents []events.Event
	bus.Subscribe(func(e events.Event) { busEvents = append(busEvents, e) })

	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"?addon_id="+itoa(id), nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// Send notification frame
	payload, _ := json.Marshal(NotificationPayload{
		EventType: "job_complete",
		Severity:  "info",
		Message:   "Burn-in test passed for all drives",
		Metadata:  map[string]string{"drives": "4"},
	})
	frame := TelemetryFrame{Type: "notification", Payload: payload}
	data, _ := json.Marshal(frame)
	conn.WriteMessage(websocket.TextMessage, data)

	time.Sleep(100 * time.Millisecond)

	if len(busEvents) != 1 {
		t.Fatalf("expected 1 bus event, got %d", len(busEvents))
	}
	if busEvents[0].Type != events.EventType("job_complete") {
		t.Errorf("expected type job_complete, got %q", busEvents[0].Type)
	}
	if busEvents[0].Severity != events.SeverityInfo {
		t.Errorf("expected severity info, got %v", busEvents[0].Severity)
	}
	if busEvents[0].Metadata["source"] != "addon_websocket" {
		t.Errorf("expected source addon_websocket, got %q", busEvents[0].Metadata["source"])
	}
}

func TestWebSocket_LogFrameBridgesToSSE(t *testing.T) {
	db := setupWSTestDB(t)
	bus := events.NewBus()
	broker := NewTelemetryBroker()

	id := registerTestAddon(t, db, "log-addon")
	_, wsURL := setupWSServer(t, db, bus, broker)

	sseCh := broker.Subscribe(id)
	defer broker.Unsubscribe(id, sseCh)

	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"?addon_id="+itoa(id), nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	payload, _ := json.Marshal(LogPayload{
		Level:   "error",
		Message: "disk /dev/sda unreachable",
		Source:  "burnin",
	})
	frame := TelemetryFrame{Type: "log", Payload: payload}
	data, _ := json.Marshal(frame)
	conn.WriteMessage(websocket.TextMessage, data)

	select {
	case evt := <-sseCh:
		if evt.Type != "log" {
			t.Errorf("expected type log, got %q", evt.Type)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for log event")
	}
}

func TestWebSocket_ActiveConnections(t *testing.T) {
	db := setupWSTestDB(t)
	bus := events.NewBus()
	broker := NewTelemetryBroker()
	hub := NewWebSocketHub(db, bus, broker)

	srv := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	id := registerTestAddon(t, db, "count-addon")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL+"?addon_id="+itoa(id), nil)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	if hub.ActiveConnections() != 1 {
		t.Errorf("expected 1 active connection, got %d", hub.ActiveConnections())
	}

	conn.Close()
	time.Sleep(100 * time.Millisecond)

	if hub.ActiveConnections() != 0 {
		t.Errorf("expected 0 active connections after close, got %d", hub.ActiveConnections())
	}

	hub.CloseAll()
}

func TestWebSocket_ReplacesExistingConnection(t *testing.T) {
	db := setupWSTestDB(t)
	bus := events.NewBus()
	broker := NewTelemetryBroker()
	hub := NewWebSocketHub(db, bus, broker)

	srv := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer srv.Close()
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http")

	id := registerTestAddon(t, db, "replace-addon")

	conn1, _, err := websocket.DefaultDialer.Dial(wsURL+"?addon_id="+itoa(id), nil)
	if err != nil {
		t.Fatalf("dial 1 failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	conn2, _, err := websocket.DefaultDialer.Dial(wsURL+"?addon_id="+itoa(id), nil)
	if err != nil {
		t.Fatalf("dial 2 failed: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// Should only have 1 active connection (second replaced first)
	if hub.ActiveConnections() != 1 {
		t.Errorf("expected 1 active connection, got %d", hub.ActiveConnections())
	}

	conn1.Close()
	conn2.Close()
	time.Sleep(100 * time.Millisecond)
	hub.CloseAll()
}

// itoa converts int64 to string for URL params.
func itoa(n int64) string {
	return fmt.Sprintf("%d", n)
}
