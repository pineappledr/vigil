package addons

import (
	"encoding/json"
	"sync"
)

// TelemetryEvent is a typed telemetry frame sent from an add-on.
type TelemetryEvent struct {
	AddonID int64           `json:"addon_id"`
	Type    string          `json:"type"` // progress, log, notification, metric
	Payload json.RawMessage `json:"payload"`
}

// TelemetryBroker fans out telemetry events to per-addon SSE subscribers.
type TelemetryBroker struct {
	mu   sync.RWMutex
	subs map[int64][]chan TelemetryEvent
}

// NewTelemetryBroker creates a ready-to-use broker.
func NewTelemetryBroker() *TelemetryBroker {
	return &TelemetryBroker{
		subs: make(map[int64][]chan TelemetryEvent),
	}
}

// Subscribe returns a channel that receives telemetry events for the
// given add-on.  The caller must call Unsubscribe when done.
func (b *TelemetryBroker) Subscribe(addonID int64) chan TelemetryEvent {
	ch := make(chan TelemetryEvent, 64)
	b.mu.Lock()
	b.subs[addonID] = append(b.subs[addonID], ch)
	b.mu.Unlock()
	return ch
}

// Unsubscribe removes a channel from the subscriber list and closes it.
func (b *TelemetryBroker) Unsubscribe(addonID int64, ch chan TelemetryEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.subs[addonID]
	for i, s := range subs {
		if s == ch {
			b.subs[addonID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			return
		}
	}
}

// Publish sends a telemetry event to all subscribers of the given add-on.
// Non-blocking: if a subscriber's buffer is full the event is dropped.
func (b *TelemetryBroker) Publish(evt TelemetryEvent) {
	b.mu.RLock()
	subs := b.subs[evt.AddonID]
	b.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- evt:
		default:
			// subscriber too slow — drop
		}
	}
}
