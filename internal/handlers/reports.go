package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"vigil/internal/agents"
	"vigil/internal/audit"
	"vigil/internal/auth"
	"vigil/internal/db"
	"vigil/internal/smart"
	"vigil/internal/validate"
	"vigil/internal/wearout"
)

// reportWork is a unit of background processing enqueued after the HTTP
// response has been sent.  Processing is serialised through a single worker
// goroutine so concurrent SMART / wearout / ZFS writes never compete for
// the SQLite write lock and cannot starve dashboard reads.
type reportWork struct {
	hostname string
	agentID  int64
	payload  map[string]interface{}
}

// reportQueue buffers pending background work.  The buffer is generous so
// the HTTP handler never blocks; if it fills up we drop the work.
var reportQueue = make(chan reportWork, 64)

func init() {
	go reportWorker()
}

func reportWorker() {
	for w := range reportQueue {
		func() {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("⚠️  Report background processing panic for %s: %v", w.hostname, r)
				}
			}()

			if err := agents.UpdateAgentLastSeen(db.DB, w.agentID); err != nil {
				log.Printf("⚠️  Failed to update last_seen_at for agent %d: %v", w.agentID, err)
			}
			if err := agents.UpdateAgentLastSeenByHostname(db.DB, w.hostname); err != nil {
				log.Printf("⚠️  Failed to update agent status by hostname %s: %v", w.hostname, err)
			}

			wearout.ProcessWearoutFromReport(db.DB, EventBus, w.hostname, w.payload)
			smart.ProcessReportWithEvents(db.DB, EventBus, w.hostname, w.payload)

			if _, ok := w.payload["zfs"].(map[string]interface{}); ok {
				ProcessZFSFromReport(w.hostname, w.payload)
			}
		}()
	}
}

// Report handles incoming agent reports.
// Requires a valid agent session token: Authorization: Bearer <token>
func Report(w http.ResponseWriter, r *http.Request) {
	session := GetAgentSessionFromRequest(r)
	if session == nil {
		w.Header().Set("X-Vigil-Auth-Required", "true")
		JSONError(w, "Agent authentication required — obtain a session token via POST /api/v1/agents/auth", http.StatusUnauthorized)
		return
	}

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

	// Store timestamps in UTC for consistency with SQLite datetime('now')
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	if _, err = db.DB.Exec("INSERT INTO reports (hostname, timestamp, data) VALUES (?, ?, ?)", hostname, now, string(jsonData)); err != nil {
		log.Printf("❌ DB Write Error: %v", err)
		JSONError(w, "Database Error", http.StatusInternalServerError)
		return
	}

	// Count drives and pools for logging.
	driveCount := 0
	if drives, ok := payload["drives"].([]interface{}); ok {
		driveCount = len(drives)
	}
	poolCount := 0
	if zfsData, ok := payload["zfs"].(map[string]interface{}); ok {
		if pools, ok := zfsData["pools"].([]interface{}); ok {
			poolCount = len(pools)
		}
	}

	if poolCount > 0 {
		log.Printf("💾 Report: %s (%d drives, %d ZFS pools)", hostname, driveCount, poolCount)
	} else {
		log.Printf("💾 Report: %s (%d drives)", hostname, driveCount)
	}

	// Respond immediately — heavy processing is serialised through a single
	// background worker so it never holds the SQLite write lock while the
	// dashboard is trying to read /api/history.
	JSONResponse(w, map[string]string{"status": "ok"})

	// Enqueue background work (non-blocking; drops if queue is full).
	select {
	case reportQueue <- reportWork{hostname: hostname, agentID: session.AgentID, payload: payload}:
	default:
		log.Printf("⚠️  Report processing queue full, dropping background work for %s", hostname)
	}
}

// History returns latest reports for all hosts with aliases
func History(w http.ResponseWriter, r *http.Request) {
	aliases := loadAliases()

	query := `
	SELECT r.hostname, r.timestamp, r.data,
	       COALESCE(ag.last_seen, r.timestamp) AS last_seen
	FROM reports r
	INNER JOIN (
		SELECT hostname, MAX(id) AS max_id
		FROM reports
		GROUP BY hostname
	) latest ON r.id = latest.max_id
	LEFT JOIN (
		SELECT hostname, MAX(last_seen_at) AS last_seen
		FROM agent_registry
		WHERE enabled = 1
		GROUP BY hostname
	) ag ON LOWER(ag.hostname) = LOWER(r.hostname)
	ORDER BY r.timestamp DESC`

	rows, err := db.DB.Query(query)
	if err != nil {
		JSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	history := make([]map[string]interface{}, 0)
	for rows.Next() {
		var host, ts, lastSeen string
		var dataRaw []byte
		if err := rows.Scan(&host, &ts, &dataRaw, &lastSeen); err != nil {
			continue
		}

		var dataMap map[string]interface{}
		if err := json.Unmarshal(dataRaw, &dataMap); err != nil {
			log.Printf("reports: unmarshal history data for %s: %v", host, err)
			continue
		}
		enrichDrivesWithAliases(dataMap, host, aliases)

		history = append(history, map[string]interface{}{
			"hostname":  host,
			"timestamp": ts,
			"last_seen": lastSeen,
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
	if err := validate.Hostname(hostname); err != nil {
		JSONError(w, err.Error(), http.StatusBadRequest)
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
	if s := auth.GetSessionFromContext(r); s != nil {
		audit.LogEvent(db.DB, r, s.UserID, s.Username, "host_delete", "host", hostname, fmt.Sprintf("%d reports deleted", affected), "success")
	}
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
		if err := json.Unmarshal(dataRaw, &dataMap); err != nil {
			log.Printf("reports: unmarshal host history data: %v", err)
			continue
		}

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
	rows, err := db.DB.Query("SELECT hostname, serial_number, alias FROM drive_aliases")
	if err != nil {
		log.Printf("reports: load aliases: %v", err)
		return aliases
	}
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
