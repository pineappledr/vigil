package handlers

import (
	"net/http"

	"vigil/internal/db"
	"vigil/internal/reports"
)

// GetHealthReport returns a self-contained HTML health report.
// GET /api/reports/health
func GetHealthReport(w http.ResponseWriter, r *http.Request) {
	html, err := reports.GenerateHealthReport(db.DB)
	if err != nil {
		JSONError(w, "Failed to generate report: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(html)
}
