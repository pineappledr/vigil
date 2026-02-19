package db

import (
	"database/sql"
	"fmt"
	"time"

	"vigil/internal/settings"
)

// AlertType constants
const (
	AlertTypeWarning  = "warning"
	AlertTypeCritical = "critical"
	AlertTypeSpike    = "spike"
	AlertTypeRecovery = "recovery"
)

// TemperatureAlert represents a temperature threshold violation
type TemperatureAlert struct {
	ID             int64     `json:"id"`
	Hostname       string    `json:"hostname"`
	SerialNumber   string    `json:"serial_number"`
	DeviceName     string    `json:"device_name,omitempty"`
	Model          string    `json:"model,omitempty"`
	AlertType      string    `json:"alert_type"` // warning, critical, spike, recovery
	Temperature    int       `json:"temperature"`
	Threshold      int       `json:"threshold,omitempty"`
	Message        string    `json:"message"`
	Acknowledged   bool      `json:"acknowledged"`
	AcknowledgedBy string    `json:"acknowledged_by,omitempty"`
	AcknowledgedAt time.Time `json:"acknowledged_at,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

// AlertSummary holds alert statistics
type AlertSummary struct {
	Total          int `json:"total"`
	Unacknowledged int `json:"unacknowledged"`
	Acknowledged   int `json:"acknowledged"`
	Warning        int `json:"warning"`
	Critical       int `json:"critical"`
	Spike          int `json:"spike"`
	Recovery       int `json:"recovery"`
	Last24Hours    int `json:"last_24h"`
	Last7Days      int `json:"last_7d"`
}

// AlertFilter for querying alerts
type AlertFilter struct {
	Hostname     string
	SerialNumber string
	AlertType    string
	Acknowledged *bool
	Since        time.Time
	Limit        int
}

// InitTemperatureAlertsTable creates the temperature_alerts table
func InitTemperatureAlertsTable(db *sql.DB) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS temperature_alerts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hostname TEXT NOT NULL,
		serial_number TEXT NOT NULL,
		alert_type TEXT NOT NULL,
		temperature INTEGER NOT NULL,
		threshold INTEGER,
		message TEXT NOT NULL,
		acknowledged INTEGER DEFAULT 0,
		acknowledged_by TEXT,
		acknowledged_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_temp_alerts_host 
		ON temperature_alerts(hostname);
	CREATE INDEX IF NOT EXISTS idx_temp_alerts_serial 
		ON temperature_alerts(serial_number);
	CREATE INDEX IF NOT EXISTS idx_temp_alerts_type 
		ON temperature_alerts(alert_type);
	CREATE INDEX IF NOT EXISTS idx_temp_alerts_ack 
		ON temperature_alerts(acknowledged);
	CREATE INDEX IF NOT EXISTS idx_temp_alerts_created 
		ON temperature_alerts(created_at);
	CREATE INDEX IF NOT EXISTS idx_temp_alerts_host_serial 
		ON temperature_alerts(hostname, serial_number);
	`

	_, err := db.Exec(createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create temperature_alerts table: %w", err)
	}

	return nil
}

// CreateAlert saves a new temperature alert
func CreateAlert(db *sql.DB, alert *TemperatureAlert) error {
	query := `
		INSERT INTO temperature_alerts (
			hostname, serial_number, alert_type, temperature, 
			threshold, message
		) VALUES (?, ?, ?, ?, ?, ?)
	`

	result, err := db.Exec(query,
		alert.Hostname,
		alert.SerialNumber,
		alert.AlertType,
		alert.Temperature,
		alert.Threshold,
		alert.Message,
	)
	if err != nil {
		return fmt.Errorf("failed to create alert: %w", err)
	}

	id, err := result.LastInsertId()
	if err == nil {
		alert.ID = id
	}

	return nil
}

