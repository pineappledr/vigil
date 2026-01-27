package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"vigil/internal/db"
)

// Report handles incoming agent reports
func Report(w http.ResponseWriter, r *http.Request) {
	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	hostname, ok := payload["hostname"].(string)
	if !ok || hostname == "" {
		JSONError(w, "Missing hostname", http.StatusBadRequest)
		return
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		JSONError(w, "Failed to process data", http.StatusInternalServerError)
		return
	}

	if _, err = db.DB.Exec("INSERT INTO reports (hostname, data) VALUES (?, ?)", hostname, string(jsonData)); err != nil {
		log.Printf("❌ DB Write Error: %v", err)
		JSONError(w, "Database Error", http.StatusInternalServerError)
		return
	}

	driveCount := 0
	if drives, ok := payload["drives"].([]interface{}); ok {
		driveCount = len(drives)
	}

	log.Printf("💾 Report: %s (%d drives)", hostname, driveCount)
	JSONResponse(w, map[string]string{"status": "ok"})
}

// History returns latest reports for all hosts with aliases
func History(w http.ResponseWriter, r *http.Request) {
	aliases := loadAliases()

	query := `
	SELECT hostname, timestamp, data 
	FROM reports 
	WHERE id IN (SELECT MAX(id) FROM reports GROUP BY hostname)
	ORDER BY timestamp DESC`

	rows, err := db.DB.Query(query)
	if err != nil {
		JSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	history := make([]map[string]interface{}, 0)
	for rows.Next() {
		var host, ts string
		var dataRaw []byte
		if err := rows.Scan(&host, &ts, &dataRaw); err != nil {
			continue
		}

		var dataMap map[string]interface{}
		json.Unmarshal(dataRaw, &dataMap)
		enrichDrivesWithAliases(dataMap, host, aliases)

		history = append(history, map[string]interface{}{
			"hostname":  host,
			"timestamp": ts,
			"details":   dataMap,
		})
	}

	JSONResponse(w, history)
}

// Hosts returns list of all hosts
func Hosts(w http.ResponseWriter, r *http.Request) {
	query := `
	SELECT hostname, MAX(timestamp) as last_seen, COUNT(*) as report_count
	FROM reports GROUP BY hostname ORDER BY last_seen DESC`

	rows, err := db.DB.Query(query)
	if err != nil {
		JSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	hosts := make([]map[string]interface{}, 0)
	for rows.Next() {
		var hostname, lastSeen string
		var reportCount int
		if err := rows.Scan(&hostname, &lastSeen, &reportCount); err != nil {
			continue
		}
		hosts = append(hosts, map[string]interface{}{
			"hostname":     hostname,
			"last_seen":    lastSeen,
			"report_count": reportCount,
		})
	}

	JSONResponse(w, hosts)
}

// DeleteHost removes a host and its data
func DeleteHost(w http.ResponseWriter, r *http.Request) {
	hostname := strings.TrimSpace(r.PathValue("hostname"))
	if hostname == "" {
		JSONError(w, "Missing hostname", http.StatusBadRequest)
		return
	}

	result, err := db.DB.Exec("DELETE FROM reports WHERE hostname = ?", hostname)
	if err != nil {
		JSONError(w, "Database error", http.StatusInternalServerError)
		return
	}

	affected, _ := result.RowsAffected()
	if affected == 0 {
		JSONError(w, "Host not found", http.StatusNotFound)
		return
	}

	db.DB.Exec("DELETE FROM drive_aliases WHERE hostname = ?", hostname)

	log.Printf("🗑️  Deleted host: %s (%d reports)", hostname, affected)
	JSONResponse(w, map[string]interface{}{
		"status":  "deleted",
		"deleted": affected,
	})
}

// HostHistory returns history for a specific host
func HostHistory(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	if hostname == "" {
		JSONError(w, "Missing hostname", http.StatusBadRequest)
		return
	}

	rows, err := db.DB.Query(
		"SELECT timestamp, data FROM reports WHERE hostname = ? ORDER BY timestamp DESC LIMIT 50",
		hostname,
	)
	if err != nil {
		JSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	history := make([]map[string]interface{}, 0)
	for rows.Next() {
		var ts string
		var dataRaw []byte
		if err := rows.Scan(&ts, &dataRaw); err != nil {
			continue
		}

		var dataMap map[string]interface{}
		json.Unmarshal(dataRaw, &dataMap)

		history = append(history, map[string]interface{}{
			"timestamp": ts,
			"details":   dataMap,
		})
	}

	JSONResponse(w, history)
}

// Helper functions

func loadAliases() map[string]string {
	aliases := make(map[string]string)
	rows, _ := db.DB.Query("SELECT hostname, serial_number, alias FROM drive_aliases")
	if rows == nil {
		return aliases
	}
	defer rows.Close()

	for rows.Next() {
		var hostname, serial, alias string
		if rows.Scan(&hostname, &serial, &alias) == nil {
			aliases[hostname+":"+serial] = alias
		}
	}
	return aliases
}

func enrichDrivesWithAliases(data map[string]interface{}, hostname string, aliases map[string]string) {
	drives, ok := data["drives"].([]interface{})
	if !ok {
		return
	}

	for i, d := range drives {
		drive, ok := d.(map[string]interface{})
		if !ok {
			continue
		}
		if serial, ok := drive["serial_number"].(string); ok {
			if alias, exists := aliases[hostname+":"+serial]; exists {
				drive["_alias"] = alias
				drives[i] = drive
			}
		}
	}
	data["drives"] = drives
}
