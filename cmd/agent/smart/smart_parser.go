package smart

import (
	"fmt"
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
	RawString  string    `json:"raw_string,omitempty"`
	Flags      string    `json:"flags"`
	WhenFailed string    `json:"when_failed,omitempty"`
	Timestamp  time.Time `json:"timestamp"`
}

// DriveSmartData represents comprehensive SMART data for a drive
type DriveSmartData struct {
	Hostname        string           `json:"hostname"`
	SerialNumber    string           `json:"serial_number"`
	DeviceName      string           `json:"device_name"`
	ModelName       string           `json:"model_name"`
	FirmwareVersion string           `json:"firmware_version"`
	DriveType       string           `json:"drive_type"` // HDD, SSD, NVMe
	RotationRate    int              `json:"rotation_rate"`
	Capacity        int64            `json:"capacity_bytes"`
	Temperature     int              `json:"temperature"`
	PowerOnHours    int64            `json:"power_on_hours"`
	PowerCycles     int64            `json:"power_cycles"`
	SmartPassed     bool             `json:"smart_passed"`
	Attributes      []SmartAttribute `json:"attributes"`
	Timestamp       time.Time        `json:"timestamp"`
}

// CriticalAttribute defines a critical SMART attribute with metadata
type CriticalAttribute struct {
	ID               int    `json:"id"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	DriveType        string `json:"drive_type"` // HDD, SSD, BOTH, NVMe
	Severity         string `json:"severity"`   // CRITICAL, WARNING, INFO
	FailureThreshold *int   `json:"failure_threshold,omitempty"`
	HigherIsBetter   bool   `json:"higher_is_better"`
}

// AttributeSeverity levels
const (
	SeverityHealthy  = "HEALTHY"
	SeverityCritical = "CRITICAL"
	SeverityWarning  = "WARNING"
	SeverityInfo     = "INFO"
)

// DriveTypes
const (
	DriveTypeHDD  = "HDD"
	DriveTypeSSD  = "SSD"
	DriveTypeNVMe = "NVMe"
	DriveTypeBoth = "BOTH"
)

// CriticalAttributeDefinitions contains all known critical SMART attributes
// with their severity levels and thresholds
var CriticalAttributeDefinitions = map[int]CriticalAttribute{
	// ─── Critical Failure Indicators ─────────────────────────────────
	5: {
		ID:               5,
		Name:             "Reallocated Sectors Count",
		Description:      "Count of reallocated sectors. When a sector is found bad, it's remapped to a spare area.",
		DriveType:        DriveTypeBoth,
		Severity:         SeverityCritical,
		FailureThreshold: intPtr(0),
		HigherIsBetter:   false,
	},
	10: {
		ID:               10,
		Name:             "Spin Retry Count",
		Description:      "Count of retry attempts to spin up the drive. Indicates motor or bearing issues.",
		DriveType:        DriveTypeHDD,
		Severity:         SeverityCritical,
		FailureThreshold: intPtr(0),
		HigherIsBetter:   false,
	},
	196: {
		ID:               196,
		Name:             "Reallocation Event Count",
		Description:      "Count of remap operations (transferring data from bad sectors to spare area).",
		DriveType:        DriveTypeBoth,
		Severity:         SeverityCritical,
		FailureThreshold: intPtr(0),
		HigherIsBetter:   false,
	},
	197: {
		ID:               197,
		Name:             "Current Pending Sector Count",
		Description:      "Count of unstable sectors waiting to be remapped. If read succeeds, sector is unmarked.",
		DriveType:        DriveTypeBoth,
		Severity:         SeverityCritical,
		FailureThreshold: intPtr(0),
		HigherIsBetter:   false,
	},
	198: {
		ID:               198,
		Name:             "Offline Uncorrectable Sector Count",
		Description:      "Count of uncorrectable errors when reading/writing sectors. Often indicates surface damage.",
		DriveType:        DriveTypeBoth,
		Severity:         SeverityCritical,
		FailureThreshold: intPtr(0),
		HigherIsBetter:   false,
	},
	187: {
		ID:               187,
		Name:             "Reported Uncorrectable Errors",
		Description:      "Count of uncorrectable errors reported to the host. Critical failure indicator.",
		DriveType:        DriveTypeBoth,
		Severity:         SeverityCritical,
		FailureThreshold: intPtr(0),
		HigherIsBetter:   false,
	},
	188: {
		ID:               188,
		Name:             "Command Timeout",
		Description:      "Count of aborted operations due to timeout. May indicate interface or drive issues.",
		DriveType:        DriveTypeBoth,
		Severity:         SeverityCritical,
		FailureThreshold: intPtr(0),
		HigherIsBetter:   false,
	},

	// ─── SSD-Specific Critical Attributes ────────────────────────────
	181: {
		ID:               181,
		Name:             "Program Fail Count",
		Description:      "Count of flash program (write) failures. Indicates NAND wear.",
		DriveType:        DriveTypeSSD,
		Severity:         SeverityCritical,
		FailureThreshold: intPtr(0),
		HigherIsBetter:   false,
	},
	182: {
		ID:               182,
		Name:             "Erase Fail Count",
		Description:      "Count of flash erase failures. Indicates NAND wear.",
		DriveType:        DriveTypeSSD,
		Severity:         SeverityCritical,
		FailureThreshold: intPtr(0),
		HigherIsBetter:   false,
	},
	183: {
		ID:               183,
		Name:             "Runtime Bad Block",
		Description:      "Count of bad blocks detected during operation.",
		DriveType:        DriveTypeSSD,
		Severity:         SeverityCritical,
		FailureThreshold: intPtr(0),
		HigherIsBetter:   false,
	},
	184: {
		ID:               184,
		Name:             "End-to-End Error",
		Description:      "Count of parity errors in data path between host and drive.",
		DriveType:        DriveTypeSSD,
		Severity:         SeverityCritical,
		FailureThreshold: intPtr(0),
		HigherIsBetter:   false,
	},
	232: {
		ID:               232,
		Name:             "Available Reserved Space",
		Description:      "Percentage of reserved space remaining for bad block replacement.",
		DriveType:        DriveTypeSSD,
		Severity:         SeverityCritical,
		FailureThreshold: intPtr(10),
		HigherIsBetter:   true,
	},
	233: {
		ID:               233,
		Name:             "Media Wearout Indicator",
		Description:      "SSD wear indicator showing percentage of rated write cycles used.",
		DriveType:        DriveTypeSSD,
		Severity:         SeverityWarning,
		FailureThreshold: intPtr(90),
		HigherIsBetter:   false,
	},

	// ─── Warning Indicators ──────────────────────────────────────────
	1: {
		ID:               1,
		Name:             "Read Error Rate",
		Description:      "Rate of hardware read errors. Vendor-specific interpretation.",
		DriveType:        DriveTypeHDD,
		Severity:         SeverityWarning,
		FailureThreshold: nil,
		HigherIsBetter:   false,
	},
	7: {
		ID:               7,
		Name:             "Seek Error Rate",
		Description:      "Rate of seek errors of the magnetic heads.",
		DriveType:        DriveTypeHDD,
		Severity:         SeverityWarning,
		FailureThreshold: nil,
		HigherIsBetter:   false,
	},
	11: {
		ID:               11,
		Name:             "Calibration Retry Count",
		Description:      "Count of recalibration retries.",
		DriveType:        DriveTypeHDD,
		Severity:         SeverityWarning,
		FailureThreshold: intPtr(0),
		HigherIsBetter:   false,
	},
	199: {
		ID:               199,
		Name:             "UltraDMA CRC Error Count",
		Description:      "Count of CRC errors during Ultra DMA transfers. Often indicates cable issues.",
		DriveType:        DriveTypeBoth,
		Severity:         SeverityWarning,
		FailureThreshold: intPtr(0),
		HigherIsBetter:   false,
	},
	200: {
		ID:               200,
		Name:             "Multi-Zone Error Rate",
		Description:      "Count of errors while writing sectors.",
		DriveType:        DriveTypeHDD,
		Severity:         SeverityWarning,
		FailureThreshold: nil,
		HigherIsBetter:   false,
	},
	201: {
		ID:               201,
		Name:             "Soft Read Error Rate",
		Description:      "Count of off-track read errors.",
		DriveType:        DriveTypeHDD,
		Severity:         SeverityWarning,
		FailureThreshold: nil,
		HigherIsBetter:   false,
	},

	// ─── Temperature ─────────────────────────────────────────────────
	194: {
		ID:               194,
		Name:             "Temperature Celsius",
		Description:      "Current internal temperature in Celsius.",
		DriveType:        DriveTypeBoth,
		Severity:         SeverityWarning,
		FailureThreshold: intPtr(60),
		HigherIsBetter:   false,
	},
	190: {
		ID:               190,
		Name:             "Airflow Temperature",
		Description:      "Temperature of air flowing across the drive.",
		DriveType:        DriveTypeBoth,
		Severity:         SeverityWarning,
		FailureThreshold: intPtr(60),
		HigherIsBetter:   false,
	},

	// ─── Informational Attributes ────────────────────────────────────
	9: {
		ID:               9,
		Name:             "Power-On Hours",
		Description:      "Total hours the drive has been powered on.",
		DriveType:        DriveTypeBoth,
		Severity:         SeverityInfo,
		FailureThreshold: nil,
		HigherIsBetter:   false,
	},
	12: {
		ID:               12,
		Name:             "Power Cycle Count",
		Description:      "Count of full power on/off cycles.",
		DriveType:        DriveTypeBoth,
		Severity:         SeverityInfo,
		FailureThreshold: nil,
		HigherIsBetter:   false,
	},
	177: {
		ID:               177,
		Name:             "Wear Leveling Count",
		Description:      "SSD wear leveling status.",
		DriveType:        DriveTypeSSD,
		Severity:         SeverityWarning,
		FailureThreshold: intPtr(10),
		HigherIsBetter:   true,
	},
	179: {
		ID:               179,
		Name:             "Used Reserved Block Count",
		Description:      "Count of used reserved blocks for bad block replacement.",
		DriveType:        DriveTypeSSD,
		Severity:         SeverityWarning,
		FailureThreshold: intPtr(0),
		HigherIsBetter:   false,
	},
	193: {
		ID:               193,
		Name:             "Load Cycle Count",
		Description:      "Count of head load/unload cycles.",
		DriveType:        DriveTypeHDD,
		Severity:         SeverityInfo,
		FailureThreshold: nil,
		HigherIsBetter:   false,
	},
	195: {
		ID:               195,
		Name:             "Hardware ECC Recovered",
		Description:      "Count of ECC error corrections.",
		DriveType:        DriveTypeSSD,
		Severity:         SeverityWarning,
		FailureThreshold: nil,
		HigherIsBetter:   false,
	},
	202: {
		ID:               202,
		Name:             "Data Address Mark Errors",
		Description:      "Count of data address mark errors.",
		DriveType:        DriveTypeSSD,
		Severity:         SeverityWarning,
		FailureThreshold: nil,
		HigherIsBetter:   false,
	},
	241: {
		ID:               241,
		Name:             "Total LBAs Written",
		Description:      "Total count of logical block addresses written.",
		DriveType:        DriveTypeSSD,
		Severity:         SeverityInfo,
		FailureThreshold: nil,
		HigherIsBetter:   false,
	},
	242: {
		ID:               242,
		Name:             "Total LBAs Read",
		Description:      "Total count of logical block addresses read.",
		DriveType:        DriveTypeSSD,
		Severity:         SeverityInfo,
		FailureThreshold: nil,
		HigherIsBetter:   false,
	},
}

// NVMe attribute pseudo-IDs (mapped from NVMe health log)
const (
	NVMeAttrTemperature      = 194
	NVMeAttrAvailableSpare   = 232
	NVMeAttrPercentageUsed   = 233
	NVMeAttrDataUnitsWritten = 241
	NVMeAttrDataUnitsRead    = 242
	NVMeAttrPowerCycles      = 12
	NVMeAttrPowerOnHours     = 9
	NVMeAttrMediaErrors      = 187
	NVMeAttrCriticalWarning  = 1
)

// ParseSmartAttributes extracts SMART attributes from smartctl JSON output
func ParseSmartAttributes(data map[string]interface{}, hostname string) (*DriveSmartData, error) {
	result := &DriveSmartData{
		Hostname:   hostname,
		Attributes: make([]SmartAttribute, 0),
		Timestamp:  time.Now().UTC(),
	}

	// Extract device information
	extractDeviceInfo(data, result)

	// Extract SMART status
	extractSmartStatus(data, result)

	// Determine drive type
	result.DriveType = determineDriveType(data, result.RotationRate)

	// Parse attributes based on drive type
	if result.DriveType == DriveTypeNVMe {
		parseNVMeAttributes(data, result)
	} else {
		parseATAAttributes(data, result)
	}

	// Extract temperature and power-on info from raw data if not already set
	extractAdditionalMetrics(data, result)

	return result, nil
}

// extractDeviceInfo extracts basic device information
func extractDeviceInfo(data map[string]interface{}, result *DriveSmartData) {
	// Device name
	if device, ok := data["device"].(map[string]interface{}); ok {
		if name, ok := device["name"].(string); ok {
			result.DeviceName = name
		}
	}

	// Serial number
	if serial, ok := data["serial_number"].(string); ok {
		result.SerialNumber = serial
	}

	// Model name (try multiple fields)
	if model, ok := data["model_name"].(string); ok {
		result.ModelName = model
	} else if model, ok := data["model_family"].(string); ok {
		result.ModelName = model
	} else if model, ok := data["scsi_model_name"].(string); ok {
		result.ModelName = model
	}

	// Firmware version
	if fw, ok := data["firmware_version"].(string); ok {
		result.FirmwareVersion = fw
	}

	// Rotation rate
	if rate, ok := data["rotation_rate"].(float64); ok {
		result.RotationRate = int(rate)
	}

	// Capacity
	if capacity, ok := data["user_capacity"].(map[string]interface{}); ok {
		if bytes, ok := capacity["bytes"].(float64); ok {
			result.Capacity = int64(bytes)
		}
	}
}

// extractSmartStatus extracts SMART overall status
func extractSmartStatus(data map[string]interface{}, result *DriveSmartData) {
	result.SmartPassed = true // Default to passed

	if smartStatus, ok := data["smart_status"].(map[string]interface{}); ok {
		if passed, ok := smartStatus["passed"].(bool); ok {
			result.SmartPassed = passed
		}
	}
}

// determineDriveType determines if the drive is HDD, SSD, or NVMe
func determineDriveType(data map[string]interface{}, rotationRate int) string {
	// Check for NVMe
	if device, ok := data["device"].(map[string]interface{}); ok {
		if protocol, ok := device["protocol"].(string); ok && protocol == "NVMe" {
			return DriveTypeNVMe
		}
		if dtype, ok := device["type"].(string); ok && dtype == "nvme" {
			return DriveTypeNVMe
		}
	}

	// Check for NVMe health log
	if _, ok := data["nvme_smart_health_information_log"].(map[string]interface{}); ok {
		return DriveTypeNVMe
	}

	// Check rotation rate for SSD vs HDD
	if rotationRate == 0 {
		return DriveTypeSSD
	}

	return DriveTypeHDD
}

// parseATAAttributes extracts ATA SMART attributes
func parseATAAttributes(data map[string]interface{}, result *DriveSmartData) {
	ataSmartAttrs, ok := data["ata_smart_attributes"].(map[string]interface{})
	if !ok {
		return
	}

	table, ok := ataSmartAttrs["table"].([]interface{})
	if !ok {
		return
	}

	for _, attrInterface := range table {
		attr, ok := attrInterface.(map[string]interface{})
		if !ok {
			continue
		}

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
			if rawStr, ok := raw["string"].(string); ok {
				smartAttr.RawString = rawStr
			}
		}

		// Parse flags
		if flags, ok := attr["flags"].(map[string]interface{}); ok {
			if flagStr, ok := flags["string"].(string); ok {
				smartAttr.Flags = flagStr
			}
		}

		// Parse when_failed
		if whenFailed, ok := attr["when_failed"].(string); ok && whenFailed != "" {
			smartAttr.WhenFailed = whenFailed
		}

		result.Attributes = append(result.Attributes, smartAttr)

		// Update result fields based on specific attributes
		updateResultFromAttribute(result, smartAttr)
	}
}

// parseNVMeAttributes extracts NVMe-specific SMART data
func parseNVMeAttributes(data map[string]interface{}, result *DriveSmartData) {
	nvmeData, ok := data["nvme_smart_health_information_log"].(map[string]interface{})
	if !ok {
		return
	}

	// Temperature
	if temp, ok := nvmeData["temperature"].(float64); ok {
		result.Temperature = int(temp)
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        NVMeAttrTemperature,
			Name:      "Temperature",
			RawValue:  int64(temp),
			Timestamp: result.Timestamp,
		})
	}

	// Critical Warning
	if warning, ok := nvmeData["critical_warning"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        NVMeAttrCriticalWarning,
			Name:      "Critical Warning",
			RawValue:  int64(warning),
			Timestamp: result.Timestamp,
		})
	}

	// Available Spare
	if spare, ok := nvmeData["available_spare"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        NVMeAttrAvailableSpare,
			Name:      "Available Spare",
			Value:     int(spare),
			RawValue:  int64(spare),
			Timestamp: result.Timestamp,
		})
	}

	// Available Spare Threshold
	if spareThresh, ok := nvmeData["available_spare_threshold"].(float64); ok {
		// Update the Available Spare attribute with threshold
		for i := range result.Attributes {
			if result.Attributes[i].ID == NVMeAttrAvailableSpare {
				result.Attributes[i].Threshold = int(spareThresh)
				break
			}
		}
	}

	// Percentage Used
	if used, ok := nvmeData["percentage_used"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        NVMeAttrPercentageUsed,
			Name:      "Percentage Used",
			RawValue:  int64(used),
			Timestamp: result.Timestamp,
		})
	}

	// Data Units Written
	if written, ok := nvmeData["data_units_written"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        NVMeAttrDataUnitsWritten,
			Name:      "Data Units Written",
			RawValue:  int64(written),
			Timestamp: result.Timestamp,
		})
	}

	// Data Units Read
	if read, ok := nvmeData["data_units_read"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        NVMeAttrDataUnitsRead,
			Name:      "Data Units Read",
			RawValue:  int64(read),
			Timestamp: result.Timestamp,
		})
	}

	// Power Cycles
	if cycles, ok := nvmeData["power_cycles"].(float64); ok {
		result.PowerCycles = int64(cycles)
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        NVMeAttrPowerCycles,
			Name:      "Power Cycles",
			RawValue:  int64(cycles),
			Timestamp: result.Timestamp,
		})
	}

	// Power On Hours
	if hours, ok := nvmeData["power_on_hours"].(float64); ok {
		result.PowerOnHours = int64(hours)
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        NVMeAttrPowerOnHours,
			Name:      "Power On Hours",
			RawValue:  int64(hours),
			Timestamp: result.Timestamp,
		})
	}

	// Media Errors
	if errors, ok := nvmeData["media_errors"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        NVMeAttrMediaErrors,
			Name:      "Media Errors",
			RawValue:  int64(errors),
			Timestamp: result.Timestamp,
		})
	}

	// Unsafe Shutdowns
	if shutdowns, ok := nvmeData["unsafe_shutdowns"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        174, // Using a pseudo-ID
			Name:      "Unsafe Shutdowns",
			RawValue:  int64(shutdowns),
			Timestamp: result.Timestamp,
		})
	}

	// Controller Busy Time
	if busyTime, ok := nvmeData["controller_busy_time"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        175, // Using a pseudo-ID
			Name:      "Controller Busy Time",
			RawValue:  int64(busyTime),
			Timestamp: result.Timestamp,
		})
	}

	// Host Read Commands
	if readCmds, ok := nvmeData["host_reads"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        176, // Using a pseudo-ID
			Name:      "Host Read Commands",
			RawValue:  int64(readCmds),
			Timestamp: result.Timestamp,
		})
	}

	// Host Write Commands
	if writeCmds, ok := nvmeData["host_writes"].(float64); ok {
		result.Attributes = append(result.Attributes, SmartAttribute{
			ID:        178, // Using a pseudo-ID
			Name:      "Host Write Commands",
			RawValue:  int64(writeCmds),
			Timestamp: result.Timestamp,
		})
	}
}

// updateResultFromAttribute updates result fields based on specific attributes
func updateResultFromAttribute(result *DriveSmartData, attr SmartAttribute) {
	switch attr.ID {
	case 194, 190: // Temperature
		if result.Temperature == 0 {
			result.Temperature = int(attr.RawValue)
		}
	case 9: // Power-On Hours
		result.PowerOnHours = attr.RawValue
	case 12: // Power Cycle Count
		result.PowerCycles = attr.RawValue
	}
}

// extractAdditionalMetrics extracts metrics that might be at the root level
func extractAdditionalMetrics(data map[string]interface{}, result *DriveSmartData) {
	// Temperature from root level
	if result.Temperature == 0 {
		if temp, ok := data["temperature"].(map[string]interface{}); ok {
			if current, ok := temp["current"].(float64); ok {
				result.Temperature = int(current)
			}
		}
	}

	// Power-on time from root level
	if result.PowerOnHours == 0 {
		if pot, ok := data["power_on_time"].(map[string]interface{}); ok {
			if hours, ok := pot["hours"].(float64); ok {
				result.PowerOnHours = int64(hours)
			}
		}
	}

	// Power cycle count from root level
	if result.PowerCycles == 0 {
		if pcc, ok := data["power_cycle_count"].(float64); ok {
			result.PowerCycles = int64(pcc)
		}
	}
}

// IsCriticalAttribute checks if an attribute ID is critical
func IsCriticalAttribute(id int) bool {
	def, exists := CriticalAttributeDefinitions[id]
	if !exists {
		return false
	}
	return def.Severity == SeverityCritical
}

// IsWarningAttribute checks if an attribute ID is a warning indicator
func IsWarningAttribute(id int) bool {
	def, exists := CriticalAttributeDefinitions[id]
	if !exists {
		return false
	}
	return def.Severity == SeverityWarning
}

// GetAttributeDefinition returns the definition for an attribute ID
func GetAttributeDefinition(id int) (CriticalAttribute, bool) {
	def, exists := CriticalAttributeDefinitions[id]
	return def, exists
}

// GetAttributeSeverity determines severity level of an attribute based on its value
func GetAttributeSeverity(id int, rawValue int64, value int, threshold int) string {
	def, exists := CriticalAttributeDefinitions[id]

	// Check if normalized value has hit threshold (SMART failure)
	if threshold > 0 && value > 0 && value <= threshold {
		return SeverityCritical
	}

	if !exists {
		return SeverityHealthy
	}

	// Check attribute-specific conditions
	switch id {
	// Critical sector/error counts - any value > 0 is bad
	case 5, 10, 196, 197, 198, 187, 188, 181, 182, 183, 184:
		if rawValue > 0 {
			return SeverityCritical
		}

	// Temperature monitoring
	case 194, 190:
		if rawValue > 65 {
			return SeverityCritical
		}
		if rawValue > 55 {
			return SeverityWarning
		}
		if rawValue > 45 {
			return SeverityInfo
		}

	// CRC errors - usually cable issues
	case 199:
		if rawValue > 100 {
			return SeverityCritical
		}
		if rawValue > 0 {
			return SeverityWarning
		}

	// Available Reserved Space (higher is better)
	case 232:
		if rawValue < 10 {
			return SeverityCritical
		}
		if rawValue < 20 {
			return SeverityWarning
		}

	// Media Wearout Indicator (percentage used)
	case 233:
		if rawValue > 95 {
			return SeverityCritical
		}
		if rawValue > 80 {
			return SeverityWarning
		}
		if rawValue > 50 {
			return SeverityInfo
		}

	// Wear Leveling Count (higher is better, usually 0-100)
	case 177:
		if value < 10 {
			return SeverityCritical
		}
		if value < 20 {
			return SeverityWarning
		}

	// Calibration retry, read/seek errors
	case 1, 7, 11, 200, 201:
		if def.FailureThreshold != nil && rawValue > int64(*def.FailureThreshold) {
			return SeverityWarning
		}
	}

	// If attribute is defined as INFO level
	if def.Severity == SeverityInfo {
		return SeverityInfo
	}

	return SeverityHealthy
}

// AnalyzeDriveHealth performs comprehensive health analysis on drive data
func AnalyzeDriveHealth(driveData *DriveSmartData) *DriveHealthAnalysis {
	analysis := &DriveHealthAnalysis{
		Hostname:      driveData.Hostname,
		SerialNumber:  driveData.SerialNumber,
		ModelName:     driveData.ModelName,
		DriveType:     driveData.DriveType,
		OverallHealth: SeverityHealthy,
		SmartPassed:   driveData.SmartPassed,
		Issues:        make([]HealthIssue, 0),
		Timestamp:     driveData.Timestamp,
	}

	// If SMART failed, that's critical
	if !driveData.SmartPassed {
		analysis.OverallHealth = SeverityCritical
		analysis.CriticalCount++
		analysis.Issues = append(analysis.Issues, HealthIssue{
			AttributeID:   0,
			AttributeName: "SMART Status",
			Severity:      SeverityCritical,
			RawValue:      0,
			Message:       "SMART overall health check FAILED",
		})
	}

	// Analyze each attribute
	for _, attr := range driveData.Attributes {
		severity := GetAttributeSeverity(attr.ID, attr.RawValue, attr.Value, attr.Threshold)

		switch severity {
		case SeverityCritical:
			analysis.CriticalCount++
			analysis.Issues = append(analysis.Issues, HealthIssue{
				AttributeID:   attr.ID,
				AttributeName: attr.Name,
				Severity:      SeverityCritical,
				RawValue:      attr.RawValue,
				Threshold:     attr.Threshold,
				Message:       generateIssueMessage(attr, severity),
			})
		case SeverityWarning:
			analysis.WarningCount++
			analysis.Issues = append(analysis.Issues, HealthIssue{
				AttributeID:   attr.ID,
				AttributeName: attr.Name,
				Severity:      SeverityWarning,
				RawValue:      attr.RawValue,
				Threshold:     attr.Threshold,
				Message:       generateIssueMessage(attr, severity),
			})
		}
	}

	// Determine overall health
	if analysis.CriticalCount > 0 {
		analysis.OverallHealth = SeverityCritical
	} else if analysis.WarningCount > 0 {
		analysis.OverallHealth = SeverityWarning
	}

	return analysis
}

// DriveHealthAnalysis represents comprehensive health analysis results
type DriveHealthAnalysis struct {
	Hostname      string        `json:"hostname"`
	SerialNumber  string        `json:"serial_number"`
	ModelName     string        `json:"model_name"`
	DriveType     string        `json:"drive_type"`
	OverallHealth string        `json:"overall_health"`
	SmartPassed   bool          `json:"smart_passed"`
	CriticalCount int           `json:"critical_count"`
	WarningCount  int           `json:"warning_count"`
	Issues        []HealthIssue `json:"issues"`
	Timestamp     time.Time     `json:"timestamp"`
}

// HealthIssue represents a single health issue detected
type HealthIssue struct {
	AttributeID   int    `json:"attribute_id"`
	AttributeName string `json:"attribute_name"`
	Severity      string `json:"severity"`
	RawValue      int64  `json:"raw_value"`
	Threshold     int    `json:"threshold,omitempty"`
	Message       string `json:"message"`
}

// generateIssueMessage creates a human-readable message for an issue
func generateIssueMessage(attr SmartAttribute, severity string) string {
	def, exists := CriticalAttributeDefinitions[attr.ID]

	var message string
	if exists {
		switch attr.ID {
		case 5:
			message = fmt.Sprintf("%d sectors have been reallocated due to defects", attr.RawValue)
		case 197:
			message = fmt.Sprintf("%d sectors are pending reallocation", attr.RawValue)
		case 198:
			message = fmt.Sprintf("%d uncorrectable sectors found offline", attr.RawValue)
		case 187:
			message = fmt.Sprintf("%d uncorrectable errors reported", attr.RawValue)
		case 188:
			message = fmt.Sprintf("%d command timeouts occurred", attr.RawValue)
		case 194, 190:
			message = fmt.Sprintf("Temperature is %d°C", attr.RawValue)
		case 199:
			message = fmt.Sprintf("%d CRC errors detected (check cables)", attr.RawValue)
		case 232:
			message = fmt.Sprintf("Only %d%% reserved space remaining", attr.RawValue)
		case 233:
			message = fmt.Sprintf("Drive is %d%% worn", attr.RawValue)
		default:
			message = fmt.Sprintf("%s: raw value %d", def.Name, attr.RawValue)
		}
	} else {
		message = fmt.Sprintf("%s has value %d", attr.Name, attr.RawValue)
	}

	switch severity {
	case SeverityCritical:
		message += " - CRITICAL"
	case SeverityWarning:
		message += " - needs attention"
	}

	return message
}

// Helper function to create int pointer
func intPtr(i int) *int {
	return &i
}
