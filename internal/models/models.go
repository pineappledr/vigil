package models

import "time"

// DriveReport represents a report from an agent
type DriveReport struct {
	Hostname  string                   `json:"hostname"`
	Timestamp time.Time                `json:"timestamp"`
	Version   string                   `json:"agent_version"`
	Drives    []map[string]interface{} `json:"drives"`
}

// ScanResult represents smartctl scan output
type ScanResult struct {
	Devices []Device `json:"devices"`
}

// Device represents a detected device
type Device struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Protocol string `json:"protocol"`
}

// User represents an authenticated user
type User struct {
	ID           int       `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

// Session represents an active user session
type Session struct {
	Token     string    `json:"token"`
	UserID    int       `json:"user_id"`
	Username  string    `json:"username"`
	ExpiresAt time.Time `json:"expires_at"`
}

// DriveAlias represents a custom name for a drive
type DriveAlias struct {
	ID           int       `json:"id"`
	Hostname     string    `json:"hostname"`
	SerialNumber string    `json:"serial_number"`
	Alias        string    `json:"alias"`
	CreatedAt    time.Time `json:"created_at"`
}

// Config holds server configuration
type Config struct {
	Port        string
	DBPath      string
	AdminUser   string
	AdminPass   string
	AuthEnabled bool
}
