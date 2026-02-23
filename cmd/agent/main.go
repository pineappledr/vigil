package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	agentcrypto "vigil/cmd/agent/crypto"
	"vigil/cmd/agent/smart"
	"vigil/cmd/agent/zfs"
)

var version = "dev"

// DriveReport contains SMART data for drives
type DriveReport struct {
	Hostname  string                   `json:"hostname"`
	Timestamp time.Time                `json:"timestamp"`
	Version   string                   `json:"agent_version"`
	Drives    []map[string]interface{} `json:"drives"`
	ZFS       *zfs.ZFSReport           `json:"zfs,omitempty"`
}

func main() {
	cfg := parseFlags()

	log.SetFlags(log.Ltime | log.Ldate)
	log.Printf("🚀 Vigil Agent v%s starting...", version)

	if err := checkSmartctl(); err != nil {
		log.Fatal(err)
	}

	zfsAvailable := zfs.IsZFSAvailable()
	if zfsAvailable {
		log.Println("✓ ZFS detected")
	} else {
		log.Println("ℹ️  ZFS not available (optional)")
	}

	hostname := getHostname(cfg.hostnameOverride)
	log.Printf("✓ Hostname: %s", hostname)
	log.Printf("✓ Server:   %s", cfg.serverURL)
	log.Printf("✓ Data dir: %s", cfg.dataDir)

	if err := os.MkdirAll(cfg.dataDir, 0o700); err != nil {
		log.Fatalf("❌ Cannot create data dir %s: %v", cfg.dataDir, err)
	}

	keys, err := agentcrypto.LoadOrGenerate(cfg.dataDir)
	if err != nil {
		log.Fatalf("❌ Failed to initialise agent keys: %v", err)
	}
	log.Println("✓ Agent keys ready")

	fingerprint, err := loadOrGenerateFingerprint(cfg.dataDir)
	if err != nil {
		log.Fatalf("❌ Failed to determine machine fingerprint: %v", err)
	}
	log.Printf("✓ Fingerprint: %.24s...", fingerprint)

	// Auto-register if TOKEN is set and agent isn't registered yet
	authSt := loadAuthState(cfg.dataDir)

	if cfg.register && authSt == nil {
		if cfg.registerToken == "" {
			log.Fatal("❌ Registration requires a token (--token or TOKEN env)")
		}
		log.Printf("🔐 Registering with server %s...", cfg.serverURL)
		state, regErr := registerAgent(cfg.serverURL, cfg.registerToken, hostname, fingerprint, keys, cfg.dataDir)
		if regErr != nil {
			log.Fatalf("❌ Registration failed: %v", regErr)
		}
		log.Printf("✅ Registered as agent ID %d", state.AgentID)
		authSt = state
	} else if cfg.register && authSt != nil {
		log.Println("✓ Already registered, skipping registration")
	}

	if authSt == nil {
		log.Fatal("❌ Agent not registered. Run with --register --token <token> --server <url> first.")
	}
	if authSt.ServerURL != cfg.serverURL {
		log.Printf("⚠️  Server URL changed from %s to %s", authSt.ServerURL, cfg.serverURL)
		authSt.ServerURL = cfg.serverURL
	}

	if sessionNeedsRefresh(authSt) {
		log.Println("🔄 Session expiring soon, re-authenticating...")
		authSt, err = authenticate(authSt, fingerprint, keys, cfg.dataDir)
		if err != nil {
			log.Fatalf("❌ Re-authentication failed: %v", err)
		}
		log.Println("✓ Session refreshed")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupSignalHandler(cancel)

	authSt = sendReport(ctx, cfg.serverURL, hostname, zfsAvailable, fingerprint, keys, authSt, cfg.dataDir)

	if cfg.interval <= 0 {
		log.Println("✅ Single run complete")
		return
	}

	runInterval(ctx, cfg.serverURL, hostname, cfg.interval, zfsAvailable, fingerprint, keys, authSt, cfg.dataDir)
}

type agentConfig struct {
	serverURL        string
	interval         int
	hostnameOverride string
	dataDir          string
	register         bool
	registerToken    string
}

func parseFlags() agentConfig {
	serverURL := flag.String("server", "http://localhost:9080", "Vigil Server URL")
	interval := flag.Int("interval", 60, "Reporting interval in seconds (0 for single run)")
	hostnameOverride := flag.String("hostname", "", "Override hostname")
	dataDir := flag.String("data-dir", defaultDataDir(), "Directory for agent keys and state")
	register := flag.Bool("register", false, "Register this agent with the server (requires --token)")
	token := flag.String("token", "", "One-time registration token (used with --register)")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("vigil-agent v%s\n", version)
		os.Exit(0)
	}

	// Environment variables override flags (for Docker deployments)
	cfg := agentConfig{
		serverURL:        envOrStr("SERVER", *serverURL),
		interval:         *interval,
		hostnameOverride: envOrStr("HOSTNAME", *hostnameOverride),
		dataDir:          *dataDir,
		register:         *register,
		registerToken:    envOrStr("TOKEN", *token),
	}

	// If TOKEN env is set but --register wasn't passed, enable auto-registration
	if cfg.registerToken != "" && !cfg.register {
		cfg.register = true
	}

	return cfg
}

