package zfs

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// ─── ZFS Availability Check ──────────────────────────────────────────────────

// IsZFSAvailable checks if ZFS tools are installed and accessible
func IsZFSAvailable() bool {
	_, err := exec.LookPath("zpool")
	return err == nil
}

// ─── Pool List Parsing ───────────────────────────────────────────────────────

// ListPools returns all ZFS pools with basic information
// Uses: zpool list -H -p -o name,size,alloc,free,frag,cap,dedup,health,altroot
func ListPools() ([]Pool, error) {
	if !IsZFSAvailable() {
		return nil, fmt.Errorf("zpool command not found")
	}

	// -H: no header, -p: parseable (exact bytes)
	// Fields: name, size, alloc, free, frag, cap, dedup, health, altroot
	cmd := exec.Command("zpool", "list", "-H", "-p", "-o",
		"name,size,alloc,free,frag,cap,dedup,health,altroot,guid")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// No pools is not an error
		if strings.Contains(stderr.String(), "no pools available") {
			return []Pool{}, nil
		}
		return nil, fmt.Errorf("zpool list failed: %v - %s", err, stderr.String())
	}

	return parsePoolList(stdout.String())
}

// parsePoolList parses the output of zpool list -H -p
func parsePoolList(output string) ([]Pool, error) {
	var pools []Pool
	scanner := bufio.NewScanner(strings.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 8 {
			continue // Skip malformed lines
		}

		pool := Pool{
			Name:      fields[0],
			Size:      parseBytes(fields[1]),
			Allocated: parseBytes(fields[2]),
			Free:      parseBytes(fields[3]),
			LastSeen:  time.Now(),
		}

		// Fragmentation (may be "-" for pools without fragmentation)
		if fields[4] != "-" {
			pool.Fragmentation = parseInt(fields[4])
		}

		// Capacity percentage
		pool.CapacityPct = parseInt(fields[5])

		// Dedup ratio (e.g., "1.00x" or just "1.00")
		pool.DedupRatio = parseFloat(strings.TrimSuffix(fields[6], "x"))

		// Health status
		pool.Health = fields[7]
		pool.Status = fields[7] // Same as health for basic list

		// Altroot (optional, may be "-")
		if len(fields) > 8 && fields[8] != "-" {
			pool.Altroot = fields[8]
		}

		// GUID (optional)
		if len(fields) > 9 && fields[9] != "-" {
			pool.GUID = fields[9]
		}

		pools = append(pools, pool)
	}

	return pools, scanner.Err()
}

// ─── Pool Status Parsing ─────────────────────────────────────────────────────

// GetPoolStatus retrieves detailed status for a specific pool
// Uses: zpool status -v <poolname>
func GetPoolStatus(poolName string) (*Pool, error) {
	if !IsZFSAvailable() {
		return nil, fmt.Errorf("zpool command not found")
	}

	cmd := exec.Command("zpool", "status", "-v", "-p", poolName)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("zpool status failed: %v - %s", err, stderr.String())
	}

	return parsePoolStatus(poolName, stdout.String())
}

// parsePoolStatus parses the output of zpool status -v
func parsePoolStatus(poolName, output string) (*Pool, error) {
	pool := &Pool{
		Name:     poolName,
		LastSeen: time.Now(),
	}

	lines := strings.Split(output, "\n")
	var inConfig bool
	var currentVdev *Device
	var vdevStack []*Device // Stack for nested vdevs

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		// Parse pool state
		if strings.HasPrefix(trimmed, "state:") {
			pool.Health = strings.TrimSpace(strings.TrimPrefix(trimmed, "state:"))
			pool.Status = pool.Health
			continue
		}

		// Parse scan/scrub status
		if strings.HasPrefix(trimmed, "scan:") {
			pool.Scan = parseScanLine(trimmed, lines, &i)
			continue
		}

		// Start of config section
		if strings.HasPrefix(trimmed, "config:") {
			inConfig = true
			continue
		}

		// End of config section
		if inConfig && (strings.HasPrefix(trimmed, "errors:") || trimmed == "") {
			if strings.HasPrefix(trimmed, "errors:") {
				// Parse error summary
				parseErrorLine(pool, trimmed)
			}
			if trimmed == "" && len(pool.Devices) > 0 {
				inConfig = false
			}
			continue
		}

		// Parse device lines in config section
		if inConfig && trimmed != "" && !strings.HasPrefix(trimmed, "NAME") {
			device := parseDeviceLine(line)
			if device != nil {
				addDeviceToPool(pool, device, &currentVdev, &vdevStack, line)
			}
		}
	}

	return pool, nil
}

