package smart

import (
	"database/sql"
	"fmt"
	"time"

	agentsmart "vigil/cmd/agent/smart"
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
		// The driver returns DATETIME as RFC 3339; a raw column read from
		// the sqlite3 CLI or an older driver is "2006-01-02 15:04:05".
		for _, layout := range []string{time.RFC3339, "2006-01-02 15:04:05"} {
			if t, err := time.Parse(layout, at); err == nil {
				e.AcknowledgedAt = t
				break
			}
		}
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
