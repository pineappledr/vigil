package main

import (
	"context"
	"database/sql"
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

const version = "1.0.0"

var db *sql.DB

type Config struct {
	Port   string
	DBPath string
}

func loadConfig() Config {
	return Config{
		Port:   getEnv("PORT", "9080"),
		DBPath: getEnv("DB_PATH", "vigil.db"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func initDB(path string) error {
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

	query := `
	CREATE TABLE IF NOT EXISTS reports (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hostname TEXT NOT NULL,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		data JSON NOT NULL
	);
	CREATE INDEX IF NOT EXISTS idx_reports_hostname ON reports(hostname);
	CREATE INDEX IF NOT EXISTS idx_reports_timestamp ON reports(timestamp);
	`

	if _, err := db.Exec(query); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}

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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

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

func main() {
	log.SetFlags(log.Ltime | log.Ldate)
	log.Printf("🚀 Vigil Server v%s starting...", version)

	config := loadConfig()

	if err := initDB(config.DBPath); err != nil {
		log.Fatalf("❌ Database error: %v", err)
	}
	defer db.Close()
	log.Printf("✓ Database: %s", config.DBPath)

	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /health", handleHealth)
	mux.HandleFunc("GET /api/version", handleVersion)

	// Agent reporting endpoint
	mux.HandleFunc("POST /api/report", handleReport)

	// Data endpoints
	mux.HandleFunc("GET /api/history", handleHistory)
	mux.HandleFunc("GET /api/hosts", handleHosts)
	mux.HandleFunc("DELETE /api/hosts/{hostname}", handleDeleteHost)
	mux.HandleFunc("GET /api/hosts/{hostname}/history", handleHostHistory)

	// Static file server
	fs := http.FileServer(http.Dir("./web"))
	mux.Handle("/", fs)

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

func handleHealth(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]string{
		"status":  "healthy",
		"version": version,
	})
}

func handleVersion(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, map[string]string{"version": version})
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
