package agents

import "time"

// Agent represents a registered monitoring agent.
type Agent struct {
	ID           int64      `json:"id"`
	Hostname     string     `json:"hostname"`
	Name         string     `json:"name"`
	Fingerprint  string     `json:"fingerprint"`
	PublicKey    string     `json:"public_key"`
	RegisteredAt time.Time  `json:"registered_at"`
	LastAuthAt   *time.Time `json:"last_auth_at,omitempty"`
	LastSeenAt   *time.Time `json:"last_seen_at,omitempty"`
	Enabled      bool       `json:"enabled"`
}

// RegistrationToken is a one-time-use token for enrolling a new agent.
// ExpiresAt is nil for tokens that never expire.
type RegistrationToken struct {
	ID            int64      `json:"id"`
	Token         string     `json:"token"`
	Name          string     `json:"name"`
	CreatedAt     time.Time  `json:"created_at"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
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

// parseDBTime parses a timestamp read from SQLite, tolerating both formats that
// show up in this DB: the bare `2006-01-02 15:04:05` written by Go/CURRENT_TIMESTAMP,
// and RFC3339 (`2006-01-02T15:04:05Z`) which the driver/some write paths produce.
// Parsing with only the bare layout silently failed on RFC3339 values and left the
// zero value — that's the "Registered never / Not Reporting" bug on the Agents page.
// Returns zero time only if neither layout matches.
func parseDBTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, timeFormat, "2006-01-02 15:04:05.999999999-07:00"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
