package zfs

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ─── ZFS Binary Path Detection ───────────────────────────────────────────────

var zpoolPaths = []string{
	"zpool",
	"/sbin/zpool",
	"/usr/sbin/zpool",
	"/usr/local/sbin/zpool",
	"/usr/local/bin/zpool",
}

var zfsPaths = []string{
	"zfs",
	"/sbin/zfs",
	"/usr/sbin/zfs",
	"/usr/local/sbin/zfs",
	"/usr/local/bin/zfs",
}

var zpoolCmd string
var zfsCmd string

func findZpoolCommand() string {
	if zpoolCmd != "" {
		return zpoolCmd
	}

	for _, path := range zpoolPaths {
		if path == "zpool" {
			if p, err := exec.LookPath("zpool"); err == nil {
				zpoolCmd = p
				return zpoolCmd
			}
		} else {
			if _, err := os.Stat(path); err == nil {
				zpoolCmd = path
				return zpoolCmd
			}
		}
	}
	return ""
}

func findZfsCommand() string {
	if zfsCmd != "" {
		return zfsCmd
	}

	for _, path := range zfsPaths {
		if path == "zfs" {
			if p, err := exec.LookPath("zfs"); err == nil {
				zfsCmd = p
				return zfsCmd
			}
		} else {
			if _, err := os.Stat(path); err == nil {
				zfsCmd = path
				return zfsCmd
			}
		}
	}
	return ""
}

func IsZFSAvailable() bool {
	return findZpoolCommand() != ""
}

// ─── Pool List Parsing ───────────────────────────────────────────────────────

func ListPools() ([]Pool, error) {
	zpoolPath := findZpoolCommand()
	if zpoolPath == "" {
		return nil, fmt.Errorf("zpool command not found")
	}

	cmd := exec.Command(zpoolPath, "list", "-H", "-p", "-o",
		"name,size,alloc,free,frag,cap,dedup,health,altroot,guid")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "no pools available") {
			return []Pool{}, nil
		}
		return nil, fmt.Errorf("zpool list failed: %v - %s", err, stderr.String())
	}

	return parsePoolList(stdout.String())
}

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
			continue
		}

		pool := Pool{
			Name:      fields[0],
			Size:      parseBytes(fields[1]),
			Allocated: parseBytes(fields[2]),
			Free:      parseBytes(fields[3]),
			LastSeen:  time.Now(),
		}

		if fields[4] != "-" {
			pool.Fragmentation = parseInt(fields[4])
		}

		pool.CapacityPct = parseInt(fields[5])
		pool.DedupRatio = parseFloat(strings.TrimSuffix(fields[6], "x"))
		pool.Health = fields[7]
		pool.Status = fields[7]

		if len(fields) > 8 && fields[8] != "-" {
			pool.Altroot = fields[8]
		}

		if len(fields) > 9 && fields[9] != "-" {
			pool.GUID = fields[9]
		}

		pools = append(pools, pool)
	}

	return pools, scanner.Err()
}

// ─── Pool Status Parsing ─────────────────────────────────────────────────────

// GetPoolStatus retrieves detailed status for a specific pool
// Uses -L flag to show device names instead of GUIDs when possible
func GetPoolStatus(poolName string) (*Pool, error) {
	zpoolPath := findZpoolCommand()
	if zpoolPath == "" {
		return nil, fmt.Errorf("zpool command not found")
	}

	// Try with -L flag first (shows device names instead of GUIDs)
	// -L: Display real paths for vdevs resolving all symbolic links
	// -P: Display real paths for vdevs instead of only the last component
	cmd := exec.Command(zpoolPath, "status", "-v", "-p", "-L", "-P", poolName)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// If -L/-P flags not supported, try without them
		cmd = exec.Command(zpoolPath, "status", "-v", "-p", poolName)
		stdout.Reset()
		stderr.Reset()
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			return nil, fmt.Errorf("zpool status failed: %v - %s", err, stderr.String())
		}
	}

	return parsePoolStatus(poolName, stdout.String())
}

