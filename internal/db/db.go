package db

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"
)

var DB *sql.DB

// Init initializes the database connection and schema
func Init(path string) error {
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
