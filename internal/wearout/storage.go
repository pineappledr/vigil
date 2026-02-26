package wearout

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// StoreSnapshot persists a wearout calculation to the database.
func StoreSnapshot(db *sql.DB, s WearoutSnapshot) error {
	_, err := db.Exec(`
		INSERT INTO wearout_history (hostname, serial_number, drive_type, percentage, factors_json, timestamp)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(hostname, serial_number, timestamp) DO UPDATE SET
			percentage   = excluded.percentage,
			factors_json = excluded.factors_json
	`, s.Hostname, s.SerialNumber, s.DriveType, s.Percentage, s.FactorsJSON, s.Timestamp.UTC().Format(timeFormat))
	return err
}

// GetLatestSnapshot returns the most recent wearout snapshot for a drive.
func GetLatestSnapshot(db *sql.DB, hostname, serialNumber string) (*WearoutSnapshot, error) {
	row := db.QueryRow(`
		SELECT id, hostname, serial_number, drive_type, percentage, factors_json, timestamp
		FROM wearout_history
		WHERE hostname = ? AND serial_number = ?
		ORDER BY timestamp DESC LIMIT 1
	`, hostname, serialNumber)

	return scanSnapshot(row)
}

// GetAllLatestSnapshots returns the most recent wearout for every drive.
func GetAllLatestSnapshots(db *sql.DB) ([]WearoutSnapshot, error) {
	rows, err := db.Query(`
		SELECT w.id, w.hostname, w.serial_number, w.drive_type, w.percentage, w.factors_json, w.timestamp
		FROM wearout_history w
		INNER JOIN (
			SELECT hostname, serial_number, MAX(timestamp) AS max_ts
			FROM wearout_history
			GROUP BY hostname, serial_number
		) latest ON w.hostname = latest.hostname
			AND w.serial_number = latest.serial_number
			AND w.timestamp = latest.max_ts
		ORDER BY w.percentage DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []WearoutSnapshot
	for rows.Next() {
		var s WearoutSnapshot
		var ts string
		var factorsJSON sql.NullString
		if err := rows.Scan(&s.ID, &s.Hostname, &s.SerialNumber, &s.DriveType, &s.Percentage, &factorsJSON, &ts); err != nil {
			continue
		}
		s.Timestamp, _ = time.Parse(timeFormat, ts)
		if factorsJSON.Valid {
			s.FactorsJSON = factorsJSON.String
		}
		snapshots = append(snapshots, s)
	}
	return snapshots, nil
}

// GetSnapshotHistory returns wearout snapshots for a drive within a day range.
func GetSnapshotHistory(db *sql.DB, hostname, serialNumber string, days int) ([]WearoutSnapshot, error) {
	since := time.Now().AddDate(0, 0, -days).UTC().Format(timeFormat)

	rows, err := db.Query(`
		SELECT id, hostname, serial_number, drive_type, percentage, factors_json, timestamp
		FROM wearout_history
		WHERE hostname = ? AND serial_number = ? AND timestamp >= ?
		ORDER BY timestamp ASC
	`, hostname, serialNumber, since)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var snapshots []WearoutSnapshot
	for rows.Next() {
		var s WearoutSnapshot
		var ts string
		var factorsJSON sql.NullString
		if err := rows.Scan(&s.ID, &s.Hostname, &s.SerialNumber, &s.DriveType, &s.Percentage, &factorsJSON, &ts); err != nil {
			continue
		}
		s.Timestamp, _ = time.Parse(timeFormat, ts)
		if factorsJSON.Valid {
			s.FactorsJSON = factorsJSON.String
		}
		snapshots = append(snapshots, s)
	}
	return snapshots, nil
}

// GetDriveSpec finds a drive spec matching the given model name using LIKE patterns.
func GetDriveSpec(db *sql.DB, modelName string) (*DriveSpec, error) {
	row := db.QueryRow(`
		SELECT id, model_pattern, rated_tbw, rated_mtbf_hours, rated_load_cycles
		FROM drive_specs
		WHERE ? LIKE model_pattern
		LIMIT 1
	`, modelName)

	var spec DriveSpec
	var tbw sql.NullFloat64
	var mtbf, loadCycles sql.NullInt64

	err := row.Scan(&spec.ID, &spec.ModelPattern, &tbw, &mtbf, &loadCycles)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if tbw.Valid {
		spec.RatedTBW = &tbw.Float64
	}
	if mtbf.Valid {
		spec.RatedMTBFHours = &mtbf.Int64
	}
	if loadCycles.Valid {
		spec.RatedLoadCycles = &loadCycles.Int64
	}
	return &spec, nil
}

// UpsertDriveSpec creates or updates a drive specification.
func UpsertDriveSpec(db *sql.DB, spec DriveSpec) error {
	_, err := db.Exec(`
		INSERT INTO drive_specs (model_pattern, rated_tbw, rated_mtbf_hours, rated_load_cycles, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(model_pattern) DO UPDATE SET
			rated_tbw         = excluded.rated_tbw,
			rated_mtbf_hours  = excluded.rated_mtbf_hours,
			rated_load_cycles = excluded.rated_load_cycles,
			updated_at        = excluded.updated_at
	`, spec.ModelPattern, spec.RatedTBW, spec.RatedMTBFHours, spec.RatedLoadCycles, nowString())
	return err
}

// DeleteDriveSpec removes a drive spec by ID.
func DeleteDriveSpec(db *sql.DB, id int) error {
	result, err := db.Exec("DELETE FROM drive_specs WHERE id = ?", id)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("drive spec not found")
	}
	return nil
}

// ListDriveSpecs returns all drive specs.
func ListDriveSpecs(db *sql.DB) ([]DriveSpec, error) {
	rows, err := db.Query(`
		SELECT id, model_pattern, rated_tbw, rated_mtbf_hours, rated_load_cycles
		FROM drive_specs ORDER BY model_pattern
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var specs []DriveSpec
	for rows.Next() {
		var spec DriveSpec
		var tbw sql.NullFloat64
		var mtbf, loadCycles sql.NullInt64
		if err := rows.Scan(&spec.ID, &spec.ModelPattern, &tbw, &mtbf, &loadCycles); err != nil {
			continue
		}
		if tbw.Valid {
			spec.RatedTBW = &tbw.Float64
		}
		if mtbf.Valid {
			spec.RatedMTBFHours = &mtbf.Int64
		}
		if loadCycles.Valid {
			spec.RatedLoadCycles = &loadCycles.Int64
		}
		specs = append(specs, spec)
	}
	return specs, nil
}

// marshalFactors serializes contributing factors to JSON.
func marshalFactors(factors []ContributingFactor) string {
	data, err := json.Marshal(factors)
	if err != nil {
		return "[]"
	}
	return string(data)
}

func scanSnapshot(row *sql.Row) (*WearoutSnapshot, error) {
	var s WearoutSnapshot
	var ts string
	var factorsJSON sql.NullString

	err := row.Scan(&s.ID, &s.Hostname, &s.SerialNumber, &s.DriveType, &s.Percentage, &factorsJSON, &ts)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	s.Timestamp, _ = time.Parse(timeFormat, ts)
	if factorsJSON.Valid {
		s.FactorsJSON = factorsJSON.String
	}
	return &s, nil
}
