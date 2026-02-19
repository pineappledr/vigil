package temperature

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"vigil/internal/settings"
)

// ============================================
// TEMPERATURE HANDLER
// ============================================

// TemperatureHandler handles temperature-related API requests
type TemperatureHandler struct {
	DB *sql.DB
}

// NewTemperatureHandler creates a new temperature handler
func NewTemperatureHandler(database *sql.DB) *TemperatureHandler {
	return &TemperatureHandler{DB: database}
}

// GetTemperatureStats handles GET /api/temperature/stats
// Query params: hostname, serial, period (24h, 7d, 30d, all)
func (h *TemperatureHandler) GetTemperatureStats(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serial := r.URL.Query().Get("serial")
	periodStr := r.URL.Query().Get("period")

	if hostname == "" || serial == "" {
		http.Error(w, "hostname and serial are required", http.StatusBadRequest)
		return
	}

	period := ParsePeriod(periodStr)

	stats, err := GetTemperatureStats(h.DB, hostname, serial, period)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if stats == nil {
		http.Error(w, "no temperature data found", http.StatusNotFound)
		return
	}

	jsonResponse(w, stats)
}

// GetAllTemperatureStats handles GET /api/temperature/stats/all
// Query params: period (24h, 7d, 30d, all)
func (h *TemperatureHandler) GetAllTemperatureStats(w http.ResponseWriter, r *http.Request) {
	periodStr := r.URL.Query().Get("period")
	period := ParsePeriod(periodStr)

	stats, err := GetAllDrivesTemperatureStats(h.DB, period)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"period": string(period),
		"drives": stats,
		"count":  len(stats),
	})
}

// GetTemperatureTimeSeries handles GET /api/temperature/timeseries
// Query params: hostname, serial, period (24h, 7d, 30d, all), interval (1h, 6h, 1d)
func (h *TemperatureHandler) GetTemperatureTimeSeries(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serial := r.URL.Query().Get("serial")
	periodStr := r.URL.Query().Get("period")
	intervalStr := r.URL.Query().Get("interval")

	if hostname == "" || serial == "" {
		http.Error(w, "hostname and serial are required", http.StatusBadRequest)
		return
	}

	period := ParsePeriod(periodStr)
	interval := ParseInterval(intervalStr)

	// Auto-select interval if not specified
	if intervalStr == "" {
		interval = autoSelectInterval(period)
	}

	data, err := GetTemperatureTimeSeries(h.DB, hostname, serial, period, interval)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if data == nil || len(data.Points) == 0 {
		http.Error(w, "no temperature data found", http.StatusNotFound)
		return
	}

	jsonResponse(w, data)
}

// GetCurrentTemperatures handles GET /api/temperature/current
// Query params: hostname, serial (both optional - if not provided, returns all)
func (h *TemperatureHandler) GetCurrentTemperatures(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serial := r.URL.Query().Get("serial")

	// If specific drive requested
	if hostname != "" && serial != "" {
		current, err := GetCurrentTemperature(h.DB, hostname, serial)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if current == nil {
			http.Error(w, "no temperature data found", http.StatusNotFound)
			return
		}
		jsonResponse(w, current)
		return
	}

	// Return all drives
	temps, err := GetAllCurrentTemperatures(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"drives": temps,
		"count":  len(temps),
	})
}

// GetTemperatureSummary handles GET /api/temperature/summary
func (h *TemperatureHandler) GetTemperatureSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := GetTemperatureSummary(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, summary)
}

// GetTemperatureHeatmap handles GET /api/temperature/heatmap
// Query params: period (24h, 7d, 30d), interval (1h, 6h, 1d)
func (h *TemperatureHandler) GetTemperatureHeatmap(w http.ResponseWriter, r *http.Request) {
	periodStr := r.URL.Query().Get("period")
	intervalStr := r.URL.Query().Get("interval")

	period := ParsePeriod(periodStr)
	interval := ParseInterval(intervalStr)

	// Auto-select interval if not specified
	if intervalStr == "" {
		interval = autoSelectInterval(period)
	}

	data, err := GetHeatmapData(h.DB, period, interval)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, data)
}

