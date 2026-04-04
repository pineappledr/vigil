package handlers

import (
	"net/http"
	"os"

	"vigil/internal/db"
)

// GetStats returns system metrics as JSON.
// GET /api/stats
func GetStats(w http.ResponseWriter, r *http.Request) {
	if Metrics == nil {
		JSONError(w, "Metrics not initialized", http.StatusServiceUnavailable)
		return
	}

	queueDepth := ReportQueueDepth()

	// Count active agents
	var activeAgents int
	row := db.DB.QueryRow("SELECT COUNT(*) FROM agent_registry WHERE enabled = 1")
	row.Scan(&activeAgents)

	// DB file size
	var dbSize int64
	if info, err := os.Stat(DBPath); err == nil {
		dbSize = info.Size()
	}

	snapshot := Metrics.Snapshot(queueDepth, activeAgents, dbSize)
	JSONResponse(w, snapshot)
}

// RegisterStatsRoutes registers the stats API route.
func RegisterStatsRoutes(mux *http.ServeMux, protect func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /api/stats", protect(GetStats))
}
