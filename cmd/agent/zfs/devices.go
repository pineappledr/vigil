package zfs

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ─── Device Serial Number Mapping ────────────────────────────────────────────

// DeviceInfo contains device identification information
type DeviceInfo struct {
	Name         string `json:"name"`          // Device name (e.g., sda, nvme0n1)
	Path         string `json:"path"`          // Full path (e.g., /dev/sda)
	SerialNumber string `json:"serial_number"` // Drive serial number
	Model        string `json:"model"`         // Drive model
	WWN          string `json:"wwn"`           // World Wide Name
	ByIDPath     string `json:"by_id_path"`    // /dev/disk/by-id path
}

// GetDeviceSerial retrieves the serial number for a device
// Tries multiple methods: smartctl, /dev/disk/by-id, hdparm, lsblk
func GetDeviceSerial(devicePath string) (string, error) {
	// Normalize device path
	if !strings.HasPrefix(devicePath, "/dev/") {
		devicePath = "/dev/" + devicePath
	}

	// Method 1: Try smartctl first (most reliable)
	if serial := getSerialFromSmartctl(devicePath); serial != "" {
		return serial, nil
	}

	// Method 2: Try /dev/disk/by-id symlinks
	if serial := getSerialFromByID(devicePath); serial != "" {
		return serial, nil
	}

	// Method 3: Try lsblk
	if serial := getSerialFromLsblk(devicePath); serial != "" {
		return serial, nil
	}

	// Method 4: Try hdparm (for ATA devices)
	if serial := getSerialFromHdparm(devicePath); serial != "" {
		return serial, nil
	}

	// Method 5: Try /sys/block
	if serial := getSerialFromSysBlock(devicePath); serial != "" {
		return serial, nil
	}

	return "", fmt.Errorf("could not determine serial number for %s", devicePath)
}

// GetDeviceInfo retrieves comprehensive device information
func GetDeviceInfo(devicePath string) (*DeviceInfo, error) {
	// Normalize device path
	if !strings.HasPrefix(devicePath, "/dev/") {
		devicePath = "/dev/" + devicePath
	}

	info := &DeviceInfo{
		Path: devicePath,
		Name: filepath.Base(devicePath),
	}

	// Get serial from smartctl (includes model)
	getInfoFromSmartctl(devicePath, info)

	// If no serial yet, try other methods
	if info.SerialNumber == "" {
		info.SerialNumber = getSerialFromByID(devicePath)
	}
	if info.SerialNumber == "" {
		info.SerialNumber = getSerialFromLsblk(devicePath)
	}
	if info.SerialNumber == "" {
		info.SerialNumber = getSerialFromSysBlock(devicePath)
	}

	// Get by-id path
	info.ByIDPath = getByIDPath(devicePath)

	return info, nil
}

// ─── Serial Number Retrieval Methods ─────────────────────────────────────────

// getSerialFromSmartctl uses smartctl to get the serial number
func getSerialFromSmartctl(devicePath string) string {
	cmd := exec.Command("smartctl", "-i", devicePath)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return ""
	}

	return parseSmartctlSerial(stdout.String())
}

// getInfoFromSmartctl populates DeviceInfo from smartctl output
func getInfoFromSmartctl(devicePath string, info *DeviceInfo) {
	cmd := exec.Command("smartctl", "-i", devicePath)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return
	}

	output := stdout.String()
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "Serial Number:") || strings.HasPrefix(line, "Serial number:") {
			info.SerialNumber = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "Serial Number:"), "Serial number:"))
		} else if strings.HasPrefix(line, "Device Model:") || strings.HasPrefix(line, "Model Number:") || strings.HasPrefix(line, "Product:") {
			info.Model = strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(line, "Device Model:"), "Model Number:"), "Product:"))
		} else if strings.HasPrefix(line, "LU WWN Device Id:") {
			info.WWN = strings.TrimSpace(strings.TrimPrefix(line, "LU WWN Device Id:"))
			info.WWN = strings.ReplaceAll(info.WWN, " ", "")
		}
	}
}

