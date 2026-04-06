package zfs

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"vigil/internal/events"
	"vigil/internal/settings"
)

// ProcessZFSReportWithEvents processes a ZFS report and publishes events
// for any unhealthy pools or failed devices.
func ProcessZFSReportWithEvents(db *sql.DB, bus *events.Bus, hostname string, zfsData json.RawMessage) error {
	if len(zfsData) == 0 || string(zfsData) == "null" {
		return nil
	}

	var report ZFSAgentReport
	if err := json.Unmarshal(zfsData, &report); err != nil {
		return fmt.Errorf("parse ZFS report: %w", err)
	}

	if !report.Available || len(report.Pools) == 0 {
		return nil
	}

	for _, pool := range report.Pools {
		// Fetch previous pool state before ingest overwrites it
		var prevPool *ZFSPool
		if bus != nil {
			prevPool, _ = GetZFSPool(db, hostname, pool.Name)
		}

		poolID, err := processPool(db, hostname, pool)
		if err != nil {
			log.Printf("⚠️  Failed to process pool %s: %v", pool.Name, err)
			continue
		}

		if bus != nil {
			publishPoolEvents(bus, hostname, pool)
			publishDeviceEvents(bus, hostname, pool)
			publishCapacityEvents(bus, db, hostname, pool)
			publishVdevErrorEvents(bus, db, hostname, pool)
			publishScrubOverdueEvents(bus, db, hostname, pool, poolID)
			publishScanTransitionEvents(bus, hostname, pool, prevPool)
		}
	}

	return nil
}

// publishPoolEvents publishes events for unhealthy ZFS pools.
func publishPoolEvents(bus *events.Bus, hostname string, pool ZFSAgentPool) {
	switch pool.Health {
	case "DEGRADED":
		bus.Publish(events.Event{
			Type:     events.ZFSPoolDegraded,
			Severity: events.SeverityWarning,
			Hostname: hostname,
			Message:  fmt.Sprintf("ZFS pool %q is DEGRADED", pool.Name),
			Metadata: map[string]string{
				"pool_name":       pool.Name,
				"pool_guid":       pool.GUID,
				"read_errors":     fmt.Sprintf("%d", pool.ReadErrors),
				"write_errors":    fmt.Sprintf("%d", pool.WriteErrors),
				"checksum_errors": fmt.Sprintf("%d", pool.ChecksumErrors),
			},
		})
	case "FAULTED", "UNAVAIL":
		bus.Publish(events.Event{
			Type:     events.ZFSPoolFaulted,
			Severity: events.SeverityCritical,
			Hostname: hostname,
			Message:  fmt.Sprintf("ZFS pool %q is %s", pool.Name, pool.Health),
			Metadata: map[string]string{
				"pool_name": pool.Name,
				"pool_guid": pool.GUID,
				"health":    pool.Health,
			},
		})
	}
}

// publishDeviceEvents publishes events for failed ZFS devices.
func publishDeviceEvents(bus *events.Bus, hostname string, pool ZFSAgentPool) {
	for _, dev := range pool.Devices {
		publishDeviceRecursive(bus, hostname, pool.Name, dev)
	}
}

func publishDeviceRecursive(bus *events.Bus, hostname, poolName string, dev ZFSAgentDevice) {
	if dev.State == "FAULTED" || dev.State == "UNAVAIL" || dev.State == "REMOVED" {
		bus.Publish(events.Event{
			Type:         events.ZFSDeviceFailed,
			Severity:     events.SeverityCritical,
			Hostname:     hostname,
			SerialNumber: dev.SerialNumber,
			Message:      fmt.Sprintf("ZFS device %q in pool %q is %s", dev.Name, poolName, dev.State),
			Metadata: map[string]string{
				"pool_name":       poolName,
				"device_name":     dev.Name,
				"device_state":    dev.State,
				"vdev_type":       dev.VdevType,
				"read_errors":     fmt.Sprintf("%d", dev.ReadErrors),
				"write_errors":    fmt.Sprintf("%d", dev.WriteErrors),
				"checksum_errors": fmt.Sprintf("%d", dev.ChecksumErrors),
			},
		})
	}

	for _, child := range dev.Children {
		publishDeviceRecursive(bus, hostname, poolName, child)
	}
}