func parsePoolStatus(poolName, output string) (*Pool, error) {
	pool := &Pool{
		Name:     poolName,
		LastSeen: time.Now(),
	}

	lines := strings.Split(output, "\n")
	var inConfig bool
	var currentVdev *Device
	var vdevStack []*Device

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "state:") {
			pool.Health = strings.TrimSpace(strings.TrimPrefix(trimmed, "state:"))
			pool.Status = pool.Health
			continue
		}

		if strings.HasPrefix(trimmed, "scan:") {
			pool.Scan = parseScanLine(trimmed, lines, &i)
			continue
		}

		if strings.HasPrefix(trimmed, "config:") {
			inConfig = true
			continue
		}

		if inConfig && (strings.HasPrefix(trimmed, "errors:") || trimmed == "") {
			if strings.HasPrefix(trimmed, "errors:") {
				parseErrorLine(pool, trimmed)
			}
			if trimmed == "" && len(pool.Devices) > 0 {
				inConfig = false
			}
			continue
		}

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

	for *lineIdx+1 < len(lines) {
		nextLine := lines[*lineIdx+1]
		if strings.HasPrefix(nextLine, "\t") || strings.HasPrefix(nextLine, "        ") {
			fullText += " " + strings.TrimSpace(nextLine)
			*lineIdx++
		} else {
			break
		}
	}

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
	} else if strings.Contains(lowerText, "repaired") {
		scan.State = ScanStateFinished
	}

	scan.StartTime = parseZFSTimestamp(fullText, "since")
	scan.EndTime = parseZFSTimestamp(fullText, "on")

	if !scan.EndTime.IsZero() && !scan.StartTime.IsZero() {
		scan.Duration = int64(scan.EndTime.Sub(scan.StartTime).Seconds())
	}

	if idx := strings.Index(lowerText, " in "); idx > 0 {
		rest := lowerText[idx+4:]
		if colonIdx := strings.Index(rest, ":"); colonIdx > 0 && colonIdx < 5 {
			parts := strings.Fields(rest)
			if len(parts) > 0 {
				scan.Duration = parseHHMMSS(parts[0])
			}
		}
	}

	scan.ProgressPct = parsePercentage(lowerText, "% done")
	if scan.ProgressPct == 0 && scan.State == ScanStateFinished {
		scan.ProgressPct = 100.0
	}

	scan.ErrorsFound = parseNumberBefore(lowerText, " errors")
	scan.DataExamined = parseSizeBefore(fullText, " scanned")

	if idx := strings.Index(lowerText, "out of "); idx >= 0 {
		rest := fullText[idx+7:]
		parts := strings.Fields(rest)
		if len(parts) > 0 {
			scan.DataTotal = parseHumanSize(parts[0])
		}
	} else if idx := strings.Index(lowerText, " total"); idx > 0 {
		scan.DataTotal = parseSizeBefore(fullText, " total")
	}

	scan.BytesRepaired = parseSizeBefore(fullText, " repaired")
	scan.Rate = parseScanRate(fullText)
	scan.TimeRemaining = parseTimeRemaining(lowerText)

	return scan
}

func parseZFSTimestamp(text, keyword string) time.Time {
	lowerText := strings.ToLower(text)
	idx := strings.Index(lowerText, keyword+" ")
	if idx < 0 {
		return time.Time{}
	}

	rest := strings.TrimSpace(text[idx+len(keyword)+1:])

	formats := []string{
		"Mon Jan _2 15:04:05 2006",
		"Mon Jan  2 15:04:05 2006",
		"Mon Jan 2 15:04:05 2006",
		"Jan _2 15:04:05 2006",
		"Jan  2 15:04:05 2006",
		"2006-01-02 15:04:05",
	}

	endMarkers := []string{" with ", " 0b ", " 0B "}
	for _, marker := range endMarkers {
		if endIdx := strings.Index(rest, marker); endIdx > 0 {
			rest = rest[:endIdx]
			break
		}
	}

	rest = strings.Join(strings.Fields(rest), " ")

	for _, format := range formats {
		if t, err := time.Parse(format, rest); err == nil {
			return t
		}
		for i := len(rest); i > 10; i-- {
			if t, err := time.Parse(format, rest[:i]); err == nil {
				return t
			}
		}
	}

	return time.Time{}
}

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

func parsePercentage(text, marker string) float64 {
	idx := strings.Index(text, marker)
	if idx <= 0 {
		return 0
	}

	start := idx - 1
	for start > 0 && (text[start] >= '0' && text[start] <= '9' || text[start] == '.') {
		start--
	}

	if pct, err := strconv.ParseFloat(strings.TrimSpace(text[start+1:idx]), 64); err == nil {
		return pct
	}
	return 0
}

func parseNumberBefore(text, marker string) int64 {
	idx := strings.Index(text, marker)
	if idx <= 0 {
		return 0
	}

	start := idx - 1
	for start > 0 && text[start] >= '0' && text[start] <= '9' {
		start--
	}

	numStr := strings.TrimSpace(text[start+1 : idx])
	if num, err := strconv.ParseInt(numStr, 10, 64); err == nil {
		return num
	}
	return 0
}