// parseSmartctlSerial extracts serial number from smartctl -i output
func parseSmartctlSerial(output string) string {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Serial Number:") || strings.HasPrefix(line, "Serial number:") {
			return strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(line, "Serial Number:"), "Serial number:"))
		}
	}
	return ""
}

// getSerialFromByID extracts serial from /dev/disk/by-id symlinks
func getSerialFromByID(devicePath string) string {
	byIDDir := "/dev/disk/by-id"
	
	entries, err := os.ReadDir(byIDDir)
	if err != nil {
		return ""
	}

	deviceName := filepath.Base(devicePath)

	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink == 0 {
			continue
		}

		linkPath := filepath.Join(byIDDir, entry.Name())
		target, err := os.Readlink(linkPath)
		if err != nil {
			continue
		}

		// Resolve to absolute path and get base name
		resolvedTarget := filepath.Base(target)
		if resolvedTarget == deviceName {
			// Extract serial from the by-id name
			// Format: ata-MODEL_SERIAL, scsi-SSERIAL, nvme-MODEL_SERIAL, wwn-0x...
			name := entry.Name()
			
			// Skip wwn- entries as they don't contain serial
			if strings.HasPrefix(name, "wwn-") {
				continue
			}

			// Extract serial from various formats
			serial := extractSerialFromByIDName(name)
			if serial != "" {
				return serial
			}
		}
	}

	return ""
}

// extractSerialFromByIDName extracts serial number from by-id filename
func extractSerialFromByIDName(name string) string {
	// Remove partition suffix if present (e.g., -part1)
	if idx := strings.Index(name, "-part"); idx > 0 {
		name = name[:idx]
	}

	// Handle different formats:
	// ata-Samsung_SSD_870_EVO_1TB_S625NJ0R444358R
	// scsi-SATA_Samsung_SSD_870_S625NJ0R444358R
	// nvme-Samsung_SSD_990_PRO_2TB_S73WNJ0X123456Y
	// usb-WD_Elements_1234567890AB-0:0

	parts := strings.Split(name, "_")
	if len(parts) < 2 {
		return ""
	}

	// The serial is usually the last part
	serial := parts[len(parts)-1]

	// Validate it looks like a serial (alphanumeric, reasonable length)
	if len(serial) >= 6 && len(serial) <= 30 && isAlphanumeric(serial) {
		return serial
	}

	// For SCSI format, serial might be after SATA_ prefix
	if strings.HasPrefix(name, "scsi-S") {
		// scsi-SATA_Model_Serial or scsi-SSERIAL
		afterPrefix := strings.TrimPrefix(name, "scsi-")
		if strings.HasPrefix(afterPrefix, "SATA_") {
			parts := strings.Split(afterPrefix, "_")
			if len(parts) >= 2 {
				serial = parts[len(parts)-1]
				if len(serial) >= 6 && isAlphanumeric(serial) {
					return serial
				}
			}
		} else if len(afterPrefix) > 1 {
			// scsi-SSERIAL format
			return afterPrefix[1:] // Skip the 'S' prefix
		}
	}

	return ""
}

// isAlphanumeric checks if a string contains only alphanumeric characters
func isAlphanumeric(s string) bool {
	for _, c := range s {
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9')) {
			return false
		}
	}
	return true
}

// getSerialFromLsblk uses lsblk to get serial number
func getSerialFromLsblk(devicePath string) string {
	cmd := exec.Command("lsblk", "-ndo", "SERIAL", devicePath)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return ""
	}

	return strings.TrimSpace(stdout.String())
}

// getSerialFromHdparm uses hdparm to get serial number (ATA devices only)
func getSerialFromHdparm(devicePath string) string {
	cmd := exec.Command("hdparm", "-I", devicePath)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return ""
	}

	scanner := bufio.NewScanner(strings.NewReader(stdout.String()))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "Serial Number:") {
			return strings.TrimSpace(strings.TrimPrefix(line, "Serial Number:"))
		}
	}

	return ""
}

