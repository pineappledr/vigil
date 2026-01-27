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
	"syscall"
	"time"

	"vigil/cmd/agent/smart"
)

var version = "dev"

type DriveReport struct {
	Hostname  string                   `json:"hostname"`
	Timestamp time.Time                `json:"timestamp"`
	Version   string                   `json:"agent_version"`
	Drives    []map[string]interface{} `json:"drives"`
}

func main() {
	cfg := parseFlags()

	log.SetFlags(log.Ltime | log.Ldate)
	log.Printf("🚀 Vigil Agent v%s starting...", version)

	if err := checkSmartctl(); err != nil {
		log.Fatal(err)
	}

	hostname := getHostname(cfg.hostnameOverride)
	log.Printf("✓ Hostname: %s", hostname)
	log.Printf("✓ Server: %s", cfg.serverURL)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	setupSignalHandler(cancel)

	// Run immediately
	sendReport(ctx, cfg.serverURL, hostname)

	if cfg.interval <= 0 {
		log.Println("✅ Single run complete")
		return
	}

	runInterval(ctx, cfg.serverURL, hostname, cfg.interval)
}

type agentConfig struct {
	serverURL        string
	interval         int
	hostnameOverride string
}

func parseFlags() agentConfig {
	serverURL := flag.String("server", "http://localhost:9080", "Vigil Server URL")
	interval := flag.Int("interval", 60, "Reporting interval in seconds (0 for single run)")
	hostnameOverride := flag.String("hostname", "", "Override hostname")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("vigil-agent v%s\n", version)
		os.Exit(0)
	}

	return agentConfig{
		serverURL:        *serverURL,
		interval:         *interval,
		hostnameOverride: *hostnameOverride,
	}
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

func runInterval(ctx context.Context, serverURL, hostname string, interval int) {
	log.Printf("📊 Reporting every %d seconds", interval)
	ticker := time.NewTicker(time.Duration(interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("👋 Agent stopped")
			return
		case <-ticker.C:
			sendReport(ctx, serverURL, hostname)
		}
	}
}

func sendReport(ctx context.Context, serverURL, hostname string) {
	report := DriveReport{
		Hostname:  hostname,
		Timestamp: time.Now().UTC(),
		Version:   version,
		Drives:    collectDriveData(ctx),
	}

	if err := postReport(ctx, serverURL, report); err != nil {
		log.Printf("❌ %v", err)
		return
	}

	log.Printf("✅ Report sent (%d drives)", len(report.Drives))
}

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

func postReport(ctx context.Context, serverURL string, report DriveReport) error {
	payload, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("Failed to marshal report: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", serverURL+"/api/report", bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("Failed to create request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("vigil-agent/%s", version))

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("Connection failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Server returned %d", resp.StatusCode)
	}

	return nil
}
