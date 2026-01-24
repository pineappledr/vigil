package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
)

// Config holds command line arguments
type Config struct {
	ServerURL string
}

// DriveReport is the payload we send to the server
type DriveReport struct {
	Hostname string                 `json:"hostname"`
	Drives   []map[string]interface{} `json:"drives"`
}

// ScanResult matches "smartctl --scan --json"
type ScanResult struct {
	Devices []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"devices"`
}

func main() {
	// 1. Parse Flags (Allow user to set server URL)
	serverURL := flag.String("server", "http://localhost:8090", "The URL of the Vigil Server")
	flag.Parse()

	log.Println("👁️  Vigil Agent Starting...")
	log.Printf("   Target Server: %s\n", *serverURL)

	// 2. Get Hostname
	hostname, _ := os.Hostname()
	report := DriveReport{
		Hostname: hostname,
		Drives:   []map[string]interface{}{},
	}

	// 3. Scan for Devices
	log.Println("   Scanning for drives...")
	scanCmd := exec.Command("smartctl", "--scan", "--json")
	scanOut, err := scanCmd.Output()
	if err != nil {
		log.Fatalf("❌ Error scanning drives: %v", err)
	}

	var scan ScanResult
	if err := json.Unmarshal(scanOut, &scan); err != nil {
		log.Fatalf("❌ Error parsing scan: %v", err)
	}

	// 4. Get Health for each drive
	for _, dev := range scan.Devices {
		log.Printf("   -> Reading SMART data for %s...", dev.Name)
		
		// Run smartctl -x (Extended info)
		cmd := exec.Command("smartctl", "-x", "--json", "--device", dev.Type, dev.Name)
		out, err := cmd.Output()
		if err != nil {
			log.Printf("      ⚠️ Failed to read %s: %v", dev.Name, err)
			continue
		}

		// Decode the raw JSON so we can add it to our report list
		var driveData map[string]interface{}
		if err := json.Unmarshal(out, &driveData); err == nil {
			report.Drives = append(report.Drives, driveData)
		}
	}

	// 5. Send Data to Server
	log.Printf("   Sending report for %d drives to %s...", len(report.Drives), *serverURL)
	payload, _ := json.Marshal(report)
	
	resp, err := http.Post(*serverURL+"/api/report", "application/json", bytes.NewBuffer(payload))
	if err != nil {
		log.Fatalf("❌ Failed to connect to server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		log.Println("✅ Success! Server acknowledged receipt.")
	} else {
		log.Printf("⚠️  Server returned status: %s", resp.Status)
	}
}