func parseSizeBefore(text, marker string) int64 {
	lowerText := strings.ToLower(text)
	lowerMarker := strings.ToLower(marker)
	idx := strings.Index(lowerText, lowerMarker)
	if idx <= 0 {
		return 0
	}

	before := strings.TrimSpace(text[:idx])
	parts := strings.Fields(before)
	if len(parts) == 0 {
		return 0
	}

	return parseHumanSize(parts[len(parts)-1])
}

func parseScanRate(text string) int64 {
	idx := strings.Index(text, "/s")
	if idx <= 0 {
		return 0
	}

	start := idx - 1
	for start > 0 && ((text[start] >= '0' && text[start] <= '9') || text[start] == '.' ||
		text[start] == 'K' || text[start] == 'M' || text[start] == 'G' || text[start] == 'T' ||
		text[start] == 'k' || text[start] == 'm' || text[start] == 'g' || text[start] == 't') {
		start--
	}

	rateStr := strings.TrimSpace(text[start+1 : idx])
	return parseHumanSize(rateStr)
}

func parseTimeRemaining(text string) int64 {
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

	if strings.Contains(timeStr, ":") {
		return parseHHMMSS(timeStr)
	}

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
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return nil
	}

	fields := strings.Fields(trimmed)
	if len(fields) < 2 {
		return nil
	}

	deviceName := fields[0]

	// Resolve the device name if it's a path or GUID
	resolvedName, resolvedPath := resolveDeviceName(deviceName)

	device := &Device{
		Name:  resolvedName,
		Path:  resolvedPath,
		State: StateOnline,
	}

	if len(fields) >= 2 {
		state := strings.ToUpper(fields[1])
		switch state {
		case "ONLINE", "DEGRADED", "FAULTED", "OFFLINE", "REMOVED", "UNAVAIL":
			device.State = state
		}
	}

	if len(fields) >= 5 {
		device.ReadErrors = parseInt64(fields[2])
		device.WriteErrors = parseInt64(fields[3])
		device.ChecksumErrors = parseInt64(fields[4])
	}

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
		device.VdevType = VdevTypeDisk
	default:
		device.VdevType = VdevTypeDisk
	}

	return device
}

// resolveDeviceName converts GUIDs, paths, or symlinks to actual device names
func resolveDeviceName(name string) (deviceName string, devicePath string) {
	// If it's already a simple device name (sda, nvme0n1, etc.), use it
	if isSimpleDeviceName(name) {
		devicePath = "/dev/" + name
		return name, devicePath
	}

	// If it's a full path like /dev/sda
	if strings.HasPrefix(name, "/dev/") && !strings.Contains(name, "/disk/") {
		deviceName = filepath.Base(name)
		if isSimpleDeviceName(deviceName) {
			return deviceName, name
		}
	}

	// If it looks like a GUID (contains dashes, alphanumeric)
	if looksLikeGUID(name) {
		// Try to find in /dev/disk/by-partuuid/
		resolved := resolveByPartUUID(name)
		if resolved != "" {
			return resolved, "/dev/" + resolved
		}

		// Try /dev/disk/by-id/
		resolved = resolveByDiskID(name)
		if resolved != "" {
			return resolved, "/dev/" + resolved
		}

		// Try gpart on FreeBSD/TrueNAS
		resolved = resolveByGpart(name)
		if resolved != "" {
			return resolved, "/dev/" + resolved
		}
	}

	// If it's a /dev/disk/by-* path, resolve the symlink
	if strings.HasPrefix(name, "/dev/disk/") {
		if target, err := filepath.EvalSymlinks(name); err == nil {
			deviceName = filepath.Base(target)
			return deviceName, target
		}
	}

	// If it's a /dev/gptid/ path (FreeBSD/TrueNAS)
	if strings.HasPrefix(name, "/dev/gptid/") || strings.HasPrefix(name, "gptid/") {
		cleanName := strings.TrimPrefix(name, "/dev/")
		resolved := resolveGptid(cleanName)
		if resolved != "" {
			return resolved, "/dev/" + resolved
		}
	}

	// Return original if we can't resolve
	return name, ""
}

// isSimpleDeviceName checks if a name is a simple device name like sda, nvme0n1, da0
func isSimpleDeviceName(name string) bool {
	// Linux: sda, sdb, nvme0n1, etc.
	// FreeBSD: da0, da1, ada0, nvd0, etc.
	prefixes := []string{"sd", "hd", "nvme", "da", "ada", "nvd", "vd", "xvd"}
	for _, prefix := range prefixes {
		if strings.HasPrefix(name, prefix) {
			rest := strings.TrimPrefix(name, prefix)
			// Should be followed by letters/numbers
			if len(rest) > 0 && (rest[0] >= 'a' && rest[0] <= 'z' || rest[0] >= '0' && rest[0] <= '9') {
				return true
			}
		}
	}
	return false
}