// GetAlerts retrieves alerts based on filter criteria
func GetAlerts(db *sql.DB, filter AlertFilter) ([]TemperatureAlert, error) {
	query := `
		SELECT id, hostname, serial_number, alert_type, temperature,
			   COALESCE(threshold, 0), message, acknowledged, 
			   COALESCE(acknowledged_by, ''), acknowledged_at, created_at
		FROM temperature_alerts
		WHERE 1=1
	`
	args := []interface{}{}

	if filter.Hostname != "" {
		query += " AND hostname = ?"
		args = append(args, filter.Hostname)
	}

	if filter.SerialNumber != "" {
		query += " AND serial_number = ?"
		args = append(args, filter.SerialNumber)
	}

	if filter.AlertType != "" {
		query += " AND alert_type = ?"
		args = append(args, filter.AlertType)
	}

	if filter.Acknowledged != nil {
		if *filter.Acknowledged {
			query += " AND acknowledged = 1"
		} else {
			query += " AND acknowledged = 0"
		}
	}

	if !filter.Since.IsZero() {
		query += " AND created_at >= ?"
		args = append(args, filter.Since)
	}

	query += " ORDER BY created_at DESC"

	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
	}

	return queryAlerts(db, query, args...)
}

// GetActiveAlerts retrieves all unacknowledged alerts
func GetActiveAlerts(db *sql.DB) ([]TemperatureAlert, error) {
	acked := false
	return GetAlerts(db, AlertFilter{Acknowledged: &acked, Limit: 100})
}

// GetAlertByID retrieves a single alert
func GetAlertByID(db *sql.DB, id int64) (*TemperatureAlert, error) {
	query := `
		SELECT id, hostname, serial_number, alert_type, temperature,
			   COALESCE(threshold, 0), message, acknowledged, 
			   COALESCE(acknowledged_by, ''), acknowledged_at, created_at
		FROM temperature_alerts
		WHERE id = ?
	`

	alerts, err := queryAlerts(db, query, id)
	if err != nil {
		return nil, err
	}

	if len(alerts) == 0 {
		return nil, nil
	}

	return &alerts[0], nil
}

