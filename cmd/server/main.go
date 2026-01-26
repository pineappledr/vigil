package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	_ "modernc.org/sqlite"
)

// Version is set at build time via -ldflags
var version = "dev"

var db *sql.DB

type Config struct {
	Port        string
	DBPath      string
	AdminUser   string
	AdminPass   string
	AuthEnabled bool
}

type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Session struct {
	Token     string    `json:"token"`
	UserID    int       `json:"user_id"`
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expires_at"`
}

type DriveAlias struct {
	ID           int       `json:"id"`
	Hostname     string    `json:"hostname"`
	SerialNumber string    `json:"serial_number"`
	Alias        string    `json:"alias"`
	CreatedAt    time.Time `json:"created_at"`
}

func loadConfig() Config {
	authEnabled := getEnv("AUTH_ENABLED", "true") == "true"
	return Config{
		Port:        getEnv("PORT", "9080"),
		DBPath:      getEnv("DB_PATH", "vigil.db"),
		AdminUser:   getEnv("ADMIN_USER", "admin"),
		AdminPass:   getEnv("ADMIN_PASS", ""),
		AuthEnabled: authEnabled,
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

func generateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func initDB(path string, config Config) error {
	var err error

	// Ensure the directory exists
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create database directory %s: %w", dir, err)
		}
	}

	// Use file: URI format for modernc.org/sqlite
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", path)
	db, err = sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database at %s: %w", path, err)
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Enable WAL mode for better concurrency
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		log.Printf("⚠️  Could not enable WAL mode: %v", err)
	}

	// Create all tables
	schema := `
	CREATE TABLE IF NOT EXISTS reports (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hostname TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		data JSON NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_reports_hostname ON reports(hostname);
	CREATE INDEX IF NOT EXISTS idx_reports_timestamp ON reports(timestamp);

	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		must_change_password INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		expires_at DATETIME NOT NULL,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);
	CREATE INDEX IF NOT EXISTS idx_sessions_expires ON sessions(expires_at);

	CREATE TABLE IF NOT EXISTS drive_aliases (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hostname TEXT NOT NULL,
		serial_number TEXT NOT NULL,
		alias TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(hostname, serial_number)
	);
	CREATE INDEX IF NOT EXISTS idx_aliases_hostname ON drive_aliases(hostname);
	`

	if _, err := db.Exec(schema); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

	// Add must_change_password column if it doesn't exist (migration for existing DBs)
	db.Exec("ALTER TABLE users ADD COLUMN must_change_password INTEGER DEFAULT 0")

	// Create default admin user if auth is enabled and no users exist
	if config.AuthEnabled {
		var count int
		db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
		if count == 0 {
			password := config.AdminPass
			mustChange := 1 // Force password change on first login
			if password == "" {
				// Generate random password if not set
				password = generateToken()[:12]
				log.Printf("🔑 Generated admin password: %s", password)
				log.Printf("   Set ADMIN_PASS environment variable to use a custom password")
			} else {
				// If admin set a password via env, don't force change
				mustChange = 0
			}
			_, err := db.Exec(
				"INSERT INTO users (username, password_hash, must_change_password) VALUES (?, ?, ?)",
				config.AdminUser,
				hashPassword(password),
				mustChange,
			)
			if err != nil {
				log.Printf("⚠️  Could not create admin user: %v", err)
			} else {
				log.Printf("✓ Created admin user: %s", config.AdminUser)
			}
		}
	}

	// Cleanup expired sessions
	db.Exec("DELETE FROM sessions WHERE expires_at < datetime('now')")

	return nil
}

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("⚠️  Failed to encode JSON response: %v", err)
	}
}

func jsonError(w http.ResponseWriter, message string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// CORS middleware
func corsMiddleware(next http.Handler) http.Handler {
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

// Logging middleware
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start).Round(time.Millisecond))
	})
}

// Get session from request
func getSession(r *http.Request) *Session {
	// Try cookie first
	cookie, err := r.Cookie("session")
	var token string
	if err == nil {
		token = cookie.Value
	} else {
		// Try Authorization header
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			token = strings.TrimPrefix(auth, "Bearer ")
		}
	}

	if token == "" {
		return nil
	}

	var session Session
	var expiresAt string
	err = db.QueryRow(`
		SELECT s.token, s.user_id, u.username, s.expires_at 
		FROM sessions s 
		JOIN users u ON s.user_id = u.id 
		WHERE s.token = ? AND s.expires_at > datetime('now')
	`, token).Scan(&session.Token, &session.UserID, &session.Username, &expiresAt)

	if err != nil {
		return nil
	}

	session.ExpiresAt, _ = time.Parse("2006-01-02 15:04:05", expiresAt)
	return &session
}