// GetTemperatureRange handles GET /api/temperature/range
// Query params: hostname, serial, from, to (ISO timestamps)
func (h *TemperatureHandler) GetTemperatureRange(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serial := r.URL.Query().Get("serial")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if hostname == "" || serial == "" {
		http.Error(w, "hostname and serial are required", http.StatusBadRequest)
		return
	}

	// Parse time range
	var from, to time.Time
	var err error

	if fromStr != "" {
		from, err = time.Parse(time.RFC3339, fromStr)
		if err != nil {
			http.Error(w, "invalid 'from' timestamp format (use RFC3339)", http.StatusBadRequest)
			return
		}
	} else {
		from = time.Now().Add(-24 * time.Hour) // Default: last 24 hours
	}

	if toStr != "" {
		to, err = time.Parse(time.RFC3339, toStr)
		if err != nil {
			http.Error(w, "invalid 'to' timestamp format (use RFC3339)", http.StatusBadRequest)
			return
		}
	} else {
		to = time.Now()
	}

	records, err := GetTemperatureRange(h.DB, hostname, serial, from, to)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"hostname":      hostname,
		"serial_number": serial,
		"from":          from,
		"to":            to,
		"records":       records,
		"count":         len(records),
	})
}

// GetDashboardTemperature handles GET /api/dashboard/temperature
// Returns a summary suitable for dashboard display
func (h *TemperatureHandler) GetDashboardTemperature(w http.ResponseWriter, r *http.Request) {
	// Get summary
	summary, err := GetTemperatureSummary(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Enhance with additional dashboard-specific data
	response := map[string]interface{}{
		"total_drives":    summary.TotalDrives,
		"drives_normal":   summary.DrivesNormal,
		"drives_warning":  summary.DrivesWarning,
		"drives_critical": summary.DrivesCritical,
		"avg_temperature": summary.AvgTemperature,
		"min_temperature": summary.MinTemperature,
		"max_temperature": summary.MaxTemperature,
		"hottest_drive":   summary.HottestDrive,
		"coolest_drive":   summary.CoolestDrive,
	}

	// Get thresholds for frontend display
	warningThreshold := settings.GetIntSettingWithDefault(h.DB, "temperature", "warning_threshold", 45)
	criticalThreshold := settings.GetIntSettingWithDefault(h.DB, "temperature", "critical_threshold", 55)

	response["thresholds"] = map[string]int{
		"warning":  warningThreshold,
		"critical": criticalThreshold,
	}

	// Add drives grouped by status
	if len(summary.Drives) > 0 {
		byStatus := map[string][]CurrentTemperature{
			"normal":   {},
			"warning":  {},
			"critical": {},
		}
		for _, d := range summary.Drives {
			byStatus[d.Status] = append(byStatus[d.Status], d)
		}
		response["drives_by_status"] = byStatus
	}

	jsonResponse(w, response)
}

// autoSelectInterval chooses an appropriate interval based on period
func autoSelectInterval(period TemperaturePeriod) AggregationInterval {
	switch period {
	case Period24Hours:
		return IntervalHourly
	case Period7Days:
		return Interval6Hours
	case Period30Days:
		return IntervalDaily
	case PeriodAllTime:
		return IntervalDaily
	default:
		return IntervalHourly
	}
}

// ============================================
// ALERT HANDLER
// ============================================

// AlertHandler handles temperature alert-related API requests
type AlertHandler struct {
	DB *sql.DB
}

// NewAlertHandler creates a new alert handler
func NewAlertHandler(database *sql.DB) *AlertHandler {
	return &AlertHandler{DB: database}
}

// GetAlerts handles GET /api/alerts/temperature
// Query params: hostname, serial, type, acknowledged, since, limit
func (h *AlertHandler) GetAlerts(w http.ResponseWriter, r *http.Request) {
	filter := AlertFilter{
		Hostname:     r.URL.Query().Get("hostname"),
		SerialNumber: r.URL.Query().Get("serial"),
		AlertType:    r.URL.Query().Get("type"),
		Limit:        50,
	}

	// Parse acknowledged filter
	if ack := r.URL.Query().Get("acknowledged"); ack != "" {
		acknowledged := ack == "true"
		filter.Acknowledged = &acknowledged
	}

	// Parse since filter
	if since := r.URL.Query().Get("since"); since != "" {
		if t, err := time.Parse(time.RFC3339, since); err == nil {
			filter.Since = t
		}
	}

	// Parse limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			filter.Limit = l
		}
	}

	alerts, err := GetAlerts(h.DB, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

// GetActiveAlerts handles GET /api/alerts/temperature/active
func (h *AlertHandler) GetActiveAlerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := GetActiveAlerts(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

// GetAlert handles GET /api/alerts/temperature/{id}
func (h *AlertHandler) GetAlert(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid alert ID", http.StatusBadRequest)
		return
	}

	alert, err := GetAlertByID(h.DB, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if alert == nil {
		http.Error(w, "alert not found", http.StatusNotFound)
		return
	}

	jsonResponse(w, alert)
}

// GetAlertSummary handles GET /api/alerts/temperature/summary
func (h *AlertHandler) GetAlertSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := GetAlertSummary(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, summary)
}

// AcknowledgeAlert handles POST /api/alerts/temperature/{id}/acknowledge
func (h *AlertHandler) AcknowledgeAlert(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid alert ID", http.StatusBadRequest)
		return
	}

	username := getUsernameFromRequest(r)

	if err := AcknowledgeAlert(h.DB, id, username); err != nil {
		if err.Error() == "alert not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the updated alert
	alert, _ := GetAlertByID(h.DB, id)
	jsonResponse(w, map[string]interface{}{
		"message": "alert acknowledged",
		"alert":   alert,
	})
}

// AcknowledgeAllAlerts handles POST /api/alerts/temperature/acknowledge-all
func (h *AlertHandler) AcknowledgeAllAlerts(w http.ResponseWriter, r *http.Request) {
	username := getUsernameFromRequest(r)

	count, err := AcknowledgeAllAlerts(h.DB, username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"message":      "alerts acknowledged",
		"acknowledged": count,
	})
}

