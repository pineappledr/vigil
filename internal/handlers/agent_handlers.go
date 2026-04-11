package handlers

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"vigil/internal/agents"
	"vigil/internal/audit"
	"vigil/internal/auth"
	"vigil/internal/crypto"
	"vigil/internal/db"
	"vigil/internal/validate"
)

// ServerKeys is set from main.go after key initialisation.
var ServerKeys *crypto.ServerKeys

// ─── Public: server identity ──────────────────────────────────────────────────

// GetServerPublicKey returns the server's Ed25519 public key in base64.
// GET /api/v1/server/pubkey
func GetServerPublicKey(w http.ResponseWriter, r *http.Request) {
	if ServerKeys == nil {
		JSONError(w, "Server keys not initialised", http.StatusServiceUnavailable)
		return
	}
	JSONResponse(w, map[string]string{
		"public_key": ServerKeys.PublicKeyBase64(),
	})
}

// ─── Public: agent registration ───────────────────────────────────────────────

type registerRequest struct {
	Token       string `json:"token"`        // one-time registration token
	Hostname    string `json:"hostname"`     // agent hostname
	Name        string `json:"name"`         // optional friendly name
	Fingerprint string `json:"fingerprint"`  // machine fingerprint
	PublicKey   string `json:"public_key"`   // base64 Ed25519 public key
}

// RegisterAgent enrolls a new agent using a one-time token.
// POST /api/v1/agents/register
func RegisterAgent(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.Token == "" || req.Hostname == "" || req.Fingerprint == "" || req.PublicKey == "" {
		JSONError(w, "Missing required fields: token, hostname, fingerprint, public_key", http.StatusBadRequest)
		return
	}

	// Validate public key is valid base64 with correct Ed25519 size (32 bytes)
	pubBytes, err := base64.StdEncoding.DecodeString(req.PublicKey)
	if err != nil || len(pubBytes) != 32 {
		JSONError(w, "Invalid public_key: must be base64-encoded 32-byte Ed25519 key", http.StatusBadRequest)
		return
	}

	// Idempotent reconnect: check by fingerprint first, then by public key.
	// The public-key fallback handles fingerprint evolution (e.g., mac: → mid:
	// when a container is recreated and /etc/machine-id becomes available).
	existing, _ := agents.GetAgentByFingerprint(db.DB, req.Fingerprint)
	if existing != nil && existing.PublicKey != req.PublicKey {
		JSONError(w, "An agent with this fingerprint is already registered with a different key", http.StatusConflict)
		return
	}
	if existing == nil {
		// Fingerprint changed but same key pair — find by public key
		existing, _ = agents.GetAgentByPublicKey(db.DB, req.PublicKey)
	}
	if existing != nil {
		// Update fingerprint if it changed
		if existing.Fingerprint != req.Fingerprint {
			log.Printf("🔄 Fingerprint updated for agent %d (%s): %.16s... → %.16s...",
				existing.ID, existing.Hostname, existing.Fingerprint, req.Fingerprint)
			agents.UpdateAgentFingerprint(db.DB, existing.ID, req.Fingerprint)
		}

		// Same agent reconnecting — issue a new session without consuming a token
		session, sessErr := agents.CreateAgentSession(db.DB, existing.ID)
		if sessErr != nil {
			log.Printf("❌ Failed to create session for reconnecting agent %d: %v", existing.ID, sessErr)
			JSONError(w, "Failed to create session", http.StatusInternalServerError)
			return
		}
		agents.UpdateAgentLastAuth(db.DB, existing.ID)

		serverPubKey := ""
		if ServerKeys != nil {
			serverPubKey = ServerKeys.PublicKeyBase64()
		}

		log.Printf("🔄 Agent reconnected: %s (id=%d, fingerprint=%.16s...)", existing.Hostname, existing.ID, req.Fingerprint)

		JSONResponse(w, map[string]interface{}{
			"agent_id":          existing.ID,
			"hostname":          existing.Hostname,
			"session_token":     session.Token,
			"session_expires":   session.ExpiresAt.UTC().Format(time.RFC3339),
			"server_public_key": serverPubKey,
		})
		return
	}

	// New agent — validate registration token (not used, not expired)
	tok, err := agents.GetRegistrationToken(db.DB, req.Token)
	if err != nil {
		JSONError(w, "Database error", http.StatusInternalServerError)
		return
	}
	if tok == nil {
		JSONError(w, "Invalid or expired registration token", http.StatusUnauthorized)
		return
	}
	if tok.UsedAt != nil {
		JSONError(w, "Registration token already used", http.StatusUnauthorized)
		return
	}

	name := req.Name
	if name == "" {
		name = req.Hostname
	}

	// Create agent record
	agent, err := agents.RegisterAgent(db.DB, req.Hostname, name, req.Fingerprint, req.PublicKey)
	if err != nil {
		log.Printf("❌ Failed to register agent %s: %v", req.Hostname, err)
		JSONError(w, "Failed to register agent", http.StatusInternalServerError)
		return
	}

	// Consume the token
	if err := agents.ConsumeRegistrationToken(db.DB, req.Token, agent.ID); err != nil {
		log.Printf("⚠️  Could not mark registration token used: %v", err)
	}

	// Issue first session token and stamp last_auth_at
	session, err := agents.CreateAgentSession(db.DB, agent.ID)
	if err != nil {
		log.Printf("❌ Failed to create session for agent %d: %v", agent.ID, err)
		JSONError(w, "Failed to create session", http.StatusInternalServerError)
		return
	}
	agents.UpdateAgentLastAuth(db.DB, agent.ID)

	serverPubKey := ""
	if ServerKeys != nil {
		serverPubKey = ServerKeys.PublicKeyBase64()
	}

	log.Printf("✅ Agent registered: %s (id=%d, fingerprint=%.16s...)", agent.Hostname, agent.ID, agent.Fingerprint)

	JSONResponse(w, map[string]interface{}{
		"agent_id":         agent.ID,
		"hostname":         agent.Hostname,
		"session_token":    session.Token,
		"session_expires":  session.ExpiresAt.UTC().Format(time.RFC3339),
		"server_public_key": serverPubKey,
	})
}