// getSerialFromSysBlock reads serial from /sys/block/*/device/serial
func getSerialFromSysBlock(devicePath string) string {
	deviceName := filepath.Base(devicePath)
	
	// Handle NVMe devices (nvme0n1 -> nvme0)
	if strings.HasPrefix(deviceName, "nvme") {
		if idx := strings.Index(deviceName, "n"); idx > 0 {
			// Find the second 'n' for namespace
			rest := deviceName[idx+1:]
			if idx2 := strings.Index(rest, "n"); idx2 >= 0 {
				deviceName = deviceName[:idx+1+idx2]
			}
		}
	}

	// Try different paths
	paths := []string{
		fmt.Sprintf("/sys/block/%s/device/serial", deviceName),
		fmt.Sprintf("/sys/class/block/%s/device/serial", deviceName),
		fmt.Sprintf("/sys/block/%s/device/wwid", deviceName),
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			serial := strings.TrimSpace(string(data))
			if serial != "" {
				return serial
			}
		}
	}

	return ""
}

// getByIDPath finds the /dev/disk/by-id path for a device
func getByIDPath(devicePath string) string {
	byIDDir := "/dev/disk/by-id"
	
	entries, err := os.ReadDir(byIDDir)
	if err != nil {
		return ""
	}

	deviceName := filepath.Base(devicePath)

	// Prefer ata-, nvme-, or scsi- entries over wwn-
	var wwwPath string

	for _, entry := range entries {
		if entry.Type()&os.ModeSymlink == 0 {
			continue
		}

		linkPath := filepath.Join(byIDDir, entry.Name())
		target, err := os.Readlink(linkPath)
		if err != nil {
			continue
		}

		resolvedTarget := filepath.Base(target)
		if resolvedTarget == deviceName {
			name := entry.Name()
			
			// Skip partition entries
			if strings.Contains(name, "-part") {
				continue
			}

			// Prefer non-wwn entries
			if strings.HasPrefix(name, "wwn-") {
				wwwPath = linkPath
				continue
			}

			return linkPath
		}
	}

	return wwwPath
}

// ─── Bulk Device Mapping ─────────────────────────────────────────────────────

// DeviceSerialMap maps device names to their serial numbers
type DeviceSerialMap map[string]string

// BuildDeviceSerialMap creates a map of all block device serials
func BuildDeviceSerialMap() DeviceSerialMap {
	deviceMap := make(DeviceSerialMap)

	// Get all block devices from /sys/block
	entries, err := os.ReadDir("/sys/block")
	if err != nil {
		return deviceMap
	}

	for _, entry := range entries {
		name := entry.Name()
		
		// Skip virtual devices
		if strings.HasPrefix(name, "loop") ||
			strings.HasPrefix(name, "ram") ||
			strings.HasPrefix(name, "dm-") ||
			strings.HasPrefix(name, "md") ||
			strings.HasPrefix(name, "zram") {
			continue
		}

		devicePath := "/dev/" + name
		serial, err := GetDeviceSerial(devicePath)
		if err == nil && serial != "" {
			deviceMap[name] = serial
			deviceMap[devicePath] = serial
		}
	}

	return deviceMap
}

// ─── ZFS Device to Serial Mapping ────────────────────────────────────────────

// MapPoolDevicesToSerials populates serial numbers for all devices in a pool
func MapPoolDevicesToSerials(pool *Pool, serialMap DeviceSerialMap) {
	for i := range pool.Devices {
		mapDeviceSerial(&pool.Devices[i], serialMap)
	}
}

// mapDeviceSerial recursively maps serial numbers to devices
func mapDeviceSerial(device *Device, serialMap DeviceSerialMap) {
	if device.VdevType == VdevTypeDisk {
		// Try to find serial by device name or path
		if serial, ok := serialMap[device.Name]; ok {
			device.SerialNumber = serial
		} else if serial, ok := serialMap[device.Path]; ok {
			device.SerialNumber = serial
		} else if device.Path != "" {
			// Try to get serial directly
			if serial, err := GetDeviceSerial(device.Path); err == nil {
				device.SerialNumber = serial
			}
		}
	}

	// Recursively process children
	for i := range device.Children {
		mapDeviceSerial(&device.Children[i], serialMap)
	}
}

