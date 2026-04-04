package backup

import (
	"compress/gzip"
	"database/sql"
	"fmt"
	"io"
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

// RunBackup creates a database backup using VACUUM INTO (safe with WAL mode),
// then gzip-compresses the result. It rotates old backups, keeping at most maxBackups files.
func RunBackup(db *sql.DB, backupDir string, maxBackups int) (BackupInfo, error) {
	if err := os.MkdirAll(backupDir, 0755); err != nil {
		return BackupInfo{}, fmt.Errorf("create backup dir: %w", err)
	}

	ts := time.Now().UTC().Format("20060102-150405")
	rawName := fmt.Sprintf("vigil-backup-%s.db", ts)
	rawDest := filepath.Join(backupDir, rawName)

	if _, err := db.Exec("VACUUM INTO ?", rawDest); err != nil {
		return BackupInfo{}, fmt.Errorf("VACUUM INTO: %w", err)
	}

	// Compress the raw backup
	gzName := rawName + ".gz"
	gzDest := filepath.Join(backupDir, gzName)
	if err := compressFile(rawDest, gzDest); err != nil {
		// If compression fails, keep the raw file
		info, _ := os.Stat(rawDest)
		return BackupInfo{Filename: rawName, SizeBytes: info.Size(), CreatedAt: info.ModTime().UTC()}, nil
	}
	os.Remove(rawDest) // remove uncompressed original

	info, err := os.Stat(gzDest)
	if err != nil {
		return BackupInfo{}, fmt.Errorf("stat backup: %w", err)
	}

	result := BackupInfo{
		Filename:  gzName,
		SizeBytes: info.Size(),
		CreatedAt: info.ModTime().UTC(),
	}

	// Rotate old backups
	if maxBackups > 0 {
		if err := rotate(backupDir, maxBackups); err != nil {
			fmt.Printf("backup: rotation warning: %v\n", err)
		}
	}

	return result, nil
}

func compressFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	gz, err := gzip.NewWriterLevel(out, gzip.BestCompression)
	if err != nil {
		return err
	}
	if _, err := io.Copy(gz, in); err != nil {
		gz.Close()
		return err
	}
	return gz.Close()
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
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "vigil-backup-") {
			continue
		}
		if !strings.HasSuffix(name, ".db") && !strings.HasSuffix(name, ".db.gz") {
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
	if !strings.HasPrefix(filename, "vigil-backup-") {
		return fmt.Errorf("invalid backup filename")
	}
	if !strings.HasSuffix(filename, ".db") && !strings.HasSuffix(filename, ".db.gz") {
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