// parseScanLine parses scrub/resilver status
func parseScanLine(firstLine string, lines []string, lineIdx *int) *ScanInfo {
	scan := &ScanInfo{}
	fullText := strings.TrimPrefix(firstLine, "scan:")
	fullText = strings.TrimSpace(fullText)

	// Collect continuation lines
	for *lineIdx+1 < len(lines) {
		nextLine := lines[*lineIdx+1]
		if strings.HasPrefix(nextLine, "\t") || strings.HasPrefix(nextLine, "        ") {
			fullText += " " + strings.TrimSpace(nextLine)
			*lineIdx++
		} else {
			break
		}
	}

	// Determine scan type and state
	lowerText := strings.ToLower(fullText)

	if strings.Contains(lowerText, "no scans") || strings.Contains(lowerText, "none requested") {
		scan.Function = ScanNone
		scan.State = ScanStateNone
		return scan
	}

	if strings.Contains(lowerText, "resilver") {
		scan.Function = ScanResilver
	} else if strings.Contains(lowerText, "scrub") {
		scan.Function = ScanScrub
	}

	if strings.Contains(lowerText, "in progress") {
		scan.State = ScanStateScanning
	} else if strings.Contains(lowerText, "canceled") {
		scan.State = ScanStateCanceled
	} else if strings.Contains(lowerText, "repaired") || strings.Contains(lowerText, "with 0 errors") {
		scan.State = ScanStateFinished
	}

	// Parse progress percentage (e.g., "45.2% done")
	if idx := strings.Index(lowerText, "% done"); idx > 0 {
		start := idx - 1
		for start > 0 && (lowerText[start] >= '0' && lowerText[start] <= '9' || lowerText[start] == '.') {
			start--
		}
		if pct, err := strconv.ParseFloat(fullText[start+1:idx], 64); err == nil {
			scan.ProgressPct = pct
		}
	}

	// Parse errors found (e.g., "0 errors" or "5 errors")
	if idx := strings.Index(lowerText, " errors"); idx > 0 {
		start := idx - 1
		for start > 0 && lowerText[start] >= '0' && lowerText[start] <= '9' {
			start--
		}
		if errs, err := strconv.ParseInt(strings.TrimSpace(fullText[start+1:idx]), 10, 64); err == nil {
			scan.ErrorsFound = errs
		}
	}

	// Parse data examined (e.g., "1.23T scanned")
	if idx := strings.Index(lowerText, " scanned"); idx > 0 {
		// Find the size before "scanned"
		parts := strings.Fields(fullText[:idx])
		if len(parts) > 0 {
			scan.DataExamined = parseHumanSize(parts[len(parts)-1])
		}
	}

	// Parse total data (e.g., "out of 2.00T")
	if idx := strings.Index(lowerText, "out of "); idx >= 0 {
		rest := fullText[idx+7:]
		parts := strings.Fields(rest)
		if len(parts) > 0 {
			scan.DataTotal = parseHumanSize(parts[0])
		}
	}

	// Parse scan rate (e.g., "100M/s")
	if idx := strings.Index(fullText, "/s"); idx > 0 {
		start := idx - 1
		for start > 0 && (fullText[start] >= '0' && fullText[start] <= '9' || fullText[start] == '.' ||
			fullText[start] == 'K' || fullText[start] == 'M' || fullText[start] == 'G') {
			start--
		}
		rateStr := fullText[start+1 : idx]
		scan.Rate = parseHumanSize(rateStr)
	}

	return scan
}

