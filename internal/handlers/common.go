package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"vigil/internal/middleware"
	"vigil/internal/models"
)

// JSONResponse sends a JSON response
func JSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("⚠️  Failed to encode JSON response: %v", err)
	}
}

// JSONError sends a JSON error response
func JSONError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// GetSessionFromContext extracts session from request context
func GetSessionFromContext(r *http.Request) *models.Session {
	if session, ok := r.Context().Value(middleware.SessionKey).(*models.Session); ok {
		return session
	}
	return nil
}
