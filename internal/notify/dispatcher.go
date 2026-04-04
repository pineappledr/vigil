package notify

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/containrrr/shoutrrr"
	"vigil/internal/events"
)

// Sender abstracts message dispatch so the dispatcher can be tested
// without hitting real services.
type Sender interface {
	Send(shoutrrrURL, message string) error
}

// ShoutrrrSender dispatches via the Shoutrrr library.
type ShoutrrrSender struct{}

func (ShoutrrrSender) Send(url, message string) error {
	return shoutrrr.Send(url, message)
}

// serviceConfig is the Shoutrrr URL extracted from a service's config_json.
type serviceConfig struct {
	ShoutrrrURL string `json:"shoutrrr_url"`
}

// Dispatcher subscribes to the event bus, evaluates rules, enforces
// cooldowns and quiet hours, and dispatches via Shoutrrr.
type Dispatcher struct {
	db     *sql.DB
	bus    *events.Bus
	sender Sender

	// OnSent is called after each successful send (for metrics).
	OnSent func()
	// OnFailed is called after each failed send (for metrics).
	OnFailed func()

	// cooldowns tracks the last dispatch time per (service_id, event_type).
	mu        sync.Mutex
	cooldowns map[string]time.Time

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// NewDispatcher creates a dispatcher wired to the given bus and database.
func NewDispatcher(db *sql.DB, bus *events.Bus, sender Sender) *Dispatcher {
	if sender == nil {
		sender = ShoutrrrSender{}
	}
	d := &Dispatcher{
		db:        db,
		bus:       bus,
		sender:    sender,
		cooldowns: make(map[string]time.Time),
		stopCh:    make(chan struct{}),
	}
	return d
}

// Start subscribes to all events and begins dispatching.
func (d *Dispatcher) Start() {
	ch := make(chan events.Event, 256)

	d.bus.Subscribe(func(e events.Event) {
		select {
		case ch <- e:
		default:
			log.Printf("notify: event queue full, dropping %s event", e.Type)
		}
	})

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		for {
			select {
			case e := <-ch:
				d.handle(e)
			case <-d.stopCh:
				// Drain remaining events
				for {
					select {
					case e := <-ch:
						d.handle(e)
					default:
						return
					}
				}
			}
		}
	}()
}

// Stop signals the dispatcher goroutine to finish and waits for it.
func (d *Dispatcher) Stop() {
	close(d.stopCh)
	d.wg.Wait()
}

// handle processes a single event against all enabled services.
func (d *Dispatcher) handle(e events.Event) {
	services, err := ListEnabledServices(d.db)
	if err != nil {
		log.Printf("notify: list services: %v", err)
		return
	}

	for _, svc := range services {
		allowed, explicit := d.eventRuleAllowed(svc.ID, e)
		if !allowed {
			continue
		}

		// An explicitly enabled event rule bypasses the service-level severity
		// filter. If no explicit rule exists, apply the severity filter.
		if !explicit && !d.severityAllowed(svc, e.Severity) {
			continue
		}

		if d.inQuietHours(svc.ID, e) {
			continue
		}

		d.dispatch(svc, e)
	}
}

// severityAllowed checks the service's severity flags.
func (d *Dispatcher) severityAllowed(svc NotificationService, sev events.Severity) bool {
	switch sev {
	case events.SeverityCritical:
		return svc.NotifyOnCritical
	case events.SeverityWarning:
		return svc.NotifyOnWarning
	case events.SeverityInfo:
		return svc.NotifyOnHealthy
	default:
		return false
	}
}

