package addons

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"vigil/internal/events"
)

// ─── WebSocket Frame Protocol ─────────────────────────────────────────────

// TelemetryFrame is the wire format for messages sent over the WebSocket.
type TelemetryFrame struct {
	Type    string          `json:"type"`    // progress, log, notification, heartbeat
	Payload json.RawMessage `json:"payload"` // type-specific data
}

// ProgressPayload is the payload for "progress" frames.
type ProgressPayload struct {
	JobID      string  `json:"job_id"`
	Phase      string  `json:"phase"`
	Percent    float64 `json:"percent"`
	Message    string  `json:"message,omitempty"`
	ETA        string  `json:"eta,omitempty"`
	BytesDone  int64   `json:"bytes_done,omitempty"`
	BytesTotal int64   `json:"bytes_total,omitempty"`
}

// LogPayload is the payload for "log" frames.
type LogPayload struct {
	Level   string `json:"level"` // info, warn, error
	Message string `json:"message"`
	Source  string `json:"source,omitempty"`
}

// NotificationPayload is the payload for "notification" frames.
// These are forwarded to the event bus for Shoutrrr dispatch.
type NotificationPayload struct {
	EventType string `json:"event_type"` // maps to events.EventType
	Severity  string `json:"severity"`   // info, warning, critical
	Message   string `json:"message"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// ─── WebSocket Hub ────────────────────────────────────────────────────────

// WebSocketHub manages active WebSocket connections for add-ons.
type WebSocketHub struct {
	db       *sql.DB
	bus      *events.Bus
	broker   *TelemetryBroker
	upgrader websocket.Upgrader

	mu    sync.Mutex
	conns map[int64]*wsConn // addon_id → active connection
}

// wsConn wraps a WebSocket connection with its metadata.
type wsConn struct {
	conn    *websocket.Conn
	addonID int64
	done    chan struct{}
}

// NewWebSocketHub creates a hub for managing add-on WebSocket connections.
func NewWebSocketHub(db *sql.DB, bus *events.Bus, broker *TelemetryBroker) *WebSocketHub {
	return &WebSocketHub{
		db:     db,
		bus:    bus,
		broker: broker,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin:     func(r *http.Request) bool { return true },
		},
		conns: make(map[int64]*wsConn),
	}
}

// HandleConnection is the HTTP handler that upgrades to WebSocket.
// It validates the add-on exists and is enabled before accepting the connection.
//
// Query parameters:
//   - addon_id: required, integer add-on ID
func (h *WebSocketHub) HandleConnection(w http.ResponseWriter, r *http.Request) {
	addonIDStr := r.URL.Query().Get("addon_id")
	if addonIDStr == "" {
		http.Error(w, "addon_id required", http.StatusBadRequest)
		return
	}

	var addonID int64
	if _, err := fmt.Sscanf(addonIDStr, "%d", &addonID); err != nil {
		http.Error(w, "invalid addon_id", http.StatusBadRequest)
		return
	}

	// Validate addon exists and is enabled
	addon, err := Get(h.db, addonID)
	if err != nil {
		log.Printf("[WS] Error looking up addon %d: %v", addonID, err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}
	if addon == nil {
		http.Error(w, "add-on not found", http.StatusNotFound)
		return
	}
	if !addon.Enabled {
		http.Error(w, "add-on is disabled", http.StatusForbidden)
		return
	}

	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade failed for addon %d: %v", addonID, err)
		return
	}

	wc := &wsConn{
		conn:    conn,
		addonID: addonID,
		done:    make(chan struct{}),
	}

	// Close any existing connection for this addon
	h.mu.Lock()
	if prev, ok := h.conns[addonID]; ok {
		close(prev.done)
		prev.conn.Close()
	}
	h.conns[addonID] = wc
	h.mu.Unlock()

	// Touch heartbeat on connect
	TouchHeartbeat(h.db, addonID)
	if addon.Status != StatusOnline {
		UpdateStatus(h.db, addonID, StatusOnline)
	}

	log.Printf("[WS] Add-on %q (id=%d) connected", addon.Name, addonID)

	// Start read loop (blocks until connection closes)
	h.readLoop(wc)

	// Cleanup
	h.mu.Lock()
	if h.conns[addonID] == wc {
		delete(h.conns, addonID)
	}
	h.mu.Unlock()

	log.Printf("[WS] Add-on %q (id=%d) disconnected", addon.Name, addonID)
}

// readLoop reads frames from the WebSocket and dispatches them.
func (h *WebSocketHub) readLoop(wc *wsConn) {
	defer wc.conn.Close()

	// Configure connection limits
	wc.conn.SetReadLimit(64 * 1024) // 64KB max message
	wc.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	wc.conn.SetPongHandler(func(string) error {
		wc.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	// Start ping writer in background
	go h.pingLoop(wc)

	for {
		_, message, err := wc.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[WS] Read error addon %d: %v", wc.addonID, err)
			}
			return
		}

		// Reset read deadline on any message
		wc.conn.SetReadDeadline(time.Now().Add(90 * time.Second))

		var frame TelemetryFrame
		if err := json.Unmarshal(message, &frame); err != nil {
			log.Printf("[WS] Invalid frame from addon %d: %v", wc.addonID, err)
			continue
		}

		h.handleFrame(wc.addonID, frame)
	}
}

// pingLoop sends periodic pings to keep the connection alive.
func (h *WebSocketHub) pingLoop(wc *wsConn) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-wc.done:
			return
		case <-ticker.C:
			if err := wc.conn.WriteControl(
				websocket.PingMessage, nil,
				time.Now().Add(10*time.Second),
			); err != nil {
				return
			}
		}
	}
}

// handleFrame routes a parsed frame to the appropriate handler.
func (h *WebSocketHub) handleFrame(addonID int64, frame TelemetryFrame) {
	switch frame.Type {
	case "heartbeat":
		TouchHeartbeat(h.db, addonID)

	case "progress":
		h.handleProgress(addonID, frame.Payload)

	case "log":
		h.handleLog(addonID, frame.Payload)

	case "notification":
		h.handleNotification(addonID, frame.Payload)

	default:
		// Forward unknown types to the broker as-is for SSE clients
		h.broker.Publish(TelemetryEvent{
			AddonID: addonID,
			Type:    frame.Type,
			Payload: frame.Payload,
		})
	}
}

// handleProgress bridges a progress frame to the SSE broker and publishes
// phase-complete events to the event bus.
func (h *WebSocketHub) handleProgress(addonID int64, raw json.RawMessage) {
	var p ProgressPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		log.Printf("[WS] Invalid progress payload from addon %d: %v", addonID, err)
		return
	}

	// Bridge to SSE subscribers
	h.broker.Publish(TelemetryEvent{
		AddonID: addonID,
		Type:    "progress",
		Payload: raw,
	})

	// Publish phase-complete events
	if p.Percent >= 100 {
		h.bus.Publish(events.Event{
			Type:     events.PhaseComplete,
			Severity: events.SeverityInfo,
			Message:  fmt.Sprintf("Phase %q completed for job %s", p.Phase, p.JobID),
			Metadata: map[string]string{
				"addon_id": fmt.Sprintf("%d", addonID),
				"job_id":   p.JobID,
				"phase":    p.Phase,
			},
		})
	}
}

// handleLog bridges a log frame to the SSE broker.
func (h *WebSocketHub) handleLog(addonID int64, raw json.RawMessage) {
	var l LogPayload
	if err := json.Unmarshal(raw, &l); err != nil {
		log.Printf("[WS] Invalid log payload from addon %d: %v", addonID, err)
		return
	}

	h.broker.Publish(TelemetryEvent{
		AddonID: addonID,
		Type:    "log",
		Payload: raw,
	})
}

// handleNotification bridges a notification frame to the event bus for
// Shoutrrr dispatch and also forwards to SSE subscribers.
func (h *WebSocketHub) handleNotification(addonID int64, raw json.RawMessage) {
	var n NotificationPayload
	if err := json.Unmarshal(raw, &n); err != nil {
		log.Printf("[WS] Invalid notification payload from addon %d: %v", addonID, err)
		return
	}

	// Bridge to SSE subscribers
	h.broker.Publish(TelemetryEvent{
		AddonID: addonID,
		Type:    "notification",
		Payload: raw,
	})

	// Determine event type and severity for the bus
	evtType := events.EventType(n.EventType)
	if evtType == "" {
		evtType = events.AddonDegraded // fallback
	}

	severity := events.SeverityInfo
	switch n.Severity {
	case "warning":
		severity = events.SeverityWarning
	case "critical":
		severity = events.SeverityCritical
	}

	metadata := n.Metadata
	if metadata == nil {
		metadata = make(map[string]string)
	}
	metadata["addon_id"] = fmt.Sprintf("%d", addonID)
	metadata["source"] = "addon_websocket"

	h.bus.Publish(events.Event{
		Type:     evtType,
		Severity: severity,
		Message:  n.Message,
		Metadata: metadata,
	})
}

// ActiveConnections returns the number of active WebSocket connections.
func (h *WebSocketHub) ActiveConnections() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.conns)
}

// CloseAll terminates all active WebSocket connections.
func (h *WebSocketHub) CloseAll() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for id, wc := range h.conns {
		close(wc.done)
		wc.conn.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseGoingAway, "server shutdown"),
			time.Now().Add(5*time.Second),
		)
		wc.conn.Close()
		delete(h.conns, id)
	}
}
