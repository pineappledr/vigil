package zfs

import (
	"database/sql"
	"time"
)

const timeFormat = "2006-01-02 15:04:05"

// parseNullTime parses a nullable timestamp read from SQLite, tolerating both the
// bare `2006-01-02 15:04:05` layout and RFC3339 (`2006-01-02T15:04:05Z`). Parsing
// with only the bare layout silently failed on RFC3339 values and left the zero
// value (the `0001-01-01T00:00:00Z` bug, same class fixed for agents in #36).
func parseNullTime(ns sql.NullString) time.Time {
	if !ns.Valid || ns.String == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, timeFormat, "2006-01-02 15:04:05.999999999-07:00"} {
		if t, err := time.Parse(layout, ns.String); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func nullTimeString(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t.Format(timeFormat)
}

func nowString() string {
	return time.Now().Format(timeFormat)
}

func getID(db *sql.DB, query string, args ...interface{}) (int64, error) {
	var id int64
	err := db.QueryRow(query, args...).Scan(&id)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	return id, err
}

func existsQuery(db *sql.DB, query string, args ...interface{}) (bool, error) {
	var exists int
	err := db.QueryRow(query, args...).Scan(&exists)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return exists > 0, nil
}
