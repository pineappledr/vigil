package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	agentcrypto "vigil/cmd/agent/crypto"
)

const authStateFile = "auth.json"

// authState is persisted to dataDir/auth.json after successful registration
// and updated on every re-authentication.
type authState struct {
	AgentID       int64     `json:"agent_id"`
	ServerURL     string    `json:"server_url"`
	ServerPubKey  string    `json:"server_public_key"`
	SessionToken  string    `json:"session_token"`
	SessionExpires time.Time `json:"session_expires"`
}

// loadAuthState reads the persisted auth state. Returns nil if not yet registered.
func loadAuthState(dataDir string) *authState {
	data, err := os.ReadFile(filepath.Join(dataDir, authStateFile))
	if err != nil {
		return nil
	}
	var s authState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil
	}
	return &s
}

func saveAuthState(dataDir string, s *authState) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dataDir, authStateFile), data, 0o600)
}

// registerAgent performs the one-time enrollment handshake.
// token is the admin-issued registration token.
func registerAgent(
	serverURL, token, hostname, fingerprint string,
	keys *agentcrypto.AgentKeys,
	dataDir string,
) (*authState, error) {
	body := map[string]string{
		"token":       token,
		"hostname":    hostname,
		"fingerprint": fingerprint,
		"public_key":  keys.PublicKeyBase64(),
	}

	payload, _ := json.Marshal(body)
	resp, err := http.Post(serverURL+"/api/v1/agents/register", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("registration request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("registration failed (HTTP %d): %s", resp.StatusCode, errResp["error"])
	}

	var result struct {
		AgentID        int64  `json:"agent_id"`
		SessionToken   string `json:"session_token"`
		SessionExpires string `json:"session_expires"`
		ServerPubKey   string `json:"server_public_key"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode registration response: %w", err)
	}

	expires, _ := time.Parse(time.RFC3339, result.SessionExpires)

	state := &authState{
		AgentID:        result.AgentID,
		ServerURL:      serverURL,
		ServerPubKey:   result.ServerPubKey,
		SessionToken:   result.SessionToken,
		SessionExpires: expires,
	}

	if err := saveAuthState(dataDir, state); err != nil {
		return nil, fmt.Errorf("save auth state: %w", err)
	}

	return state, nil
}

// authenticate performs the Ed25519 challenge and obtains a new session token.
func authenticate(
	state *authState,
	fingerprint string,
	keys *agentcrypto.AgentKeys,
	dataDir string,
) (*authState, error) {
	ts := time.Now().Unix()
	msg := []byte(fmt.Sprintf("%d:%s:%d", state.AgentID, fingerprint, ts))
	sig := keys.Sign(msg)

	body := map[string]interface{}{
		"agent_id":    state.AgentID,
		"fingerprint": fingerprint,
		"timestamp":   ts,
		"signature":   sig,
	}

	payload, _ := json.Marshal(body)
	resp, err := http.Post(state.ServerURL+"/api/v1/agents/auth", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return nil, fmt.Errorf("auth request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("fingerprint mismatch — re-registration required (run with --register --token <new-token>)")
	}
	if resp.StatusCode != http.StatusOK {
		var errResp map[string]string
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("auth failed (HTTP %d): %s", resp.StatusCode, errResp["error"])
	}

	var result struct {
		SessionToken   string `json:"session_token"`
		SessionExpires string `json:"session_expires"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode auth response: %w", err)
	}

	expires, _ := time.Parse(time.RFC3339, result.SessionExpires)
	state.SessionToken = result.SessionToken
	state.SessionExpires = expires

	if err := saveAuthState(dataDir, state); err != nil {
		return nil, fmt.Errorf("save auth state: %w", err)
	}

	return state, nil
}

// sessionNeedsRefresh reports true if the session expires within 5 minutes.
func sessionNeedsRefresh(state *authState) bool {
	return time.Until(state.SessionExpires) < 5*time.Minute
}
