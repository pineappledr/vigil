package addons

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"vigil/internal/events"
)

// HeartbeatMonitor tracks per-addon heartbeats and marks add-ons as degraded
// when they miss consecutive heartbeats.
type HeartbeatMonitor struct {
	db       *sql.DB
	bus      *events.Bus
	interval time.Duration
	missed   int // consecutive misses before degraded

	mu      sync.Mutex
	running bool
	stop    chan struct{}
}

// NewHeartbeatMonitor creates a heartbeat monitor.
// interval is the expected heartbeat period; missed is how many consecutive
// intervals without a heartbeat triggers degradation (typically 3).
func NewHeartbeatMonitor(db *sql.DB, bus *events.Bus, interval time.Duration, missed int) *HeartbeatMonitor {
	if missed <= 0 {
		missed = 3
	}
	return &HeartbeatMonitor{
		db:       db,
		bus:      bus,
		interval: interval,
		missed:   missed,
		stop:     make(chan struct{}),
	}
}

// Start begins the periodic heartbeat check loop.
func (h *HeartbeatMonitor) Start() {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return
	}
	h.running = true
	h.mu.Unlock()

	go h.loop()
	log.Printf("[Heartbeat] Monitor started (interval=%s, missed=%d)", h.interval, h.missed)
}

// Stop halts the heartbeat monitor.
func (h *HeartbeatMonitor) Stop() {
	h.mu.Lock()
	if !h.running {
		h.mu.Unlock()
		return
	}
	h.running = false
	h.mu.Unlock()

	close(h.stop)
	log.Println("[Heartbeat] Monitor stopped")
}

func (h *HeartbeatMonitor) loop() {
	ticker := time.NewTicker(h.interval)
	defer ticker.Stop()

	for {
		select {
		case <-h.stop:
			return
		case <-ticker.C:
			h.check()
		}
	}
}

// check inspects all enabled add-ons and transitions status based on last_seen.
func (h *HeartbeatMonitor) check() {
	addons, err := h.enabledAddons()
	if err != nil {
		log.Printf("[Heartbeat] Failed to list addons: %v", err)
		return
	}

	deadline := time.Now().UTC().Add(-h.interval * time.Duration(h.missed))

	for _, a := range addons {
		switch {
		case a.LastSeen.IsZero():
			// Never seen — skip (waiting for first heartbeat)
			continue

		case a.LastSeen.Before(deadline) && a.Status != StatusDegraded:
			// Missed too many heartbeats → degrade
			if err := UpdateStatus(h.db, a.ID, StatusDegraded); err != nil {
				log.Printf("[Heartbeat] Failed to degrade addon %d: %v", a.ID, err)
				continue
			}
			log.Printf("[Heartbeat] Add-on %q degraded (last_seen=%s)", a.Name, a.LastSeen.Format(timeFormat))

			h.bus.Publish(events.Event{
				Type:     events.AddonDegraded,
				Severity: events.SeverityWarning,
				Message:  fmt.Sprintf("Add-on %q missed %d heartbeats", a.Name, h.missed),
				Metadata: map[string]string{
					"addon_id":   fmt.Sprintf("%d", a.ID),
					"addon_name": a.Name,
					"last_seen":  a.LastSeen.Format(timeFormat),
				},
			})

		case !a.LastSeen.Before(deadline) && a.Status == StatusDegraded:
			// Heartbeat recovered → back to online
			if err := UpdateStatus(h.db, a.ID, StatusOnline); err != nil {
				log.Printf("[Heartbeat] Failed to recover addon %d: %v", a.ID, err)
				continue
			}
			log.Printf("[Heartbeat] Add-on %q recovered", a.Name)

			h.bus.Publish(events.Event{
				Type:     events.AddonOnline,
				Severity: events.SeverityInfo,
				Message:  fmt.Sprintf("Add-on %q is back online", a.Name),
				Metadata: map[string]string{
					"addon_id":   fmt.Sprintf("%d", a.ID),
					"addon_name": a.Name,
				},
			})
		}
	}
}

// enabledAddons returns all enabled add-ons.
func (h *HeartbeatMonitor) enabledAddons() ([]Addon, error) {
	rows, err := h.db.Query(`
		SELECT id, name, version, COALESCE(description,''), manifest_json,
		       status, enabled, COALESCE(last_seen,''), created_at, updated_at
		FROM addons WHERE enabled = 1`)
	if err != nil {
		return nil, fmt.Errorf("query enabled addons: %w", err)
	}
	defer rows.Close()

	var out []Addon
	for rows.Next() {
		a, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
