// Package crypto manages the agent's Ed25519 key pair.
package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const (
	privateKeyFile = "agent.key"
	keyPEMType     = "VIGIL AGENT PRIVATE KEY"
)

// AgentKeys holds the agent's Ed25519 key pair.
type AgentKeys struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

// LoadOrGenerate loads existing agent keys from dataDir, or generates and saves
// a new key pair if none exist.
func LoadOrGenerate(dataDir string) (*AgentKeys, error) {
	privPath := filepath.Join(dataDir, privateKeyFile)

	if _, err := os.Stat(privPath); err == nil {
		return loadKeys(privPath)
	}

	return generateAndSave(dataDir, privPath)
}

// PublicKeyBase64 returns the standard base64 encoding of the public key.
func (k *AgentKeys) PublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(k.PublicKey)
}

// Sign signs msg with the agent private key and returns the base64-encoded
// signature.
func (k *AgentKeys) Sign(msg []byte) string {
	sig := ed25519.Sign(k.PrivateKey, msg)
	return base64.StdEncoding.EncodeToString(sig)
}

// ─── private helpers ─────────────────────────────────────────────────────────

func loadKeys(privPath string) (*AgentKeys, error) {
	data, err := os.ReadFile(privPath)
	if err != nil {
		return nil, fmt.Errorf("read agent key: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != keyPEMType {
		return nil, errors.New("invalid agent key PEM format")
	}
	if len(block.Bytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("unexpected agent key size %d", len(block.Bytes))
	}

	priv := ed25519.PrivateKey(block.Bytes)
	pub := priv.Public().(ed25519.PublicKey)
	return &AgentKeys{PrivateKey: priv, PublicKey: pub}, nil
}

func generateAndSave(dataDir, privPath string) (*AgentKeys, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create agent data dir: %w", err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate agent key pair: %w", err)
	}

	block := &pem.Block{Type: keyPEMType, Bytes: []byte(priv)}
	if err := os.WriteFile(privPath, pem.EncodeToMemory(block), 0o600); err != nil {
		return nil, fmt.Errorf("write agent key: %w", err)
	}

	return &AgentKeys{PrivateKey: priv, PublicKey: pub}, nil
}
