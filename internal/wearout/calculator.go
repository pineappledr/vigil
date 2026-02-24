package wearout

import (
	"database/sql"
	"log"
	"time"

	agentsmart "vigil/cmd/agent/smart"
)

// strategies is the registry of available wearout strategies.
var strategies = map[string]WearoutStrategy{
	"SSD": &SSDStrategy{},
	"HDD": &HDDStrategy{},
}

// CalculateAndStore runs the wearout calculation for a drive and persists the result.
func CalculateAndStore(db *sql.DB, driveData *agentsmart.DriveSmartData) (*WearoutResult, error) {
	strategy, ok := strategies[driveData.DriveType]
	if !ok {
		return nil, nil // unsupported drive type (e.g. NVMe) — skip silently
	}

	input := buildInput(db, driveData)
	result := strategy.Calculate(input)

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

	return &result, nil
}

// ProcessWearoutFromReport calculates and stores wearout for all drives in a report.
func ProcessWearoutFromReport(db *sql.DB, hostname string, reportData map[string]interface{}) {
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

		if _, err := CalculateAndStore(db, driveData); err != nil {
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