// Auth middleware - returns handler that checks auth
func authMiddleware(config Config, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !config.AuthEnabled {
			next(w, r)
			return
		}

		session := getSession(r)
		if session == nil {
			jsonError(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Add session to context
		ctx := context.WithValue(r.Context(), "session", session)
		next(w, r.WithContext(ctx))
	}
}

// Check if request is authenticated (for conditional UI)
func isAuthenticated(r *http.Request) bool {
	return getSession(r) != nil
}

func main() {
	log.SetFlags(log.Ltime | log.Ldate)
	log.Printf("🚀 Vigil Server v%s starting...", version)

	config := loadConfig()

	if err := initDB(config.DBPath, config); err != nil {
		log.Fatalf("❌ Database error: %v", err)
	}
	defer db.Close()
	log.Printf("✓ Database: %s", config.DBPath)

	if config.AuthEnabled {
		log.Printf("✓ Authentication: enabled")
	} else {
		log.Printf("⚠️  Authentication: disabled (set AUTH_ENABLED=true to enable)")
	}

	mux := http.NewServeMux()

	// Public endpoints (no auth required)
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /api/version", handleVersion)
	mux.HandleFunc("GET /api/auth/status", handleAuthStatus(config))

	// Agent reporting endpoint (no auth - agents use their own auth if needed)
	mux.HandleFunc("POST /api/report", handleReport)

	// Auth endpoints
	mux.HandleFunc("POST /api/auth/login", handleLogin(config))
	mux.HandleFunc("POST /api/auth/logout", handleLogout)

	// Protected data endpoints
	mux.HandleFunc("GET /api/history", authMiddleware(config, handleHistory))
	mux.HandleFunc("GET /api/hosts", authMiddleware(config, handleHosts))
	mux.HandleFunc("DELETE /api/hosts/{hostname}", authMiddleware(config, handleDeleteHost))
	mux.HandleFunc("GET /api/hosts/{hostname}/history", authMiddleware(config, handleHostHistory))

	// Drive alias endpoints (protected)
	mux.HandleFunc("GET /api/aliases", authMiddleware(config, handleGetAliases))
	mux.HandleFunc("POST /api/aliases", authMiddleware(config, handleSetAlias))
	mux.HandleFunc("DELETE /api/aliases/{id}", authMiddleware(config, handleDeleteAlias))

	// User management (protected)
	mux.HandleFunc("GET /api/users/me", authMiddleware(config, handleGetCurrentUser))
	mux.HandleFunc("POST /api/users/password", authMiddleware(config, handleChangePassword))
	mux.HandleFunc("POST /api/users/username", authMiddleware(config, handleChangeUsername)) // <--- ADDED THIS

	// Static file server with login redirect
	mux.HandleFunc("/", handleStaticFiles(config))

	// Apply middleware
	handler := loggingMiddleware(corsMiddleware(mux))

	server := &http.Server{
		Addr:         ":" + config.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		log.Println("\n⏹️  Shutting down server...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := server.Shutdown(ctx); err != nil {
			log.Printf("⚠️  Shutdown error: %v", err)
		}
	}()

	log.Printf("✓ Listening on port %s", config.Port)
	log.Printf("🌐 Dashboard: http://localhost:%s", config.Port)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("❌ Server error: %v", err)
	}

	log.Println("👋 Server stopped")
}

// Static file handler with auth check
func handleStaticFiles(config Config) http.HandlerFunc {
	fs := http.FileServer(http.Dir("./web"))

	return func(w http.ResponseWriter, r *http.Request) {
		// Always allow access to login page and static assets
		if r.URL.Path == "/login.html" ||
			strings.HasSuffix(r.URL.Path, ".css") ||
			strings.HasSuffix(r.URL.Path, ".js") ||
			strings.HasSuffix(r.URL.Path, ".ico") ||
			strings.HasSuffix(r.URL.Path, ".png") ||
			strings.HasSuffix(r.URL.Path, ".svg") {
			fs.ServeHTTP(w, r)
			return
		}

		// Check auth for protected pages
		if config.AuthEnabled && !isAuthenticated(r) {
			http.Redirect(w, r, "/login.html", http.StatusFound)
			return
		}

		fs.ServeHTTP(w, r)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]string{
		"status":  "healthy",
		"version": version,
	})
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]string{"version": version})
}

