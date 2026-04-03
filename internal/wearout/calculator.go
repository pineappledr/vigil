package wearout

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	agentsmart "vigil/cmd/agent/smart"
	"vigil/internal/events"
)

// strategies is the registry of available wearout strategies.
var strategies = map[string]WearoutStrategy{
	"SSD":  &SSDStrategy{},
	"HDD":  &HDDStrategy{},
	"NVMe": &NVMeStrategy{},
}

// wearout threshold levels that trigger events.
const (
	thresholdWarning  = 60.0
	thresholdCritical = 80.0
)

// CalculateAndStore runs the wearout calculation for a drive and persists the result.
// If bus is non-nil, threshold-crossing events are published.
func CalculateAndStore(db *sql.DB, bus *events.Bus, driveData *agentsmart.DriveSmartData) (*WearoutResult, error) {
	strategy, ok := strategies[driveData.DriveType]
	if !ok {
		return nil, nil // unsupported drive type — skip silently
	}

	input := buildInput(db, driveData)
	result := strategy.Calculate(input)

	// Capture previous snapshot before storing so we can detect threshold crossings.
	prev, _ := GetLatestSnapshot(db, driveData.Hostname, driveData.SerialNumber)

	// Attach trend prediction from historical data
	history, err := GetSnapshotHistory(db, driveData.Hostname, driveData.SerialNumber, 365)
	if err == nil && len(history) >= 3 {
		result.Prediction = PredictTrend(history)
	}

	// Persist snapshot
	snapshot := WearoutSnapshot{
		Hostname:     driveData.Hostname,
		SerialNumber: driveData.SerialNumber,
		DriveType:    driveData.DriveType,
		Percentage:   result.Percentage,
		FactorsJSON:  marshalFactors(result.Factors),
		Timestamp:    time.Now().UTC(),
	}

	if err := StoreSnapshot(db, snapshot); err != nil {
		log.Printf("Warning: failed to store wearout snapshot for %s: %v", driveData.SerialNumber, err)
	}

	// Publish events for threshold crossings.
	if bus != nil {
		publishWearoutEvents(bus, driveData, prev, &result)
	}

	return &result, nil
}

// publishWearoutEvents fires wearout events when a drive crosses a threshold
// for the first time, and when a trend predicts failure within 3 months.
func publishWearoutEvents(bus *events.Bus, driveData *agentsmart.DriveSmartData, prev *WearoutSnapshot, result *WearoutResult) {
	newPct := result.Percentage
	var prevPct float64
	if prev != nil {
		prevPct = prev.Percentage
	}

	label := driveData.ModelName
	if label == "" {
		label = driveData.SerialNumber
	}

	// Critical threshold crossing (80%).
	if newPct >= thresholdCritical && prevPct < thresholdCritical {
		bus.Publish(events.Event{
			Type:         events.WearoutCritical,
			Severity:     events.SeverityCritical,
			Hostname:     driveData.Hostname,
			SerialNumber: driveData.SerialNumber,
			Message:      fmt.Sprintf("Drive wearout critical: %s (%s) reached %.0f%%", label, driveData.SerialNumber, newPct),
		})
		return // critical supersedes warning
	}

	// Warning threshold crossing (60%).
	if newPct >= thresholdWarning && prevPct < thresholdWarning {
		bus.Publish(events.Event{
			Type:         events.WearoutWarning,
			Severity:     events.SeverityWarning,
			Hostname:     driveData.Hostname,
			SerialNumber: driveData.SerialNumber,
			Message:      fmt.Sprintf("Drive wearout warning: %s (%s) reached %.0f%%", label, driveData.SerialNumber, newPct),
		})
	}

	// Predicted failure within 3 months (7-day dispatcher cooldown suppresses repeats).
	if result.Prediction != nil && result.Prediction.MonthsRemaining != nil &&
		result.Prediction.Confidence != "low" {
		months := *result.Prediction.MonthsRemaining
		if months < 3.0 {
			bus.Publish(events.Event{
				Type:         events.WearoutPredicted,
				Severity:     events.SeverityWarning,
				Hostname:     driveData.Hostname,
				SerialNumber: driveData.SerialNumber,
				Message:      fmt.Sprintf("Drive failure predicted: %s (%s) has ~%.0f month(s) remaining", label, driveData.SerialNumber, months),
			})
		}
	}
}

