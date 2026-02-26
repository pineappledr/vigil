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

	// Migration: make expires_at nullable (was NOT NULL in initial schema).
	// SQLite doesn't support ALTER COLUMN, so we recreate the table if needed.
	if err := migrateTokensNullableExpiry(db); err != nil {
		return fmt.Errorf("migration failed at [tokens nullable expiry]: %w", err)
	}

	log.Println("🔐 Migration completed: agent authentication tables ready")
	return nil
}

// migrateTokensNullableExpiry recreates the tokens table with expires_at
// nullable, preserving all existing data. It's a no-op if already migrated.
func migrateTokensNullableExpiry(db *sql.DB) error {
	// Check if expires_at is already nullable by inserting NULL and rolling back
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	_, testErr := tx.Exec(`INSERT INTO agent_registration_tokens (token, name, expires_at) VALUES ('__test_nullable__', '', NULL)`)
	tx.Rollback()
	if testErr == nil {
		// Column already nullable — clean up test row just in case
		db.Exec(`DELETE FROM agent_registration_tokens WHERE token = '__test_nullable__'`)
		return nil
	}

	log.Println("  ↻ Migrating agent_registration_tokens: making expires_at nullable")

	tx, err = db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmts := []string{
		`CREATE TABLE agent_registration_tokens_new (
			id               INTEGER PRIMARY KEY AUTOINCREMENT,
			token            TEXT    NOT NULL UNIQUE,
			name             TEXT,
			created_at       DATETIME DEFAULT CURRENT_TIMESTAMP,
			expires_at       DATETIME,
			used_at          DATETIME,
			used_by_agent_id INTEGER,
			FOREIGN KEY (used_by_agent_id) REFERENCES agent_registry(id) ON DELETE SET NULL
		)`,
		`INSERT INTO agent_registration_tokens_new SELECT * FROM agent_registration_tokens`,
		`DROP TABLE agent_registration_tokens`,
		`ALTER TABLE agent_registration_tokens_new RENAME TO agent_registration_tokens`,
		`CREATE INDEX IF NOT EXISTS idx_reg_tokens_token   ON agent_registration_tokens(token)`,
		`CREATE INDEX IF NOT EXISTS idx_reg_tokens_expires ON agent_registration_tokens(expires_at)`,
	}
	for _, s := range stmts {
		if _, err := tx.Exec(s); err != nil {
			return fmt.Errorf("nullable expiry migration: %w", err)
		}
	}

	log.Println("  ✓ agent_registration_tokens: expires_at now nullable")
	return tx.Commit()
}
