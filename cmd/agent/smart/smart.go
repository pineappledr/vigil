package smart

import (
	"context"
	"encoding/json"
	"log"
	"os/exec"
)

// FallbackDeviceTypes are tried when the detected type fails
var FallbackDeviceTypes = []string{"sat", "scsi", "auto"}

// ScanDevices returns list of detected devices
func ScanDevices(ctx context.Context) ([]Device, error) {
	cmd := exec.CommandContext(ctx, "smartctl", "--scan", "--json")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result ScanResult
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, err
	}

	return result.Devices, nil
}

// ReadDrive attempts to read SMART data using detected type first, then fallbacks
func ReadDrive(ctx context.Context, name, detectedType string) map[string]interface{} {
	typesToTry := buildTypesToTry(detectedType)

	for i, devType := range typesToTry {
		logAttempt(name, devType, i)

		data := readWithType(ctx, name, devType)
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

func buildTypesToTry(detectedType string) []string {
	types := []string{detectedType}
	for _, ft := range FallbackDeviceTypes {
		if ft != detectedType {
			types = append(types, ft)
		}
	}
	return types
}

func logAttempt(name, devType string, attempt int) {
	if attempt == 0 {
		log.Printf("   📀 Scanning %s (%s)...", name, devType)
	} else {
		log.Printf("   🔄 Retrying %s with -d %s...", name, devType)
	}
}

func readWithType(ctx context.Context, name, devType string) map[string]interface{} {
	cmd := exec.CommandContext(ctx, "smartctl", "-x", "--json", "-d", devType, name)
	out, _ := cmd.Output()

	if len(out) == 0 {
		return nil
	}

	var data map[string]interface{}
	if err := json.Unmarshal(out, &data); err != nil {
		return nil
	}

	return data
}

func hasValidSmartData(data map[string]interface{}) bool {
	if _, ok := data["device"]; !ok {
		return false
	}

	// Check for various SMART data types
	smartIndicators := []string{
		"ata_smart_attributes",
		"scsi_error_counter_log",
		"nvme_smart_health_information_log",
	}

	for _, indicator := range smartIndicators {
		if _, ok := data[indicator]; ok {
			return true
		}
	}

	// Check smart_status
	if smartStatus, ok := data["smart_status"].(map[string]interface{}); ok {
		if _, ok := smartStatus["passed"]; ok {
			return true
		}
	}

	return false
}

// Types

type ScanResult struct {
	Devices []Device `json:"devices"`
}

type Device struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Protocol string `json:"protocol"`
}
