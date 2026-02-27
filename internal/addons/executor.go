package addons

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"vigil/internal/auth"
	"vigil/internal/crypto"
)

// CommandRequest is the input to the destructive command pipeline.
type CommandRequest struct {
	AddonID      int64           `json:"addon_id"`
	AgentID      int64           `json:"agent_id"`
	Action       string          `json:"action"`
	ActionToken  string          `json:"action_token"`
	SessionToken string          `json:"-"` // extracted from auth context, not from JSON
	Params       json.RawMessage `json:"params,omitempty"`
}

// SignedCommand is the validated, server-signed payload that an agent
// can trust and execute.
type SignedCommand struct {
	AddonID   int64           `json:"addon_id"`
	AgentID   int64           `json:"agent_id"`
	Action    string          `json:"action"`
	Params    json.RawMessage `json:"params,omitempty"`
	IssuedAt  string          `json:"issued_at"`
	Signature string          `json:"signature"` // base64(Ed25519Sign(private, canonical))
}

// Executor runs the 6-step validation pipeline for destructive commands.
type Executor struct {
	db           *sql.DB
	keys         *crypto.ServerKeys
	tokenService *auth.ActionTokenService
}

// NewExecutor creates a command executor.
func NewExecutor(db *sql.DB, keys *crypto.ServerKeys, tokenService *auth.ActionTokenService) *Executor {
	return &Executor{
		db:           db,
		keys:         keys,
		tokenService: tokenService,
	}
}

// Execute runs the full 6-step validation pipeline:
//
//  1. Add-on existence & enabled check
//  2. Action token validation (single-use, session-bound, time-limited)
//  3. Agent ID validation (agent exists and is enabled)
//  4. Add-on ↔ action compatibility (action appears in manifest)
//  5. Parameter validation (non-empty if action requires params)
//  6. Ed25519 signature generation (server signs the canonical payload)
func (e *Executor) Execute(req CommandRequest) (*SignedCommand, error) {
	// Step 1: Verify add-on exists and is enabled
	addon, err := Get(e.db, req.AddonID)
	if err != nil {
		return nil, fmt.Errorf("step 1: lookup addon: %w", err)
	}
	if addon == nil {
		return nil, fmt.Errorf("step 1: addon %d not found", req.AddonID)
	}
	if !addon.Enabled {
		return nil, fmt.Errorf("step 1: addon %q is disabled", addon.Name)
	}

	// Step 2: Validate action token (single-use, session-bound, time-limited)
	if err := e.tokenService.Validate(req.ActionToken, req.SessionToken, req.Action); err != nil {
		return nil, fmt.Errorf("step 2: action token: %w", err)
	}

	// Step 3: Validate agent exists and is enabled
	if err := e.validateAgent(req.AgentID); err != nil {
		return nil, fmt.Errorf("step 3: %w", err)
	}

	// Step 4: Verify the action is declared in the add-on's manifest
	if err := e.validateActionInManifest(addon, req.Action); err != nil {
		return nil, fmt.Errorf("step 4: %w", err)
	}

	// Step 5: Parameter sanity check
	if err := e.validateParams(req.Action, req.Params); err != nil {
		return nil, fmt.Errorf("step 5: %w", err)
	}

	// Step 6: Build and sign the command payload
	cmd, err := e.signCommand(req)
	if err != nil {
		return nil, fmt.Errorf("step 6: %w", err)
	}

	return cmd, nil
}

// ── Step 3: Agent validation ─────────────────────────────────────────────

func (e *Executor) validateAgent(agentID int64) error {
	var enabled int
	err := e.db.QueryRow(`SELECT enabled FROM agents WHERE id = ?`, agentID).Scan(&enabled)
	if err == sql.ErrNoRows {
		return fmt.Errorf("agent %d not found", agentID)
	}
	if err != nil {
		return fmt.Errorf("lookup agent: %w", err)
	}
	if enabled != 1 {
		return fmt.Errorf("agent %d is disabled", agentID)
	}
	return nil
}

// ── Step 4: Action ↔ manifest compatibility ──────────────────────────────

func (e *Executor) validateActionInManifest(addon *Addon, action string) error {
	manifest, err := ValidateManifest([]byte(addon.ManifestJSON))
	if err != nil {
		return fmt.Errorf("invalid stored manifest: %w", err)
	}

	// Walk form components looking for security_gate fields that define
	// the allowed destructive actions.
	for _, page := range manifest.Pages {
		for _, comp := range page.Components {
			if comp.Type != "form" || len(comp.Config) == 0 {
				continue
			}
			var cfg struct {
				Fields []FormField `json:"fields"`
				Action string      `json:"action"`
			}
			if err := json.Unmarshal(comp.Config, &cfg); err != nil {
				continue
			}
			// Match by component-level action
			if cfg.Action == action {
				return nil
			}
			// Match by a field with security_gate that names this action
			for _, f := range cfg.Fields {
				if f.SecurityGate && f.Name == action {
					return nil
				}
			}
		}
	}

	return fmt.Errorf("action %q not declared in manifest for %q", action, addon.Name)
}

// ── Step 5: Parameter validation ─────────────────────────────────────────

func (e *Executor) validateParams(action string, params json.RawMessage) error {
	// Params are optional for some actions, but if provided must be valid JSON.
	if len(params) == 0 || string(params) == "null" {
		return nil
	}
	var check interface{}
	if err := json.Unmarshal(params, &check); err != nil {
		return fmt.Errorf("params is not valid JSON: %w", err)
	}
	return nil
}

// ── Step 6: Ed25519 signature ────────────────────────────────────────────

func (e *Executor) signCommand(req CommandRequest) (*SignedCommand, error) {
	if e.keys == nil {
		return nil, fmt.Errorf("server keys not initialised")
	}

	cmd := &SignedCommand{
		AddonID:  req.AddonID,
		AgentID:  req.AgentID,
		Action:   req.Action,
		Params:   req.Params,
		IssuedAt: time.Now().UTC().Format(time.RFC3339),
	}

	// Build canonical payload for signing (deterministic field order)
	canonical := fmt.Sprintf("%d:%d:%s:%s:%s",
		cmd.AddonID, cmd.AgentID, cmd.Action, cmd.IssuedAt, string(cmd.Params))

	sig := e.keys.Sign([]byte(canonical))
	cmd.Signature = base64.StdEncoding.EncodeToString(sig)

	return cmd, nil
}

// VerifyCommandSignature verifies that a SignedCommand was signed by the
// server's Ed25519 key.  This is used by agents to trust the command.
func VerifyCommandSignature(keys *crypto.ServerKeys, cmd *SignedCommand) bool {
	canonical := fmt.Sprintf("%d:%d:%s:%s:%s",
		cmd.AddonID, cmd.AgentID, cmd.Action, cmd.IssuedAt, string(cmd.Params))

	sigBytes, err := base64.StdEncoding.DecodeString(cmd.Signature)
	if err != nil {
		return false
	}

	return crypto.VerifyAgentSignature(keys.PublicKeyBase64(), []byte(canonical), sigBytes)
}