// looksLikeGUID checks if a string looks like a GUID/UUID
func looksLikeGUID(s string) bool {
	// GUIDs typically have dashes and are 36 chars (with dashes) or 32 chars (without)
	if len(s) >= 32 && strings.Contains(s, "-") {
		// Count alphanumeric chars
		count := 0
		for _, c := range s {
			if (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F') {
				count++
			}
		}
		return count >= 32
	}
	return false
}

// resolveByPartUUID looks up a GUID in /dev/disk/by-partuuid/
func resolveByPartUUID(guid string) string {
	byPartuuid := "/dev/disk/by-partuuid"
	entries, err := os.ReadDir(byPartuuid)
	if err != nil {
		return ""
	}

	guidLower := strings.ToLower(guid)
	for _, entry := range entries {
		if strings.ToLower(entry.Name()) == guidLower {
			linkPath := filepath.Join(byPartuuid, entry.Name())
			if target, err := filepath.EvalSymlinks(linkPath); err == nil {
				return filepath.Base(target)
			}
		}
	}
	return ""
}

// resolveByDiskID looks up in /dev/disk/by-id/ for matching entries
func resolveByDiskID(guid string) string {
	byID := "/dev/disk/by-id"
	entries, err := os.ReadDir(byID)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if strings.Contains(entry.Name(), guid) {
			linkPath := filepath.Join(byID, entry.Name())
			if target, err := filepath.EvalSymlinks(linkPath); err == nil {
				return filepath.Base(target)
			}
		}
	}
	return ""
}

// resolveGptid resolves FreeBSD/TrueNAS gptid paths
func resolveGptid(gptid string) string {
	// Try /dev/gptid/GUID -> actual device
	gptidPath := "/dev/" + gptid
	if target, err := filepath.EvalSymlinks(gptidPath); err == nil {
		base := filepath.Base(target)
		// Remove partition suffix (da0p1 -> da0)
		if idx := strings.LastIndex(base, "p"); idx > 0 {
			return base[:idx]
		}
		return base
	}

	// Try glabel command (FreeBSD)
	cmd := exec.Command("glabel", "status", "-s")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err == nil {
		scanner := bufio.NewScanner(&stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, gptid) {
				fields := strings.Fields(line)
				if len(fields) >= 3 {
					device := fields[2]
					// Remove partition suffix
					if idx := strings.LastIndex(device, "p"); idx > 0 {
						return device[:idx]
					}
					return device
				}
			}
		}
	}

	return ""
}

// resolveByGpart uses gpart on FreeBSD to find device by GUID
func resolveByGpart(guid string) string {
	// List all GEOM providers
	cmd := exec.Command("geom", "part", "list")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		return ""
	}

	guidLower := strings.ToLower(guid)
	var currentDevice string
	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Geom name:") {
			currentDevice = strings.TrimSpace(strings.TrimPrefix(line, "Geom name:"))
		}
		if strings.Contains(strings.ToLower(line), guidLower) && currentDevice != "" {
			return currentDevice
		}
	}

	return ""
}

// addDeviceToPool adds a device to the pool structure with proper hierarchy
func addDeviceToPool(pool *Pool, device *Device, currentVdev **Device, vdevStack *[]*Device, line string) {
	indent := len(line) - len(strings.TrimLeft(line, " \t"))
	level := indent / 2

	if device.Name == pool.Name {
		return
	}

	if level <= 3 && (device.VdevType == VdevTypeMirror || device.VdevType == VdevTypeRaidz1 ||
		device.VdevType == VdevTypeRaidz2 || device.VdevType == VdevTypeRaidz3 ||
		device.VdevType == VdevTypeSpare || device.VdevType == VdevTypeLog ||
		device.VdevType == VdevTypeCache) {
		pool.Devices = append(pool.Devices, *device)
		*currentVdev = &pool.Devices[len(pool.Devices)-1]
		*vdevStack = []*Device{*currentVdev}
		return
	}

	if *currentVdev != nil && device.VdevType == VdevTypeDisk {
		device.VdevParent = (*currentVdev).Name
		device.VdevIndex = len((*currentVdev).Children)
		(*currentVdev).Children = append((*currentVdev).Children, *device)
		return
	}

	if device.VdevType == VdevTypeDisk {
		pool.Devices = append(pool.Devices, *device)
	}
}

