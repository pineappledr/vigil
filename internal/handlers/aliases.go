package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"vigil/internal/db"
	"vigil/internal/models"
)

// GetAliases returns drive aliases, optionally filtered by hostname
func GetAliases(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")

	var query string
	var args []interface{}

	if hostname != "" {
		query = "SELECT id, hostname, serial_number, alias, created_at FROM drive_aliases WHERE hostname = ? ORDER BY alias"
		args = []interface{}{hostname}
	} else {
		query = "SELECT id, hostname, serial_number, alias, created_at FROM drive_aliases ORDER BY hostname, alias"
	}

	rows, err := db.DB.Query(query, args...)
	if err != nil {
		JSONError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	aliases := make([]models.DriveAlias, 0)
	for rows.Next() {
		var a models.DriveAlias
		var createdAt string
		if err := rows.Scan(&a.ID, &a.Hostname, &a.SerialNumber, &a.Alias, &createdAt); err != nil {
			continue
		}
		a.CreatedAt, _ = time.Parse("2006-01-02 15:04:05", createdAt)
		aliases = append(aliases, a)
	}

	JSONResponse(w, aliases)
}

// SetAlias creates or updates a drive alias
func SetAlias(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Hostname     string `json:"hostname"`
		SerialNumber string `json:"serial_number"`
		Alias        string `json:"alias"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if req.Hostname == "" || req.SerialNumber == "" {
		JSONError(w, "Missing hostname or serial_number", http.StatusBadRequest)
		return
	}

	req.Alias = strings.TrimSpace(req.Alias)

	// Delete if alias is empty
	if req.Alias == "" {
		db.DB.Exec("DELETE FROM drive_aliases WHERE hostname = ? AND serial_number = ?",
			req.Hostname, req.SerialNumber)
		JSONResponse(w, map[string]string{"status": "deleted"})
		return
	}

	// Upsert alias
	_, err := db.DB.Exec(`
		INSERT INTO drive_aliases (hostname, serial_number, alias) 
		VALUES (?, ?, ?)
		ON CONFLICT(hostname, serial_number) 
		DO UPDATE SET alias = excluded.alias
	`, req.Hostname, req.SerialNumber, req.Alias)

	if err != nil {
		JSONError(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("📝 Alias set: %s/%s -> %s", req.Hostname, req.SerialNumber, req.Alias)
	JSONResponse(w, map[string]string{"status": "ok"})
}

// DeleteAlias removes a drive alias by ID
func DeleteAlias(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		JSONError(w, "Missing alias ID", http.StatusBadRequest)
		return
	}

	result, err := db.DB.Exec("DELETE FROM drive_aliases WHERE id = ?", id)
	if err != nil {
		JSONError(w, "Database error", http.StatusInternalServerError)
		return
	}

	if affected, _ := result.RowsAffected(); affected == 0 {
		JSONError(w, "Alias not found", http.StatusNotFound)
		return
	}

	JSONResponse(w, map[string]string{"status": "deleted"})
}
