package smart

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	agentsmart "vigil/cmd/agent/smart"
	"vigil/internal/events"
)

// BaselineEntry is one acknowledged counter of one drive.
type BaselineEntry struct {
	Hostname       string    `json:"hostname"`
	SerialNumber   string    `json:"serial_number"`
	AttributeID    int       `json:"attribute_id"`
	RawValue       int64     `json:"raw_value"`
	AcknowledgedBy string    `json:"acknowledged_by"`
	AcknowledgedAt time.Time `json:"acknowledged_at"`
}

// LoadBaseline returns the acknowledged counters of one drive. A drive with
// none (or any read error) gets an empty baseline, which is exactly the
// behaviour without this feature: failing open here means judging the drive
// normally, never hiding an issue.
func LoadBaseline(db *sql.DB, hostname, serialNumber string) agentsmart.Baseline {
	b := agentsmart.Baseline{}
	if db == nil {
		return b
	}
	rows, err := db.Query(`SELECT attribute_id, raw_value FROM drive_health_baselines
		WHERE hostname = ? AND serial_number = ?`, hostname, serialNumber)
	if err != nil {
		return b
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var v int64
		if rows.Scan(&id, &v) == nil {
			b[id] = v
		}
	}
	return b
}

// ListBaselines returns every acknowledged counter, optionally for one host.
func ListBaselines(db *sql.DB, hostname string) ([]BaselineEntry, error) {
	q := `SELECT hostname, serial_number, attribute_id, raw_value,
		COALESCE(acknowledged_by, ''), acknowledged_at FROM drive_health_baselines`
	var args []interface{}
	if hostname != "" {
		q += " WHERE hostname = ?"
		args = append(args, hostname)
	}
	q += " ORDER BY hostname, serial_number, attribute_id"
	rows, err := db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]BaselineEntry, 0)
	for rows.Next() {
		var e BaselineEntry
		var at string
		if err := rows.Scan(&e.Hostname, &e.SerialNumber, &e.AttributeID, &e.RawValue, &e.AcknowledgedBy, &at); err != nil {
			continue
		}
		e.AcknowledgedAt, _ = parseDBTime(at)
		out = append(out, e)
	}
	return out, nil
}

// AcknowledgeCurrent stores the drive's CURRENT value of every acknowledgeable
// counter that is above zero, as read from the latest stored report. The
// values come from Vigil's own data, never from the request: an operator
// acknowledges "what the drive says now", not a number they typed.
// Returns how many counters were acknowledged.
func AcknowledgeCurrent(db *sql.DB, hostname, serialNumber, by string) (int, error) {
	attrs, err := GetLatestSmartAttributes(db, hostname, serialNumber)
	if err != nil {
		return 0, err
	}
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM drive_health_baselines WHERE hostname = ? AND serial_number = ?`,
		hostname, serialNumber); err != nil {
		return 0, err
	}
	n := 0
	for _, a := range attrs {
		if !agentsmart.IsAcknowledgeable(a.ID) || a.RawValue <= 0 {
			continue
		}
		if _, err := tx.Exec(`INSERT INTO drive_health_baselines
			(hostname, serial_number, attribute_id, raw_value, acknowledged_by)
			VALUES (?, ?, ?, ?, ?)`, hostname, serialNumber, a.ID, a.RawValue, by); err != nil {
			return 0, err
		}
		n++
	}
	if n == 0 {
		return 0, fmt.Errorf("no error counters above zero to acknowledge on %s/%s", hostname, serialNumber)
	}
	return n, tx.Commit()
}

// ClearBaseline removes every acknowledged counter of one drive.
func ClearBaseline(db *sql.DB, hostname, serialNumber string) (int64, error) {
	res, err := db.Exec(`DELETE FROM drive_health_baselines WHERE hostname = ? AND serial_number = ?`,
		hostname, serialNumber)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DueReminder is one drive whose acknowledged counters are due a reminder.
type DueReminder struct {
	Hostname       string
	SerialNumber   string
	AcknowledgedAt time.Time
	Counters       map[int]int64
}

// DueReminders returns the drives whose last reminder (or, if never
// reminded, whose acknowledgement) is at least `every` old.
func DueReminders(db *sql.DB, now time.Time, every time.Duration) ([]DueReminder, error) {
	rows, err := db.Query(`SELECT hostname, serial_number, attribute_id, raw_value,
		acknowledged_at, COALESCE(last_reminded_at, acknowledged_at)
		FROM drive_health_baselines ORDER BY hostname, serial_number`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byDrive := map[string]*DueReminder{}
	var order []string
	for rows.Next() {
		var host, serial, ackAt, since string
		var id int
		var v int64
		if err := rows.Scan(&host, &serial, &id, &v, &ackAt, &since); err != nil {
			continue
		}
		last, ok := parseDBTime(since)
		if !ok || now.Sub(last) < every {
			continue
		}
		key := host + "\x00" + serial
		d := byDrive[key]
		if d == nil {
			acked, _ := parseDBTime(ackAt)
			d = &DueReminder{Hostname: host, SerialNumber: serial, AcknowledgedAt: acked, Counters: map[int]int64{}}
			byDrive[key] = d
			order = append(order, key)
		}
		d.Counters[id] = v
	}
	out := make([]DueReminder, 0, len(order))
	for _, k := range order {
		out = append(out, *byDrive[k])
	}
	return out, nil
}

// RunBaselineReminders publishes one SmartAcknowledgedReminder per drive
// that is due, then records when it was reminded. The timestamp lives in the
// database, so a restart (every image update) never re-sends a reminder.
// everyDays <= 0 disables reminders.
func RunBaselineReminders(db *sql.DB, bus *events.Bus, now time.Time, everyDays int) int {
	if db == nil || bus == nil || everyDays <= 0 {
		return 0
	}
	due, err := DueReminders(db, now, time.Duration(everyDays)*24*time.Hour)
	if err != nil {
		log.Printf("Warning: baseline reminders: %v", err)
		return 0
	}
	for _, d := range due {
		ids := make([]int, 0, len(d.Counters))
		for id := range d.Counters {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		parts := make([]string, 0, len(ids))
		for _, id := range ids {
			parts = append(parts, fmt.Sprintf("#%d=%d", id, d.Counters[id]))
		}
		bus.Publish(events.Event{
			Type:         events.SmartAcknowledgedReminder,
			Severity:     events.SeverityWarning,
			Hostname:     d.Hostname,
			SerialNumber: d.SerialNumber,
			Message: fmt.Sprintf("🔔 Reminder: %s still has acknowledged SMART errors (%s), acknowledged on %s. They are not growing; the drive is still worth replacing.",
				d.SerialNumber, strings.Join(parts, ", "), d.AcknowledgedAt.Format("2006-01-02")),
			Metadata: map[string]string{"acknowledged_at": d.AcknowledgedAt.Format(time.RFC3339)},
		})
		if _, err := db.Exec(`UPDATE drive_health_baselines SET last_reminded_at = ?
			WHERE hostname = ? AND serial_number = ?`,
			now.UTC().Format("2006-01-02 15:04:05"), d.Hostname, d.SerialNumber); err != nil {
			log.Printf("Warning: baseline reminder for %s/%s not recorded: %v", d.Hostname, d.SerialNumber, err)
		}
	}
	return len(due)
}

func parseDBTime(s string) (time.Time, bool) {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
