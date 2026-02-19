package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"vigil/internal/db"
	"vigil/internal/settings"
)

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

	data, err := db.GetDashboardTemperatureData(h.DB, includeDetails)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, data)
}

// GetDashboardOverview handles GET /api/dashboard/overview
// Returns quick summary for header/sidebar display
func (h *DashboardHandler) GetDashboardOverview(w http.ResponseWriter, r *http.Request) {
	overview, err := db.GetDashboardOverview(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, overview)
}

// GetTemperatureTrends handles GET /api/dashboard/temperature/trends
// Query params: period (24h, 7d, 30d), limit (default 20)
func (h *DashboardHandler) GetTemperatureTrends(w http.ResponseWriter, r *http.Request) {
	periodStr := r.URL.Query().Get("period")
	period := db.ParsePeriod(periodStr)

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	trends, err := db.GetTemperatureTrends(h.DB, period, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]interface{}{
		"period": string(period),
		"trends": trends,
		"count":  len(trends),
	})
}

// GetTemperatureDistribution handles GET /api/dashboard/temperature/distribution
// Returns histogram data for temperature distribution
func (h *DashboardHandler) GetTemperatureDistribution(w http.ResponseWriter, r *http.Request) {
	dist, err := db.GetTemperatureDistribution(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, dist)
}

// GetDashboardAlerts handles GET /api/dashboard/alerts
// Returns combined alert summary from all sources
func (h *DashboardHandler) GetDashboardAlerts(w http.ResponseWriter, r *http.Request) {
	// Get temperature alert summary
	alertSummary, err := db.GetAlertSummary(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get spike summary
	spikeSummary, err := db.GetSpikeSummary(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get recent active alerts
	activeAlerts, err := db.GetActiveAlerts(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Limit to most recent
	if len(activeAlerts) > 10 {
		activeAlerts = activeAlerts[:10]
	}

	JSONResponse(w, map[string]interface{}{
		"temperature_alerts": alertSummary,
		"spikes":             spikeSummary,
		"active_alerts":      activeAlerts,
		"total_active":       alertSummary.Unacknowledged + spikeSummary.Unacknowledged,
	})
}

// GetDashboardStatus handles GET /api/dashboard/status
// Returns overall system status for health checks
func (h *DashboardHandler) GetDashboardStatus(w http.ResponseWriter, r *http.Request) {
	overview, err := db.GetDashboardOverview(h.DB)
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

	JSONResponse(w, map[string]interface{}{
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
	overview, err := db.GetDashboardOverview(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	thresholds := db.DefaultThresholds()
	warningThreshold, _ := settings.GetIntSetting(h.DB, "temperature", "warning_threshold")
	criticalThreshold, _ := settings.GetIntSetting(h.DB, "temperature", "critical_threshold")
	if warningThreshold > 0 {
		thresholds.Warning = warningThreshold
	}
	if criticalThreshold > 0 {
		thresholds.Critical = criticalThreshold
	}

	JSONResponse(w, map[string]interface{}{
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
	alertSummary, _ := db.GetAlertSummary(h.DB)
	spikeSummary, _ := db.GetSpikeSummary(h.DB)

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

	JSONResponse(w, map[string]interface{}{
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
		dist, err := db.GetTemperatureDistribution(h.DB)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		JSONResponse(w, map[string]interface{}{
			"type": "distribution",
			"data": dist,
		})
		return
	}

	// Return time series for specific drive
	period := db.ParsePeriod(periodStr)
	var interval db.AggregationInterval
	switch period {
	case db.Period7Days:
		interval = db.Interval6Hours
	case db.Period30Days:
		interval = db.IntervalDaily
	default:
		interval = db.IntervalHourly
	}

	timeSeries, err := db.GetTemperatureTimeSeries(h.DB, hostname, serial, period, interval)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]interface{}{
		"type": "timeseries",
		"data": timeSeries,
	})
}

// Widget: Drive status summary
func (h *DashboardHandler) getDriveStatusWidget(w http.ResponseWriter, _ *http.Request) {
	data, err := db.GetDashboardTemperatureData(h.DB, false)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]interface{}{
		"total":    data.TotalDrives,
		"normal":   data.DrivesNormal,
		"warning":  data.DrivesWarning,
		"critical": data.DrivesCritical,
		"hottest":  data.HottestDrive,
		"coolest":  data.CoolestDrive,
	})
}
