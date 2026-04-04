package handlers

import (
	"log"
	"net/http"
	"path/filepath"
	"time"

	"vigil/internal/backup"
	"vigil/internal/db"
	"vigil/internal/settings"
)

// BackupDir is the directory where backups are stored. Set from main.go.
var BackupDir string

// TriggerBackup runs an immediate database backup.
// POST /api/backup
func TriggerBackup(w http.ResponseWriter, r *http.Request) {
	maxBackups := settings.GetInt(db.DB, "backup", "max_backups", 7)
	info, err := backup.RunBackup(db.DB, BackupDir, maxBackups)
	if err != nil {
		log.Printf("❌ Manual backup failed: %v", err)
		JSONError(w, "Backup failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("💾 Manual backup created: %s (%d bytes)", info.Filename, info.SizeBytes)
	JSONResponse(w, info)
}

// ListBackups returns all existing backup files.
// GET /api/backups
func ListBackups(w http.ResponseWriter, r *http.Request) {
	backups, err := backup.ListBackups(BackupDir)
	if err != nil {
		JSONError(w, "Failed to list backups", http.StatusInternalServerError)
		return
	}
	if backups == nil {
		backups = []backup.BackupInfo{}
	}
	JSONResponse(w, backups)
}

// DeleteBackupFile removes a backup file.
// DELETE /api/backups/{filename}
func DeleteBackupFile(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if filename == "" {
		JSONError(w, "Missing filename", http.StatusBadRequest)
		return
	}

	if err := backup.DeleteBackup(BackupDir, filename); err != nil {
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("🗑️ Backup deleted: %s", filename)
	JSONResponse(w, map[string]string{"status": "deleted"})
}

// RegisterBackupRoutes registers backup API routes.
func RegisterBackupRoutes(mux *http.ServeMux, protect func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /api/backup", protect(TriggerBackup))
	mux.HandleFunc("GET /api/backups", protect(ListBackups))
	mux.HandleFunc("DELETE /api/backups/{filename}", protect(DeleteBackupFile))
}

// RunScheduledBackup checks settings and runs a backup if due.
func RunScheduledBackup(lastBackupUnix *int64) {
	if !settings.GetBool(db.DB, "backup", "enabled", true) {
		return
	}

	intervalHours := settings.GetInt(db.DB, "backup", "interval_hours", 24)
	if intervalHours <= 0 {
		intervalHours = 24
	}

	now := time.Now().Unix()
	if now-*lastBackupUnix < int64(intervalHours*3600) {
		return
	}

	dir := BackupDir
	if dir == "" {
		dir = filepath.Join(".", "backups")
	}

	maxBackups := settings.GetInt(db.DB, "backup", "max_backups", 7)
	info, err := backup.RunBackup(db.DB, dir, maxBackups)
	if err != nil {
		log.Printf("⚠️  Scheduled backup failed: %v", err)
		return
	}

	*lastBackupUnix = now
	log.Printf("💾 Scheduled backup: %s (%d bytes)", info.Filename, info.SizeBytes)
}