// parseDeviceLine parses a device line from zpool status config section
func parseDeviceLine(line string) *Device {
	// Line format: "  NAME                      STATE     READ WRITE CKSUM"
	// or device:   "    sda                     ONLINE       0     0     0"
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return nil
	}

	fields := strings.Fields(trimmed)
	if len(fields) < 2 {
		return nil
	}

	device := &Device{
		Name:  fields[0],
		State: StateOnline, // Default
	}

	// Parse state if present
	if len(fields) >= 2 {
		state := strings.ToUpper(fields[1])
		switch state {
		case "ONLINE", "DEGRADED", "FAULTED", "OFFLINE", "REMOVED", "UNAVAIL":
			device.State = state
		default:
			// Not a state, might be a different format
		}
	}

	// Parse error counts (READ WRITE CKSUM)
	if len(fields) >= 5 {
		device.ReadErrors = parseInt64(fields[2])
		device.WriteErrors = parseInt64(fields[3])
		device.ChecksumErrors = parseInt64(fields[4])
	}

	// Determine device type based on name
	nameLower := strings.ToLower(device.Name)
	switch {
	case strings.HasPrefix(nameLower, "mirror"):
		device.VdevType = VdevTypeMirror
	case strings.HasPrefix(nameLower, "raidz3"):
		device.VdevType = VdevTypeRaidz3
	case strings.HasPrefix(nameLower, "raidz2"):
		device.VdevType = VdevTypeRaidz2
	case strings.HasPrefix(nameLower, "raidz"):
		device.VdevType = VdevTypeRaidz1
	case nameLower == "spares" || strings.HasPrefix(nameLower, "spare"):
		device.VdevType = VdevTypeSpare
		device.IsSpare = true
	case nameLower == "logs" || strings.HasPrefix(nameLower, "log"):
		device.VdevType = VdevTypeLog
		device.IsLog = true
	case nameLower == "cache":
		device.VdevType = VdevTypeCache
		device.IsCache = true
	case strings.HasPrefix(nameLower, "replacing"):
		device.IsReplacing = true
	default:
		device.VdevType = VdevTypeDisk
	}

	// Set path for actual devices
	if device.VdevType == VdevTypeDisk && !strings.Contains(device.Name, "-") {
		if strings.HasPrefix(device.Name, "/dev/") {
			device.Path = device.Name
		} else {
			device.Path = "/dev/" + device.Name
		}
	}

	return device
}

// addDeviceToPool adds a device to the pool structure with proper hierarchy
func addDeviceToPool(pool *Pool, device *Device, currentVdev **Device, vdevStack *[]*Device, line string) {
	// Determine indentation level (each level is typically 2 spaces)
	indent := len(line) - len(strings.TrimLeft(line, " \t"))
	level := indent / 2

	// Pool name line (level 1-2)
	if device.Name == pool.Name {
		return // Skip pool name line
	}

	// Top-level vdev (level 2-3)
	if level <= 3 && (device.VdevType == VdevTypeMirror || device.VdevType == VdevTypeRaidz1 ||
		device.VdevType == VdevTypeRaidz2 || device.VdevType == VdevTypeRaidz3 ||
		device.VdevType == VdevTypeSpare || device.VdevType == VdevTypeLog ||
		device.VdevType == VdevTypeCache) {
		pool.Devices = append(pool.Devices, *device)
		*currentVdev = &pool.Devices[len(pool.Devices)-1]
		*vdevStack = []*Device{*currentVdev}
		return
	}

	// Child device
	if *currentVdev != nil && device.VdevType == VdevTypeDisk {
		device.VdevParent = (*currentVdev).Name
		device.VdevIndex = len((*currentVdev).Children)
		(*currentVdev).Children = append((*currentVdev).Children, *device)
		return
	}

	// Standalone disk (no vdev parent - simple stripe)
	if device.VdevType == VdevTypeDisk {
		pool.Devices = append(pool.Devices, *device)
	}
}

// parseErrorLine parses the errors line
func parseErrorLine(pool *Pool, line string) {
	// Example: "errors: No known data errors"
	// Example: "errors: 5 data errors, use '-v' for a list"
	lower := strings.ToLower(line)
	if strings.Contains(lower, "no known") {
		return // No errors
	}

	// Try to extract error count from the line
	// Format: "errors: N data errors, use '-v' for a list"
	if idx := strings.Index(lower, " data error"); idx > 0 {
		// Find the number before "data error"
		numStr := strings.TrimSpace(lower[8:idx]) // Skip "errors: "
		if count, err := strconv.ParseInt(numStr, 10, 64); err == nil && count > 0 {
			// Distribute errors across error types (we can't distinguish without -v)
			// For now, just note that there are data errors
			pool.ChecksumErrors += count
		}
	}
}

