package agents

import (
	"database/sql"
	"fmt"
	"log"
)

// Migrate creates the agent authentication tables.
func Migrate(db *sql.DB) error {
	log.Println("🔐 Running migration: agent authentication schema")

	statements := []struct {
		label string
		sql   string
	}{
		{"agent_registry", `
			CREATE TABLE IF NOT EXISTS agent_registry (
				id             INTEGER PRIMARY KEY AUTOINCREMENT,
				hostname       TEXT    NOT NULL,
				name           TEXT,
				fingerprint    TEXT    NOT NULL UNIQUE,
				public_key     TEXT    NOT NULL,
				registered_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
				last_auth_at   DATETIME,
				last_seen_at   DATETIME,
				enabled        INTEGER DEFAULT 1
			);`},
		{"agent_registry indexes", `
			CREATE INDEX IF NOT EXISTS idx_agents_hostname    ON agent_registry(hostname);
			CREATE INDEX IF NOT EXISTS idx_agents_fingerprint ON agent_registry(fingerprint);
			CREATE INDEX IF NOT EXISTS idx_agents_enabled     ON agent_registry(enabled);`},

		{"agent_registration_tokens", `
			CREATE TABLE IF NOT EXISTS agent_registration_tokens (
				id               INTEGER PRIMARY KEY AUTOINCREMENT,
				token            TEXT    NOT NULL UNIQUE,
				name             TEXT,
				created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
				expires_at       DATETIME NOT NULL,
				used_at          DATETIME,
				used_by_agent_id INTEGER,
				FOREIGN KEY (used_by_agent_id) REFERENCES agent_registry(id) ON DELETE SET NULL
			);`},
		{"agent_registration_tokens indexes", `
			CREATE INDEX IF NOT EXISTS idx_reg_tokens_token   ON agent_registration_tokens(token);
			CREATE INDEX IF NOT EXISTS idx_reg_tokens_expires ON agent_registration_tokens(expires_at);`},

		{"agent_sessions", `
			CREATE TABLE IF NOT EXISTS agent_sessions (
				token      TEXT    PRIMARY KEY,
				agent_id   INTEGER NOT NULL,
				expires_at DATETIME NOT NULL,
				created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
				FOREIGN KEY (agent_id) REFERENCES agent_registry(id) ON DELETE CASCADE
			);`},
		{"agent_sessions indexes", `
			CREATE INDEX IF NOT EXISTS idx_agent_sessions_agent   ON agent_sessions(agent_id);
			CREATE INDEX IF NOT EXISTS idx_agent_sessions_expires ON agent_sessions(expires_at);`},
	}

	for _, s := range statements {
		if _, err := db.Exec(s.sql); err != nil {
			return fmt.Errorf("migration failed at [%s]: %w", s.label, err)
		}
		log.Printf("  ✓ %s", s.label)
	}

	log.Println("🔐 Migration completed: agent authentication tables ready")
	return nil
}
