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
// Example outputs:
//
//	scan: scrub repaired 0B in 12:34:56 with 0 errors on Sun Feb  2 12:00:00 2025
//	scan: scrub in progress since Sun Feb  2 10:00:00 2025
//	      1.23T scanned at 100M/s, 500G issued at 50M/s, 2.00T total
//	      0B repaired, 25.00% done, 01:30:00 to go
//	scan: resilver in progress since Sun Feb  2 10:00:00 2025
//	      500G scanned out of 2.00T at 100M/s, 25.00% done, 04:00:00 to go
//	scan: scrub canceled on Sun Feb  2 11:00:00 2025
//	scan: none requested
func parseScanLine(firstLine string, lines []string, lineIdx *int) *ScanInfo {
	scan := &ScanInfo{}
	fullText := strings.TrimPrefix(firstLine, "scan:")
	fullText = strings.TrimSpace(fullText)

	// Collect continuation lines (indented lines that are part of scan info)
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

	// No scans
	if strings.Contains(lowerText, "no scans") || strings.Contains(lowerText, "none requested") {
		scan.Function = ScanNone
		scan.State = ScanStateNone
		return scan
	}

	// Determine scan type
	if strings.Contains(lowerText, "resilver") {
		scan.Function = ScanResilver
	} else if strings.Contains(lowerText, "scrub") {
		scan.Function = ScanScrub
	}

	// Determine scan state
	if strings.Contains(lowerText, "in progress") {
		scan.State = ScanStateScanning
	} else if strings.Contains(lowerText, "canceled") {
		scan.State = ScanStateCanceled
	} else if strings.Contains(lowerText, "repaired") {
		scan.State = ScanStateFinished
	}

	// Parse timestamps
	scan.StartTime = parseZFSTimestamp(fullText, "since")
	scan.EndTime = parseZFSTimestamp(fullText, "on")

	// Calculate duration if we have end time (finished/canceled)
	if !scan.EndTime.IsZero() && !scan.StartTime.IsZero() {
		scan.Duration = int64(scan.EndTime.Sub(scan.StartTime).Seconds())
	}

	// Parse duration from "in HH:MM:SS" format (for finished scans)
	// Example: "repaired 0B in 12:34:56 with 0 errors"
	if idx := strings.Index(lowerText, " in "); idx > 0 {
		rest := lowerText[idx+4:]
		if colonIdx := strings.Index(rest, ":"); colonIdx > 0 && colonIdx < 5 {
			// Looks like a time format HH:MM:SS
			parts := strings.Fields(rest)
			if len(parts) > 0 {
				scan.Duration = parseHHMMSS(parts[0])
			}
		}
	}

	// Parse progress percentage (e.g., "45.2% done" or "25.00% done")
	scan.ProgressPct = parsePercentage(lowerText, "% done")
	if scan.ProgressPct == 0 && scan.State == ScanStateFinished {
		scan.ProgressPct = 100.0
	}

	// Parse errors found (e.g., "0 errors" or "with 0 errors")
	scan.ErrorsFound = parseNumberBefore(lowerText, " errors")

	// Parse data scanned/examined (e.g., "1.23T scanned")
	scan.DataExamined = parseSizeBefore(fullText, " scanned")

	// Parse total data (e.g., "out of 2.00T" or "2.00T total")
	if idx := strings.Index(lowerText, "out of "); idx >= 0 {
		rest := fullText[idx+7:]
		parts := strings.Fields(rest)
		if len(parts) > 0 {
			scan.DataTotal = parseHumanSize(parts[0])
		}
	} else if idx := strings.Index(lowerText, " total"); idx > 0 {
		scan.DataTotal = parseSizeBefore(fullText, " total")
	}

	// Parse bytes repaired (e.g., "0B repaired" or "123K repaired")
	scan.BytesRepaired = parseSizeBefore(fullText, " repaired")

	// Parse scan rate (e.g., "100M/s" or "at 100M/s")
	scan.Rate = parseScanRate(fullText)

	// Parse time remaining (e.g., "01:30:00 to go" or "4h30m to go")
	scan.TimeRemaining = parseTimeRemaining(lowerText)

	return scan
}

