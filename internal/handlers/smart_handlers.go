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
		JSONError(w, "Failed to retrieve SMART attributes", http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]interface{}{
		"hostname":        hostname,
		"serial_number":   serialNumber,
		"attributes":      attributes,
		"attribute_count": len(attributes),
	})
}

// GetSmartAttributeHistory returns historical data for a specific attribute
func GetSmartAttributeHistory(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")
	serialNumber := r.URL.Query().Get("serial")
	attrIDStr := r.URL.Query().Get("attribute_id")
	limitStr := r.URL.Query().Get("limit")

	if hostname == "" || serialNumber == "" || attrIDStr == "" {
		JSONError(w, "Missing required parameters", http.StatusBadRequest)
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
		JSONError(w, "Failed to retrieve attribute history", http.StatusInternalServerError)
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
		JSONError(w, "Missing required parameters", http.StatusBadRequest)
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
		JSONError(w, "Failed to retrieve attribute trend", http.StatusInternalServerError)
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
		JSONError(w, "Failed to retrieve health summary", http.StatusInternalServerError)
		return
	}

	JSONResponse(w, summary)
}

// GetCriticalAttributes returns the list of critical SMART attributes
func GetCriticalAttributes(w http.ResponseWriter, r *http.Request) {
	attributes, err := db.GetCriticalSmartAttributes()
	if err != nil {
		JSONError(w, "Failed to retrieve critical attributes", http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]interface{}{
		"attributes": attributes,
		"count":      len(attributes),
	})
}

// GetAllDrivesHealthSummary returns health summaries for all drives
func GetAllDrivesHealthSummary(w http.ResponseWriter, r *http.Request) {
	// Get all unique drive serial numbers from reports
	query := `
		SELECT DISTINCT hostname, 
		       json_extract(d.value, '$.serial_number') as serial_number
		FROM reports r,
		     json_each(json_extract(r.data, '$.drives')) as d
		WHERE serial_number IS NOT NULL
		ORDER BY hostname, serial_number
	`

	rows, err := db.DB.Query(query)
	if err != nil {
		JSONError(w, "Failed to query drives", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	summaries := make([]map[string]interface{}, 0)

	for rows.Next() {
		var hostname, serialNumber string
		if err := rows.Scan(&hostname, &serialNumber); err != nil {
			continue
		}

		summary, err := db.GetDriveHealthSummary(hostname, serialNumber)
		if err != nil {
			continue
		}

		summaries = append(summaries, summary)
	}

	JSONResponse(w, map[string]interface{}{
		"summaries": summaries,
		"count":     len(summaries),
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
		JSONError(w, "Failed to cleanup old data", http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]interface{}{
		"deleted_records": deleted,
		"days_kept":       days,
		"status":          "success",
	})
}
