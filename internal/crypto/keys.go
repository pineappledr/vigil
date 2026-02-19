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
	privateKeyFile = "vigil.key"
	publicKeyFile  = "vigil.pub"
	keyPEMType     = "VIGIL PRIVATE KEY"
	pubPEMType     = "VIGIL PUBLIC KEY"
)

// ServerKeys holds the server's Ed25519 key pair.
type ServerKeys struct {
	PrivateKey ed25519.PrivateKey
	PublicKey  ed25519.PublicKey
}

// LoadOrGenerate loads existing keys from dataDir, or generates and saves a
// new key pair if none exist.
func LoadOrGenerate(dataDir string) (*ServerKeys, error) {
	privPath := filepath.Join(dataDir, privateKeyFile)

	if _, err := os.Stat(privPath); err == nil {
		return loadKeys(privPath)
	}

	return generateAndSave(dataDir, privPath)
}

// PublicKeyBase64 returns the standard base64 encoding of the public key.
func (k *ServerKeys) PublicKeyBase64() string {
	return base64.StdEncoding.EncodeToString(k.PublicKey)
}

// Sign signs msg with the server private key and returns the signature.
func (k *ServerKeys) Sign(msg []byte) []byte {
	return ed25519.Sign(k.PrivateKey, msg)
}

// VerifyAgentSignature verifies a base64-encoded signature from an agent using
// the agent's stored base64-encoded Ed25519 public key.
func VerifyAgentSignature(publicKeyBase64 string, msg, sig []byte) bool {
	pubBytes, err := base64.StdEncoding.DecodeString(publicKeyBase64)
	if err != nil || len(pubBytes) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(pubBytes), msg, sig)
}

// ─── private helpers ─────────────────────────────────────────────────────────

func loadKeys(privPath string) (*ServerKeys, error) {
	data, err := os.ReadFile(privPath)
	if err != nil {
		return nil, fmt.Errorf("read private key: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil || block.Type != keyPEMType {
		return nil, errors.New("invalid private key PEM format")
	}
	if len(block.Bytes) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("unexpected private key size %d", len(block.Bytes))
	}

	priv := ed25519.PrivateKey(block.Bytes)
	pub := priv.Public().(ed25519.PublicKey)
	return &ServerKeys{PrivateKey: priv, PublicKey: pub}, nil
}

func generateAndSave(dataDir, privPath string) (*ServerKeys, error) {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key pair: %w", err)
	}

	// Private key — readable only by owner
	privBlock := &pem.Block{Type: keyPEMType, Bytes: []byte(priv)}
	if err := os.WriteFile(privPath, pem.EncodeToMemory(privBlock), 0o600); err != nil {
		return nil, fmt.Errorf("write private key: %w", err)
	}

	// Public key — world-readable
	pubPath := filepath.Join(dataDir, publicKeyFile)
	pubBlock := &pem.Block{Type: pubPEMType, Bytes: []byte(pub)}
	if err := os.WriteFile(pubPath, pem.EncodeToMemory(pubBlock), 0o644); err != nil {
		return nil, fmt.Errorf("write public key: %w", err)
	}

	return &ServerKeys{PrivateKey: priv, PublicKey: pub}, nil
}