func handleAuthStatus(config Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session := getSession(r)

		var mustChangePassword bool
		var username string

		if session != nil {
			username = session.Username
			// Check if user must change password
			var mustChange int
			db.QueryRow("SELECT COALESCE(must_change_password, 0) FROM users WHERE id = ?", session.UserID).Scan(&mustChange)
			mustChangePassword = mustChange == 1
		}

		jsonResponse(w, map[string]interface{}{
			"auth_enabled":         config.AuthEnabled,
			"authenticated":        session != nil,
			"username":             username,
			"must_change_password": mustChangePassword,
		})
	}
}

func handleLogin(config Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !config.AuthEnabled {
			jsonResponse(w, map[string]interface{}{
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
			jsonError(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Find user
		var user User
		var createdAt string
		var mustChange int
		err := db.QueryRow(
			"SELECT id, username, password_hash, COALESCE(must_change_password, 0), created_at FROM users WHERE username = ?",
			creds.Username,
		).Scan(&user.ID, &user.Username, &user.PasswordHash, &mustChange, &createdAt)

		if err != nil || user.PasswordHash != hashPassword(creds.Password) {
			jsonError(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		// Create session
		token := generateToken()
		expiresAt := time.Now().Add(24 * time.Hour * 7) // 7 days

		_, err = db.Exec(
			"INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)",
			token, user.ID, expiresAt.Format("2006-01-02 15:04:05"),
		)
		if err != nil {
			jsonError(w, "Failed to create session", http.StatusInternalServerError)
			return
		}

		// Set cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "session",
			Value:    token,
			Path:     "/",
			Expires:  expiresAt,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
		})

		log.Printf("🔓 Login: %s", user.Username)
		jsonResponse(w, map[string]interface{}{
			"success":              true,
			"token":                token,
			"username":             user.Username,
			"must_change_password": mustChange == 1,
		})
	}
}

func handleLogout(w http.ResponseWriter, r *http.Request) {
	session := getSession(r)
	if session != nil {
		db.Exec("DELETE FROM sessions WHERE token = ?", session.Token)
		log.Printf("🔒 Logout: %s", session.Username)
	}

	// Clear cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	})

	jsonResponse(w, map[string]string{"status": "logged_out"})
}

func handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*Session)
	jsonResponse(w, map[string]interface{}{
		"id":       session.UserID,
		"username": session.Username,
	})
}

func handleChangePassword(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*Session)

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if len(req.NewPassword) < 6 {
		jsonError(w, "Password must be at least 6 characters", http.StatusBadRequest)
		return
	}

	// Verify current password
	var currentHash string
	db.QueryRow("SELECT password_hash FROM users WHERE id = ?", session.UserID).Scan(&currentHash)
	if currentHash != hashPassword(req.CurrentPassword) {
		jsonError(w, "Current password is incorrect", http.StatusUnauthorized)
		return
	}

	// Update password and clear must_change_password flag
	_, err := db.Exec(
		"UPDATE users SET password_hash = ?, must_change_password = 0 WHERE id = ?",
		hashPassword(req.NewPassword), session.UserID,
	)
	if err != nil {
		jsonError(w, "Failed to update password", http.StatusInternalServerError)
		return
	}

	log.Printf("🔑 Password changed: %s", session.Username)
	jsonResponse(w, map[string]string{"status": "password_changed"})
}

