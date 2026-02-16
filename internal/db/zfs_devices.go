package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"
)

// ─── Device Operations ───────────────────────────────────────────────────────

// UpsertZFSPoolDevice inserts or updates a ZFS device
func UpsertZFSPoolDevice(poolID int64, device *ZFSPoolDevice) error {
	device.PoolID = poolID
	device.LastSeen = time.Now()

	existingID, err := GetID(
		"SELECT id FROM zfs_pool_devices WHERE pool_id = ? AND device_name = ?",
		poolID, device.DeviceName,
	)
	if err != nil {
		return fmt.Errorf("check device exists: %w", err)
	}

	if existingID == 0 {
		device.CreatedAt = time.Now()
		result, err := DB.Exec(`
			INSERT INTO zfs_pool_devices (
				pool_id, hostname, pool_name, device_name, device_path, device_guid,
				serial_number, vdev_type, vdev_parent, vdev_index, state,
				read_errors, write_errors, checksum_errors, size_bytes, allocated_bytes,
				is_spare, is_log, is_cache, is_replacing, last_seen, created_at
			) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			device.PoolID, device.Hostname, device.PoolName, device.DeviceName,
			device.DevicePath, device.DeviceGUID, device.SerialNumber, device.VdevType,
			device.VdevParent, device.VdevIndex, device.State,
			device.ReadErrors, device.WriteErrors, device.ChecksumErrors,
			device.SizeBytes, device.AllocatedBytes,
			device.IsSpare, device.IsLog, device.IsCache, device.IsReplacing,
			device.LastSeen, device.CreatedAt,
		)
		if err != nil {
			return fmt.Errorf("insert device: %w", err)
		}
		device.ID, _ = result.LastInsertId()
	} else {
		_, err := DB.Exec(`
			UPDATE zfs_pool_devices SET
				hostname = ?, pool_name = ?, device_path = ?, device_guid = ?,
				serial_number = ?, vdev_type = ?, vdev_parent = ?, vdev_index = ?,
				state = ?, read_errors = ?, write_errors = ?, checksum_errors = ?,
				size_bytes = ?, allocated_bytes = ?, is_spare = ?, is_log = ?,
				is_cache = ?, is_replacing = ?, last_seen = ?
			WHERE id = ?`,
			device.Hostname, device.PoolName, device.DevicePath, device.DeviceGUID,
			device.SerialNumber, device.VdevType, device.VdevParent, device.VdevIndex,
			device.State, device.ReadErrors, device.WriteErrors, device.ChecksumErrors,
			device.SizeBytes, device.AllocatedBytes, device.IsSpare, device.IsLog,
			device.IsCache, device.IsReplacing, device.LastSeen,
			existingID,
		)
		if err != nil {
			return fmt.Errorf("update device: %w", err)
		}
		device.ID = existingID
	}

	return nil
}

// GetZFSPoolDevices returns all devices for a pool
func GetZFSPoolDevices(poolID int64) ([]ZFSPoolDevice, error) {
	rows, err := DB.Query(`
		SELECT id, pool_id, hostname, pool_name, device_name, device_path, device_guid,
			   serial_number, vdev_type, vdev_parent, vdev_index, state,
			   read_errors, write_errors, checksum_errors, size_bytes, allocated_bytes,
			   is_spare, is_log, is_cache, is_replacing, last_seen, created_at
		FROM zfs_pool_devices
		WHERE pool_id = ?
		ORDER BY vdev_index`, poolID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []ZFSPoolDevice
	for rows.Next() {
		var d ZFSPoolDevice
		err := rows.Scan(
			&d.ID, &d.PoolID, &d.Hostname, &d.PoolName, &d.DeviceName, &d.DevicePath,
			&d.DeviceGUID, &d.SerialNumber, &d.VdevType, &d.VdevParent, &d.VdevIndex,
			&d.State, &d.ReadErrors, &d.WriteErrors, &d.ChecksumErrors,
			&d.SizeBytes, &d.AllocatedBytes, &d.IsSpare, &d.IsLog, &d.IsCache,
			&d.IsReplacing, &d.LastSeen, &d.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}

	return devices, rows.Err()
}

// CountZFSDevices returns total device count for a pool
func CountZFSDevices(poolID int64) (int, error) {
	var count int
	err := DB.QueryRow(
		"SELECT COUNT(*) FROM zfs_pool_devices WHERE pool_id = ?",
		poolID,
	).Scan(&count)
	return count, err
}

// CountZFSDisks returns count of unique disk devices (not vdevs)
// Deduplicates by:
// 1. vdev_parent + vdev_index (same position = same physical disk)
// 2. Base device name (mmcblk0p3 -> mmcblk0, sda2 -> sda)
// This handles GUID/device name duplicates AND partition duplicates
func CountZFSDisks(poolID int64) (int, error) {
	// First try position-based counting (for mirrors/raidz with vdev_parent)
	var countByPosition int
	err := DB.QueryRow(`
		SELECT COUNT(DISTINCT vdev_parent || ':' || vdev_index)
		FROM zfs_pool_devices 
		WHERE pool_id = ? 
		AND vdev_type = 'disk'
		AND vdev_parent != ''
		AND is_spare = 0 
		AND is_log = 0 
		AND is_cache = 0`,
		poolID,
	).Scan(&countByPosition)

	if err == nil && countByPosition > 0 {
		return countByPosition, nil
	}

	// For stripe pools (no vdev_parent), count by unique base device name
	// This groups mmcblk0p1, mmcblk0p2, mmcblk0p3 as one device
	// And sda1, sda2 as one device
	var countByName int
	err = DB.QueryRow(`
		SELECT COUNT(DISTINCT 
			CASE 
				-- Handle mmcblk devices: mmcblk0p3 -> mmcblk0
				WHEN device_name LIKE 'mmcblk%' THEN 
					SUBSTR(device_name, 1, INSTR(device_name || 'p', 'p') + 
						CASE WHEN INSTR(SUBSTR(device_name, INSTR(device_name, 'mmcblk') + 6), 'p') > 0 
						THEN INSTR(SUBSTR(device_name, INSTR(device_name, 'mmcblk') + 6), 'p') + 5
						ELSE LENGTH(device_name) END)
				-- Handle nvme devices: nvme0n1p1 -> nvme0n1
				WHEN device_name LIKE 'nvme%' THEN
					CASE WHEN INSTR(device_name, 'p') > 0 
					THEN SUBSTR(device_name, 1, INSTR(device_name, 'n1p') + 1)
					ELSE device_name END
				-- Handle sd/hd devices: sda2 -> sda
				WHEN device_name GLOB 'sd[a-z]*' OR device_name GLOB 'hd[a-z]*' THEN
					RTRIM(device_name, '0123456789')
				-- Default: use full name
				ELSE device_name
			END
		)
		FROM zfs_pool_devices 
		WHERE pool_id = ? 
		AND vdev_type = 'disk'
		AND is_spare = 0 
		AND is_log = 0 
		AND is_cache = 0`,
		poolID,
	).Scan(&countByName)

	if err != nil {
		return 0, err
	}

	return countByName, nil
}

// GetZFSDeviceBySerial finds a device by its serial number
func GetZFSDeviceBySerial(hostname, serial string) (*ZFSPoolDevice, error) {
	var d ZFSPoolDevice
	err := DB.QueryRow(`
		SELECT id, pool_id, hostname, pool_name, device_name, device_path, device_guid,
			   serial_number, vdev_type, vdev_parent, vdev_index, state,
			   read_errors, write_errors, checksum_errors, size_bytes, allocated_bytes,
			   is_spare, is_log, is_cache, is_replacing, last_seen, created_at
		FROM zfs_pool_devices
		WHERE hostname = ? AND serial_number = ?
		LIMIT 1`,
		hostname, serial,
	).Scan(
		&d.ID, &d.PoolID, &d.Hostname, &d.PoolName, &d.DeviceName, &d.DevicePath,
		&d.DeviceGUID, &d.SerialNumber, &d.VdevType, &d.VdevParent, &d.VdevIndex,
		&d.State, &d.ReadErrors, &d.WriteErrors, &d.ChecksumErrors,
		&d.SizeBytes, &d.AllocatedBytes, &d.IsSpare, &d.IsLog, &d.IsCache,
		&d.IsReplacing, &d.LastSeen, &d.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &d, nil
}

// GetZFSDeviceByPath finds a device by its path
func GetZFSDeviceByPath(hostname, devicePath string) (*ZFSPoolDevice, error) {
	var d ZFSPoolDevice
	err := DB.QueryRow(`
		SELECT id, pool_id, hostname, pool_name, device_name, device_path, device_guid,
			   serial_number, vdev_type, vdev_parent, vdev_index, state,
			   read_errors, write_errors, checksum_errors, size_bytes, allocated_bytes,
			   is_spare, is_log, is_cache, is_replacing, last_seen, created_at
		FROM zfs_pool_devices
		WHERE hostname = ? AND (device_path = ? OR device_name = ?)
		LIMIT 1`,
		hostname, devicePath, devicePath,
	).Scan(
		&d.ID, &d.PoolID, &d.Hostname, &d.PoolName, &d.DeviceName, &d.DevicePath,
		&d.DeviceGUID, &d.SerialNumber, &d.VdevType, &d.VdevParent, &d.VdevIndex,
		&d.State, &d.ReadErrors, &d.WriteErrors, &d.ChecksumErrors,
		&d.SizeBytes, &d.AllocatedBytes, &d.IsSpare, &d.IsLog, &d.IsCache,
		&d.IsReplacing, &d.LastSeen, &d.CreatedAt,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &d, nil
}

// DeleteStaleZFSDevices removes devices not seen since cutoff
func DeleteStaleZFSDevices(poolID int64, cutoff time.Time) (int64, error) {
	result, err := DB.Exec(
		"DELETE FROM zfs_pool_devices WHERE pool_id = ? AND last_seen < ?",
		poolID, cutoff,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// DeleteZFSPoolDevices removes all devices for a pool
func DeleteZFSPoolDevices(poolID int64) error {
	_, err := DB.Exec("DELETE FROM zfs_pool_devices WHERE pool_id = ?", poolID)
	return err
}

// GetDevicesWithErrors returns devices that have errors
func GetDevicesWithErrors(poolID int64) ([]ZFSPoolDevice, error) {
	rows, err := DB.Query(`
		SELECT id, pool_id, hostname, pool_name, device_name, device_path, device_guid,
			   serial_number, vdev_type, vdev_parent, vdev_index, state,
			   read_errors, write_errors, checksum_errors, size_bytes, allocated_bytes,
			   is_spare, is_log, is_cache, is_replacing, last_seen, created_at
		FROM zfs_pool_devices
		WHERE pool_id = ?
		AND (read_errors > 0 OR write_errors > 0 OR checksum_errors > 0)
		ORDER BY (read_errors + write_errors + checksum_errors) DESC`,
		poolID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []ZFSPoolDevice
	for rows.Next() {
		var d ZFSPoolDevice
		err := rows.Scan(
			&d.ID, &d.PoolID, &d.Hostname, &d.PoolName, &d.DeviceName, &d.DevicePath,
			&d.DeviceGUID, &d.SerialNumber, &d.VdevType, &d.VdevParent, &d.VdevIndex,
			&d.State, &d.ReadErrors, &d.WriteErrors, &d.ChecksumErrors,
			&d.SizeBytes, &d.AllocatedBytes, &d.IsSpare, &d.IsLog, &d.IsCache,
			&d.IsReplacing, &d.LastSeen, &d.CreatedAt,
		)
		if err != nil {
			log.Printf("⚠️  Error scanning device: %v", err)
			continue
		}
		devices = append(devices, d)
	}

	return devices, rows.Err()
}