// AcknowledgeAlert marks an alert as acknowledged
func AcknowledgeAlert(db *sql.DB, id int64, username string) error {
	query := `
		UPDATE temperature_alerts
		SET acknowledged = 1, acknowledged_by = ?, acknowledged_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	result, err := db.Exec(query, username, id)
	if err != nil {
		return fmt.Errorf("failed to acknowledge alert: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("alert not found")
	}

	return nil
}

// AcknowledgeAllAlerts marks all unacknowledged alerts as acknowledged
func AcknowledgeAllAlerts(db *sql.DB, username string) (int64, error) {
	query := `
		UPDATE temperature_alerts
		SET acknowledged = 1, acknowledged_by = ?, acknowledged_at = CURRENT_TIMESTAMP
		WHERE acknowledged = 0
	`

	result, err := db.Exec(query, username)
	if err != nil {
		return 0, fmt.Errorf("failed to acknowledge alerts: %w", err)
	}

	return result.RowsAffected()
}

// DeleteAlert removes an alert
func DeleteAlert(db *sql.DB, id int64) error {
	result, err := db.Exec("DELETE FROM temperature_alerts WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("failed to delete alert: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("alert not found")
	}

	return nil
}

// GetAlertSummary returns alert statistics
func GetAlertSummary(db *sql.DB) (*AlertSummary, error) {
	query := `
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN acknowledged = 0 THEN 1 ELSE 0 END) as unacknowledged,
			SUM(CASE WHEN acknowledged = 1 THEN 1 ELSE 0 END) as acknowledged,
			SUM(CASE WHEN alert_type = 'warning' THEN 1 ELSE 0 END) as warning,
			SUM(CASE WHEN alert_type = 'critical' THEN 1 ELSE 0 END) as critical,
			SUM(CASE WHEN alert_type = 'spike' THEN 1 ELSE 0 END) as spike,
			SUM(CASE WHEN alert_type = 'recovery' THEN 1 ELSE 0 END) as recovery,
			SUM(CASE WHEN created_at >= datetime('now', '-24 hours') THEN 1 ELSE 0 END) as last_24h,
			SUM(CASE WHEN created_at >= datetime('now', '-7 days') THEN 1 ELSE 0 END) as last_7d
		FROM temperature_alerts
	`

	var summary AlertSummary
	err := db.QueryRow(query).Scan(
		&summary.Total,
		&summary.Unacknowledged,
		&summary.Acknowledged,
		&summary.Warning,
		&summary.Critical,
		&summary.Spike,
		&summary.Recovery,
		&summary.Last24Hours,
		&summary.Last7Days,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get alert summary: %w", err)
	}

	return &summary, nil
}

// GetAlertsByDrive retrieves alerts for a specific drive
func GetAlertsByDrive(db *sql.DB, hostname, serial string, limit int) ([]TemperatureAlert, error) {
	return GetAlerts(db, AlertFilter{
		Hostname:     hostname,
		SerialNumber: serial,
		Limit:        limit,
	})
}

// CleanupOldAlerts removes alerts older than retention period
func CleanupOldAlerts(db *sql.DB, retentionDays int) (int64, error) {
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	result, err := db.Exec(`
		DELETE FROM temperature_alerts
		WHERE created_at < ?
	`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("failed to cleanup old alerts: %w", err)
	}

	return result.RowsAffected()
}

// Helper function to query alerts
func queryAlerts(db *sql.DB, query string, args ...interface{}) ([]TemperatureAlert, error) {
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query alerts: %w", err)
	}
	defer rows.Close()

	var alerts []TemperatureAlert
	for rows.Next() {
		var alert TemperatureAlert
		var ackAt sql.NullTime

		err := rows.Scan(
			&alert.ID, &alert.Hostname, &alert.SerialNumber,
			&alert.AlertType, &alert.Temperature, &alert.Threshold,
			&alert.Message, &alert.Acknowledged, &alert.AcknowledgedBy,
			&ackAt, &alert.CreatedAt,
		)
		if err != nil {
			continue
		}

		if ackAt.Valid {
			alert.AcknowledgedAt = ackAt.Time
		}

		// Get drive info
		driveInfo, _ := getDriveInfo(db, alert.Hostname, alert.SerialNumber)
		if driveInfo != nil {
			alert.DeviceName = driveInfo.DeviceName
			alert.Model = driveInfo.Model
		}

		alerts = append(alerts, alert)
	}

	return alerts, rows.Err()
}

// ============================================
// ALERT GENERATION LOGIC
// ============================================

// AlertState tracks the current alert state for a drive
type AlertState struct {
	DriveKey      string    // "hostname:serial"
	LastAlertType string    // Last alert type generated
	LastAlertTime time.Time // When last alert was generated
	InAlertState  bool      // Currently in alert state
}

// alertStateCache stores recent alert states to prevent duplicates
var alertStateCache = make(map[string]*AlertState)

// CheckTemperatureAndAlert checks temperature against thresholds and generates alerts
func CheckTemperatureAndAlert(db *sql.DB, hostname, serial string, temperature int) (*TemperatureAlert, error) {
	// Get thresholds from settings
	warningThreshold := settings.GetIntSettingWithDefault(db, "temperature", "warning_threshold", 45)
	criticalThreshold := settings.GetIntSettingWithDefault(db, "temperature", "critical_threshold", 55)
	cooldownMinutes := settings.GetIntSettingWithDefault(db, "alerts", "cooldown_minutes", 60)
	alertsEnabled := settings.GetBoolSettingWithDefault(db, "alerts", "enabled", true)
	recoveryEnabled := settings.GetBoolSettingWithDefault(db, "alerts", "recovery_enabled", true)

	if !alertsEnabled {
		return nil, nil
	}

	driveKey := fmt.Sprintf("%s:%s", hostname, serial)
	state := alertStateCache[driveKey]
	if state == nil {
		state = &AlertState{DriveKey: driveKey}
		alertStateCache[driveKey] = state
	}

	now := time.Now()
	cooldown := time.Duration(cooldownMinutes) * time.Minute

	// Determine current status
	var alertType string
	var threshold int
	var message string

	if temperature >= criticalThreshold {
		alertType = AlertTypeCritical
		threshold = criticalThreshold
		message = fmt.Sprintf("Temperature %d°C exceeds critical threshold (%d°C)", temperature, criticalThreshold)
	} else if temperature >= warningThreshold {
		alertType = AlertTypeWarning
		threshold = warningThreshold
		message = fmt.Sprintf("Temperature %d°C exceeds warning threshold (%d°C)", temperature, warningThreshold)
	} else if state.InAlertState && recoveryEnabled {
		// Temperature returned to normal - generate recovery alert
		alertType = AlertTypeRecovery
		threshold = warningThreshold
		message = fmt.Sprintf("Temperature recovered to %d°C (below warning threshold %d°C)", temperature, warningThreshold)
		state.InAlertState = false
	} else {
		// Normal temperature, no alert needed
		state.InAlertState = false
		return nil, nil
	}

	// Check cooldown for non-recovery alerts
	if alertType != AlertTypeRecovery {
		// Check if we should generate a new alert
		if state.LastAlertType == alertType && now.Sub(state.LastAlertTime) < cooldown {
			// Within cooldown period for same alert type
			return nil, nil
		}
		state.InAlertState = true
	}

	// Create alert
	alert := &TemperatureAlert{
		Hostname:     hostname,
		SerialNumber: serial,
		AlertType:    alertType,
		Temperature:  temperature,
		Threshold:    threshold,
		Message:      message,
	}

	if err := CreateAlert(db, alert); err != nil {
		return nil, err
	}

	// Update state
	state.LastAlertType = alertType
	state.LastAlertTime = now

	return alert, nil
}

// CreateSpikeAlert creates an alert for a temperature spike
func CreateSpikeAlert(db *sql.DB, spike *TemperatureSpike) (*TemperatureAlert, error) {
	alertsEnabled := settings.GetBoolSettingWithDefault(db, "alerts", "enabled", true)
	if !alertsEnabled {
		return nil, nil
	}

	message := fmt.Sprintf("Temperature spike detected: %d°C → %d°C (%+d°C) in %s",
		spike.StartTemp, spike.EndTemp, spike.Change,
		spike.EndTime.Sub(spike.StartTime).Round(time.Minute).String())

	alert := &TemperatureAlert{
		Hostname:     spike.Hostname,
		SerialNumber: spike.SerialNumber,
		AlertType:    AlertTypeSpike,
		Temperature:  spike.EndTemp,
		Message:      message,
	}

	if err := CreateAlert(db, alert); err != nil {
		return nil, err
	}

	return alert, nil
}

// ProcessTemperatureReading processes a temperature reading and generates appropriate alerts
func ProcessTemperatureReading(db *sql.DB, hostname, serial string, temperature int) ([]TemperatureAlert, error) {
	var alerts []TemperatureAlert

	// Check threshold alerts
	alert, err := CheckTemperatureAndAlert(db, hostname, serial, temperature)
	if err != nil {
		return nil, err
	}
	if alert != nil {
		alerts = append(alerts, *alert)
	}

	// Check for spikes
	spikes, err := DetectAndRecordSpikes(db, hostname, serial)
	if err != nil {
		return alerts, err
	}

	// Create alerts for new spikes
	for i := range spikes {
		spikeAlert, err := CreateSpikeAlert(db, &spikes[i])
		if err != nil {
			continue
		}
		if spikeAlert != nil {
			alerts = append(alerts, *spikeAlert)
		}
	}

	return alerts, nil
}

// GetDriveAlertStatus returns the current alert status for a drive
func GetDriveAlertStatus(db *sql.DB, hostname, serial string) (string, error) {
	// Get latest unacknowledged alert for this drive
	query := `
		SELECT alert_type
		FROM temperature_alerts
		WHERE hostname = ? AND serial_number = ? AND acknowledged = 0
		ORDER BY 
			CASE alert_type 
				WHEN 'critical' THEN 1 
				WHEN 'warning' THEN 2 
				WHEN 'spike' THEN 3 
				ELSE 4 
			END,
			created_at DESC
		LIMIT 1
	`

	var alertType string
	err := db.QueryRow(query, hostname, serial).Scan(&alertType)
	if err == sql.ErrNoRows {
		return "normal", nil
	}
	if err != nil {
		return "", err
	}

	return alertType, nil
}

// ClearAlertStateCache clears the in-memory alert state cache
// Useful for testing or after restart
func ClearAlertStateCache() {
	alertStateCache = make(map[string]*AlertState)
}
