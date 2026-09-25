package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"vigil/internal/audit"
	"vigil/internal/auth"
	"vigil/internal/db"
	"vigil/internal/smart"
)

// GetBaselines returns acknowledged SMART counters, optionally for one host.
// GET /api/baselines[?hostname=X]
func GetBaselines(w http.ResponseWriter, r *http.Request) {
	list, err := smart.ListBaselines(db.DB, r.URL.Query().Get("hostname"))
	if err != nil {
		JSONError(w, "Database error", http.StatusInternalServerError)
		return
	}
	JSONResponse(w, list)
}

type baselineRequest struct {
	Hostname     string `json:"hostname"`
	SerialNumber string `json:"serial_number"`
}

func decodeBaselineRequest(w http.ResponseWriter, r *http.Request) (baselineRequest, bool) {
	var req baselineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid request", http.StatusBadRequest)
		return req, false
	}
	if req.Hostname == "" || req.SerialNumber == "" {
		JSONError(w, "Missing hostname or serial_number", http.StatusBadRequest)
		return req, false
	}
	return req, true
}

// AcknowledgeBaseline acknowledges the drive's CURRENT error counters.
// The values are read from Vigil's latest report, never from the body.
// POST /api/baselines {hostname, serial_number}
func AcknowledgeBaseline(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBaselineRequest(w, r)
	if !ok {
		return
	}
	by := ""
	s := auth.GetSessionFromContext(r)
	if s != nil {
		by = s.Username
	}
	n, err := smart.AcknowledgeCurrent(db.DB, req.Hostname, req.SerialNumber, by)
	if err != nil {
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}
	log.Printf("✅ Baseline acknowledged: %s/%s (%d counters) by %s", req.Hostname, req.SerialNumber, n, by)
	if s != nil {
		audit.LogEvent(db.DB, r, s.UserID, s.Username, "baseline_ack", "drive", req.SerialNumber,
			fmt.Sprintf("%s/%s: %d counters", req.Hostname, req.SerialNumber, n), "success")
	}
	JSONResponse(w, map[string]interface{}{"status": "ok", "acknowledged": n})
}

// ClearBaseline removes a drive's acknowledged counters: it is judged on its
// raw totals again.
// DELETE /api/baselines {hostname, serial_number}
func ClearBaseline(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeBaselineRequest(w, r)
	if !ok {
		return
	}
	n, err := smart.ClearBaseline(db.DB, req.Hostname, req.SerialNumber)
	if err != nil {
		JSONError(w, "Database error", http.StatusInternalServerError)
		return
	}
	if s := auth.GetSessionFromContext(r); s != nil {
		audit.LogEvent(db.DB, r, s.UserID, s.Username, "baseline_clear", "drive", req.SerialNumber,
			fmt.Sprintf("%s/%s", req.Hostname, req.SerialNumber), "success")
	}
	JSONResponse(w, map[string]interface{}{"status": "deleted", "cleared": n})
}