// publishCapacityEvents fires warnings when pool capacity or fragmentation
// exceeds the configured thresholds.
func publishCapacityEvents(bus *events.Bus, db *sql.DB, hostname string, pool ZFSAgentPool) {
	capWarning := settings.GetInt(db, "zfs", "capacity_warning_pct", 80)
	capCritical := settings.GetInt(db, "zfs", "capacity_critical_pct", 90)
	fragWarning := settings.GetInt(db, "zfs", "fragmentation_warning_pct", 75)

	if pool.CapacityPct >= capCritical {
		bus.Publish(events.Event{
			Type:     events.ZFSCapacityCritical,
			Severity: events.SeverityCritical,
			Hostname: hostname,
			Message:  fmt.Sprintf("ZFS pool %q is %d%% full", pool.Name, pool.CapacityPct),
			Metadata: map[string]string{
				"pool_name":    pool.Name,
				"capacity_pct": fmt.Sprintf("%d", pool.CapacityPct),
				"threshold":    fmt.Sprintf("%d", capCritical),
			},
		})
	} else if pool.CapacityPct >= capWarning {
		bus.Publish(events.Event{
			Type:     events.ZFSCapacityWarning,
			Severity: events.SeverityWarning,
			Hostname: hostname,
			Message:  fmt.Sprintf("ZFS pool %q is %d%% full", pool.Name, pool.CapacityPct),
			Metadata: map[string]string{
				"pool_name":    pool.Name,
				"capacity_pct": fmt.Sprintf("%d", pool.CapacityPct),
				"threshold":    fmt.Sprintf("%d", capWarning),
			},
		})
	}

	if pool.Fragmentation >= fragWarning {
		bus.Publish(events.Event{
			Type:     events.ZFSFragmentationWarning,
			Severity: events.SeverityWarning,
			Hostname: hostname,
			Message:  fmt.Sprintf("ZFS pool %q fragmentation is %d%%", pool.Name, pool.Fragmentation),
			Metadata: map[string]string{
				"pool_name":        pool.Name,
				"fragmentation_pct": fmt.Sprintf("%d", pool.Fragmentation),
				"threshold":        fmt.Sprintf("%d", fragWarning),
			},
		})
	}
}

// publishVdevErrorEvents fires warnings when any device in the pool has
// error counts exceeding the configured threshold.
func publishVdevErrorEvents(bus *events.Bus, db *sql.DB, hostname string, pool ZFSAgentPool) {
	threshold := int64(settings.GetInt(db, "zfs", "vdev_error_threshold", 1))

	for _, dev := range pool.Devices {
		publishVdevErrorRecursive(bus, hostname, pool.Name, dev, threshold)
	}
}

func publishVdevErrorRecursive(bus *events.Bus, hostname, poolName string, dev ZFSAgentDevice, threshold int64) {
	totalErrors := dev.ReadErrors + dev.WriteErrors + dev.ChecksumErrors

	// Only fire for devices with errors above threshold that aren't already
	// covered by the FAULTED/UNAVAIL/REMOVED state events.
	if totalErrors >= threshold && dev.State != "FAULTED" && dev.State != "UNAVAIL" && dev.State != "REMOVED" {
		bus.Publish(events.Event{
			Type:         events.ZFSVdevErrors,
			Severity:     events.SeverityWarning,
			Hostname:     hostname,
			SerialNumber: dev.SerialNumber,
			Message: fmt.Sprintf("ZFS device %q in pool %q has errors (R:%d W:%d C:%d)",
				dev.Name, poolName, dev.ReadErrors, dev.WriteErrors, dev.ChecksumErrors),
			Metadata: map[string]string{
				"pool_name":       poolName,
				"device_name":     dev.Name,
				"read_errors":     fmt.Sprintf("%d", dev.ReadErrors),
				"write_errors":    fmt.Sprintf("%d", dev.WriteErrors),
				"checksum_errors": fmt.Sprintf("%d", dev.ChecksumErrors),
			},
		})
	}

	for _, child := range dev.Children {
		publishVdevErrorRecursive(bus, hostname, poolName, child, threshold)
	}
}

