package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

// DownloadBackup serves a backup file for download.
// GET /api/backups/{filename}/download
func DownloadBackup(w http.ResponseWriter, r *http.Request) {
	filename := r.PathValue("filename")
	if filename == "" || strings.ContainsAny(filename, `/\`) || strings.Contains(filename, "..") {
		JSONError(w, "Invalid filename", http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(filename, "vigil-backup-") || !strings.HasSuffix(filename, ".db") {
		JSONError(w, "Invalid backup filename", http.StatusBadRequest)
		return
	}

	path := filepath.Join(BackupDir, filename)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			JSONError(w, "Backup not found", http.StatusNotFound)
		} else {
			JSONError(w, "Failed to open backup", http.StatusInternalServerError)
		}
		return
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		JSONError(w, "Failed to read backup", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
	w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
	io.Copy(w, f)
}

// RestoreBackup replaces the current database with an uploaded backup.
// POST /api/backups/restore
func RestoreBackup(w http.ResponseWriter, r *http.Request) {
	// Limit upload to 500MB
	r.Body = http.MaxBytesReader(w, r.Body, 500<<20)

	file, header, err := r.FormFile("backup")
	if err != nil {
		JSONError(w, "Missing backup file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	if !strings.HasSuffix(header.Filename, ".db") {
		JSONError(w, "File must be a .db file", http.StatusBadRequest)
		return
	}

	// Save uploaded file to a temp location in the backup dir
	if err := os.MkdirAll(BackupDir, 0755); err != nil {
		JSONError(w, "Failed to prepare restore", http.StatusInternalServerError)
		return
	}

	tmpPath := filepath.Join(BackupDir, "restore-upload.db.tmp")
	out, err := os.Create(tmpPath)
	if err != nil {
		JSONError(w, "Failed to save upload", http.StatusInternalServerError)
		return
	}
	if _, err := io.Copy(out, file); err != nil {
		out.Close()
		os.Remove(tmpPath)
		JSONError(w, "Failed to save upload", http.StatusInternalServerError)
		return
	}
	out.Close()

	// Validate the uploaded file is a valid SQLite database
	testDB, err := db.OpenValidate(tmpPath)
	if err != nil {
		os.Remove(tmpPath)
		JSONError(w, "Invalid database file: "+err.Error(), http.StatusBadRequest)
		return
	}
	testDB.Close()

	// Create a safety backup of the current database before restoring
	safetyName := fmt.Sprintf("vigil-pre-restore-%s.db", time.Now().UTC().Format("20060102-150405"))
	safetyPath := filepath.Join(BackupDir, safetyName)
	if _, err := db.DB.Exec("VACUUM INTO ?", safetyPath); err != nil {
		log.Printf("⚠️  Pre-restore safety backup failed: %v", err)
		// Continue anyway — the user explicitly asked to restore
	} else {
		log.Printf("💾 Pre-restore safety backup: %s", safetyName)
	}

	// Close current DB, replace, reopen
	dbPath := DBPath
	db.DB.Close()

	if err := os.Rename(tmpPath, dbPath); err != nil {
		// Try to reopen original
		db.Init(dbPath)
		JSONError(w, "Failed to replace database: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := db.Init(dbPath); err != nil {
		JSONError(w, "Failed to reopen database after restore: "+err.Error(), http.StatusInternalServerError)
		return
	}

	log.Printf("✅ Database restored from upload: %s (%d bytes)", header.Filename, header.Size)
	JSONResponse(w, map[string]string{"status": "restored", "safety_backup": safetyName})
}

// RegisterBackupRoutes registers backup API routes.
func RegisterBackupRoutes(mux *http.ServeMux, protect func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /api/backup", protect(TriggerBackup))
	mux.HandleFunc("GET /api/backups", protect(ListBackups))
	mux.HandleFunc("DELETE /api/backups/{filename}", protect(DeleteBackupFile))
	mux.HandleFunc("GET /api/backups/{filename}/download", protect(DownloadBackup))
	mux.HandleFunc("POST /api/backups/restore", protect(RestoreBackup))
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
