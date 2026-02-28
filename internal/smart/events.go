package smart

import (
	"database/sql"
	"fmt"
	"log"

	agentsmart "vigil/cmd/agent/smart"
	"vigil/internal/events"
)

// ProcessReportWithEvents extracts SMART data from an incoming report, stores
// it, and publishes events for any drives with health warnings or failures.
func ProcessReportWithEvents(db *sql.DB, bus *events.Bus, hostname string, reportData map[string]interface{}) error {
	drives, ok := reportData["drives"].([]interface{})
	if !ok {
		return nil
	}

	var lastErr error
	for _, driveInterface := range drives {
		driveMap, ok := driveInterface.(map[string]interface{})
		if !ok {
			continue
		}

		driveData, err := agentsmart.ParseSmartAttributes(driveMap, hostname)
		if err != nil {
			log.Printf("Warning: Failed to parse SMART data for drive: %v", err)
			lastErr = err
			continue
		}

		if driveData.SerialNumber == "" {
			continue
		}

		// Store attributes
		if len(driveData.Attributes) > 0 {
			if err := StoreSmartAttributes(db, driveData); err != nil {
				log.Printf("Warning: Failed to store SMART attributes for %s: %v", driveData.SerialNumber, err)
				lastErr = err
			}
		}

		// Publish health events
		if bus != nil {
			publishSmartHealthEvents(bus, driveData)
		}
	}

	return lastErr
}

// publishSmartHealthEvents analyzes a drive's SMART data and publishes events
// for any warnings or critical issues detected.
func publishSmartHealthEvents(bus *events.Bus, driveData *agentsmart.DriveSmartData) {
	analysis := agentsmart.AnalyzeDriveHealth(driveData)
	if analysis.OverallHealth == agentsmart.SeverityHealthy {
		return
	}

	// Publish per-issue events for critical reallocated sectors
	for _, issue := range analysis.Issues {
		if issue.AttributeID == 5 && issue.RawValue > 0 {
			bus.Publish(events.Event{
				Type:         events.ReallocatedSectors,
				Severity:     mapSeverity(issue.Severity),
				Hostname:     driveData.Hostname,
				SerialNumber: driveData.SerialNumber,
				Message:      issue.Message,
				Metadata: map[string]string{
					"attribute_id": fmt.Sprintf("%d", issue.AttributeID),
					"raw_value":    fmt.Sprintf("%d", issue.RawValue),
					"model":        driveData.ModelName,
				},
			})
		}
	}

	// Publish overall SMART health event
	if analysis.CriticalCount > 0 {
		bus.Publish(events.Event{
			Type:         events.SmartCritical,
			Severity:     events.SeverityCritical,
			Hostname:     driveData.Hostname,
			SerialNumber: driveData.SerialNumber,
			Message: fmt.Sprintf("SMART critical: %d issue(s) on %s (%s)",
				analysis.CriticalCount, driveData.SerialNumber, driveData.ModelName),
			Metadata: map[string]string{
				"model":          driveData.ModelName,
				"drive_type":     driveData.DriveType,
				"smart_passed":   fmt.Sprintf("%t", driveData.SmartPassed),
				"critical_count": fmt.Sprintf("%d", analysis.CriticalCount),
				"warning_count":  fmt.Sprintf("%d", analysis.WarningCount),
			},
		})
	} else if analysis.WarningCount > 0 {
		bus.Publish(events.Event{
			Type:         events.SmartWarning,
			Severity:     events.SeverityWarning,
			Hostname:     driveData.Hostname,
			SerialNumber: driveData.SerialNumber,
			Message: fmt.Sprintf("SMART warning: %d issue(s) on %s (%s)",
				analysis.WarningCount, driveData.SerialNumber, driveData.ModelName),
			Metadata: map[string]string{
				"model":         driveData.ModelName,
				"drive_type":    driveData.DriveType,
				"smart_passed":  fmt.Sprintf("%t", driveData.SmartPassed),
				"warning_count": fmt.Sprintf("%d", analysis.WarningCount),
			},
		})
	}
}

func mapSeverity(s string) events.Severity {
	switch s {
	case agentsmart.SeverityCritical:
		return events.SeverityCritical
	case agentsmart.SeverityWarning:
		return events.SeverityWarning
	default:
		return events.SeverityInfo
	}
}