// eventRuleAllowed checks per-event-type rules and enforces cooldowns.
// Returns (allowed, explicit) where explicit is true when a DB rule for this
// event type exists and is enabled. An explicit rule bypasses the service-level
// severity filter in handle(), allowing specific event types to fire regardless
// of the global Critical/Warning/Healthy threshold.
func (d *Dispatcher) eventRuleAllowed(serviceID int64, e events.Event) (allowed bool, explicit bool) {
	rules, err := GetEventRules(d.db, serviceID)
	if err != nil {
		log.Printf("notify: get rules for service %d: %v", serviceID, err)
		return true, false // fail open — if rules can't load, allow
	}

	// If no rules are configured, allow all events (severity filter still applies).
	if len(rules) == 0 {
		return true, false
	}

	for _, r := range rules {
		if r.EventType != string(e.Type) {
			continue
		}
		if !r.Enabled {
			return false, true
		}

		// Cooldown check.
		// -1 = permanent (fire once, never again until server restart).
		//  0 = no cooldown (fire every time).
		// >0 = cooldown in seconds.
		if r.Cooldown != 0 {
			// Per-source cooldown: include hostname + serial so each drive
			// gets its own independent cooldown window. For events without
			// host/serial (e.g. addon_degraded), fall back to addon_name
			// so each add-on gets its own cooldown.
			source := e.Hostname + ":" + e.SerialNumber
			if source == ":" && e.Metadata != nil {
				if name := e.Metadata["addon_name"]; name != "" {
					source = "addon:" + name
				}
			}
			key := fmt.Sprintf("%d:%s:%s", serviceID, e.Type, source)
			d.mu.Lock()
			last, seen := d.cooldowns[key]
			now := time.Now()
			if seen {
				if r.Cooldown < 0 {
					// Permanent: already fired once — suppress forever.
					d.mu.Unlock()
					return false, true
				}
				if now.Sub(last) < time.Duration(r.Cooldown)*time.Second {
					d.mu.Unlock()
					return false, true
				}
			}
			d.cooldowns[key] = now
			d.mu.Unlock()
		}

		return true, true
	}

	// Event type not in rules list — allow by default, not explicit.
	return true, false
}

// inQuietHours returns true if the event should be suppressed.
// Critical events are never suppressed by quiet hours.
func (d *Dispatcher) inQuietHours(serviceID int64, e events.Event) bool {
	if e.Severity == events.SeverityCritical {
		return false
	}

	qh, err := GetQuietHours(d.db, serviceID)
	if err != nil || qh == nil || !qh.Enabled {
		return false
	}

	now := time.Now().UTC()
	nowMinutes := now.Hour()*60 + now.Minute()

	start := parseHHMM(qh.StartTime)
	end := parseHHMM(qh.EndTime)

	if start < end {
		// e.g. 22:00–23:00
		return nowMinutes >= start && nowMinutes < end
	}
	// Wraps midnight, e.g. 22:00–07:00
	return nowMinutes >= start || nowMinutes < end
}

// dispatch sends the notification and records the result.
func (d *Dispatcher) dispatch(svc NotificationService, e events.Event) {
	var cfg serviceConfig
	if err := json.Unmarshal([]byte(svc.ConfigJSON), &cfg); err != nil {
		log.Printf("notify: bad config for service %d (%s): %v", svc.ID, svc.Name, err)
		return
	}
	if cfg.ShoutrrrURL == "" {
		log.Printf("notify: service %d (%s) has no shoutrrr_url", svc.ID, svc.Name)
		return
	}

	msg := formatMessage(e)
	err := d.sender.Send(cfg.ShoutrrrURL, msg)

	rec := &NotificationRecord{
		SettingID:    svc.ID,
		EventType:    string(e.Type),
		Hostname:     e.Hostname,
		SerialNumber: e.SerialNumber,
		Message:      msg,
	}

	if err != nil {
		rec.Status = "failed"
		rec.ErrorMessage = err.Error()
		log.Printf("notify: send to %s failed: %v", svc.Name, err)
		if d.OnFailed != nil {
			d.OnFailed()
		}
	} else {
		rec.Status = "sent"
		rec.SentAt = time.Now().UTC()
		if d.OnSent != nil {
			d.OnSent()
		}
	}

	if _, dbErr := RecordNotification(d.db, rec); dbErr != nil {
		log.Printf("notify: record history: %v", dbErr)
	}
}

// formatMessage builds a human-readable notification string.
func formatMessage(e events.Event) string {
	severity := e.Severity.String()
	msg := fmt.Sprintf("[%s] %s", severity, e.Message)
	if e.Hostname != "" {
		msg = fmt.Sprintf("[%s] [%s] %s", severity, e.Hostname, e.Message)
	}
	return msg
}

// parseHHMM converts "HH:MM" to minutes since midnight.
func parseHHMM(s string) int {
	t, err := time.Parse("15:04", s)
	if err != nil {
		return 0
	}
	return t.Hour()*60 + t.Minute()
}
