package auth

import (
	"context"
	"net/http"
	"strings"

	"vigil/internal/models"
)

// contextKey is the type for context keys in the auth package
type contextKey string

// SessionKey is the context key for session data
const SessionKey contextKey = "session"

// Middleware checks for valid authentication before calling next
func Middleware(config models.Config, next http.HandlerFunc) http.HandlerFunc {
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

// GetSessionFromRequest extracts a session from the request cookie or Authorization header
func GetSessionFromRequest(r *http.Request) *models.Session {
	var token string

	if cookie, err := r.Cookie("session"); err == nil {
		token = cookie.Value
	} else if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		token = strings.TrimPrefix(authHeader, "Bearer ")
	}

	return GetSession(token)
}

// GetSessionFromContext extracts the session stored in the request context
func GetSessionFromContext(r *http.Request) *models.Session {
	if session, ok := r.Context().Value(SessionKey).(*models.Session); ok {
		return session
	}
	return nil
}

// IsAuthenticated reports whether the request carries a valid session
func IsAuthenticated(r *http.Request) bool {
	return GetSessionFromRequest(r) != nil
}