func parseErrorLine(pool *Pool, line string) {
	lower := strings.ToLower(line)
	if strings.Contains(lower, "no known") {
		return
	}

	if idx := strings.Index(lower, " data error"); idx > 0 {
		numStr := strings.TrimSpace(lower[8:idx])
		if count, err := strconv.ParseInt(numStr, 10, 64); err == nil && count > 0 {
			pool.ChecksumErrors += count
		}
	}
}

// ─── Helper Functions ────────────────────────────────────────────────────────

func parseBytes(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "-" || s == "" {
		return 0
	}
	val, _ := strconv.ParseInt(s, 10, 64)
	return val
}

func parseInt(s string) int {
	s = strings.TrimSpace(s)
	if s == "-" || s == "" {
		return 0
	}
	s = strings.TrimSuffix(s, "%")
	val, _ := strconv.Atoi(s)
	return val
}

func parseInt64(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "-" || s == "" {
		return 0
	}
	val, _ := strconv.ParseInt(s, 10, 64)
	return val
}

func parseFloat(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "-" || s == "" {
		return 0
	}
	val, _ := strconv.ParseFloat(s, 64)
	return val
}

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

func CollectZFSData(hostname string) (*ZFSReport, error) {
	report := &ZFSReport{
		Hostname:  hostname,
		Timestamp: time.Now(),
		Available: IsZFSAvailable(),
	}

	if !report.Available {
		return report, nil
	}

	pools, err := ListPools()
	if err != nil {
		return report, fmt.Errorf("failed to list pools: %w", err)
	}

	serialMap := BuildDeviceSerialMap()

	for i := range pools {
		status, err := GetPoolStatus(pools[i].Name)
		if err != nil {
			pools[i].Hostname = hostname
			continue
		}

		pools[i].Hostname = hostname
		pools[i].Health = status.Health
		pools[i].Status = status.Status
		pools[i].Scan = status.Scan
		pools[i].Devices = status.Devices
		pools[i].ReadErrors = status.ReadErrors
		pools[i].WriteErrors = status.WriteErrors
		pools[i].ChecksumErrors = status.ChecksumErrors

		MapPoolDevicesToSerials(&pools[i], serialMap)

		for _, dev := range pools[i].Devices {
			pools[i].ReadErrors += sumDeviceErrors(dev, "read")
			pools[i].WriteErrors += sumDeviceErrors(dev, "write")
			pools[i].ChecksumErrors += sumDeviceErrors(dev, "checksum")
		}
	}

	report.Pools = pools
	return report, nil
}

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

func GetScrubHistory(poolName string, limit int) ([]ScrubRecord, error) {
	zpoolPath := findZpoolCommand()
	if zpoolPath == "" {
		return nil, fmt.Errorf("zpool command not found")
	}

	cmd := exec.Command(zpoolPath, "history", "-i", poolName)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("zpool history failed: %v - %s", err, stderr.String())
	}

	return parseZpoolHistory(poolName, stdout.String(), limit)
}

func parseZpoolHistory(poolName, output string, limit int) ([]ScrubRecord, error) {
	var records []ScrubRecord
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		lowerLine := strings.ToLower(line)
		if !strings.Contains(lowerLine, "scrub") {
			continue
		}

		record := ScrubRecord{
			PoolName: poolName,
			ScanType: ScanScrub,
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			if t, err := time.Parse("2006-01-02.15:04:05", parts[0]); err == nil {
				record.StartTime = t
			}
		}

		if strings.Contains(lowerLine, "scrub done") || strings.Contains(lowerLine, "completed") {
			record.State = ScanStateFinished
		} else if strings.Contains(lowerLine, "scrub canceled") || strings.Contains(lowerLine, "cancelled") {
			record.State = ScanStateCanceled
		} else if strings.Contains(lowerLine, "zpool scrub") {
			record.State = ScanStateScanning
		}

		if !record.StartTime.IsZero() {
			records = append(records, record)
		}

		if limit > 0 && len(records) >= limit {
			break
		}
	}

	for i, j := 0, len(records)-1; i < j; i, j = i+1, j-1 {
		records[i], records[j] = records[j], records[i]
	}

	return records, nil
}

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

type PoolHealthSummary struct {
	TotalPools      int   `json:"total_pools"`
	HealthyPools    int   `json:"healthy_pools"`
	DegradedPools   int   `json:"degraded_pools"`
	FaultedPools    int   `json:"faulted_pools"`
	TotalErrors     int64 `json:"total_errors"`
	ActiveScrubs    int   `json:"active_scrubs"`
	ActiveResilvers int   `json:"active_resilvers"`
}

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