// parseZFSTimestamp extracts a timestamp after a keyword like "since" or "on"
// Example: "since Sun Feb  2 10:00:00 2025" or "on Sun Feb  2 12:00:00 2025"
func parseZFSTimestamp(text, keyword string) time.Time {
	lowerText := strings.ToLower(text)
	idx := strings.Index(lowerText, keyword+" ")
	if idx < 0 {
		return time.Time{}
	}

	// Extract the rest of the string after the keyword
	rest := strings.TrimSpace(text[idx+len(keyword)+1:])

	// ZFS timestamp formats to try
	formats := []string{
		"Mon Jan _2 15:04:05 2006",
		"Mon Jan  2 15:04:05 2006",
		"Mon Jan 2 15:04:05 2006",
		"Jan _2 15:04:05 2006",
		"Jan  2 15:04:05 2006",
		"2006-01-02 15:04:05",
	}

	// Try to find where the timestamp ends (usually before "with" or end of meaningful text)
	endMarkers := []string{" with ", " 0b ", " 0B "}
	for _, marker := range endMarkers {
		if endIdx := strings.Index(rest, marker); endIdx > 0 {
			rest = rest[:endIdx]
			break
		}
	}

	// Normalize multiple spaces to single space
	rest = strings.Join(strings.Fields(rest), " ")

	for _, format := range formats {
		if t, err := time.Parse(format, rest); err == nil {
			return t
		}
		// Try parsing just the beginning of the string
		for i := len(rest); i > 10; i-- {
			if t, err := time.Parse(format, rest[:i]); err == nil {
				return t
			}
		}
	}

	return time.Time{}
}

// parseHHMMSS parses a duration in HH:MM:SS format
func parseHHMMSS(s string) int64 {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return 0
	}
	hours, _ := strconv.ParseInt(parts[0], 10, 64)
	mins, _ := strconv.ParseInt(parts[1], 10, 64)
	secs, _ := strconv.ParseInt(parts[2], 10, 64)
	return hours*3600 + mins*60 + secs
}

// parsePercentage extracts a percentage value before a marker
func parsePercentage(text, marker string) float64 {
	idx := strings.Index(text, marker)
	if idx <= 0 {
		return 0
	}

	// Walk backwards to find the start of the number
	start := idx - 1
	for start > 0 && (text[start] >= '0' && text[start] <= '9' || text[start] == '.') {
		start--
	}

	if pct, err := strconv.ParseFloat(strings.TrimSpace(text[start+1:idx]), 64); err == nil {
		return pct
	}
	return 0
}

// parseNumberBefore extracts an integer before a marker
func parseNumberBefore(text, marker string) int64 {
	idx := strings.Index(text, marker)
	if idx <= 0 {
		return 0
	}

	// Walk backwards to find the start of the number
	start := idx - 1
	for start > 0 && text[start] >= '0' && text[start] <= '9' {
		start--
	}

	if num, err := strconv.ParseInt(strings.TrimSpace(text[start+1:idx]), 10, 64); err == nil {
		return num
	}
	return 0
}

// parseSizeBefore extracts a size value (like "1.23T") before a marker
func parseSizeBefore(text, marker string) int64 {
	lowerText := strings.ToLower(text)
	lowerMarker := strings.ToLower(marker)
	idx := strings.Index(lowerText, lowerMarker)
	if idx <= 0 {
		return 0
	}

	// Find the size value before the marker
	before := strings.TrimSpace(text[:idx])
	parts := strings.Fields(before)
	if len(parts) == 0 {
		return 0
	}

	// The size should be the last word before the marker
	return parseHumanSize(parts[len(parts)-1])
}

// parseScanRate extracts scan rate in bytes/sec
func parseScanRate(text string) int64 {
	// Look for patterns like "100M/s" or "at 100M/s"
	idx := strings.Index(text, "/s")
	if idx <= 0 {
		return 0
	}

	// Walk backwards to find the start of the rate value
	start := idx - 1
	for start > 0 && (text[start] >= '0' && text[start] <= '9' || text[start] == '.' ||
		text[start] == 'K' || text[start] == 'k' ||
		text[start] == 'M' || text[start] == 'm' ||
		text[start] == 'G' || text[start] == 'g' ||
		text[start] == 'T' || text[start] == 't') {
		start--
	}

	rateStr := strings.TrimSpace(text[start+1 : idx])
	return parseHumanSize(rateStr)
}

