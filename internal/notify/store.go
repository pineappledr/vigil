package notify

import (
	"database/sql"
	"fmt"
	"time"
)

const timeFormat = "2006-01-02 15:04:05"

// ── NotificationService CRUD ────────────────────────────────────────────

// CreateService inserts a new notification service destination.
func CreateService(db *sql.DB, svc *NotificationService) (int64, error) {
	res, err := db.Exec(`
		INSERT INTO notification_settings
			(name, service_type, config_json, enabled,
			 notify_on_critical, notify_on_warning, notify_on_healthy)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		svc.Name, svc.ServiceType, svc.ConfigJSON,
		boolInt(svc.Enabled),
		boolInt(svc.NotifyOnCritical),
		boolInt(svc.NotifyOnWarning),
		boolInt(svc.NotifyOnHealthy))
	if err != nil {
		return 0, fmt.Errorf("create notification service: %w", err)
	}
	return res.LastInsertId()
}

// GetService retrieves a notification service by ID.
func GetService(db *sql.DB, id int64) (*NotificationService, error) {
	row := db.QueryRow(`
		SELECT id, name, service_type, config_json, enabled,
		       notify_on_critical, notify_on_warning, notify_on_healthy,
		       created_at, updated_at
		FROM notification_settings WHERE id = ?`, id)
	return scanService(row)
}

// ListServices returns all notification services.
func ListServices(db *sql.DB) ([]NotificationService, error) {
	rows, err := db.Query(`
		SELECT id, name, service_type, config_json, enabled,
		       notify_on_critical, notify_on_warning, notify_on_healthy,
		       created_at, updated_at
		FROM notification_settings ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list notification services: %w", err)
	}
	defer rows.Close()

	var out []NotificationService
	for rows.Next() {
		svc, err := scanServiceRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, svc)
	}
	return out, rows.Err()
}

// ListEnabledServices returns only enabled notification services.
func ListEnabledServices(db *sql.DB) ([]NotificationService, error) {
	rows, err := db.Query(`
		SELECT id, name, service_type, config_json, enabled,
		       notify_on_critical, notify_on_warning, notify_on_healthy,
		       created_at, updated_at
		FROM notification_settings WHERE enabled = 1 ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list enabled notification services: %w", err)
	}
	defer rows.Close()

	var out []NotificationService
	for rows.Next() {
		svc, err := scanServiceRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, svc)
	}
	return out, rows.Err()
}

// UpdateService updates a notification service's configuration.
func UpdateService(db *sql.DB, svc *NotificationService) error {
	res, err := db.Exec(`
		UPDATE notification_settings SET
			name = ?, service_type = ?, config_json = ?, enabled = ?,
			notify_on_critical = ?, notify_on_warning = ?, notify_on_healthy = ?,
			updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		svc.Name, svc.ServiceType, svc.ConfigJSON,
		boolInt(svc.Enabled),
		boolInt(svc.NotifyOnCritical),
		boolInt(svc.NotifyOnWarning),
		boolInt(svc.NotifyOnHealthy),
		svc.ID)
	if err != nil {
		return fmt.Errorf("update notification service: %w", err)
	}
	return expectOneRow(res, "update notification service")
}

