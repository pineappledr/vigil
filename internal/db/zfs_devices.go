package db

import (
	"database/sql"
	"fmt"
	"time"
)

// ─── Device CRUD Operations ──────────────────────────────────────────────────

// UpsertZFSPoolDevice inserts or updates a pool device
// Uses SELECT + INSERT/UPDATE pattern for SQLite compatibility
func UpsertZFSPoolDevice(poolID int64, device *ZFSPoolDevice) error {
	now := NowString()

	// Check if device exists
	existingID, err := GetID(
		"SELECT id FROM zfs_pool_devices WHERE pool_id = ? AND device_name = ?",
		poolID, device.DeviceName,
	)
	if err != nil {
		return fmt.Errorf("check device exists: %w", err)
	}

	if existingID == 0 {
		// Insert new device
		_, err = DB.Exec(`
			INSERT INTO zfs_pool_devices (
				pool_id, hostname, pool_name, device_name, device_path, device_guid,
				serial_number, vdev_type, vdev_parent, vdev_index, state,
				read_errors, write_errors, checksum_errors,
				size_bytes, allocated_bytes,
				is_spare, is_log, is_cache, is_replacing,
				last_seen, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
			poolID, device.Hostname, device.PoolName, device.DeviceName, device.DevicePath, device.DeviceGUID,
			device.SerialNumber, device.VdevType, device.VdevParent, device.VdevIndex, device.State,
			device.ReadErrors, device.WriteErrors, device.ChecksumErrors,
			device.SizeBytes, device.AllocatedBytes,
			BoolToInt(device.IsSpare), BoolToInt(device.IsLog), BoolToInt(device.IsCache), BoolToInt(device.IsReplacing),
			now, now,
		)
	} else {
		// Update existing device
		_, err = DB.Exec(`
			UPDATE zfs_pool_devices SET
				device_path = ?, device_guid = ?, serial_number = ?,
				vdev_type = ?, vdev_parent = ?, vdev_index = ?, state = ?,
				read_errors = ?, write_errors = ?, checksum_errors = ?,
				size_bytes = ?, allocated_bytes = ?,
				is_spare = ?, is_log = ?, is_cache = ?, is_replacing = ?,
				last_seen = ?
			WHERE id = ?
		`,
			device.DevicePath, device.DeviceGUID, device.SerialNumber,
			device.VdevType, device.VdevParent, device.VdevIndex, device.State,
			device.ReadErrors, device.WriteErrors, device.ChecksumErrors,
			device.SizeBytes, device.AllocatedBytes,
			BoolToInt(device.IsSpare), BoolToInt(device.IsLog), BoolToInt(device.IsCache), BoolToInt(device.IsReplacing),
			now, existingID,
		)
	}

	if err != nil {
		return fmt.Errorf("upsert ZFS device: %w", err)
	}
	return nil
}

// GetZFSPoolDevices retrieves all devices for a pool
func GetZFSPoolDevices(poolID int64) ([]ZFSPoolDevice, error) {
	rows, err := DB.Query(`
		SELECT id, pool_id, hostname, pool_name, device_name, device_path, device_guid,
			serial_number, vdev_type, vdev_parent, vdev_index, state,
			read_errors, write_errors, checksum_errors,
			size_bytes, allocated_bytes,
			is_spare, is_log, is_cache, is_replacing,
			last_seen, created_at
		FROM zfs_pool_devices
		WHERE pool_id = ?
		ORDER BY vdev_parent, vdev_index
	`, poolID)
	if err != nil {
		return nil, fmt.Errorf("query ZFS devices: %w", err)
	}
	defer rows.Close()

	return scanDevices(rows)
}

// GetZFSDeviceBySerial finds a ZFS device by serial number
func GetZFSDeviceBySerial(hostname, serialNumber string) (*ZFSPoolDevice, error) {
	dev := &ZFSPoolDevice{}
	var isSpare, isLog, isCache, isReplacing int
	var lastSeen, createdAt sql.NullString

	err := DB.QueryRow(`
		SELECT id, pool_id, hostname, pool_name, device_name, device_path, device_guid,
			serial_number, vdev_type, vdev_parent, vdev_index, state,
			read_errors, write_errors, checksum_errors,
			size_bytes, allocated_bytes,
			is_spare, is_log, is_cache, is_replacing,
			last_seen, created_at
		FROM zfs_pool_devices
		WHERE hostname = ? AND serial_number = ?
		LIMIT 1
	`, hostname, serialNumber).Scan(
		&dev.ID, &dev.PoolID, &dev.Hostname, &dev.PoolName, &dev.DeviceName, &dev.DevicePath, &dev.DeviceGUID,
		&dev.SerialNumber, &dev.VdevType, &dev.VdevParent, &dev.VdevIndex, &dev.State,
		&dev.ReadErrors, &dev.WriteErrors, &dev.ChecksumErrors,
		&dev.SizeBytes, &dev.AllocatedBytes,
		&isSpare, &isLog, &isCache, &isReplacing,
		&lastSeen, &createdAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get ZFS device by serial: %w", err)
	}

	dev.IsSpare = IntToBool(isSpare)
	dev.IsLog = IntToBool(isLog)
	dev.IsCache = IntToBool(isCache)
	dev.IsReplacing = IntToBool(isReplacing)
	dev.LastSeen = ParseNullTime(lastSeen)
	dev.CreatedAt = ParseNullTime(createdAt)

	return dev, nil
}

// GetZFSDevicesByHostname retrieves all devices for a hostname
func GetZFSDevicesByHostname(hostname string) ([]ZFSPoolDevice, error) {
	rows, err := DB.Query(`
		SELECT id, pool_id, hostname, pool_name, device_name, device_path, device_guid,
			serial_number, vdev_type, vdev_parent, vdev_index, state,
			read_errors, write_errors, checksum_errors,
			size_bytes, allocated_bytes,
			is_spare, is_log, is_cache, is_replacing,
			last_seen, created_at
		FROM zfs_pool_devices
		WHERE hostname = ?
		ORDER BY pool_name, vdev_parent, vdev_index
	`, hostname)
	if err != nil {
		return nil, fmt.Errorf("query ZFS devices by hostname: %w", err)
	}
	defer rows.Close()

	return scanDevices(rows)
}

// DeleteStaleZFSDevices removes devices not seen since cutoff time
func DeleteStaleZFSDevices(poolID int64, cutoff time.Time) (int64, error) {
	result, err := DB.Exec(
		"DELETE FROM zfs_pool_devices WHERE pool_id = ? AND last_seen < ?",
		poolID, cutoff.Format(timeFormat),
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// CountZFSDevices returns the device count for a pool
func CountZFSDevices(poolID int64) (int, error) {
	var count int
	err := DB.QueryRow("SELECT COUNT(*) FROM zfs_pool_devices WHERE pool_id = ?", poolID).Scan(&count)
	return count, err
}

// ─── Helper Functions ────────────────────────────────────────────────────────

func scanDevices(rows *sql.Rows) ([]ZFSPoolDevice, error) {
	var devices []ZFSPoolDevice

	for rows.Next() {
		var dev ZFSPoolDevice
		var isSpare, isLog, isCache, isReplacing int
		var lastSeen, createdAt sql.NullString

		err := rows.Scan(
			&dev.ID, &dev.PoolID, &dev.Hostname, &dev.PoolName, &dev.DeviceName, &dev.DevicePath, &dev.DeviceGUID,
			&dev.SerialNumber, &dev.VdevType, &dev.VdevParent, &dev.VdevIndex, &dev.State,
			&dev.ReadErrors, &dev.WriteErrors, &dev.ChecksumErrors,
			&dev.SizeBytes, &dev.AllocatedBytes,
			&isSpare, &isLog, &isCache, &isReplacing,
			&lastSeen, &createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan ZFS device row: %w", err)
		}

		dev.IsSpare = IntToBool(isSpare)
		dev.IsLog = IntToBool(isLog)
		dev.IsCache = IntToBool(isCache)
		dev.IsReplacing = IntToBool(isReplacing)
		dev.LastSeen = ParseNullTime(lastSeen)
		dev.CreatedAt = ParseNullTime(createdAt)

		devices = append(devices, dev)
	}

	return devices, nil
}
