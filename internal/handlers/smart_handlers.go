package handlers

import (
	"net/http"
	"strconv"

	"vigil/internal/db"
)

// GetSmartAttributes returns the latest SMART attributes for a drive
func GetSmartAttributes(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serialNumber := r.URL.Query().Get("serial")

	if hostname == "" || serialNumber == "" {
		JSONError(w, "Missing hostname or serial number", http.StatusBadRequest)
		return
	}

	attributes, err := db.GetLatestSmartAttributes(hostname, serialNumber)
	if err != nil {
		JSONError(w, "Failed to retrieve SMART attributes: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Get additional drive info
	driveInfo, _ := db.GetDriveInfo(hostname, serialNumber)

	response := map[string]interface{}{
		"hostname":        hostname,
		"serial_number":   serialNumber,
		"attributes":      attributes,
		"attribute_count": len(attributes),
	}

	if driveInfo != nil {
		response["model_name"] = driveInfo.ModelName
		response["drive_type"] = driveInfo.DriveType
		response["smart_passed"] = driveInfo.SmartPassed
	}

	JSONResponse(w, response)
}

// GetSmartAttributeHistory returns historical data for a specific attribute
func GetSmartAttributeHistory(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serialNumber := r.URL.Query().Get("serial")
	attrIDStr := r.URL.Query().Get("attribute_id")
	limitStr := r.URL.Query().Get("limit")

	if hostname == "" || serialNumber == "" || attrIDStr == "" {
		JSONError(w, "Missing required parameters (hostname, serial, attribute_id)", http.StatusBadRequest)
		return
	}

	attrID, err := strconv.Atoi(attrIDStr)
	if err != nil {
		JSONError(w, "Invalid attribute ID", http.StatusBadRequest)
		return
	}

	limit := 100 // Default limit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	history, err := db.GetSmartAttributeHistory(hostname, serialNumber, attrID, limit)
	if err != nil {
		JSONError(w, "Failed to retrieve attribute history: "+err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]interface{}{
		"hostname":      hostname,
		"serial_number": serialNumber,
		"attribute_id":  attrID,
		"history":       history,
		"data_points":   len(history),
	})
}

// GetSmartAttributeTrend returns trend analysis for a specific attribute
func GetSmartAttributeTrend(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serialNumber := r.URL.Query().Get("serial")
	attrIDStr := r.URL.Query().Get("attribute_id")
	daysStr := r.URL.Query().Get("days")

	if hostname == "" || serialNumber == "" || attrIDStr == "" {
		JSONError(w, "Missing required parameters (hostname, serial, attribute_id)", http.StatusBadRequest)
		return
	}

	attrID, err := strconv.Atoi(attrIDStr)
	if err != nil {
		JSONError(w, "Invalid attribute ID", http.StatusBadRequest)
		return
	}

	days := 30 // Default to 30 days
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 && d <= 365 {
			days = d
		}
	}

	trend, err := db.GetAttributeTrend(hostname, serialNumber, attrID, days)
	if err != nil {
		JSONError(w, "Failed to retrieve attribute trend: "+err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, trend)
}

// GetDriveHealthSummary returns comprehensive health analysis for a drive
func GetDriveHealthSummary(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serialNumber := r.URL.Query().Get("serial")

	if hostname == "" || serialNumber == "" {
		JSONError(w, "Missing hostname or serial number", http.StatusBadRequest)
		return
	}

	summary, err := db.GetDriveHealthSummary(hostname, serialNumber)
	if err != nil {
		JSONError(w, "Failed to retrieve health summary: "+err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, summary)
}

// GetCriticalAttributes returns the list of critical SMART attributes and their definitions
func GetCriticalAttributes(w http.ResponseWriter, r *http.Request) {
	attributes, err := db.GetCriticalSmartAttributes()
	if err != nil {
		JSONError(w, "Failed to retrieve critical attributes: "+err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]interface{}{
		"attributes": attributes,
		"count":      len(attributes),
	})
}

// GetAllDrivesHealthSummary returns health summaries for all monitored drives
func GetAllDrivesHealthSummary(w http.ResponseWriter, r *http.Request) {
	summaries, err := db.GetAllDrivesHealthSummary()
	if err != nil {
		JSONError(w, "Failed to retrieve health summaries: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Calculate aggregate stats
	totalDrives := len(summaries)
	healthyCount := 0
	warningCount := 0
	criticalCount := 0

	for _, s := range summaries {
		switch s.OverallHealth {
		case "HEALTHY":
			healthyCount++
		case "WARNING":
			warningCount++
		case "CRITICAL":
			criticalCount++
		}
	}

	JSONResponse(w, map[string]interface{}{
		"summaries":      summaries,
		"total_drives":   totalDrives,
		"healthy_count":  healthyCount,
		"warning_count":  warningCount,
		"critical_count": criticalCount,
	})
}

// GetTemperatureHistory returns temperature history for a drive
func GetTemperatureHistory(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serialNumber := r.URL.Query().Get("serial")
	hoursStr := r.URL.Query().Get("hours")

	if hostname == "" || serialNumber == "" {
		JSONError(w, "Missing hostname or serial number", http.StatusBadRequest)
		return
	}

	hours := 24 // Default to last 24 hours
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 && h <= 720 { // Max 30 days
			hours = h
		}
	}

	history, err := db.GetTemperatureHistory(hostname, serialNumber, hours)
	if err != nil {
		JSONError(w, "Failed to retrieve temperature history: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Calculate stats
	var minTemp, maxTemp, avgTemp int
	if len(history) > 0 {
		minTemp = history[0].Temperature
		maxTemp = history[0].Temperature
		sum := 0

		for _, record := range history {
			if record.Temperature < minTemp {
				minTemp = record.Temperature
			}
			if record.Temperature > maxTemp {
				maxTemp = record.Temperature
			}
			sum += record.Temperature
		}
		avgTemp = sum / len(history)
	}

	JSONResponse(w, map[string]interface{}{
		"hostname":      hostname,
		"serial_number": serialNumber,
		"hours":         hours,
		"history":       history,
		"data_points":   len(history),
		"min_temp":      minTemp,
		"max_temp":      maxTemp,
		"avg_temp":      avgTemp,
	})
}

// CleanupOldSmartData removes old SMART data (admin endpoint)
func CleanupOldSmartData(w http.ResponseWriter, r *http.Request) {
	daysStr := r.URL.Query().Get("days")
	days := 90 // Default to 90 days

	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	deleted, err := db.CleanupOldSmartData(days)
	if err != nil {
		JSONError(w, "Failed to cleanup old data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]interface{}{
		"deleted_records": deleted,
		"days_kept":       days,
		"status":          "success",
	})
}

// GetDrivesWithIssues returns all drives that have health issues
func GetDrivesWithIssues(w http.ResponseWriter, r *http.Request) {
	summaries, err := db.GetAllDrivesHealthSummary()
	if err != nil {
		JSONError(w, "Failed to retrieve health summaries: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Filter to only drives with issues
	var drivesWithIssues []*struct {
		Hostname      string `json:"hostname"`
		SerialNumber  string `json:"serial_number"`
		ModelName     string `json:"model_name"`
		OverallHealth string `json:"overall_health"`
		CriticalCount int    `json:"critical_count"`
		WarningCount  int    `json:"warning_count"`
		TopIssue      string `json:"top_issue"`
	}

	for _, s := range summaries {
		if s.OverallHealth != "HEALTHY" {
			topIssue := ""
			if len(s.Issues) > 0 {
				topIssue = s.Issues[0].Message
			}

			drivesWithIssues = append(drivesWithIssues, &struct {
				Hostname      string `json:"hostname"`
				SerialNumber  string `json:"serial_number"`
				ModelName     string `json:"model_name"`
				OverallHealth string `json:"overall_health"`
				CriticalCount int    `json:"critical_count"`
				WarningCount  int    `json:"warning_count"`
				TopIssue      string `json:"top_issue"`
			}{
				Hostname:      s.Hostname,
				SerialNumber:  s.SerialNumber,
				ModelName:     s.ModelName,
				OverallHealth: s.OverallHealth,
				CriticalCount: s.CriticalCount,
				WarningCount:  s.WarningCount,
				TopIssue:      topIssue,
			})
		}
	}

	JSONResponse(w, map[string]interface{}{
		"drives": drivesWithIssues,
		"count":  len(drivesWithIssues),
	})
}
