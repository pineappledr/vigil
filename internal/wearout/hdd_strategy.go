package wearout

// HDDStrategy calculates wearout for hard disk drives.
type HDDStrategy struct{}

func (h *HDDStrategy) DriveType() string { return "HDD" }

func (h *HDDStrategy) Calculate(input CalculationInput) WearoutResult {
	mtbfHours := int64(1_000_000) // default MTBF
	if input.RatedMTBFHours != nil {
		mtbfHours = *input.RatedMTBFHours
	}

	loadCycleRating := int64(600_000) // default rated load cycles
	if input.RatedLoadCycles != nil {
		loadCycleRating = *input.RatedLoadCycles
	}

	// Combine pending + uncorrectable sectors before building factors
	combinedPending := int64(0)
	if p, ok := input.Attributes[AttrPendingSectors]; ok {
		combinedPending += p.RawValue
	}
	if u, ok := input.Attributes[AttrUncorrectable]; ok {
		combinedPending += u.RawValue
	}
	if combinedPending > 0 {
		input.Attributes[AttrPendingSectors] = AttributeData{RawValue: combinedPending}
	}

	factors := []FactorDef{
		{
			AttrID: AttrPowerOnHours, Name: "Power-On Hours vs MTBF", Weight: 0.30,
			Desc: "Operational hours as percentage of manufacturer MTBF rating",
			Calc: func(a AttributeData) float64 {
				return safeDiv(float64(a.RawValue), float64(mtbfHours)) * 100
			},
		},
		{
			AttrID: AttrReallocatedSectors, Name: "Reallocated Sectors", Weight: 0.30,
			Desc: "Bad sectors remapped to spare area (50 sectors = full concern)",
			Calc: func(a AttributeData) float64 {
				return clamp(safeDiv(float64(a.RawValue), 50)*100, 0, 100)
			},
		},
		{
			AttrID: AttrLoadCycleCount, Name: "Load Cycle Exhaustion", Weight: 0.20,
			Desc: "Head load/unload cycles as percentage of rated lifetime cycles",
			Calc: func(a AttributeData) float64 {
				return safeDiv(float64(a.RawValue), float64(loadCycleRating)) * 100
			},
		},
		{
			AttrID: AttrPendingSectors, Name: "Pending + Uncorrectable Sectors", Weight: 0.20,
			Desc: "Unstable and uncorrectable sectors (20 combined = full concern)",
			Calc: func(a AttributeData) float64 {
				return clamp(safeDiv(float64(a.RawValue), 20)*100, 0, 100)
			},
		},
	}

	return calculateWeighted(input, factors)
}
