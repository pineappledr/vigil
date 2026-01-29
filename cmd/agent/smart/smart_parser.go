package smart

import (
	"time"
)

// SmartAttribute represents a single S.M.A.R.T. attribute
type SmartAttribute struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	Value      int       `json:"value"`
	Worst      int       `json:"worst"`
	Threshold  int       `json:"thresh"`
	RawValue   int64     `json:"raw_value"`
	Flags      string    `json:"flags"`
	WhenFailed string    `json:"when_failed,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// DriveSmartData represents comprehensive SMART data for a drive
type DriveSmartData struct {
	Hostname     string           `json:"hostname"`
	SerialNumber string           `json:"serial_number"`
	DeviceName   string           `json:"device_name"`
	ModelName    string           `json:"model_name"`
	Attributes   []SmartAttribute `json:"attributes"`
	Timestamp    time.Time        `json:"timestamp"`
}

// CriticalAttribute defines a critical SMART attribute
type CriticalAttribute struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	DriveType        string `json:"drive_type"` // HDD, SSD, BOTH
	Severity         string `json:"severity"`   // CRITICAL, WARNING, INFO
	FailureThreshold *int   `json:"failure_threshold,omitempty"`
}

// ParseSmartAttributes extracts SMART attributes from smartctl JSON output
func ParseSmartAttributes(data map[string]interface{}, hostname string) (*DriveSmartData, error) {
	result := &DriveSmartData{
		Hostname:   hostname,
		Attributes: make([]SmartAttribute, 0),
		Timestamp:  time.Now(),
	}

	// Extract device information
	if device, ok := data["device"].(map[string]interface{}); ok {
		if name, ok := device["name"].(string); ok {
			result.DeviceName = name
		}
	}

	// Extract serial number
	if serial, ok := data["serial_number"].(string); ok {
		result.SerialNumber = serial
	}

	// Extract model name
	if model, ok := data["model_name"].(string); ok {
		result.ModelName = model
	} else if model, ok := data["model_family"].(string); ok {
		result.ModelName = model
	}

	// Parse ATA SMART attributes table
	if ataSmartAttrs, ok := data["ata_smart_attributes"].(map[string]interface{}); ok {
		if table, ok := ataSmartAttrs["table"].([]interface{}); ok {
			for _, attrInterface := range table {
				if attr, ok := attrInterface.(map[string]interface{}); ok {
					smartAttr := SmartAttribute{
						Timestamp: result.Timestamp,
					}

					// Parse attribute ID
					if id, ok := attr["id"].(float64); ok {
						smartAttr.ID = int(id)
					}

					// Parse attribute name
					if name, ok := attr["name"].(string); ok {
						smartAttr.Name = name
					}

					// Parse value
					if value, ok := attr["value"].(float64); ok {
						smartAttr.Value = int(value)
					}

					// Parse worst
					if worst, ok := attr["worst"].(float64); ok {
						smartAttr.Worst = int(worst)
					}

					// Parse threshold
					if thresh, ok := attr["thresh"].(float64); ok {
						smartAttr.Threshold = int(thresh)
					}

					// Parse raw value
					if raw, ok := attr["raw"].(map[string]interface{}); ok {
						if rawVal, ok := raw["value"].(float64); ok {
							smartAttr.RawValue = int64(rawVal)
						}
					}

					// Parse flags
					if flags, ok := attr["flags"].(map[string]interface{}); ok {
						if flagStr, ok := flags["string"].(string); ok {
							smartAttr.Flags = flagStr
						}
					}

					// Parse when_failed
					if whenFailed, ok := attr["when_failed"].(string); ok {
						smartAttr.WhenFailed = whenFailed
					}

					result.Attributes = append(result.Attributes, smartAttr)
				}
			}
		}
	}

	// Parse NVMe SMART attributes (if present)
	if nvmeSmartAttrs, ok := data["nvme_smart_health_information_log"].(map[string]interface{}); ok {
		parseNVMeAttributes(nvmeSmartAttrs, result)
	}

	return result, nil
}

// parseNVMeAttributes extracts NVMe-specific SMART data
func parseNVMeAttributes(nvmeData map[string]interface{}, result *DriveSmartData) {
	// NVMe drives have different attribute structure
	// Map common NVMe attributes to pseudo-SMART IDs for consistency

	// Temperature
	if temp, ok := nvmeData["temperature"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        194, // Standard temperature attribute ID
			Name:      "Temperature",
			RawValue:  int64(temp),
			Timestamp: result.Timestamp,
		})
	}

	// Available Spare
	if spare, ok := nvmeData["available_spare"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        232, // SSD Available Reserved Space
			Name:      "Available Spare",
			RawValue:  int64(spare),
			Timestamp: result.Timestamp,
		})
	}

	// Percentage Used
	if used, ok := nvmeData["percentage_used"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        233, // Media Wearout Indicator
			Name:      "Percentage Used",
			RawValue:  int64(used),
			Timestamp: result.Timestamp,
		})
	}

	// Data Units Written
	if written, ok := nvmeData["data_units_written"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        241, // Total LBAs Written
			Name:      "Data Units Written",
			RawValue:  int64(written),
			Timestamp: result.Timestamp,
		})
	}

	// Data Units Read
	if read, ok := nvmeData["data_units_read"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        242, // Total LBAs Read
			Name:      "Data Units Read",
			RawValue:  int64(read),
			Timestamp: result.Timestamp,
		})
	}

	// Power Cycles
	if cycles, ok := nvmeData["power_cycles"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        12, // Power Cycle Count
			Name:      "Power Cycles",
			RawValue:  int64(cycles),
			Timestamp: result.Timestamp,
		})
	}

	// Power On Hours
	if hours, ok := nvmeData["power_on_hours"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        9, // Power-On Hours
			Name:      "Power On Hours",
			RawValue:  int64(hours),
			Timestamp: result.Timestamp,
		})
	}

	// Media Errors
	if errors, ok := nvmeData["media_errors"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        187, // Reported Uncorrectable Errors
			Name:      "Media Errors",
			RawValue:  int64(errors),
			Timestamp: result.Timestamp,
		})
	}
}

// IsCriticalAttribute checks if an attribute ID is critical
func IsCriticalAttribute(id int) bool {
	criticalIDs := map[int]bool{
		5:   true, // Reallocated Sectors Count
		10:  true, // Spin Retry Count
		187: true, // Reported Uncorrectable Errors
		188: true, // Command Timeout
		196: true, // Reallocation Event Count
		197: true, // Current Pending Sector Count
		198: true, // Offline Uncorrectable Sector Count
		181: true, // Program Fail Count (SSD)
		182: true, // Erase Fail Count (SSD)
		183: true, // Runtime Bad Block (SSD)
		184: true, // End-to-End Error (SSD)
		232: true, // Available Reserved Space (SSD)
	}
	return criticalIDs[id]
}

// GetAttributeSeverity determines severity level of an attribute
func GetAttributeSeverity(id int, rawValue int64, threshold int) string {
	// Check if attribute has failed (value <= threshold)
	if threshold > 0 && rawValue > 0 {
		if int(rawValue) <= threshold {
			return "CRITICAL"
		}
	}

	// Check specific attribute conditions
	switch id {
	case 5, 196, 197, 198: // Reallocated sectors, pending sectors
		if rawValue > 0 {
			return "CRITICAL"
		}
	case 187, 188: // Uncorrectable errors, command timeout
		if rawValue > 0 {
			return "CRITICAL"
		}
	case 194: // Temperature
		if rawValue > 60 {
			return "WARNING"
		}
		if rawValue > 50 {
			return "INFO"
		}
	case 199: // CRC errors
		if rawValue > 0 {
			return "WARNING"
		}
	case 232: // Available Reserved Space (SSD)
		if rawValue < 10 {
			return "CRITICAL"
		}
		if rawValue < 20 {
			return "WARNING"
		}
	case 233: // Media Wearout Indicator (SSD)
		if rawValue > 90 {
			return "CRITICAL"
		}
		if rawValue > 80 {
			return "WARNING"
		}
	}

	return "HEALTHY"
}