// envOrStr returns the environment variable value if set, otherwise the fallback.
func envOrStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func defaultDataDir() string {
	if runtime.GOOS == "linux" && os.Getuid() == 0 {
		return "/var/lib/vigil-agent"
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".vigil-agent")
}

func checkSmartctl() error {
	if _, err := exec.LookPath("smartctl"); err != nil {
		return fmt.Errorf("❌ Error: 'smartctl' not found. Please install smartmontools")
	}
	log.Println("✓ smartctl found")
	return nil
}

func getHostname(override string) string {
	if override != "" {
		return override
	}
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("❌ Failed to get hostname: %v", err)
	}
	return hostname
}

func setupSignalHandler(cancel context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\n⏹️  Shutting down...")
		cancel()
	}()
}

func runInterval(
	ctx context.Context,
	serverURL, hostname string,
	interval int,
	zfsAvailable bool,
	fingerprint string,
	keys *agentcrypto.AgentKeys,
	state *authState,
	dataDir string,
) {
	log.Printf("📊 Reporting every %d seconds", interval)
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("👋 Agent stopped")
			return
		case <-ticker.C:
			state = sendReport(ctx, serverURL, hostname, zfsAvailable, fingerprint, keys, state, dataDir)
		}
	}
}

// sendReport builds and POSTs a report, transparently handling session expiry.
func sendReport(
	ctx context.Context,
	serverURL, hostname string,
	zfsAvailable bool,
	fingerprint string,
	keys *agentcrypto.AgentKeys,
	state *authState,
	dataDir string,
) *authState {
	if sessionNeedsRefresh(state) {
		log.Println("🔄 Proactive re-auth before report...")
		if newState, err := authenticate(state, fingerprint, keys, dataDir); err == nil {
			state = newState
		}
	}

	report := DriveReport{
		Hostname:  hostname,
		Timestamp: time.Now().UTC(),
		Version:   version,
		Drives:    collectDriveData(ctx),
	}

	if zfsAvailable {
		if zfsReport, err := collectZFSData(hostname); err != nil {
			log.Printf("⚠️  ZFS collection failed: %v", err)
		} else if zfsReport != nil && len(zfsReport.Pools) > 0 {
			report.ZFS = zfsReport
			log.Printf("📦 ZFS: %d pool(s) detected", len(zfsReport.Pools))
		}
	}

	err := postReport(ctx, serverURL, report, state.SessionToken)
	if err == errUnauthorized {
		log.Println("🔄 Session expired, re-authenticating...")
		newState, authErr := authenticate(state, fingerprint, keys, dataDir)
		if authErr != nil {
			log.Printf("❌ Re-authentication failed: %v", authErr)
			return state
		}
		state = newState
		if err = postReport(ctx, serverURL, report, state.SessionToken); err != nil {
			log.Printf("❌ Report failed after re-auth: %v", err)
			return state
		}
	} else if err != nil {
		log.Printf("❌ %v", err)
		return state
	}

	logMsg := fmt.Sprintf("✅ Report sent (%d drives", len(report.Drives))
	if report.ZFS != nil && len(report.ZFS.Pools) > 0 {
		logMsg += fmt.Sprintf(", %d ZFS pools", len(report.ZFS.Pools))
	}
	log.Println(logMsg + ")")

	return state
}

var errUnauthorized = fmt.Errorf("session token rejected (401)")

func collectDriveData(ctx context.Context) []map[string]interface{} {
	devices, err := smart.ScanDevices(ctx)
	if err != nil {
		log.Printf("⚠️  Device scan failed: %v", err)
		return nil
	}
	if len(devices) == 0 {
		log.Println("⚠️  No drives detected (check permissions)")
		return nil
	}

	var drives []map[string]interface{}
	for _, dev := range devices {
		if data := smart.ReadDrive(ctx, dev.Name, dev.Type); data != nil {
			drives = append(drives, data)
		}
	}
	return drives
}

func collectZFSData(hostname string) (*zfs.ZFSReport, error) {
	return zfs.CollectZFSData(hostname)
}

func postReport(ctx context.Context, serverURL string, report DriveReport, sessionToken string) error {
	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("failed to marshal report: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", serverURL+"/api/report", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("vigil-agent/%s", version))
	req.Header.Set("Authorization", "Bearer "+sessionToken)

	resp, err := client.Do(req) // #nosec G107 -- URL is the configured server endpoint
	if err != nil {
		return fmt.Errorf("connection failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return errUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %d", resp.StatusCode)
	}
	return nil
}
