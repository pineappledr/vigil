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
)

const version = "1.0.0"

type DriveReport struct {
	Hostname  string                   `json:"hostname"`
	Timestamp time.Time                `json:"timestamp"`
	Version   string                   `json:"agent_version"`
	Drives    []map[string]interface{} `json:"drives"`
}

type ScanResult struct {
	Devices []struct {
		Name     string `json:"name"`
		Type     string `json:"type"`
		Protocol string `json:"protocol"`
	} `json:"devices"`
}

func main() {
	serverURL := flag.String("server", "http://localhost:9080", "Vigil Server URL")
	interval := flag.Int("interval", 60, "Reporting interval in seconds (0 for single run)")
	hostnameOverride := flag.String("hostname", "", "Override hostname")
	showVersion := flag.Bool("version", false, "Show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("vigil-agent v%s\n", version)
		os.Exit(0)
	}

	log.SetFlags(log.Ltime | log.Ldate)
	log.Printf("🚀 Vigil Agent v%s starting...", version)

	// Check for smartctl
	if _, err := exec.LookPath("smartctl"); err != nil {
		log.Fatal("❌ Error: 'smartctl' not found. Please install smartmontools.")
	}
	log.Println("✓ smartctl found")

	// Determine hostname
	hostname := *hostnameOverride
	if hostname == "" {
		var err error
		hostname, err = os.Hostname()
		if err != nil {
			log.Fatalf("❌ Failed to get hostname: %v", err)
		}
	}
	log.Printf("✓ Hostname: %s", hostname)
	log.Printf("✓ Server: %s", *serverURL)

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("\n⏹️  Shutting down...")
		cancel()
	}()

	// Run immediately
	sendReport(ctx, *serverURL, hostname)

	// Exit if single run mode
	if *interval <= 0 {
		log.Println("✅ Single run complete")
		return
	}

	// Start interval loop
	log.Printf("📊 Reporting every %d seconds", *interval)
	ticker := time.NewTicker(time.Duration(*interval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("👋 Agent stopped")
			return
		case <-ticker.C:
			sendReport(ctx, *serverURL, hostname)
		}
	}
}

func sendReport(ctx context.Context, serverURL, hostname string) {
	report := DriveReport{
		Hostname:  hostname,
		Timestamp: time.Now().UTC(),
		Version:   version,
		Drives:    []map[string]interface{}{},
	}

	// Scan for devices
	scanCmd := exec.CommandContext(ctx, "smartctl", "--scan", "--json")
	scanOut, err := scanCmd.Output()
	if err != nil {
		log.Printf("⚠️  Device scan failed: %v", err)
	}

	var scan ScanResult
	if err := json.Unmarshal(scanOut, &scan); err != nil {
		log.Printf("⚠️  Failed to parse scan output: %v", err)
	}

	if len(scan.Devices) == 0 {
		log.Println("⚠️  No drives detected (check permissions)")
	}

	// Get details for each device
	for _, dev := range scan.Devices {
		log.Printf("   📀 Scanning %s (%s)...", dev.Name, dev.Type)

		cmd := exec.CommandContext(ctx, "smartctl", "-x", "--json", "--device", dev.Type, dev.Name)
		out, err := cmd.Output()
		if err != nil {
			log.Printf("   ⚠️  Failed to read %s: %v", dev.Name, err)
			continue
		}

		var data map[string]interface{}
		if err := json.Unmarshal(out, &data); err != nil {
			log.Printf("   ⚠️  Failed to parse %s: %v", dev.Name, err)
			continue
		}

		report.Drives = append(report.Drives, data)
	}

	// Send to server
	payload, err := json.Marshal(report)
	if err != nil {
		log.Printf("❌ Failed to marshal report: %v", err)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, "POST", serverURL+"/api/report", bytes.NewBuffer(payload))
	if err != nil {
		log.Printf("❌ Failed to create request: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", fmt.Sprintf("vigil-agent/%s", version))

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ Connection failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ Server returned %d", resp.StatusCode)
		return
	}

	log.Printf("✅ Report sent (%d drives)", len(report.Drives))
}
