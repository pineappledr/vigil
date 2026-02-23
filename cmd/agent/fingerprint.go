package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

const fingerprintFile = "fingerprint"

// loadOrGenerateFingerprint returns a stable machine-unique identifier,
// persisting it to dataDir/fingerprint on first generation.
func loadOrGenerateFingerprint(dataDir string) (string, error) {
	fp, err := loadFingerprintFile(dataDir)
	if err == nil {
		return fp, nil
	}

	fp, err = generateFingerprint()
	if err != nil {
		return "", err
	}

	// Persist for future runs
	saveFingerprintFile(dataDir, fp)
	return fp, nil
}

func loadFingerprintFile(dataDir string) (string, error) {
	data, err := os.ReadFile(filepath.Join(dataDir, fingerprintFile))
	if err != nil {
		return "", err
	}
	fp := strings.TrimSpace(string(data))
	if fp == "" {
		return "", fmt.Errorf("empty fingerprint file")
	}
	return fp, nil
}

func saveFingerprintFile(dataDir, fp string) {
	os.MkdirAll(dataDir, 0o700)
	os.WriteFile(filepath.Join(dataDir, fingerprintFile), []byte(fp+"\n"), 0o600)
}

func generateFingerprint() (string, error) {
	// 1. /etc/machine-id
	if data, err := os.ReadFile("/etc/machine-id"); err == nil {
		id := strings.TrimSpace(string(data))
		if id != "" {
			return "mid:" + id, nil
		}
	}

	// 2. SHA256 of first non-loopback MAC
	if mac := firstMACAddress(); mac != "" {
		h := sha256.Sum256([]byte(mac))
		return "mac:" + hex.EncodeToString(h[:16]), nil
	}

	// 3. Random fallback
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random fingerprint: %w", err)
	}
	return "rnd:" + hex.EncodeToString(b), nil
}

func firstMACAddress() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		if len(iface.HardwareAddr) == 0 {
			continue
		}
		return iface.HardwareAddr.String()
	}
	return ""
}
