package middleware

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"vigil/internal/db"
	"vigil/internal/models"
)

// CORS adds CORS headers to responses
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Logging logs request details
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

// Auth checks for valid authentication
func Auth(config models.Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !config.AuthEnabled {
			next(w, r)
			return
		}

		session := GetSessionFromRequest(r)
		if session == nil {
			http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
			return
		}

		ctx := context.WithValue(r.Context(), SessionKey, session)
		next(w, r.WithContext(ctx))
	}
}

// SessionKey is the context key for session data
type contextKey string

const SessionKey contextKey = "session"

// GetSessionFromRequest extracts session from cookie or header
func GetSessionFromRequest(r *http.Request) *models.Session {
	var token string

	if cookie, err := r.Cookie("session"); err == nil {
		token = cookie.Value
	} else if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		token = strings.TrimPrefix(auth, "Bearer ")
	}

	return db.GetSession(token)
}

// IsAuthenticated checks if request has valid session
func IsAuthenticated(r *http.Request) bool {
	return GetSessionFromRequest(r) != nil
}