// publishScrubOverdueEvents fires a warning when the pool hasn't been
// scrubbed within the configured number of days.
func publishScrubOverdueEvents(bus *events.Bus, db *sql.DB, hostname string, pool ZFSAgentPool, poolID int64) {
	overdueThreshold := settings.GetInt(db, "zfs", "scrub_overdue_days", 14)

	lastScrub, err := GetLastScrub(db, poolID)
	if err != nil || lastScrub == nil {
		// No scrub history — don't alert for pools that have never been scrubbed
		return
	}

	daysSince := int(time.Since(lastScrub.EndTime).Hours() / 24)
	if daysSince > overdueThreshold {
		bus.Publish(events.Event{
			Type:     events.ZFSScrubOverdue,
			Severity: events.SeverityWarning,
			Hostname: hostname,
			Message:  fmt.Sprintf("ZFS pool %q last scrub was %d days ago (threshold: %d)", pool.Name, daysSince, overdueThreshold),
			Metadata: map[string]string{
				"pool_name":      pool.Name,
				"days_since":     fmt.Sprintf("%d", daysSince),
				"threshold_days": fmt.Sprintf("%d", overdueThreshold),
			},
		})
	}
}

// publishScanTransitionEvents detects scrub/resilver state transitions and
// publishes events for resilver start and scrub/resilver completion.
func publishScanTransitionEvents(bus *events.Bus, hostname string, pool ZFSAgentPool, prevPool *ZFSPool) {
	if pool.Scan == nil {
		return
	}

	curFunc := pool.Scan.Function
	curState := pool.Scan.State
	prevFunc := ""
	prevState := ""
	if prevPool != nil {
		prevFunc = prevPool.ScanFunction
		prevState = prevPool.ScanState
	}

	// Resilver started: current scan is resilver in progress, previous was not
	if curFunc == "resilver" && (curState == "scanning" || curState == "in_progress") {
		if prevFunc != "resilver" || (prevState != "scanning" && prevState != "in_progress") {
			bus.Publish(events.Event{
				Type:     events.ZFSResilverStarted,
				Severity: events.SeverityWarning,
				Hostname: hostname,
				Message:  fmt.Sprintf("ZFS pool %q resilver started", pool.Name),
				Metadata: map[string]string{
					"pool_name":    pool.Name,
					"progress_pct": fmt.Sprintf("%.1f", pool.Scan.ProgressPct),
				},
			})
		}
		return
	}

	// Scan completed: current state is finished, previous was in progress
	if curState == "finished" && (prevState == "scanning" || prevState == "in_progress") {
		if curFunc == "resilver" || prevFunc == "resilver" {
			bus.Publish(events.Event{
				Type:     events.ZFSResilverCompleted,
				Severity: events.SeverityInfo,
				Hostname: hostname,
				Message:  fmt.Sprintf("ZFS pool %q resilver completed (%d errors)", pool.Name, pool.Scan.ErrorsFound),
				Metadata: map[string]string{
					"pool_name":     pool.Name,
					"errors_found":  fmt.Sprintf("%d", pool.Scan.ErrorsFound),
					"duration_secs": fmt.Sprintf("%d", pool.Scan.Duration),
				},
			})
		} else {
			bus.Publish(events.Event{
				Type:     events.ZFSScrubCompleted,
				Severity: events.SeverityInfo,
				Hostname: hostname,
				Message:  fmt.Sprintf("ZFS pool %q scrub completed (%d errors)", pool.Name, pool.Scan.ErrorsFound),
				Metadata: map[string]string{
					"pool_name":     pool.Name,
					"errors_found":  fmt.Sprintf("%d", pool.Scan.ErrorsFound),
					"duration_secs": fmt.Sprintf("%d", pool.Scan.Duration),
				},
			})
		}
	}
}