// ─── Helper Functions ────────────────────────────────────────────────────────

// parseBytes parses a byte value (from -p output, already in bytes)
func parseBytes(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "-" || s == "" {
		return 0
	}
	val, _ := strconv.ParseInt(s, 10, 64)
	return val
}

// parseInt parses an integer, returning 0 on error
func parseInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "-" || s == "" {
		return 0
	}
	// Remove % suffix if present
	s = strings.TrimSuffix(s, "%")
	val, _ := strconv.Atoi(s)
	return val
}

// parseInt64 parses an int64, returning 0 on error
func parseInt64(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "-" || s == "" {
		return 0
	}
	val, _ := strconv.ParseInt(s, 10, 64)
	return val
}

// parseFloat parses a float, returning 0 on error
func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "-" || s == "" {
		return 0
	}
	val, _ := strconv.ParseFloat(s, 64)
	return val
}

// parseHumanSize parses human-readable sizes (e.g., "1.5T", "500G", "100M")
func parseHumanSize(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-" {
		return 0
	}

	multiplier := int64(1)
	s = strings.ToUpper(s)

	if strings.HasSuffix(s, "K") {
		multiplier = 1024
		s = s[:len(s)-1]
	} else if strings.HasSuffix(s, "M") {
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	} else if strings.HasSuffix(s, "G") {
		multiplier = 1024 * 1024 * 1024
		s = s[:len(s)-1]
	} else if strings.HasSuffix(s, "T") {
		multiplier = 1024 * 1024 * 1024 * 1024
		s = s[:len(s)-1]
	} else if strings.HasSuffix(s, "P") {
		multiplier = 1024 * 1024 * 1024 * 1024 * 1024
		s = s[:len(s)-1]
	}

	val, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}

	return int64(val * float64(multiplier))
}

// ─── Collect All ZFS Data ────────────────────────────────────────────────────

// CollectZFSData gathers all ZFS information for the current host
func CollectZFSData(hostname string) (*ZFSReport, error) {
	report := &ZFSReport{
		Hostname:  hostname,
		Timestamp: time.Now(),
		Available: IsZFSAvailable(),
	}

	if !report.Available {
		return report, nil
	}

	// Get pool list
	pools, err := ListPools()
	if err != nil {
		return report, fmt.Errorf("failed to list pools: %w", err)
	}

	// Get detailed status for each pool
	for i := range pools {
		status, err := GetPoolStatus(pools[i].Name)
		if err != nil {
			// Keep basic info even if status fails
			pools[i].Hostname = hostname
			continue
		}

		// Merge status details into pool
		pools[i].Hostname = hostname
		pools[i].Health = status.Health
		pools[i].Status = status.Status
		pools[i].Scan = status.Scan
		pools[i].Devices = status.Devices
		pools[i].ReadErrors = status.ReadErrors
		pools[i].WriteErrors = status.WriteErrors
		pools[i].ChecksumErrors = status.ChecksumErrors

		// Calculate total errors from devices
		for _, dev := range pools[i].Devices {
			pools[i].ReadErrors += sumDeviceErrors(dev, "read")
			pools[i].WriteErrors += sumDeviceErrors(dev, "write")
			pools[i].ChecksumErrors += sumDeviceErrors(dev, "checksum")
		}
	}

	report.Pools = pools
	return report, nil
}

// sumDeviceErrors recursively sums errors for a device and its children
func sumDeviceErrors(dev Device, errType string) int64 {
	var total int64
	switch errType {
	case "read":
		total = dev.ReadErrors
	case "write":
		total = dev.WriteErrors
	case "checksum":
		total = dev.ChecksumErrors
	}

	for _, child := range dev.Children {
		total += sumDeviceErrors(child, errType)
	}

	return total
}