// ─── Public: agent authentication ─────────────────────────────────────────────

type authRequest struct {
	AgentID     int64  `json:"agent_id"`
	Fingerprint string `json:"fingerprint"`
	Timestamp   int64  `json:"timestamp"`  // Unix seconds
	Signature   string `json:"signature"`  // base64(Ed25519Sign(private, "{agent_id}:{fingerprint}:{timestamp}"))
}

// AuthAgent authenticates an existing agent via Ed25519 signature and issues
// a new 1-hour session token.
// POST /api/v1/agents/auth
func AuthAgent(w http.ResponseWriter, r *http.Request) {
	var req authRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if req.AgentID == 0 || req.Fingerprint == "" || req.Timestamp == 0 || req.Signature == "" {
		JSONError(w, "Missing required fields: agent_id, fingerprint, timestamp, signature", http.StatusBadRequest)
		return
	}

	// Reject stale or future timestamps (±5 minutes)
	ts := time.Unix(req.Timestamp, 0)
	delta := time.Since(ts)
	if delta < -5*time.Minute || delta > 5*time.Minute {
		JSONError(w, "Timestamp out of acceptable range (±5 minutes)", http.StatusUnauthorized)
		return
	}

	// Look up agent
	agent, err := agents.GetAgentByID(db.DB, req.AgentID)
	if err != nil || agent == nil {
		JSONError(w, "Agent not found", http.StatusUnauthorized)
		return
	}
	if !agent.Enabled {
		JSONError(w, "Agent is disabled", http.StatusForbidden)
		return
	}

	// Verify Ed25519 signature over "{agent_id}:{fingerprint}:{timestamp}"
	// Signature is verified BEFORE fingerprint check — the private key IS
	// the true identity proof.  A valid signature with a changed fingerprint
	// just means the identity source rotated (e.g., container recreated).
	msg := []byte(fmt.Sprintf("%d:%s:%d", req.AgentID, req.Fingerprint, req.Timestamp))
	sigBytes, err := base64.StdEncoding.DecodeString(req.Signature)
	if err != nil || !crypto.VerifyAgentSignature(agent.PublicKey, msg, sigBytes) {
		JSONError(w, "Invalid signature", http.StatusUnauthorized)
		return
	}

	// Fingerprint evolution: if the signature is valid but the fingerprint
	// source changed (e.g., mac: → mid:), update the stored value.
	if agent.Fingerprint != req.Fingerprint {
		log.Printf("🔄 Fingerprint updated for agent %d (%s): %.16s... → %.16s...",
			agent.ID, agent.Hostname, agent.Fingerprint, req.Fingerprint)
		if err := agents.UpdateAgentFingerprint(db.DB, agent.ID, req.Fingerprint); err != nil {
			log.Printf("⚠️  Failed to update fingerprint for agent %d: %v", agent.ID, err)
		}
	}

	// Issue new session
	session, err := agents.CreateAgentSession(db.DB, agent.ID)
	if err != nil {
		log.Printf("❌ Failed to create session for agent %d: %v", agent.ID, err)
		JSONError(w, "Failed to create session", http.StatusInternalServerError)
		return
	}

	agents.UpdateAgentLastAuth(db.DB, agent.ID)

	JSONResponse(w, map[string]interface{}{
		"session_token":   session.Token,
		"session_expires": session.ExpiresAt.UTC().Format(time.RFC3339),
	})
}

