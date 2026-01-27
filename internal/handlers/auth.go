package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"vigil/internal/db"
	"vigil/internal/middleware"
	"vigil/internal/models"
)

// AuthStatus returns authentication status
func AuthStatus(config models.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := middleware.GetSessionFromRequest(r)

		var mustChangePassword bool
		var username string

		if session != nil {
			username = session.Username
			var mustChange int
			db.DB.QueryRow("SELECT COALESCE(must_change_password, 0) FROM users WHERE id = ?", session.UserID).Scan(&mustChange)
			mustChangePassword = mustChange == 1
		}

		JSONResponse(w, map[string]interface{}{
			"auth_enabled":         config.AuthEnabled,
			"authenticated":        session != nil,
			"username":             username,
			"must_change_password": mustChangePassword,
		})
	}
}

// Login handles user authentication
func Login(config models.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !config.AuthEnabled {
			JSONResponse(w, map[string]interface{}{
				"success": true,
				"message": "Authentication disabled",
			})
			return
		}

		var creds struct {
			Username string `json:"username"`
			Password string `json:"password"`
		}

		if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
			JSONError(w, "Invalid request", http.StatusBadRequest)
			return
		}

		var user models.User
		var createdAt string
		var mustChange int

		err := db.DB.QueryRow(
			"SELECT id, username, password_hash, COALESCE(must_change_password, 0), created_at FROM users WHERE username = ?",
			creds.Username,
		).Scan(&user.ID, &user.Username, &user.PasswordHash, &mustChange, &createdAt)

		if err != nil || user.PasswordHash != db.HashPassword(creds.Password) {
			JSONError(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		token, expiresAt, err := db.CreateSession(user.ID)
		if err != nil {
			JSONError(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    token,
			Path:     "/",
			Expires:  expiresAt,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		log.Printf("🔓 Login: %s", user.Username)
		JSONResponse(w, map[string]interface{}{
			"success":              true,
			"token":                token,
			"username":             user.Username,
			"must_change_password": mustChange == 1,
		})
	}
}

// Logout handles user logout
func Logout(w http.ResponseWriter, r *http.Request) {
	session := middleware.GetSessionFromRequest(r)
	if session != nil {
		db.DeleteSession(session.Token)
		log.Printf("🔒 Logout: %s", session.Username)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	})

	JSONResponse(w, map[string]string{"status": "logged_out"})
}

// GetCurrentUser returns current user info
func GetCurrentUser(w http.ResponseWriter, r *http.Request) {
	session := GetSessionFromContext(r)
	JSONResponse(w, map[string]interface{}{
		"id":       session.UserID,
		"username": session.Username,
	})
}

// ChangePassword handles password changes
func ChangePassword(w http.ResponseWriter, r *http.Request) {
	session := GetSessionFromContext(r)

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) < 6 {
		JSONError(w, "Password must be at least 6 characters", http.StatusBadRequest)
		return
	}

	var currentHash string
	db.DB.QueryRow("SELECT password_hash FROM users WHERE id = ?", session.UserID).Scan(&currentHash)
	if currentHash != db.HashPassword(req.CurrentPassword) {
		JSONError(w, "Current password is incorrect", http.StatusUnauthorized)
		return
	}

	_, err := db.DB.Exec(
		"UPDATE users SET password_hash = ?, must_change_password = 0 WHERE id = ?",
		db.HashPassword(req.NewPassword), session.UserID,
	)
	if err != nil {
		JSONError(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	log.Printf("🔑 Password changed: %s", session.Username)
	JSONResponse(w, map[string]string{"status": "password_changed"})
}

// ChangeUsername handles username changes
func ChangeUsername(w http.ResponseWriter, r *http.Request) {
	session := GetSessionFromContext(r)

	var req struct {
		NewUsername     string `json:"new_username"`
		CurrentPassword string `json:"current_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	req.NewUsername = strings.TrimSpace(req.NewUsername)
	if req.NewUsername == "" {
		JSONError(w, "Username cannot be empty", http.StatusBadRequest)
		return
	}

	var currentHash string
	if err := db.DB.QueryRow("SELECT password_hash FROM users WHERE id = ?", session.UserID).Scan(&currentHash); err != nil {
		JSONError(w, "User not found", http.StatusInternalServerError)
		return
	}

	if currentHash != db.HashPassword(req.CurrentPassword) {
		JSONError(w, "Incorrect password", http.StatusUnauthorized)
		return
	}

	_, err := db.DB.Exec("UPDATE users SET username = ? WHERE id = ?", req.NewUsername, session.UserID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			JSONError(w, "Username already taken", http.StatusConflict)
			return
		}
		JSONError(w, "Database error", http.StatusInternalServerError)
		return
	}

	log.Printf("👤 Username changed: %s -> %s", session.Username, req.NewUsername)
	JSONResponse(w, map[string]string{
		"status":       "username_updated",
		"new_username": req.NewUsername,
	})
}
