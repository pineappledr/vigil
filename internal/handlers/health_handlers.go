package handlers

import (
	"net/http"

	"vigil/internal/db"
	"vigil/internal/health"
)

// GetHealthScore returns the aggregate health score.
func GetHealthScore(w http.ResponseWriter, r *http.Request) {
	score, err := health.Calculate(db.DB)
	if err != nil {
		JSONError(w, "Failed to calculate health score: "+err.Error(), http.StatusInternalServerError)
		return
	}
	JSONResponse(w, score)
}

// RegisterHealthRoutes registers health-related API routes.
func RegisterHealthRoutes(mux *http.ServeMux, protect func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /api/health/score", protect(GetHealthScore))
}