// DeleteAlert handles DELETE /api/alerts/temperature/{id}
func (h *AlertHandler) DeleteAlert(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid alert ID", http.StatusBadRequest)
		return
	}

	if err := DeleteAlert(h.DB, id); err != nil {
		if err.Error() == "alert not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"message": "alert deleted",
		"id":      id,
	})
}

// GetAlertsByDrive handles GET /api/alerts/temperature/drive/{hostname}/{serial}
func (h *AlertHandler) GetAlertsByDrive(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	serial := r.PathValue("serial")

	if hostname == "" || serial == "" {
		http.Error(w, "hostname and serial are required", http.StatusBadRequest)
		return
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	alerts, err := GetAlertsByDrive(h.DB, hostname, serial, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get current alert status
	status, _ := GetDriveAlertStatus(h.DB, hostname, serial)

	jsonResponse(w, map[string]interface{}{
		"hostname":      hostname,
		"serial_number": serial,
		"status":        status,
		"alerts":        alerts,
		"count":         len(alerts),
	})
}

// TestAlert handles POST /api/alerts/temperature/test
// Creates a test alert for debugging/testing purposes
func (h *AlertHandler) TestAlert(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hostname     string `json:"hostname"`
		SerialNumber string `json:"serial_number"`
		Temperature  int    `json:"temperature"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Hostname == "" || req.SerialNumber == "" {
		http.Error(w, "hostname and serial_number are required", http.StatusBadRequest)
		return
	}

	if req.Temperature == 0 {
		req.Temperature = 60 // Default to a critical temperature for testing
	}

	// Process the temperature reading
	alerts, err := ProcessTemperatureReading(h.DB, req.Hostname, req.SerialNumber, req.Temperature)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"message": "test alert processed",
		"alerts":  alerts,
		"count":   len(alerts),
	})
}

// CleanupAlerts handles POST /api/alerts/temperature/cleanup
// Removes old alerts based on retention settings
func (h *AlertHandler) CleanupAlerts(w http.ResponseWriter, r *http.Request) {
	retentionDays := settings.GetIntSettingWithDefault(h.DB, "system", "data_retention_days", 365)

	deleted, err := CleanupOldAlerts(h.DB, retentionDays)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"message":        "cleanup completed",
		"deleted":        deleted,
		"retention_days": retentionDays,
	})
}

// ============================================
// SPIKE HANDLER
// ============================================

// SpikeHandler handles temperature spike-related API requests
type SpikeHandler struct {
	DB *sql.DB
}

// NewSpikeHandler creates a new spike handler
func NewSpikeHandler(database *sql.DB) *SpikeHandler {
	return &SpikeHandler{DB: database}
}

// GetSpikes handles GET /api/temperature/spikes
// Query params: hostname, serial (optional - filter by drive), limit (default 50)
func (h *SpikeHandler) GetSpikes(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serial := r.URL.Query().Get("serial")
	limitStr := r.URL.Query().Get("limit")

	limit := 50
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 200 {
			limit = l
		}
	}

	var spikes []TemperatureSpike
	var err error

	if hostname != "" && serial != "" {
		spikes, err = GetRecentSpikes(h.DB, hostname, serial, limit)
	} else {
		spikes, err = GetAllRecentSpikes(h.DB, limit)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"spikes": spikes,
		"count":  len(spikes),
	})
}

// GetUnacknowledgedSpikes handles GET /api/temperature/spikes/unacknowledged
func (h *SpikeHandler) GetUnacknowledgedSpikes(w http.ResponseWriter, r *http.Request) {
	spikes, err := GetUnacknowledgedSpikes(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"spikes": spikes,
		"count":  len(spikes),
	})
}

// GetSpike handles GET /api/temperature/spikes/{id}
func (h *SpikeHandler) GetSpike(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid spike ID", http.StatusBadRequest)
		return
	}

	spike, err := GetSpikeByID(h.DB, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if spike == nil {
		http.Error(w, "spike not found", http.StatusNotFound)
		return
	}

	jsonResponse(w, spike)
}

// AcknowledgeSpike handles POST /api/temperature/spikes/{id}/acknowledge
func (h *SpikeHandler) AcknowledgeSpike(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid spike ID", http.StatusBadRequest)
		return
	}

	// Get username from context or request
	username := getUsernameFromRequest(r)

	if err := AcknowledgeSpike(h.DB, id, username); err != nil {
		if err.Error() == "spike not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the updated spike
	spike, _ := GetSpikeByID(h.DB, id)
	jsonResponse(w, map[string]interface{}{
		"message": "spike acknowledged",
		"spike":   spike,
	})
}

// DeleteSpike handles DELETE /api/temperature/spikes/{id}
func (h *SpikeHandler) DeleteSpike(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "invalid spike ID", http.StatusBadRequest)
		return
	}

	if err := DeleteSpike(h.DB, id); err != nil {
		if err.Error() == "spike not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"message": "spike deleted",
		"id":      id,
	})
}

// GetSpikeSummary handles GET /api/temperature/spikes/summary
func (h *SpikeHandler) GetSpikeSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := GetSpikeSummary(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, summary)
}

// DetectSpikes handles POST /api/temperature/spikes/detect
// Manually trigger spike detection for all drives or a specific drive
func (h *SpikeHandler) DetectSpikes(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serial := r.URL.Query().Get("serial")

	var spikes []TemperatureSpike
	var err error

	if hostname != "" && serial != "" {
		// Detect for specific drive
		spikes, err = DetectAndRecordSpikes(h.DB, hostname, serial)
	} else {
		// Detect for all drives
		spikes, err = DetectAllDrivesSpikes(h.DB)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"message":    "spike detection completed",
		"new_spikes": spikes,
		"count":      len(spikes),
	})
}

// AcknowledgeAllSpikes handles POST /api/temperature/spikes/acknowledge-all
func (h *SpikeHandler) AcknowledgeAllSpikes(w http.ResponseWriter, r *http.Request) {
	username := getUsernameFromRequest(r)

	// Get all unacknowledged spikes
	spikes, err := GetUnacknowledgedSpikes(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Acknowledge each
	acknowledged := 0
	for _, spike := range spikes {
		if err := AcknowledgeSpike(h.DB, spike.ID, username); err == nil {
			acknowledged++
		}
	}

	jsonResponse(w, map[string]interface{}{
		"message":      "spikes acknowledged",
		"acknowledged": acknowledged,
	})
}

// ============================================
// DASHBOARD HANDLER
// ============================================

// DashboardHandler handles dashboard-related API requests
type DashboardHandler struct {
	DB *sql.DB
}

// NewDashboardHandler creates a new dashboard handler
func NewDashboardHandler(database *sql.DB) *DashboardHandler {
	return &DashboardHandler{DB: database}
}

// GetTemperatureDashboard handles GET /api/dashboard/temperature
// Query params: details=true (include drives by status and recent alerts)
func (h *DashboardHandler) GetTemperatureDashboard(w http.ResponseWriter, r *http.Request) {
	includeDetails := r.URL.Query().Get("details") == "true"

	data, err := GetDashboardTemperatureData(h.DB, includeDetails)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, data)
}

// GetDashboardOverview handles GET /api/dashboard/overview
// Returns quick summary for header/sidebar display
func (h *DashboardHandler) GetDashboardOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := GetDashboardOverview(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, overview)
}

// GetTemperatureTrends handles GET /api/dashboard/temperature/trends
// Query params: period (24h, 7d, 30d), limit (default 20)
func (h *DashboardHandler) GetTemperatureTrends(w http.ResponseWriter, r *http.Request) {
	periodStr := r.URL.Query().Get("period")
	period := ParsePeriod(periodStr)

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	trends, err := GetTemperatureTrends(h.DB, period, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"period": string(period),
		"trends": trends,
		"count":  len(trends),
	})
}

// GetTemperatureDistribution handles GET /api/dashboard/temperature/distribution
// Returns histogram data for temperature distribution
func (h *DashboardHandler) GetTemperatureDistribution(w http.ResponseWriter, r *http.Request) {
	dist, err := GetTemperatureDistribution(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, dist)
}

// GetDashboardAlerts handles GET /api/dashboard/alerts
// Returns combined alert summary from all sources
func (h *DashboardHandler) GetDashboardAlerts(w http.ResponseWriter, r *http.Request) {
	// Get temperature alert summary
	alertSummary, err := GetAlertSummary(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get spike summary
	spikeSummary, err := GetSpikeSummary(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get recent active alerts
	activeAlerts, err := GetActiveAlerts(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Limit to most recent
	if len(activeAlerts) > 10 {
		activeAlerts = activeAlerts[:10]
	}

	jsonResponse(w, map[string]interface{}{
		"temperature_alerts": alertSummary,
		"spikes":             spikeSummary,
		"active_alerts":      activeAlerts,
		"total_active":       alertSummary.Unacknowledged + spikeSummary.Unacknowledged,
	})
}

// GetDashboardStatus handles GET /api/dashboard/status
// Returns overall system status for health checks
func (h *DashboardHandler) GetDashboardStatus(w http.ResponseWriter, r *http.Request) {
	overview, err := GetDashboardOverview(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Determine overall health
	health := "healthy"
	if overview.Status == "critical" {
		health = "critical"
	} else if overview.Status == "warning" || overview.ActiveAlerts > 0 {
		health = "degraded"
	}

	jsonResponse(w, map[string]interface{}{
		"health":             health,
		"status":             overview.Status,
		"total_drives":       overview.TotalDrives,
		"drives_with_issues": overview.DrivesWithIssues,
		"active_alerts":      overview.ActiveAlerts,
		"avg_temperature":    overview.AvgTemperature,
		"max_temperature":    overview.MaxTemperature,
	})
}

// GetDashboardWidget handles GET /api/dashboard/widget/{type}
// Returns data formatted for specific dashboard widgets
func (h *DashboardHandler) GetDashboardWidget(w http.ResponseWriter, r *http.Request) {
	widgetType := r.PathValue("type")

	switch widgetType {
	case "temperature-gauge":
		h.getTemperatureGaugeWidget(w, r)
	case "alert-badge":
		h.getAlertBadgeWidget(w, r)
	case "temperature-chart":
		h.getTemperatureChartWidget(w, r)
	case "drive-status":
		h.getDriveStatusWidget(w, r)
	default:
		http.Error(w, "unknown widget type", http.StatusBadRequest)
	}
}

// Widget: Temperature gauge data
func (h *DashboardHandler) getTemperatureGaugeWidget(w http.ResponseWriter, _ *http.Request) {
	overview, err := GetDashboardOverview(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	thresholds := DefaultThresholds()
	warningThreshold, _ := settings.GetIntSetting(h.DB, "temperature", "warning_threshold")
	criticalThreshold, _ := settings.GetIntSetting(h.DB, "temperature", "critical_threshold")
	if warningThreshold > 0 {
		thresholds.Warning = warningThreshold
	}
	if criticalThreshold > 0 {
		thresholds.Critical = criticalThreshold
	}

	jsonResponse(w, map[string]interface{}{
		"avg_temperature": overview.AvgTemperature,
		"max_temperature": overview.MaxTemperature,
		"status":          overview.Status,
		"thresholds": map[string]int{
			"warning":  thresholds.Warning,
			"critical": thresholds.Critical,
		},
	})
}

// Widget: Alert badge data
func (h *DashboardHandler) getAlertBadgeWidget(w http.ResponseWriter, _ *http.Request) {
	alertSummary, _ := GetAlertSummary(h.DB)
	spikeSummary, _ := GetSpikeSummary(h.DB)

	total := 0
	if alertSummary != nil {
		total += alertSummary.Unacknowledged
	}
	if spikeSummary != nil {
		total += spikeSummary.Unacknowledged
	}

	severity := "none"
	if alertSummary != nil {
		if alertSummary.Critical > 0 {
			severity = "critical"
		} else if alertSummary.Warning > 0 {
			severity = "warning"
		} else if total > 0 {
			severity = "info"
		}
	}

	jsonResponse(w, map[string]interface{}{
		"count":    total,
		"severity": severity,
	})
}

// Widget: Temperature chart data
func (h *DashboardHandler) getTemperatureChartWidget(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serial := r.URL.Query().Get("serial")
	periodStr := r.URL.Query().Get("period")

	if hostname == "" || serial == "" {
		// Return aggregated data for all drives
		dist, err := GetTemperatureDistribution(h.DB)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		jsonResponse(w, map[string]interface{}{
			"type": "distribution",
			"data": dist,
		})
		return
	}

	// Return time series for specific drive
	period := ParsePeriod(periodStr)
	var interval AggregationInterval
	switch period {
	case Period7Days:
		interval = Interval6Hours
	case Period30Days:
		interval = IntervalDaily
	default:
		interval = IntervalHourly
	}

	timeSeries, err := GetTemperatureTimeSeries(h.DB, hostname, serial, period, interval)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"type": "timeseries",
		"data": timeSeries,
	})
}

// Widget: Drive status summary
func (h *DashboardHandler) getDriveStatusWidget(w http.ResponseWriter, _ *http.Request) {
	data, err := GetDashboardTemperatureData(h.DB, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jsonResponse(w, map[string]interface{}{
		"total":    data.TotalDrives,
		"normal":   data.DrivesNormal,
		"warning":  data.DrivesWarning,
		"critical": data.DrivesCritical,
		"hottest":  data.HottestDrive,
		"coolest":  data.CoolestDrive,
	})
}

// ============================================
// SHARED HELPERS
// ============================================

// getUsernameFromRequest extracts username from request context or headers
func getUsernameFromRequest(r *http.Request) string {
	// Try to get from context (set by auth middleware)
	if username, ok := r.Context().Value("username").(string); ok {
		return username
	}

	// Fallback to header
	if username := r.Header.Get("X-Username"); username != "" {
		return username
	}

	return "system"
}

// jsonResponse writes data as JSON to the response writer
func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