// parseTimeRemaining extracts remaining time in seconds
func parseTimeRemaining(text string) int64 {
	// Look for "HH:MM:SS to go" pattern
	idx := strings.Index(text, " to go")
	if idx <= 0 {
		return 0
	}

	before := strings.TrimSpace(text[:idx])
	parts := strings.Fields(before)
	if len(parts) == 0 {
		return 0
	}

	timeStr := parts[len(parts)-1]

	// Try HH:MM:SS format
	if strings.Contains(timeStr, ":") {
		return parseHHMMSS(timeStr)
	}

	// Try formats like "4h30m" or "30m" or "45s"
	var total int64
	current := ""
	for _, c := range timeStr {
		switch c {
		case 'h', 'H':
			if n, err := strconv.ParseInt(current, 10, 64); err == nil {
				total += n * 3600
			}
			current = ""
		case 'm', 'M':
			if n, err := strconv.ParseInt(current, 10, 64); err == nil {
				total += n * 60
			}
			current = ""
		case 's', 'S':
			if n, err := strconv.ParseInt(current, 10, 64); err == nil {
				total += n
			}
			current = ""
		default:
			if c >= '0' && c <= '9' {
				current += string(c)
			}
		}
	}

	return total
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

	// Build device serial map once for efficiency
	serialMap := BuildDeviceSerialMap()

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

		// Map device serial numbers
		MapPoolDevicesToSerials(&pools[i], serialMap)

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

// ─── Scrub History Functions ─────────────────────────────────────────────────

// GetScrubHistory retrieves scrub history using zpool history (if available)
// This provides historical scrub records beyond the last scan shown in zpool status
func GetScrubHistory(poolName string, limit int) ([]ScrubRecord, error) {
	if !IsZFSAvailable() {
		return nil, fmt.Errorf("zpool command not found")
	}

	// zpool history shows all pool operations including scrubs
	cmd := exec.Command("zpool", "history", "-i", poolName)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		// History might not be available on all systems
		return nil, fmt.Errorf("zpool history failed: %v - %s", err, stderr.String())
	}

	return parseZpoolHistory(poolName, stdout.String(), limit)
}

// parseZpoolHistory parses zpool history output for scrub-related entries
func parseZpoolHistory(poolName, output string, limit int) ([]ScrubRecord, error) {
	var records []ScrubRecord
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Look for scrub-related entries
		// Format: "2025-02-02.10:00:00 zpool scrub tank"
		// Or internal: "2025-02-02.10:00:00 [internal pool scrub done]"
		lowerLine := strings.ToLower(line)
		if !strings.Contains(lowerLine, "scrub") {
			continue
		}

		record := ScrubRecord{
			PoolName: poolName,
			ScanType: ScanScrub,
		}

		// Parse timestamp at the beginning
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			// Try to parse the timestamp
			if t, err := time.Parse("2006-01-02.15:04:05", parts[0]); err == nil {
				record.StartTime = t
			}
		}

		// Determine state from the line content
		if strings.Contains(lowerLine, "scrub done") || strings.Contains(lowerLine, "completed") {
			record.State = ScanStateFinished
		} else if strings.Contains(lowerLine, "scrub canceled") || strings.Contains(lowerLine, "cancelled") {
			record.State = ScanStateCanceled
		} else if strings.Contains(lowerLine, "zpool scrub") {
			record.State = ScanStateScanning // Started
		}

		if !record.StartTime.IsZero() {
			records = append(records, record)
		}

		if limit > 0 && len(records) >= limit {
			break
		}
	}

	// Reverse to get newest first
	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

// ConvertScanToScrubRecord converts a ScanInfo to a ScrubRecord for storage
func ConvertScanToScrubRecord(scan *ScanInfo, hostname, poolName string, poolID int64) *ScrubRecord {
	if scan == nil || scan.Function == ScanNone {
		return nil
	}

	record := &ScrubRecord{
		PoolID:        poolID,
		Hostname:      hostname,
		PoolName:      poolName,
		ScanType:      scan.Function,
		State:         scan.State,
		StartTime:     scan.StartTime,
		EndTime:       scan.EndTime,
		Duration:      scan.Duration,
		DataExamined:  scan.DataExamined,
		DataTotal:     scan.DataTotal,
		ErrorsFound:   scan.ErrorsFound,
		BytesRepaired: scan.BytesRepaired,
		ProgressPct:   scan.ProgressPct,
		Rate:          scan.Rate,
		TimeRemaining: scan.TimeRemaining,
	}

	return record
}

// ─── Pool Health Summary ─────────────────────────────────────────────────────

// PoolHealthSummary provides a quick health overview
type PoolHealthSummary struct {
	TotalPools      int   `json:"total_pools"`
	HealthyPools    int   `json:"healthy_pools"`
	DegradedPools   int   `json:"degraded_pools"`
	FaultedPools    int   `json:"faulted_pools"`
	TotalErrors     int64 `json:"total_errors"`
	ActiveScrubs    int   `json:"active_scrubs"`
	ActiveResilvers int   `json:"active_resilvers"`
}

// GetPoolHealthSummary returns a quick summary of all pools health
func GetPoolHealthSummary(pools []Pool) PoolHealthSummary {
	summary := PoolHealthSummary{
		TotalPools: len(pools),
	}

	for _, pool := range pools {
		switch pool.Health {
		case StateOnline:
			summary.HealthyPools++
		case StateDegraded:
			summary.DegradedPools++
		case StateFaulted:
			summary.FaultedPools++
		}

		summary.TotalErrors += pool.TotalErrors()

		if pool.Scan != nil && pool.Scan.State == ScanStateScanning {
			switch pool.Scan.Function {
			case ScanScrub:
				summary.ActiveScrubs++
			case ScanResilver:
				summary.ActiveResilvers++
			}
		}
	}

	return summary
}