// ─── Agent session middleware ─────────────────────────────────────────────────

// AgentSessionKey is the context key for the authenticated agent session.
type agentCtxKey struct{}

// GetAgentSessionFromRequest extracts and validates the agent bearer token.
// Returns nil if no valid session is present.
func GetAgentSessionFromRequest(r *http.Request) *agents.AgentSession {
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")
	session, _ := agents.GetAgentSession(db.DB, token)
	return session
}

// ─── Admin: agent management ──────────────────────────────────────────────────

// ListAgents returns all registered agents.
// GET /api/v1/agents
func ListAgents(w http.ResponseWriter, r *http.Request) {
	agentList, err := agents.ListAgents(db.DB)
	if err != nil {
		JSONError(w, "Failed to list agents: "+err.Error(), http.StatusInternalServerError)
		return
	}
	JSONResponse(w, map[string]interface{}{
		"agents": agentList,
		"count":  len(agentList),
	})
}

// DeleteRegisteredAgent removes an agent by ID and cascades to all
// hostname-keyed data (reports, ZFS pools, aliases, SMART, wearout).
// DELETE /api/v1/agents/{id}
func DeleteRegisteredAgent(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		JSONError(w, "Invalid agent ID", http.StatusBadRequest)
		return
	}

	// Look up agent to get hostname before deletion
	agent, err := agents.GetAgentByID(db.DB, id)
	if err != nil || agent == nil {
		JSONError(w, "Agent not found", http.StatusNotFound)
		return
	}
	hostname := agent.Hostname

	// Delete agent record (sessions cascade via FK)
	if err := agents.DeleteAgent(db.DB, id); err != nil {
		JSONError(w, "Failed to delete agent: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Cascade: remove all hostname-keyed data
	deleted := agents.DeleteHostData(db.DB, hostname)

	log.Printf("🗑️  Deleted agent id=%d (%s) — cascade: %v", id, hostname, deleted)
	if s := auth.GetSessionFromContext(r); s != nil {
		audit.LogEvent(db.DB, r, s.UserID, s.Username, "agent_delete", "agent", idStr, hostname, "success")
	}
	JSONResponse(w, map[string]interface{}{
		"status":  "deleted",
		"cascade": deleted,
	})
}

// ─── Admin: registration token management ─────────────────────────────────────

type createTokenRequest struct {
	Name      string `json:"name"`
	ExpiresIn *int   `json:"expires_in"` // seconds; null or 0 = never expires
}

// CreateToken generates a new one-time enrollment token.
// POST /api/v1/tokens
// Body: {"name":"...", "expires_in": 86400}  — expires_in in seconds, null/0 for never
func CreateToken(w http.ResponseWriter, r *http.Request) {
	var req createTokenRequest
	json.NewDecoder(r.Body).Decode(&req)

	if req.Name != "" {
		if err := validate.Name(req.Name, 128); err != nil {
			JSONError(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	var expiresIn *time.Duration
	if req.ExpiresIn != nil && *req.ExpiresIn > 0 {
		d := time.Duration(*req.ExpiresIn) * time.Second
		expiresIn = &d
	}

	tok, err := agents.CreateRegistrationToken(db.DB, req.Name, expiresIn)
	if err != nil {
		JSONError(w, "Failed to create token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("🔑 Registration token created: %.16s... (name=%q)", tok.Token, tok.Name)
	if s := auth.GetSessionFromContext(r); s != nil {
		audit.LogEvent(db.DB, r, s.UserID, s.Username, "token_create", "registration_token", "", tok.Name, "success")
	}
	JSONResponse(w, tok)
}

// ListTokens returns all registration tokens.
// GET /api/v1/tokens
func ListTokens(w http.ResponseWriter, r *http.Request) {
	toks, err := agents.ListRegistrationTokens(db.DB)
	if err != nil {
		JSONError(w, "Failed to list tokens: "+err.Error(), http.StatusInternalServerError)
		return
	}
	JSONResponse(w, map[string]interface{}{
		"tokens": toks,
		"count":  len(toks),
	})
}

// DeleteToken removes a registration token by ID.
// DELETE /api/v1/tokens/{id}
func DeleteToken(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		JSONError(w, "Invalid token ID", http.StatusBadRequest)
		return
	}

	if err := agents.DeleteRegistrationToken(db.DB, id); err != nil {
		JSONError(w, "Failed to delete token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if s := auth.GetSessionFromContext(r); s != nil {
		audit.LogEvent(db.DB, r, s.UserID, s.Username, "token_delete", "registration_token", idStr, "", "success")
	}
	JSONResponse(w, map[string]string{"status": "deleted"})
}

// ─── LED Identify proxy ─────────────────────────────────────────────────────

// IdentifyDrive proxies a LED identify request to the agent's command server.
// POST /api/v1/agents/{hostname}/identify
func IdentifyDrive(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	if hostname == "" {
		JSONError(w, "Missing hostname", http.StatusBadRequest)
		return
	}

	listenAddr, _, err := agents.GetAgentByHostname(db.DB, hostname)
	if err != nil {
		JSONError(w, "Agent not found", http.StatusNotFound)
		return
	}
	if listenAddr == "" {
		JSONError(w, "Agent does not have a command server enabled", http.StatusNotImplemented)
		return
	}

	// Read and forward the request body to the agent
	body, err := io.ReadAll(r.Body)
	if err != nil {
		JSONError(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	agentURL := fmt.Sprintf("http://%s/api/identify", listenAddr)
	resp, err := http.Post(agentURL, "application/json", bytes.NewReader(body)) // #nosec G107 -- URL built from trusted DB value
	if err != nil {
		log.Printf("⚠️  LED identify proxy to %s failed: %v", listenAddr, err)
		JSONError(w, "Failed to reach agent command server", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Forward the agent's response back to the caller
	respBody, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(respBody) //nolint:errcheck
}

