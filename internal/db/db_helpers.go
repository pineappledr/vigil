package db

import (
	"database/sql"
	"time"
)

// ─── Time Helpers ────────────────────────────────────────────────────────────

const timeFormat = "2006-01-02 15:04:05"

// ParseNullTime parses a nullable time string from SQLite
func ParseNullTime(ns sql.NullString) time.Time {
	if !ns.Valid || ns.String == "" {
		return time.Time{}
	}
	t, _ := time.Parse(timeFormat, ns.String)
	return t
}

// NullTimeString converts a time to a nullable string for SQLite storage
func NullTimeString(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t.Format(timeFormat)
}

// TimeString converts a time to string, using current time if zero
func TimeString(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.Format(timeFormat)
}

// NowString returns current time as formatted string
func NowString() string {
	return time.Now().Format(timeFormat)
}

// ─── Type Conversion Helpers ─────────────────────────────────────────────────

// BoolToInt converts a bool to int for SQLite storage
func BoolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// IntToBool converts an int to bool from SQLite storage
func IntToBool(i int) bool {
	return i == 1
}

// ─── Query Helpers ───────────────────────────────────────────────────────────

// ExistsQuery checks if a record exists
func ExistsQuery(query string, args ...interface{}) (bool, error) {
	var exists int
	err := DB.QueryRow(query, args...).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}

// GetID retrieves an ID by query, returns 0 if not found
func GetID(query string, args ...interface{}) (int64, error) {
	var id int64
	err := DB.QueryRow(query, args...).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return id, err
}
