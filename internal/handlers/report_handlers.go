package handlers

import (
	"net/http"

	"vigil/internal/db"
	"vigil/internal/reports"
)

// GetHealthReport returns a self-contained HTML health report.
// Query params: ?hostname= (optional filter), ?format=json (returns JSON).
func GetHealthReport(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	format := r.URL.Query().Get("format")

	if format == "json" {
		data, err := reports.GenerateHealthReportJSON(db.DB, hostname)
		if err != nil {
			JSONError(w, "Failed to generate report: "+err.Error(), http.StatusInternalServerError)
			return
		}
		JSONResponse(w, data)
		return
	}

	html, err := reports.GenerateHealthReport(db.DB, hostname)
	if err != nil {
		http.Error(w, "Failed to generate report: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(html)
}

// RegisterReportRoutes registers report-related API routes.
func RegisterReportRoutes(mux *http.ServeMux, protect func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /api/reports/health", protect(GetHealthReport))
}
