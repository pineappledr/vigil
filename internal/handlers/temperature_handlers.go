package handlers

import (
	"database/sql"
	"net/http"
	"time"

	"vigil/internal/db"
)

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

	period := db.ParsePeriod(periodStr)

	stats, err := db.GetTemperatureStats(h.DB, hostname, serial, period)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if stats == nil {
		http.Error(w, "no temperature data found", http.StatusNotFound)
		return
	}

	respondJSON(w, stats)
}

// GetAllTemperatureStats handles GET /api/temperature/stats/all
// Query params: period (24h, 7d, 30d, all)
func (h *TemperatureHandler) GetAllTemperatureStats(w http.ResponseWriter, r *http.Request) {
	periodStr := r.URL.Query().Get("period")
	period := db.ParsePeriod(periodStr)

	stats, err := db.GetAllDrivesTemperatureStats(h.DB, period)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
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

	period := db.ParsePeriod(periodStr)
	interval := db.ParseInterval(intervalStr)

	// Auto-select interval if not specified
	if intervalStr == "" {
		interval = autoSelectInterval(period)
	}

	data, err := db.GetTemperatureTimeSeries(h.DB, hostname, serial, period, interval)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if data == nil || len(data.Points) == 0 {
		http.Error(w, "no temperature data found", http.StatusNotFound)
		return
	}

	respondJSON(w, data)
}

// GetCurrentTemperatures handles GET /api/temperature/current
// Query params: hostname, serial (both optional - if not provided, returns all)
func (h *TemperatureHandler) GetCurrentTemperatures(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serial := r.URL.Query().Get("serial")

	// If specific drive requested
	if hostname != "" && serial != "" {
		current, err := db.GetCurrentTemperature(h.DB, hostname, serial)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if current == nil {
			http.Error(w, "no temperature data found", http.StatusNotFound)
			return
		}
		respondJSON(w, current)
		return
	}

	// Return all drives
	temps, err := db.GetAllCurrentTemperatures(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
		"drives": temps,
		"count":  len(temps),
	})
}

// GetTemperatureSummary handles GET /api/temperature/summary
func (h *TemperatureHandler) GetTemperatureSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := db.GetTemperatureSummary(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, summary)
}

// GetTemperatureHeatmap handles GET /api/temperature/heatmap
// Query params: period (24h, 7d, 30d), interval (1h, 6h, 1d)
func (h *TemperatureHandler) GetTemperatureHeatmap(w http.ResponseWriter, r *http.Request) {
	periodStr := r.URL.Query().Get("period")
	intervalStr := r.URL.Query().Get("interval")

	period := db.ParsePeriod(periodStr)
	interval := db.ParseInterval(intervalStr)

	// Auto-select interval if not specified
	if intervalStr == "" {
		interval = autoSelectInterval(period)
	}

	data, err := db.GetHeatmapData(h.DB, period, interval)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, data)
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

	records, err := db.GetTemperatureRange(h.DB, hostname, serial, from, to)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
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
	summary, err := db.GetTemperatureSummary(h.DB)
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
	warningThreshold := db.GetIntSettingWithDefault(h.DB, "temperature", "warning_threshold", 45)
	criticalThreshold := db.GetIntSettingWithDefault(h.DB, "temperature", "critical_threshold", 55)

	response["thresholds"] = map[string]int{
		"warning":  warningThreshold,
		"critical": criticalThreshold,
	}

	// Add drives grouped by status
	if len(summary.Drives) > 0 {
		byStatus := map[string][]db.CurrentTemperature{
			"normal":   {},
			"warning":  {},
			"critical": {},
		}
		for _, d := range summary.Drives {
			byStatus[d.Status] = append(byStatus[d.Status], d)
		}
		response["drives_by_status"] = byStatus
	}

	respondJSON(w, response)
}

// autoSelectInterval chooses an appropriate interval based on period
func autoSelectInterval(period db.TemperaturePeriod) db.AggregationInterval {
	switch period {
	case db.Period24Hours:
		return db.IntervalHourly
	case db.Period7Days:
		return db.Interval6Hours
	case db.Period30Days:
		return db.IntervalDaily
	case db.PeriodAllTime:
		return db.IntervalDaily
	default:
		return db.IntervalHourly
	}
}

// respondJSON helper is defined in settings_handlers.go
// If this file is used standalone, uncomment below:
/*
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
*/
