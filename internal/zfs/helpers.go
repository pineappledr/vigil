package zfs

import (
	"database/sql"
	"time"
)

const timeFormat = "2006-01-02 15:04:05"

func parseNullTime(ns sql.NullString) time.Time {
	if !ns.Valid || ns.String == "" {
		return time.Time{}
	}
	t, _ := time.Parse(timeFormat, ns.String)
	return t
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
