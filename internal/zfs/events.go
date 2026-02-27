package zfs

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"vigil/internal/events"
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
		if err := processPool(db, hostname, pool); err != nil {
			log.Printf("⚠️  Failed to process pool %s: %v", pool.Name, err)
			continue
		}

		if bus != nil {
			publishPoolEvents(bus, hostname, pool)
			publishDeviceEvents(bus, hostname, pool)
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
