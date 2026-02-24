// internal/handlers/wearout_handlers.go
package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"vigil/internal/db"
	"vigil/internal/wearout"
)

// GetDriveWearout returns the latest wearout snapshot for a single drive.
func GetDriveWearout(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serial := r.URL.Query().Get("serial")
	if hostname == "" || serial == "" {
		JSONError(w, "Missing hostname or serial", http.StatusBadRequest)
		return
	}

	snapshot, err := wearout.GetLatestSnapshot(db.DB, hostname, serial)
	if err != nil {
		JSONError(w, "Failed to retrieve wearout: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if snapshot == nil {
		JSONError(w, "No wearout data for this drive", http.StatusNotFound)
		return
	}

	JSONResponse(w, snapshot)
}

// GetAllWearout returns the latest wearout for every monitored drive.
func GetAllWearout(w http.ResponseWriter, r *http.Request) {
	snapshots, err := wearout.GetAllLatestSnapshots(db.DB)
	if err != nil {
		JSONError(w, "Failed to retrieve wearout data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]interface{}{
		"drives": snapshots,
		"count":  len(snapshots),
	})
}

// GetWearoutHistory returns historical wearout snapshots for a drive.
func GetWearoutHistory(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serial := r.URL.Query().Get("serial")
	if hostname == "" || serial == "" {
		JSONError(w, "Missing hostname or serial", http.StatusBadRequest)
		return
	}

	days := 90
	if d := r.URL.Query().Get("days"); d != "" {
		if v, err := strconv.Atoi(d); err == nil && v > 0 && v <= 365 {
			days = v
		}
	}

	history, err := wearout.GetSnapshotHistory(db.DB, hostname, serial, days)
	if err != nil {
		JSONError(w, "Failed to retrieve wearout history: "+err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]interface{}{
		"hostname":      hostname,
		"serial_number": serial,
		"days":          days,
		"history":       history,
		"data_points":   len(history),
	})
}

// GetWearoutTrend returns the trend prediction for a drive.
func GetWearoutTrend(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serial := r.URL.Query().Get("serial")
	if hostname == "" || serial == "" {
		JSONError(w, "Missing hostname or serial", http.StatusBadRequest)
		return
	}

	history, err := wearout.GetSnapshotHistory(db.DB, hostname, serial, 365)
	if err != nil {
		JSONError(w, "Failed to retrieve wearout history: "+err.Error(), http.StatusInternalServerError)
		return
	}

	prediction := wearout.PredictTrend(history)

	// Get latest percentage for context
	var currentPct float64
	if len(history) > 0 {
		currentPct = history[len(history)-1].Percentage
	}

	JSONResponse(w, map[string]interface{}{
		"hostname":           hostname,
		"serial_number":      serial,
		"current_percentage": currentPct,
		"prediction":         prediction,
		"data_points":        len(history),
	})
}

// GetDriveSpecs returns all configured drive specifications.
func GetDriveSpecs(w http.ResponseWriter, r *http.Request) {
	specs, err := wearout.ListDriveSpecs(db.DB)
	if err != nil {
		JSONError(w, "Failed to retrieve drive specs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]interface{}{
		"specs": specs,
		"count": len(specs),
	})
}

// UpsertDriveSpec creates or updates a drive specification.
func UpsertDriveSpec(w http.ResponseWriter, r *http.Request) {
	var spec wearout.DriveSpec
	if err := json.NewDecoder(r.Body).Decode(&spec); err != nil {
		JSONError(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}
	if spec.ModelPattern == "" {
		JSONError(w, "model_pattern is required", http.StatusBadRequest)
		return
	}

	if err := wearout.UpsertDriveSpec(db.DB, spec); err != nil {
		JSONError(w, "Failed to save drive spec: "+err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]string{"status": "ok"})
}

// DeleteDriveSpec removes a drive specification by ID.
func DeleteDriveSpec(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id <= 0 {
		JSONError(w, "Invalid spec ID", http.StatusBadRequest)
		return
	}

	if err := wearout.DeleteDriveSpec(db.DB, id); err != nil {
		JSONError(w, err.Error(), http.StatusNotFound)
		return
	}

	JSONResponse(w, map[string]string{"status": "deleted"})
}

// RegisterWearoutRoutes registers all wearout API endpoints.
func RegisterWearoutRoutes(mux *http.ServeMux, protect func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /api/wearout/drive", protect(GetDriveWearout))
	mux.HandleFunc("GET /api/wearout/all", protect(GetAllWearout))
	mux.HandleFunc("GET /api/wearout/history", protect(GetWearoutHistory))
	mux.HandleFunc("GET /api/wearout/trend", protect(GetWearoutTrend))
	mux.HandleFunc("GET /api/wearout/specs", protect(GetDriveSpecs))
	mux.HandleFunc("POST /api/wearout/specs", protect(UpsertDriveSpec))
	mux.HandleFunc("DELETE /api/wearout/specs/{id}", protect(DeleteDriveSpec))
}
