package agents

import "time"

// Agent represents a registered monitoring agent.
type Agent struct {
	ID           int64     `json:"id"`
	Hostname     string    `json:"hostname"`
	Name         string    `json:"name"`
	Fingerprint  string    `json:"fingerprint"`
	PublicKey    string    `json:"public_key"`
	RegisteredAt time.Time `json:"registered_at"`
	LastAuthAt   time.Time `json:"last_auth_at,omitempty"`
	LastSeenAt   time.Time `json:"last_seen_at,omitempty"`
	Enabled      bool      `json:"enabled"`
}

// RegistrationToken is a one-time-use token for enrolling a new agent.
// Tokens are valid for 24 hours and become unusable after first consumption.
type RegistrationToken struct {
	ID            int64      `json:"id"`
	Token         string     `json:"token"`
	Name          string     `json:"name"`
	CreatedAt     time.Time  `json:"created_at"`
	ExpiresAt     time.Time  `json:"expires_at"`
	UsedAt        *time.Time `json:"used_at,omitempty"`
	UsedByAgentID *int64     `json:"used_by_agent_id,omitempty"`
}

// AgentSession is an active authenticated session for an agent (1-hour TTL).
type AgentSession struct {
	Token     string    `json:"token"`
	AgentID   int64     `json:"agent_id"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

const timeFormat = "2006-01-02 15:04:05"