// DeleteService removes a notification service and its related rules
// (cascaded by foreign keys).
func DeleteService(db *sql.DB, id int64) error {
	res, err := db.Exec(`DELETE FROM notification_settings WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete notification service: %w", err)
	}
	return expectOneRow(res, "delete notification service")
}

// ── EventRule CRUD ──────────────────────────────────────────────────────

// UpsertEventRule creates or updates a per-event-type rule for a service.
func UpsertEventRule(db *sql.DB, rule *EventRule) error {
	_, err := db.Exec(`
		INSERT INTO notification_event_rules (service_id, event_type, enabled, cooldown_secs)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(service_id, event_type) DO UPDATE SET
			enabled       = excluded.enabled,
			cooldown_secs = excluded.cooldown_secs`,
		rule.ServiceID, rule.EventType, boolInt(rule.Enabled), rule.Cooldown)
	if err != nil {
		return fmt.Errorf("upsert event rule: %w", err)
	}
	return nil
}

// GetEventRules returns all event rules for a service.
func GetEventRules(db *sql.DB, serviceID int64) ([]EventRule, error) {
	rows, err := db.Query(`
		SELECT id, service_id, event_type, enabled, cooldown_secs
		FROM notification_event_rules WHERE service_id = ?
		ORDER BY event_type`, serviceID)
	if err != nil {
		return nil, fmt.Errorf("get event rules: %w", err)
	}
	defer rows.Close()

	var out []EventRule
	for rows.Next() {
		var r EventRule
		var enabled int
		if err := rows.Scan(&r.ID, &r.ServiceID, &r.EventType, &enabled, &r.Cooldown); err != nil {
			return nil, fmt.Errorf("scan event rule: %w", err)
		}
		r.Enabled = enabled == 1
		out = append(out, r)
	}
	return out, rows.Err()
}

// DeleteEventRule removes a specific event rule.
func DeleteEventRule(db *sql.DB, id int64) error {
	_, err := db.Exec(`DELETE FROM notification_event_rules WHERE id = ?`, id)
	return err
}

// ── QuietHours CRUD ─────────────────────────────────────────────────────

// UpsertQuietHours sets the quiet hours for a service.
func UpsertQuietHours(db *sql.DB, qh *QuietHours) error {
	_, err := db.Exec(`
		INSERT INTO notification_quiet_hours (service_id, start_time, end_time, enabled)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(service_id) DO UPDATE SET
			start_time = excluded.start_time,
			end_time   = excluded.end_time,
			enabled    = excluded.enabled`,
		qh.ServiceID, qh.StartTime, qh.EndTime, boolInt(qh.Enabled))
	if err != nil {
		return fmt.Errorf("upsert quiet hours: %w", err)
	}
	return nil
}

// GetQuietHours returns the quiet hours for a service, or nil if unset.
func GetQuietHours(db *sql.DB, serviceID int64) (*QuietHours, error) {
	var qh QuietHours
	var enabled int
	err := db.QueryRow(`
		SELECT id, service_id, start_time, end_time, enabled
		FROM notification_quiet_hours WHERE service_id = ?`, serviceID).
		Scan(&qh.ID, &qh.ServiceID, &qh.StartTime, &qh.EndTime, &enabled)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get quiet hours: %w", err)
	}
	qh.Enabled = enabled == 1
	return &qh, nil
}

// ── DigestConfig CRUD ───────────────────────────────────────────────────

// UpsertDigestConfig sets the digest configuration for a service.
func UpsertDigestConfig(db *sql.DB, dc *DigestConfig) error {
	_, err := db.Exec(`
		INSERT INTO notification_digest_config (service_id, enabled, send_at)
		VALUES (?, ?, ?)
		ON CONFLICT(service_id) DO UPDATE SET
			enabled = excluded.enabled,
			send_at = excluded.send_at`,
		dc.ServiceID, boolInt(dc.Enabled), dc.SendAt)
	if err != nil {
		return fmt.Errorf("upsert digest config: %w", err)
	}
	return nil
}

// GetDigestConfig returns the digest config for a service, or nil if unset.
func GetDigestConfig(db *sql.DB, serviceID int64) (*DigestConfig, error) {
	var dc DigestConfig
	var enabled int
	err := db.QueryRow(`
		SELECT id, service_id, enabled, send_at
		FROM notification_digest_config WHERE service_id = ?`, serviceID).
		Scan(&dc.ID, &dc.ServiceID, &enabled, &dc.SendAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get digest config: %w", err)
	}
	dc.Enabled = enabled == 1
	return &dc, nil
}

// ── NotificationHistory ─────────────────────────────────────────────────

// RecordNotification inserts a row into notification_history.
func RecordNotification(db *sql.DB, rec *NotificationRecord) (int64, error) {
	var sentAt interface{}
	if !rec.SentAt.IsZero() {
		sentAt = rec.SentAt.UTC().Format(timeFormat)
	}

	res, err := db.Exec(`
		INSERT INTO notification_history
			(setting_id, event_type, hostname, serial_number, message, status, error_message, sent_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		rec.SettingID, rec.EventType, rec.Hostname, rec.SerialNumber,
		rec.Message, rec.Status, rec.ErrorMessage, sentAt)
	if err != nil {
		return 0, fmt.Errorf("record notification: %w", err)
	}
	return res.LastInsertId()
}

// RecentHistory returns the latest N notification records.
func RecentHistory(db *sql.DB, limit int) ([]NotificationRecord, error) {
	rows, err := db.Query(`
		SELECT id, COALESCE(setting_id,0), event_type,
		       COALESCE(hostname,''), COALESCE(serial_number,''),
		       message, status, COALESCE(error_message,''),
		       COALESCE(sent_at,''), created_at
		FROM notification_history
		ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("recent history: %w", err)
	}
	defer rows.Close()

	var out []NotificationRecord
	for rows.Next() {
		var r NotificationRecord
		var sentAt, createdAt string
		if err := rows.Scan(&r.ID, &r.SettingID, &r.EventType,
			&r.Hostname, &r.SerialNumber, &r.Message, &r.Status,
			&r.ErrorMessage, &sentAt, &createdAt); err != nil {
			return nil, fmt.Errorf("scan history: %w", err)
		}
		r.SentAt = parseTime(sentAt)
		r.CreatedAt = parseTime(createdAt)
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── helpers ──────────────────────────────────────────────────────────────

func scanService(row *sql.Row) (*NotificationService, error) {
	var svc NotificationService
	var enabled, critical, warning, healthy int
	var createdAt, updatedAt string

	err := row.Scan(&svc.ID, &svc.Name, &svc.ServiceType, &svc.ConfigJSON,
		&enabled, &critical, &warning, &healthy, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan notification service: %w", err)
	}
	svc.Enabled = enabled == 1
	svc.NotifyOnCritical = critical == 1
	svc.NotifyOnWarning = warning == 1
	svc.NotifyOnHealthy = healthy == 1
	svc.CreatedAt = parseTime(createdAt)
	svc.UpdatedAt = parseTime(updatedAt)
	return &svc, nil
}

type scannable interface {
	Scan(dest ...interface{}) error
}

func scanServiceRow(s scannable) (NotificationService, error) {
	var svc NotificationService
	var enabled, critical, warning, healthy int
	var createdAt, updatedAt string

	err := s.Scan(&svc.ID, &svc.Name, &svc.ServiceType, &svc.ConfigJSON,
		&enabled, &critical, &warning, &healthy, &createdAt, &updatedAt)
	if err != nil {
		return svc, fmt.Errorf("scan notification service row: %w", err)
	}
	svc.Enabled = enabled == 1
	svc.NotifyOnCritical = critical == 1
	svc.NotifyOnWarning = warning == 1
	svc.NotifyOnHealthy = healthy == 1
	svc.CreatedAt = parseTime(createdAt)
	svc.UpdatedAt = parseTime(updatedAt)
	return svc, nil
}

func parseTime(s string) time.Time {
	t, _ := time.Parse(timeFormat, s)
	return t
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func expectOneRow(res sql.Result, op string) error {
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s: rows affected: %w", op, err)
	}
	if n == 0 {
		return fmt.Errorf("%s: not found", op)
	}
	return nil
}
