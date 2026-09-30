package handlers

import (
	"bytes"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	agentsmart "vigil/cmd/agent/smart"
	"vigil/internal/agents"
	"vigil/internal/db"
	"vigil/internal/metrics"
	"vigil/internal/smart"
	"vigil/internal/wearout"
	"vigil/internal/zfs"
)

// MinMetricsTokenLength is the shortest VIGIL_METRICS_TOKEN the server accepts.
// The endpoint is not rate limited, so the token has to be unguessable on its
// own; anything shorter leaves /metrics disabled.
const MinMetricsTokenLength = 16

// prometheusSmartAttrIDs are the ATA attributes worth graphing and alerting
// on: the counters that predict failure plus power-on hours. Exporting the
// whole table would multiply the series count for no alerting value.
var prometheusSmartAttrIDs = map[int]bool{
	5:   true, // Reallocated_Sector_Ct
	9:   true, // Power_On_Hours
	10:  true, // Spin_Retry_Count
	187: true, // Reported_Uncorrect
	188: true, // Command_Timeout
	197: true, // Current_Pending_Sector
	198: true, // Offline_Uncorrectable
	199: true, // UDMA_CRC_Error_Count
}

// PrometheusMetrics serves GET /metrics in the Prometheus text format.
//
// It is independent of the session login: Prometheus cannot do Vigil's cookie
// dance, so the endpoint takes a static bearer token from VIGIL_METRICS_TOKEN.
//   - token unset (or too short) → 404, as if the route did not exist
//   - missing / wrong token      → 401
//   - right token                → 200 with the exposition
//
// It is read-only by construction: it only reads the latest stored data and
// never triggers an agent, a scan or a write.
func PrometheusMetrics(token string) http.HandlerFunc {
	enabled := len(token) >= MinMetricsTokenLength
	want := sha256.Sum256([]byte(token))

	return func(w http.ResponseWriter, r *http.Request) {
		if !enabled {
			http.NotFound(w, r)
			return
		}
		if !metricsTokenOK(r.Header.Get("Authorization"), want) {
			w.Header().Set("WWW-Authenticate", `Bearer realm="vigil-metrics"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		e, err := buildPrometheusExposition()
		if err != nil {
			log.Printf("⚠️  /metrics: %v", err)
			http.Error(w, "failed to collect metrics", http.StatusInternalServerError)
			return
		}
		var buf bytes.Buffer
		if _, err := e.WriteTo(&buf); err != nil {
			http.Error(w, "failed to render metrics", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", metrics.ContentType)
		w.Header().Set("Cache-Control", "no-store")
		w.Write(buf.Bytes())
	}
}

// metricsTokenOK compares hashes so the comparison is constant-time and does
// not leak the token length either.
func metricsTokenOK(header string, want [32]byte) bool {
	const prefix = "bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return false
	}
	got := sha256.Sum256([]byte(strings.TrimSpace(header[len(prefix):])))
	return subtle.ConstantTimeCompare(got[:], want[:]) == 1
}

// promMetrics holds every family, declared up front so the page always lists
// them in the same order.
type promMetrics struct {
	buildInfo *metrics.Family

	reportsProcessed, reportsDropped       *metrics.Family
	notificationsSent, notificationsFailed *metrics.Family
	reportQueueDepth                       *metrics.Family

	agentLastSeen, agentEnabled *metrics.Family
	hostLastReport, hostDrives  *metrics.Family

	driveInfo, driveHealth, driveHealthIssues, driveSmartPassed *metrics.Family
	driveTemp, drivePowerOn, driveAttrRaw                       *metrics.Family
	nvmeMediaErrors, nvmePercentUsed, nvmeUnsafeShutdowns       *metrics.Family
	nvmeCriticalWarning, nvmeAvailableSpare                     *metrics.Family
	driveWearout, driveBaselineAck, driveBaselineCounters       *metrics.Family

	poolHealth, poolOnline, poolSize, poolAlloc, poolFree *metrics.Family
	poolFrag, poolErrors, poolLastScan, poolScanRunning   *metrics.Family
	poolScanErrors, deviceErrors                          *metrics.Family
}

func newPromMetrics(e *metrics.Exposition) *promMetrics {
	return &promMetrics{
		buildInfo: e.Gauge("vigil_build_info", "Vigil server build information; always 1."),

		reportsProcessed:    e.Counter("vigil_reports_processed_total", "Agent reports processed since the server started."),
		reportsDropped:      e.Counter("vigil_reports_dropped_total", "Agent reports whose background processing was dropped (queue full) since the server started."),
		notificationsSent:   e.Counter("vigil_notifications_sent_total", "Notifications sent since the server started."),
		notificationsFailed: e.Counter("vigil_notifications_failed_total", "Notifications that failed to send since the server started."),
		reportQueueDepth:    e.Gauge("vigil_report_queue_depth", "Agent reports waiting for background processing."),

		agentLastSeen:  e.Gauge("vigil_agent_last_seen_timestamp_seconds", "Unix time an agent of this host last reported (agent registry)."),
		agentEnabled:   e.Gauge("vigil_agent_enabled", "1 if at least one registered agent of this host is enabled."),
		hostLastReport: e.Gauge("vigil_host_last_report_timestamp_seconds", "Unix time of the newest stored report of this host."),
		hostDrives:     e.Gauge("vigil_host_drives", "Drives in the newest report of this host."),

		driveInfo:         e.Gauge("vigil_drive_info", "Drive identity; always 1. Join on (host, serial) for model/alias/device."),
		driveHealth:       e.Gauge("vigil_drive_health", "Vigil's health verdict (same as the UI): 0 healthy, 1 warning, 2 critical."),
		driveHealthIssues: e.Gauge("vigil_drive_health_issues", "Number of health issues Vigil found on the drive (acknowledged counters excluded)."),
		driveSmartPassed:  e.Gauge("vigil_drive_smart_passed", "1 if the drive's own SMART overall self-assessment passed."),
		driveTemp:         e.Gauge("vigil_drive_temperature_celsius", "Current drive temperature."),
		drivePowerOn:      e.Gauge("vigil_drive_power_on_seconds", "Drive power-on time (SMART power-on hours × 3600)."),
		driveAttrRaw:      e.Gauge("vigil_drive_smart_attribute_raw", "Raw value of key ATA SMART attributes (5, 9, 10, 187, 188, 197, 198, 199)."),

		nvmeMediaErrors:     e.Gauge("vigil_drive_nvme_media_errors", "NVMe media and data integrity errors."),
		nvmePercentUsed:     e.Gauge("vigil_drive_nvme_percentage_used", "NVMe endurance used, as reported by the drive (can exceed 100)."),
		nvmeUnsafeShutdowns: e.Gauge("vigil_drive_nvme_unsafe_shutdowns", "NVMe unsafe shutdown count."),
		nvmeCriticalWarning: e.Gauge("vigil_drive_nvme_critical_warning", "NVMe critical warning bitfield (0 = none)."),
		nvmeAvailableSpare:  e.Gauge("vigil_drive_nvme_available_spare_percent", "NVMe available spare capacity."),

		driveWearout:          e.Gauge("vigil_drive_wearout_percent", "Vigil's latest wear-out estimate for the drive (0 = new, 100 = worn out)."),
		driveBaselineAck:      e.Gauge("vigil_drive_baseline_acknowledged", "1 if the drive's current error counters were acknowledged (baseline)."),
		driveBaselineCounters: e.Gauge("vigil_drive_baseline_acknowledged_counters", "Number of SMART counters acknowledged in the drive's baseline."),

		poolHealth:      e.Gauge("vigil_zfs_pool_health", "ZFS pool health state; always 1, the state is in the label."),
		poolOnline:      e.Gauge("vigil_zfs_pool_online", "1 if the ZFS pool health is ONLINE."),
		poolSize:        e.Gauge("vigil_zfs_pool_size_bytes", "ZFS pool size."),
		poolAlloc:       e.Gauge("vigil_zfs_pool_allocated_bytes", "ZFS pool allocated space."),
		poolFree:        e.Gauge("vigil_zfs_pool_free_bytes", "ZFS pool free space."),
		poolFrag:        e.Gauge("vigil_zfs_pool_fragmentation_percent", "ZFS pool fragmentation."),
		poolErrors:      e.Gauge("vigil_zfs_pool_errors", "ZFS pool error counters since the last zpool clear."),
		poolLastScan:    e.Gauge("vigil_zfs_pool_last_scan_timestamp_seconds", "Unix time the pool's latest scrub/resilver started (or ended, if the start is unknown)."),
		poolScanRunning: e.Gauge("vigil_zfs_pool_scan_running", "1 while a scrub or resilver is in progress."),
		poolScanErrors:  e.Gauge("vigil_zfs_pool_scan_errors", "Errors found by the pool's latest scrub/resilver."),
		deviceErrors:    e.Gauge("vigil_zfs_device_errors", "ZFS vdev/device error counters since the last zpool clear."),
	}
}

// buildPrometheusExposition reads the latest stored state. Only the report
// query is fatal; every other source is best-effort, so one broken table
// costs its own metrics, not the whole scrape.
func buildPrometheusExposition() (*metrics.Exposition, error) {
	e := metrics.NewExposition()
	m := newPromMetrics(e)

	m.buildInfo.Add(1, "version", Version)
	if Metrics != nil {
		m.reportsProcessed.Add(float64(Metrics.ReportsProcessed.Load()))
		m.reportsDropped.Add(float64(Metrics.ReportsDropped.Load()))
		m.notificationsSent.Add(float64(Metrics.NotificationsSent.Load()))
		m.notificationsFailed.Add(float64(Metrics.NotificationsFailed.Load()))
	}
	m.reportQueueDepth.Add(float64(ReportQueueDepth()))

	collectAgentMetrics(m)
	if err := collectDriveMetrics(m); err != nil {
		return nil, err
	}
	collectZFSMetrics(m)
	return e, nil
}

func collectAgentMetrics(m *promMetrics) {
	list, err := agents.ListAgents(db.DB)
	if err != nil {
		log.Printf("⚠️  /metrics: agents: %v", err)
		return
	}
	// A host can have several registry rows (re-registered agents): report
	// the newest last_seen and "enabled if any is", one series per host.
	type agg struct {
		lastSeen time.Time
		enabled  bool
	}
	byHost := map[string]*agg{}
	var order []string
	for _, a := range list {
		h, ok := byHost[a.Hostname]
		if !ok {
			h = &agg{}
			byHost[a.Hostname] = h
			order = append(order, a.Hostname)
		}
		if a.LastSeenAt != nil && a.LastSeenAt.After(h.lastSeen) {
			h.lastSeen = *a.LastSeenAt
		}
		h.enabled = h.enabled || a.Enabled
	}
	for _, host := range order {
		h := byHost[host]
		if !h.lastSeen.IsZero() {
			m.agentLastSeen.Add(unixSeconds(h.lastSeen), "host", host)
		}
		m.agentEnabled.Add(boolFloat(h.enabled), "host", host)
	}
}

func collectDriveMetrics(m *promMetrics) error {
	rows, err := db.DB.Query(latestReportsSQL)
	if err != nil {
		return fmt.Errorf("latest reports: %w", err)
	}
	defer rows.Close()

	aliases := loadAliases()
	wear := latestWearout()
	baselines := baselineCounts()

	for rows.Next() {
		var host, ts, lastSeen string
		var dataRaw []byte
		if err := rows.Scan(&host, &ts, &dataRaw, &lastSeen); err != nil {
			continue
		}
		var data map[string]interface{}
		if err := json.Unmarshal(dataRaw, &data); err != nil {
			continue
		}
		if t, ok := parseStoredTime(ts); ok {
			m.hostLastReport.Add(unixSeconds(t), "host", host)
		}

		// Same verdict the UI shows: one definition of "critical".
		enrichDrivesWithHealth(data, host)

		drives, _ := data["drives"].([]interface{})
		m.hostDrives.Add(float64(len(drives)), "host", host)
		for _, d := range drives {
			drive, ok := d.(map[string]interface{})
			if !ok {
				continue
			}
			addDriveMetrics(m, host, drive, aliases, wear, baselines)
		}
	}
	return rows.Err()
}

func addDriveMetrics(m *promMetrics, host string, drive map[string]interface{},
	aliases map[string]string, wear map[string]float64, baselines map[string]int) {
	parsed, err := agentsmart.ParseSmartAttributes(drive, host)
	if err != nil || parsed.SerialNumber == "" {
		return // without a serial there is no stable identity to label with
	}
	serial := parsed.SerialNumber
	key := host + ":" + serial
	id := []string{"host", host, "serial", serial}

	m.driveInfo.Add(1, "host", host, "serial", serial, "model", parsed.ModelName,
		"alias", aliases[key], "device", parsed.DeviceName, "type", parsed.DriveType)

	if v, ok := healthValue(drive["_health"]); ok {
		m.driveHealth.Add(v, id...)
	}
	if n, ok := drive["_health_issues"].(int); ok {
		m.driveHealthIssues.Add(float64(n), id...)
	}
	if st, ok := drive["smart_status"].(map[string]interface{}); ok {
		if p, ok := st["passed"].(bool); ok {
			m.driveSmartPassed.Add(boolFloat(p), id...)
		}
	}
	// 0 means "no reading" in the parser; above the ceiling is corrupt data.
	if t := parsed.Temperature; t > 0 && t <= maxPlausibleTempC {
		m.driveTemp.Add(float64(t), id...)
	}
	if parsed.PowerOnHours > 0 {
		m.drivePowerOn.Add(float64(parsed.PowerOnHours)*3600, id...)
	}

	if parsed.DriveType == agentsmart.DriveTypeNVMe {
		addNVMeMetrics(m, drive, id)
	} else {
		for _, a := range parsed.Attributes {
			if prometheusSmartAttrIDs[a.ID] {
				m.driveAttrRaw.Add(float64(a.RawValue), append(id, "id", strconv.Itoa(a.ID), "name", a.Name)...)
			}
		}
	}

	if pct, ok := wear[key]; ok {
		m.driveWearout.Add(pct, id...)
	}
	n := baselines[key]
	m.driveBaselineAck.Add(boolFloat(n > 0), id...)
	m.driveBaselineCounters.Add(float64(n), id...)
}

// addNVMeMetrics reads the NVMe health log directly: the parser maps it onto
// pseudo ATA ids (187, 9, 12…) that would collide with real ATA attributes in
// vigil_drive_smart_attribute_raw.
func addNVMeMetrics(m *promMetrics, drive map[string]interface{}, id []string) {
	hl, ok := drive["nvme_smart_health_information_log"].(map[string]interface{})
	if !ok {
		return
	}
	for field, fam := range map[string]*metrics.Family{
		"media_errors":     m.nvmeMediaErrors,
		"percentage_used":  m.nvmePercentUsed,
		"unsafe_shutdowns": m.nvmeUnsafeShutdowns,
		"critical_warning": m.nvmeCriticalWarning,
		"available_spare":  m.nvmeAvailableSpare,
	} {
		if v, ok := hl[field].(float64); ok {
			fam.Add(v, id...)
		}
	}
}

func collectZFSMetrics(m *promMetrics) {
	pools, err := zfs.GetAllZFSPools(db.DB)
	if err != nil {
		log.Printf("⚠️  /metrics: zfs pools: %v", err)
		return
	}
	for _, p := range pools {
		id := []string{"host", p.Hostname, "pool", p.PoolName}
		m.poolHealth.Add(1, append(id, "state", p.Health)...)
		m.poolOnline.Add(boolFloat(p.Health == "ONLINE"), id...)
		m.poolSize.Add(float64(p.SizeBytes), id...)
		m.poolAlloc.Add(float64(p.AllocatedBytes), id...)
		m.poolFree.Add(float64(p.FreeBytes), id...)
		m.poolFrag.Add(float64(p.Fragmentation), id...)
		m.poolErrors.Add(float64(p.ReadErrors), append(id, "type", "read")...)
		m.poolErrors.Add(float64(p.WriteErrors), append(id, "type", "write")...)
		m.poolErrors.Add(float64(p.ChecksumErrors), append(id, "type", "checksum")...)
		if !p.LastScanTime.IsZero() {
			m.poolLastScan.Add(unixSeconds(p.LastScanTime), append(id, "function", p.ScanFunction)...)
			m.poolScanErrors.Add(float64(p.ScanErrors), id...)
		}
		m.poolScanRunning.Add(boolFloat(p.ScanState == "scanning" || p.ScanState == "in_progress"), id...)
	}

	devices, err := zfs.GetAllZFSPoolDevices(db.DB)
	if err != nil {
		log.Printf("⚠️  /metrics: zfs devices: %v", err)
		return
	}
	for _, d := range devices {
		id := []string{"host", d.Hostname, "pool", d.PoolName, "device", d.DeviceName,
			"vdev_type", d.VdevType, "serial", d.SerialNumber}
		m.deviceErrors.Add(float64(d.ReadErrors), append(id, "type", "read")...)
		m.deviceErrors.Add(float64(d.WriteErrors), append(id, "type", "write")...)
		m.deviceErrors.Add(float64(d.ChecksumErrors), append(id, "type", "checksum")...)
	}
}

// latestWearout maps "host:serial" to the newest wear-out percentage.
func latestWearout() map[string]float64 {
	out := map[string]float64{}
	snaps, err := wearout.GetAllLatestSnapshots(db.DB)
	if err != nil {
		log.Printf("⚠️  /metrics: wearout: %v", err)
		return out
	}
	for _, s := range snaps {
		out[s.Hostname+":"+s.SerialNumber] = s.Percentage
	}
	return out
}

// baselineCounts maps "host:serial" to how many counters are acknowledged.
func baselineCounts() map[string]int {
	out := map[string]int{}
	list, err := smart.ListBaselines(db.DB, "")
	if err != nil {
		log.Printf("⚠️  /metrics: baselines: %v", err)
		return out
	}
	for _, b := range list {
		out[b.Hostname+":"+b.SerialNumber]++
	}
	return out
}

func healthValue(v interface{}) (float64, bool) {
	switch v {
	case agentsmart.SeverityHealthy:
		return 0, true
	case agentsmart.SeverityWarning:
		return 1, true
	case agentsmart.SeverityCritical:
		return 2, true
	}
	return 0, false
}

// parseStoredTime parses a timestamp as SQLite hands it back: the bare UTC
// layout the report handler writes, or RFC3339. A discarded parse error here
// would silently become 1970.
func parseStoredTime(s string) (time.Time, bool) {
	for _, layout := range []string{"2006-01-02 15:04:05", time.RFC3339Nano} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

func unixSeconds(t time.Time) float64 { return float64(t.Unix()) }

func boolFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}