// ProcessWearoutFromReport calculates and stores wearout for all drives in a report.
// Pass a non-nil bus to enable threshold-crossing notifications.
func ProcessWearoutFromReport(db *sql.DB, bus *events.Bus, hostname string, reportData map[string]interface{}) {
	drives, ok := reportData["drives"].([]interface{})
	if !ok {
		return
	}

	for _, driveInterface := range drives {
		driveMap, ok := driveInterface.(map[string]interface{})
		if !ok {
			continue
		}

		driveData, err := agentsmart.ParseSmartAttributes(driveMap, hostname)
		if err != nil || driveData.SerialNumber == "" {
			continue
		}

		if _, err := CalculateAndStore(db, bus, driveData); err != nil {
			log.Printf("Warning: wearout calculation failed for %s: %v", driveData.SerialNumber, err)
		}
	}
}

// buildInput converts DriveSmartData into the strategy's CalculationInput.
func buildInput(db *sql.DB, d *agentsmart.DriveSmartData) CalculationInput {
	attrs := make(map[int]AttributeData, len(d.Attributes))
	for _, a := range d.Attributes {
		attrs[a.ID] = AttributeData{Value: a.Value, RawValue: a.RawValue}
	}

	input := CalculationInput{
		Hostname:     d.Hostname,
		SerialNumber: d.SerialNumber,
		ModelName:    d.ModelName,
		DriveType:    d.DriveType,
		Capacity:     d.Capacity,
		RotationRate: d.RotationRate,
		Attributes:   attrs,
	}

	// Look up manufacturer specs
	spec, err := GetDriveSpec(db, d.ModelName)
	if err == nil && spec != nil {
		input.RatedTBW = spec.RatedTBW
		input.RatedMTBFHours = spec.RatedMTBFHours
		input.RatedLoadCycles = spec.RatedLoadCycles
	}

	return input
}

// FactorDef defines a wearout factor used by strategies.
// DRY: both SSD and HDD build []FactorDef and delegate to calculateWeighted.
type FactorDef struct {
	AttrID int
	Name   string
	Desc   string
	Weight float64
	Calc   func(AttributeData) float64
}

// calculateWeighted is the shared weighted-average engine.
// Missing attributes have their weight redistributed proportionally.
func calculateWeighted(input CalculationInput, defs []FactorDef) WearoutResult {
	var present []struct {
		def FactorDef
		pct float64
	}
	totalWeight := 0.0

	if input.Attributes == nil {
		return WearoutResult{
			Hostname:     input.Hostname,
			SerialNumber: input.SerialNumber,
			DriveType:    input.DriveType,
		}
	}

	for _, d := range defs {
		attr, ok := input.Attributes[d.AttrID]
		if !ok {
			continue
		}
		pct := clamp(d.Calc(attr), 0, 100)
		present = append(present, struct {
			def FactorDef
			pct float64
		}{d, pct})
		totalWeight += d.Weight
	}

	result := WearoutResult{
		Hostname:     input.Hostname,
		SerialNumber: input.SerialNumber,
		DriveType:    input.DriveType,
	}

	if len(present) == 0 {
		return result
	}

	var overall float64
	for _, p := range present {
		// Redistribute weight proportionally among present factors
		normalizedWeight := p.def.Weight / totalWeight
		overall += p.pct * normalizedWeight

		result.Factors = append(result.Factors, ContributingFactor{
			Name:        p.def.Name,
			Percentage:  p.pct,
			Weight:      normalizedWeight,
			Description: p.def.Desc,
		})
	}

	result.Percentage = clamp(overall, 0, 100)
	return result
}
