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

// Version is set at build time via -ldflags
var version = "dev"

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

// Device types to try when default fails (in order of preference)
// sat = SATA via SAS HBA (most common for LSI controllers)
// scsi = Pure SCSI/SAS drives
// auto = Let smartctl figure it out
var fallbackDeviceTypes = []string{"sat", "scsi", "auto"}

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
		data := tryReadDrive(ctx, dev.Name, dev.Type)
		if data != nil {
			report.Drives = append(report.Drives, data)
		}
	}

	// Send to server
	payload, err := json.Marshal(report)
	if err != nil {
		log.Printf("❌ Failed to marshal report: %v", err)
		return
	}

	client := &http.Client{Timeout: 30 * time.Second}
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

// tryReadDrive attempts to read SMART data using the detected type first,
// then falls back to alternative device types (sat, scsi, auto) for HBA/RAID controllers
func tryReadDrive(ctx context.Context, name, detectedType string) map[string]interface{} {
	// Build list of types to try: detected type first, then fallbacks
	typesToTry := []string{detectedType}
	for _, ft := range fallbackDeviceTypes {
		if ft != detectedType {
			typesToTry = append(typesToTry, ft)
		}
	}

	for i, devType := range typesToTry {
		if i == 0 {
			log.Printf("   📀 Scanning %s (%s)...", name, devType)
		} else {
			log.Printf("   🔄 Retrying %s with -d %s...", name, devType)
		}

		data, _ := readDriveWithType(ctx, name, devType)
		if data != nil && hasValidSmartData(data) {
			if i > 0 {
				log.Printf("   ✓ Success with -d %s", devType)
			}
			return data
		}
	}

	log.Printf("   ⚠️  Skipping %s (no SMART support or incompatible)", name)
	return nil
}

// readDriveWithType reads SMART data with a specific device type
func readDriveWithType(ctx context.Context, name, devType string) (map[string]interface{}, error) {
	cmd := exec.CommandContext(ctx, "smartctl", "-x", "--json", "-d", devType, name)
	out, err := cmd.Output()

	// smartctl returns non-zero exit codes for various warnings/errors
	// but may still produce valid JSON output, so try to parse it anyway
	if len(out) == 0 {
		return nil, err
	}

	var data map[string]interface{}
	if jsonErr := json.Unmarshal(out, &data); jsonErr != nil {
		// If JSON parsing fails, return the original error
		if err != nil {
			return nil, err
		}
		return nil, jsonErr
	}

	// If we got valid JSON data, return it even if there was an exit error
	return data, nil
}

// hasValidSmartData checks if the response contains meaningful SMART data
func hasValidSmartData(data map[string]interface{}) bool {
	// Check for device info
	if _, ok := data["device"]; !ok {
		return false
	}

	// Check for either ATA or SCSI SMART data
	if _, ok := data["ata_smart_attributes"]; ok {
		return true
	}
	if _, ok := data["scsi_error_counter_log"]; ok {
		return true
	}
	if _, ok := data["nvme_smart_health_information_log"]; ok {
		return true
	}

	// Check smart_status exists and is valid
	if smartStatus, ok := data["smart_status"]; ok {
		if status, ok := smartStatus.(map[string]interface{}); ok {
			if _, ok := status["passed"]; ok {
				return true
			}
		}
	}

	return false
}