// [ADDED FUNCTION]
func handleChangeUsername(w http.ResponseWriter, r *http.Request) {
	session := r.Context().Value("session").(*Session)

	var req struct {
		NewUsername     string `json:"new_username"`
		CurrentPassword string `json:"current_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	req.NewUsername = strings.TrimSpace(req.NewUsername)
	if req.NewUsername == "" {
		jsonError(w, "Username cannot be empty", http.StatusBadRequest)
		return
	}

	// Verify current password for security
	var currentHash string
	err := db.QueryRow("SELECT password_hash FROM users WHERE id = ?", session.UserID).Scan(&currentHash)
	if err != nil {
		jsonError(w, "User not found", http.StatusInternalServerError)
		return
	}

	if currentHash != hashPassword(req.CurrentPassword) {
		jsonError(w, "Incorrect password", http.StatusUnauthorized)
		return
	}

	// Update username
	_, err = db.Exec("UPDATE users SET username = ? WHERE id = ?", req.NewUsername, session.UserID)
	if err != nil {
		// Handle unique constraint violation
		if strings.Contains(err.Error(), "UNIQUE constraint") {
			jsonError(w, "Username already taken", http.StatusConflict)
			return
		}
		jsonError(w, "Database error", http.StatusInternalServerError)
		return
	}

	log.Printf("👤 Username changed: %s -> %s", session.Username, req.NewUsername)
	jsonResponse(w, map[string]string{
		"status":       "username_updated",
		"new_username": req.NewUsername,
	})
}

func handleReport(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		jsonError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	hostname, ok := payload["hostname"].(string)
	if !ok || hostname == "" {
		jsonError(w, "Missing hostname", http.StatusBadRequest)
		return
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		jsonError(w, "Failed to process data", http.StatusInternalServerError)
		return
	}

	_, err = db.Exec("INSERT INTO reports (hostname, data) VALUES (?, ?)", hostname, string(jsonData))
	if err != nil {
		log.Printf("❌ DB Write Error: %v", err)
		jsonError(w, "Database Error", http.StatusInternalServerError)
		return
	}

	// Get drive count for logging
	driveCount := 0
	if drives, ok := payload["drives"].([]interface{}); ok {
		driveCount = len(drives)
	}

	log.Printf("💾 Report: %s (%d drives)", hostname, driveCount)
	jsonResponse(w, map[string]string{"status": "ok"})
}

func handleHistory(w http.ResponseWriter, r *http.Request) {
	// Get all aliases for enriching drive data
	aliases := make(map[string]string)
	aliasRows, _ := db.Query("SELECT hostname, serial_number, alias FROM drive_aliases")
	if aliasRows != nil {
		defer aliasRows.Close()
		for aliasRows.Next() {
			var hostname, serial, alias string
			if aliasRows.Scan(&hostname, &serial, &alias) == nil {
				key := hostname + ":" + serial
				aliases[key] = alias
			}
		}
	}

	query := `
	SELECT hostname, timestamp, data 
	FROM reports 
	WHERE id IN (
		SELECT MAX(id) 
		FROM reports 
		GROUP BY hostname
	)
	ORDER BY timestamp DESC`

	rows, err := db.Query(query)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var history []map[string]interface{}
	for rows.Next() {
		var host, ts string
		var dataRaw []byte
		if err := rows.Scan(&host, &ts, &dataRaw); err != nil {
			continue
		}

		var dataMap map[string]interface{}
		json.Unmarshal(dataRaw, &dataMap)

		// Enrich drives with aliases
		if drives, ok := dataMap["drives"].([]interface{}); ok {
			for i, d := range drives {
				if drive, ok := d.(map[string]interface{}); ok {
					if serial, ok := drive["serial_number"].(string); ok {
						key := host + ":" + serial
						if alias, exists := aliases[key]; exists {
							drive["_alias"] = alias
							drives[i] = drive
						}
					}
				}
			}
			dataMap["drives"] = drives
		}

		history = append(history, map[string]interface{}{
			"hostname":  host,
			"timestamp": ts,
			"details":   dataMap,
		})
	}

	if history == nil {
		history = []map[string]interface{}{}
	}

	jsonResponse(w, history)
}

func handleHosts(w http.ResponseWriter, r *http.Request) {
	query := `
	SELECT hostname, MAX(timestamp) as last_seen, COUNT(*) as report_count
	FROM reports 
	GROUP BY hostname
	ORDER BY last_seen DESC`

	rows, err := db.Query(query)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var hosts []map[string]interface{}
	for rows.Next() {
		var hostname, lastSeen string
		var reportCount int
		if err := rows.Scan(&hostname, &lastSeen, &reportCount); err != nil {
			continue
		}

		hosts = append(hosts, map[string]interface{}{
			"hostname":     hostname,
			"last_seen":    lastSeen,
			"report_count": reportCount,
		})
	}

	if hosts == nil {
		hosts = []map[string]interface{}{}
	}

	jsonResponse(w, hosts)
}

func handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	if hostname == "" {
		jsonError(w, "Missing hostname", http.StatusBadRequest)
		return
	}

	// Sanitize hostname
	hostname = strings.TrimSpace(hostname)

	result, err := db.Exec("DELETE FROM reports WHERE hostname = ?", hostname)
	if err != nil {
		jsonError(w, "Database error", http.StatusInternalServerError)
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		jsonError(w, "Host not found", http.StatusNotFound)
		return
	}

	// Also delete aliases for this host
	db.Exec("DELETE FROM drive_aliases WHERE hostname = ?", hostname)

	log.Printf("🗑️  Deleted host: %s (%d reports)", hostname, affected)
	jsonResponse(w, map[string]interface{}{
		"status":  "deleted",
		"deleted": affected,
	})
}

func handleHostHistory(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	if hostname == "" {
		jsonError(w, "Missing hostname", http.StatusBadRequest)
		return
	}

	limit := 50
	query := `
	SELECT timestamp, data 
	FROM reports 
	WHERE hostname = ?
	ORDER BY timestamp DESC
	LIMIT ?`

	rows, err := db.Query(query, hostname, limit)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var history []map[string]interface{}
	for rows.Next() {
		var ts string
		var dataRaw []byte
		if err := rows.Scan(&ts, &dataRaw); err != nil {
			continue
		}

		var dataMap map[string]interface{}
		json.Unmarshal(dataRaw, &dataMap)

		history = append(history, map[string]interface{}{
			"timestamp": ts,
			"details":   dataMap,
		})
	}

	if history == nil {
		history = []map[string]interface{}{}
	}

	jsonResponse(w, history)
}

// Drive alias handlers
func handleGetAliases(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")

	var rows *sql.Rows
	var err error

	if hostname != "" {
		rows, err = db.Query(
			"SELECT id, hostname, serial_number, alias, created_at FROM drive_aliases WHERE hostname = ? ORDER BY alias",
			hostname,
		)
	} else {
		rows, err = db.Query(
			"SELECT id, hostname, serial_number, alias, created_at FROM drive_aliases ORDER BY hostname, alias",
		)
	}

	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var aliases []DriveAlias
	for rows.Next() {
		var a DriveAlias
		var createdAt string
		if err := rows.Scan(&a.ID, &a.Hostname, &a.SerialNumber, &a.Alias, &createdAt); err != nil {
			continue
		}
		a.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		aliases = append(aliases, a)
	}

	if aliases == nil {
		aliases = []DriveAlias{}
	}

	jsonResponse(w, aliases)
}

func handleSetAlias(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hostname     string `json:"hostname"`
		SerialNumber string `json:"serial_number"`
		Alias        string `json:"alias"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Hostname == "" || req.SerialNumber == "" {
		jsonError(w, "Missing hostname or serial_number", http.StatusBadRequest)
		return
	}

	req.Alias = strings.TrimSpace(req.Alias)

	// If alias is empty, delete the entry
	if req.Alias == "" {
		db.Exec("DELETE FROM drive_aliases WHERE hostname = ? AND serial_number = ?",
			req.Hostname, req.SerialNumber)
		jsonResponse(w, map[string]string{"status": "deleted"})
		return
	}

	// Upsert the alias
	_, err := db.Exec(`
		INSERT INTO drive_aliases (hostname, serial_number, alias) 
		VALUES (?, ?, ?)
		ON CONFLICT(hostname, serial_number) 
		DO UPDATE SET alias = excluded.alias
	`, req.Hostname, req.SerialNumber, req.Alias)

	if err != nil {
		jsonError(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("📝 Alias set: %s/%s -> %s", req.Hostname, req.SerialNumber, req.Alias)
	jsonResponse(w, map[string]string{"status": "ok"})
}

func handleDeleteAlias(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		jsonError(w, "Missing alias ID", http.StatusBadRequest)
		return
	}

	result, err := db.Exec("DELETE FROM drive_aliases WHERE id = ?", id)
	if err != nil {
		jsonError(w, "Database error", http.StatusInternalServerError)
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		jsonError(w, "Alias not found", http.StatusNotFound)
		return
	}

	jsonResponse(w, map[string]string{"status": "deleted"})
}