// ─── Cross-Reference with SMART Data ─────────────────────────────────────────

// DriveMatch represents a matched ZFS device to SMART drive
type DriveMatch struct {
	ZFSDevice    *Device `json:"zfs_device"`
	SerialNumber string  `json:"serial_number"`
	Hostname     string  `json:"hostname"`
	PoolName     string  `json:"pool_name"`
}

// FindDriveMatches finds all ZFS devices that can be matched to SMART drives
func FindDriveMatches(pools []Pool) []DriveMatch {
	var matches []DriveMatch

	for _, pool := range pools {
		for _, device := range pool.Devices {
			collectDriveMatches(&device, pool.Hostname, pool.Name, &matches)
		}
	}

	return matches
}

// collectDriveMatches recursively collects device matches
func collectDriveMatches(device *Device, hostname, poolName string, matches *[]DriveMatch) {
	if device.VdevType == VdevTypeDisk && device.SerialNumber != "" {
		*matches = append(*matches, DriveMatch{
			ZFSDevice:    device,
			SerialNumber: device.SerialNumber,
			Hostname:     hostname,
			PoolName:     poolName,
		})
	}

	for i := range device.Children {
		collectDriveMatches(&device.Children[i], hostname, poolName, matches)
	}
}

// ─── Device Resolution Helpers ───────────────────────────────────────────────

// ResolveDevicePath resolves various device references to actual device path
// Handles: /dev/sdX, sdX, /dev/disk/by-id/..., /dev/disk/by-path/..., etc.
func ResolveDevicePath(deviceRef string) (string, error) {
	// If it's already a /dev/sdX or /dev/nvmeX style path
	if strings.HasPrefix(deviceRef, "/dev/sd") || strings.HasPrefix(deviceRef, "/dev/nvme") {
		if _, err := os.Stat(deviceRef); err == nil {
			return deviceRef, nil
		}
	}

	// If it's just a device name like "sda"
	if !strings.HasPrefix(deviceRef, "/") {
		path := "/dev/" + deviceRef
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	// If it's a symlink (like /dev/disk/by-id/...)
	if strings.HasPrefix(deviceRef, "/dev/disk/") {
		target, err := filepath.EvalSymlinks(deviceRef)
		if err == nil {
			return target, nil
		}
	}

	// Try to resolve as symlink anyway
	target, err := filepath.EvalSymlinks(deviceRef)
	if err == nil && strings.HasPrefix(target, "/dev/") {
		return target, nil
	}

	return "", fmt.Errorf("could not resolve device path: %s", deviceRef)
}

// GetDeviceFromSerial finds a device path given a serial number
func GetDeviceFromSerial(serial string) (string, error) {
	if serial == "" {
		return "", fmt.Errorf("empty serial number")
	}

	// Search in /dev/disk/by-id for matching serial
	byIDDir := "/dev/disk/by-id"
	entries, err := os.ReadDir(byIDDir)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		name := entry.Name()
		
		// Skip partitions and wwn entries
		if strings.Contains(name, "-part") || strings.HasPrefix(name, "wwn-") {
			continue
		}

		// Check if serial is in the name
		if strings.Contains(name, serial) {
			linkPath := filepath.Join(byIDDir, name)
			target, err := filepath.EvalSymlinks(linkPath)
			if err == nil {
				return target, nil
			}
		}
	}

	// Fallback: scan all devices
	deviceMap := BuildDeviceSerialMap()
	for path, deviceSerial := range deviceMap {
		if deviceSerial == serial && strings.HasPrefix(path, "/dev/") {
			return path, nil
		}
	}

	return "", fmt.Errorf("device with serial %s not found", serial)
}