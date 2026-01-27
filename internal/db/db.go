package db

import (
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"vigil/internal/models"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// HashPassword creates a SHA256 hash of the password
func HashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// GenerateToken creates a secure random token
func GenerateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// Init initializes the database connection and schema
func Init(path string, config models.Config) error {
	var err error

	if err = ensureDirectory(path); err != nil {
		return err
	}

	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)", path)
	DB, err = sql.Open("sqlite", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database at %s: %w", path, err)
	}

	if err = DB.Ping(); err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	enableWAL()
	if err = createSchema(); err != nil {
		return err
	}
	migrateSchema()

	if config.AuthEnabled {
		createDefaultAdmin(config)
	}

	cleanupExpiredSessions()
	return nil
}

func ensureDirectory(path string) error {
	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create database directory %s: %w", dir, err)
		}
	}
	return nil
}

func enableWAL() {
	if _, err := DB.Exec("PRAGMA journal_mode=WAL"); err != nil {
		log.Printf("⚠️  Could not enable WAL mode: %v", err)
	}
}

func createSchema() error {
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

	if _, err := DB.Exec(schema); err != nil {
		return fmt.Errorf("failed to create tables: %w", err)
	}
	return nil
}

func migrateSchema() {
	// Add must_change_password column if it doesn't exist
	DB.Exec("ALTER TABLE users ADD COLUMN must_change_password INTEGER DEFAULT 0")
}

func createDefaultAdmin(config models.Config) {
	var count int
	DB.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count > 0 {
		return
	}

	password := config.AdminPass
	mustChange := 1

	if password == "" {
		password = GenerateToken()[:12]
		log.Printf("🔑 Generated admin password: %s", password)
		log.Printf("   Set ADMIN_PASS environment variable to use a custom password")
	} else {
		mustChange = 0
	}

	_, err := DB.Exec(
		"INSERT INTO users (username, password_hash, must_change_password) VALUES (?, ?, ?)",
		config.AdminUser, HashPassword(password), mustChange,
	)
	if err != nil {
		log.Printf("⚠️  Could not create admin user: %v", err)
	} else {
		log.Printf("✓ Created admin user: %s", config.AdminUser)
	}
}

func cleanupExpiredSessions() {
	DB.Exec("DELETE FROM sessions WHERE expires_at < datetime('now')")
}

// GetSession retrieves a session by token
func GetSession(token string) *models.Session {
	if token == "" {
		return nil
	}

	var session models.Session
	var expiresAt string

	err := DB.QueryRow(`
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

// CreateSession creates a new session for a user
func CreateSession(userID int) (string, time.Time, error) {
	token := GenerateToken()
	expiresAt := time.Now().Add(24 * time.Hour * 7)

	_, err := DB.Exec(
		"INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)",
		token, userID, expiresAt.Format("2006-01-02 15:04:05"),
	)
	return token, expiresAt, err
}

// DeleteSession removes a session
func DeleteSession(token string) {
	DB.Exec("DELETE FROM sessions WHERE token = ?", token)
}
