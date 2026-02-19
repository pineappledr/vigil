package handlers

import (
	"database/sql"
	"net/http"
	"strconv"

	"vigil/internal/db"
)

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

	var spikes []db.TemperatureSpike
	var err error

	if hostname != "" && serial != "" {
		spikes, err = db.GetRecentSpikes(h.DB, hostname, serial, limit)
	} else {
		spikes, err = db.GetAllRecentSpikes(h.DB, limit)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]interface{}{
		"spikes": spikes,
		"count":  len(spikes),
	})
}

// GetUnacknowledgedSpikes handles GET /api/temperature/spikes/unacknowledged
func (h *SpikeHandler) GetUnacknowledgedSpikes(w http.ResponseWriter, r *http.Request) {
	spikes, err := db.GetUnacknowledgedSpikes(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]interface{}{
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

	spike, err := db.GetSpikeByID(h.DB, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if spike == nil {
		http.Error(w, "spike not found", http.StatusNotFound)
		return
	}

	JSONResponse(w, spike)
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

	if err := db.AcknowledgeSpike(h.DB, id, username); err != nil {
		if err.Error() == "spike not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the updated spike
	spike, _ := db.GetSpikeByID(h.DB, id)
	JSONResponse(w, map[string]interface{}{
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

	if err := db.DeleteSpike(h.DB, id); err != nil {
		if err.Error() == "spike not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]interface{}{
		"message": "spike deleted",
		"id":      id,
	})
}

// GetSpikeSummary handles GET /api/temperature/spikes/summary
func (h *SpikeHandler) GetSpikeSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := db.GetSpikeSummary(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, summary)
}

// DetectSpikes handles POST /api/temperature/spikes/detect
// Manually trigger spike detection for all drives or a specific drive
func (h *SpikeHandler) DetectSpikes(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serial := r.URL.Query().Get("serial")

	var spikes []db.TemperatureSpike
	var err error

	if hostname != "" && serial != "" {
		// Detect for specific drive
		spikes, err = db.DetectAndRecordSpikes(h.DB, hostname, serial)
	} else {
		// Detect for all drives
		spikes, err = db.DetectAllDrivesSpikes(h.DB)
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]interface{}{
		"message":    "spike detection completed",
		"new_spikes": spikes,
		"count":      len(spikes),
	})
}

// AcknowledgeAllSpikes handles POST /api/temperature/spikes/acknowledge-all
func (h *SpikeHandler) AcknowledgeAllSpikes(w http.ResponseWriter, r *http.Request) {
	username := getUsernameFromRequest(r)

	// Get all unacknowledged spikes
	spikes, err := db.GetUnacknowledgedSpikes(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Acknowledge each
	acknowledged := 0
	for _, spike := range spikes {
		if err := db.AcknowledgeSpike(h.DB, spike.ID, username); err == nil {
			acknowledged++
		}
	}

	JSONResponse(w, map[string]interface{}{
		"message":      "spikes acknowledged",
		"acknowledged": acknowledged,
	})
}

// Helper to get username from request context
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
