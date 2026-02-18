package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"vigil/internal/db"
)

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
	filter := db.AlertFilter{
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

	alerts, err := db.GetAlerts(h.DB, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
		"alerts": alerts,
		"count":  len(alerts),
	})
}

// GetActiveAlerts handles GET /api/alerts/temperature/active
func (h *AlertHandler) GetActiveAlerts(w http.ResponseWriter, r *http.Request) {
	alerts, err := db.GetActiveAlerts(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
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

	alert, err := db.GetAlertByID(h.DB, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if alert == nil {
		http.Error(w, "alert not found", http.StatusNotFound)
		return
	}

	respondJSON(w, alert)
}

// GetAlertSummary handles GET /api/alerts/temperature/summary
func (h *AlertHandler) GetAlertSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := db.GetAlertSummary(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, summary)
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

	if err := db.AcknowledgeAlert(h.DB, id, username); err != nil {
		if err.Error() == "alert not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the updated alert
	alert, _ := db.GetAlertByID(h.DB, id)
	respondJSON(w, map[string]interface{}{
		"message": "alert acknowledged",
		"alert":   alert,
	})
}

// AcknowledgeAllAlerts handles POST /api/alerts/temperature/acknowledge-all
func (h *AlertHandler) AcknowledgeAllAlerts(w http.ResponseWriter, r *http.Request) {
	username := getUsernameFromRequest(r)

	count, err := db.AcknowledgeAllAlerts(h.DB, username)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
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

	if err := db.DeleteAlert(h.DB, id); err != nil {
		if err.Error() == "alert not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
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

	alerts, err := db.GetAlertsByDrive(h.DB, hostname, serial, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get current alert status
	status, _ := db.GetDriveAlertStatus(h.DB, hostname, serial)

	respondJSON(w, map[string]interface{}{
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
	alerts, err := db.ProcessTemperatureReading(h.DB, req.Hostname, req.SerialNumber, req.Temperature)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
		"message": "test alert processed",
		"alerts":  alerts,
		"count":   len(alerts),
	})
}

// CleanupAlerts handles POST /api/alerts/temperature/cleanup
// Removes old alerts based on retention settings
func (h *AlertHandler) CleanupAlerts(w http.ResponseWriter, r *http.Request) {
	retentionDays := db.GetIntSettingWithDefault(h.DB, "system", "data_retention_days", 365)

	deleted, err := db.CleanupOldAlerts(h.DB, retentionDays)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
		"message":        "cleanup completed",
		"deleted":        deleted,
		"retention_days": retentionDays,
	})
}
