package addons

import "time"

// Status represents the operational state of a registered add-on.
type Status string

const (
	StatusOnline   Status = "online"
	StatusDegraded Status = "degraded"
	StatusOffline  Status = "offline"
)

// Addon is a registered add-on with its manifest and runtime state.
type Addon struct {
	ID           int64           `json:"id"`
	Name         string          `json:"name"`
	Version      string          `json:"version"`
	Description  string          `json:"description,omitempty"`
	ManifestJSON string          `json:"manifest_json"`
	Status       Status          `json:"status"`
	Enabled      bool            `json:"enabled"`
	LastSeen     time.Time       `json:"last_seen"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}
