package backup

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BackupInfo describes a single backup file.
type BackupInfo struct {
	Filename  string    `json:"filename"`
	SizeBytes int64     `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

// RunBackup creates a database backup using VACUUM INTO (safe with WAL mode).
// It rotates old backups, keeping at most maxBackups files.
func RunBackup(db *sql.DB, backupDir string, maxBackups int) (BackupInfo, error) {
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return BackupInfo{}, fmt.Errorf("create backup dir: %w", err)
	}

	filename := fmt.Sprintf("vigil-backup-%s.db", time.Now().UTC().Format("20060102-150405"))
	dest := filepath.Join(backupDir, filename)

	if _, err := db.Exec("VACUUM INTO ?", dest); err != nil {
		return BackupInfo{}, fmt.Errorf("VACUUM INTO: %w", err)
	}

	info, err := os.Stat(dest)
	if err != nil {
		return BackupInfo{}, fmt.Errorf("stat backup: %w", err)
	}

	result := BackupInfo{
		Filename:  filename,
		SizeBytes: info.Size(),
		CreatedAt: info.ModTime().UTC(),
	}

	// Rotate old backups
	if maxBackups > 0 {
		if err := rotate(backupDir, maxBackups); err != nil {
			// Non-fatal: backup succeeded, rotation failed
			fmt.Printf("backup: rotation warning: %v\n", err)
		}
	}

	return result, nil
}

// ListBackups returns all backup files in the directory, newest first.
func ListBackups(backupDir string) ([]BackupInfo, error) {
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []BackupInfo{}, nil
		}
		return nil, fmt.Errorf("read backup dir: %w", err)
	}

	var backups []BackupInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasPrefix(e.Name(), "vigil-backup-") || !strings.HasSuffix(e.Name(), ".db") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		backups = append(backups, BackupInfo{
			Filename:  e.Name(),
			SizeBytes: info.Size(),
			CreatedAt: info.ModTime().UTC(),
		})
	}

	// Sort newest first
	sort.Slice(backups, func(i, j int) bool {
		return backups[i].Filename > backups[j].Filename
	})

	return backups, nil
}

// DeleteBackup removes a backup file. The filename must not contain path separators.
func DeleteBackup(backupDir, filename string) error {
	if strings.ContainsAny(filename, `/\`) || strings.Contains(filename, "..") {
		return fmt.Errorf("invalid filename")
	}
	if !strings.HasPrefix(filename, "vigil-backup-") || !strings.HasSuffix(filename, ".db") {
		return fmt.Errorf("invalid backup filename")
	}
	return os.Remove(filepath.Join(backupDir, filename))
}

func rotate(backupDir string, maxBackups int) error {
	backups, err := ListBackups(backupDir)
	if err != nil {
		return err
	}
	if len(backups) <= maxBackups {
		return nil
	}
	// Remove oldest (backups are sorted newest-first)
	for _, b := range backups[maxBackups:] {
		os.Remove(filepath.Join(backupDir, b.Filename))
	}
	return nil
}
