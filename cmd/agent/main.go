package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
)

type DriveReport struct {
	Hostname string                   `json:"hostname"`
	Drives   []map[string]interface{} `json:"drives"`
}

type ScanResult struct {
	Devices []struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"devices"`
}

func main() {
	serverURL := flag.String("server", "http://localhost:8090", "Vigil Server URL")
	flag.Parse()

	log.Println("Vigil Agent Starting...")

	// 1. Check for smartctl
	if _, err := exec.LookPath("smartctl"); err != nil {
		log.Fatal("❌ Error: 'smartctl' not found. Please install smartmontools.")
	}

	// 2. Prepare Report
	hostname, _ := os.Hostname()
	report := DriveReport{Hostname: hostname}

	// 3. Scan Devices
	scanCmd := exec.Command("smartctl", "--scan", "--json")
	scanOut, _ := scanCmd.Output()

	var scan ScanResult
	if err := json.Unmarshal(scanOut, &scan); err != nil {
		log.Printf("⚠️  No drives found or permission denied.")
	}

	// 4. Get Health Details
	for _, dev := range scan.Devices {
		log.Printf("   -> Checking %s...", dev.Name)
		cmd := exec.Command("smartctl", "-x", "--json", "--device", dev.Type, dev.Name)
		out, err := cmd.Output()
		if err == nil {
			var data map[string]interface{}
			json.Unmarshal(out, &data)
			report.Drives = append(report.Drives, data)
		}
	}

	// 5. Send to Server
	payload, _ := json.Marshal(report)
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Post(*serverURL+"/api/report", "application/json", bytes.NewBuffer(payload))
	
	if err != nil {
		log.Printf("❌ Connection failed: %v", err)
		return
	}
	defer resp.Body.Close()
	
	log.Println("✅ Report sent successfully.")
}