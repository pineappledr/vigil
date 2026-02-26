package wearout

// SSDStrategy calculates wearout for solid-state drives.
type SSDStrategy struct{}

func (s *SSDStrategy) DriveType() string { return "SSD" }

func (s *SSDStrategy) Calculate(input CalculationInput) WearoutResult {
	// TBW rating: use provided spec or estimate from capacity
	tbwBytes := defaultTBWBytes(input.Capacity)
	if input.RatedTBW != nil {
		tbwBytes = *input.RatedTBW * 1e12
	}

	factors := []FactorDef{
		{
			AttrID: AttrTotalLBAsWritten, Name: "LBAs Written vs TBW", Weight: 0.35,
			Desc: "Write endurance consumed based on total bytes written vs TBW rating",
			Calc: func(a AttributeData) float64 {
				bytesWritten := float64(a.RawValue) * 512
				return safeDiv(bytesWritten, tbwBytes) * 100
			},
		},
		{
			AttrID: AttrMediaWearout, Name: "Media Wearout Indicator", Weight: 0.30,
			Desc: "Manufacturer media wearout indicator (starts at 100, decreases)",
			Calc: func(a AttributeData) float64 {
				return clamp(float64(100-a.Value), 0, 100)
			},
		},
		{
			AttrID: AttrWearLeveling, Name: "Wear Leveling Count", Weight: 0.20,
			Desc: "NAND wear leveling status (starts at 100, decreases)",
			Calc: func(a AttributeData) float64 {
				return clamp(float64(100-a.Value), 0, 100)
			},
		},
		{
			AttrID: AttrReallocatedSectors, Name: "Reallocated Sectors", Weight: 0.15,
			Desc: "Bad sectors remapped to spare area",
			Calc: func(a AttributeData) float64 {
				return clamp(float64(a.RawValue), 0, 100)
			},
		},
	}

	return calculateWeighted(input, factors)
}

// defaultTBWBytes estimates TBW from capacity for consumer SSDs.
// Conservative estimate: 0.3 TBW per GB of capacity.
func defaultTBWBytes(capacityBytes int64) float64 {
	capacityGB := float64(capacityBytes) / 1e9
	if capacityGB <= 0 {
		capacityGB = 500 // fallback: assume 500GB
	}
	return capacityGB * 0.3 * 1e12
}
