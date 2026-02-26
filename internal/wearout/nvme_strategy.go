package wearout

// NVMe attribute pseudo-IDs (match cmd/agent/smart/smart_parser.go constants).
const (
	AttrNVMePercentageUsed   = 233
	AttrNVMeAvailableSpare   = 232
	AttrNVMeDataUnitsWritten = 241
	AttrNVMeMediaErrors      = 187
)

// NVMeStrategy calculates wearout for NVMe drives.
type NVMeStrategy struct{}

func (n *NVMeStrategy) DriveType() string { return "NVMe" }

func (n *NVMeStrategy) Calculate(input CalculationInput) WearoutResult {
	tbwBytes := defaultTBWBytes(input.Capacity)
	if input.RatedTBW != nil {
		tbwBytes = *input.RatedTBW * 1e12
	}

	factors := []FactorDef{
		{
			AttrID: AttrNVMePercentageUsed, Name: "Percentage Used", Weight: 0.40,
			Desc: "NVMe endurance indicator reported by controller (0-100%+)",
			Calc: func(a AttributeData) float64 {
				return clamp(float64(a.RawValue), 0, 100)
			},
		},
		{
			AttrID: AttrNVMeAvailableSpare, Name: "Available Spare", Weight: 0.25,
			Desc: "Remaining spare capacity for bad block replacement (100% = full)",
			Calc: func(a AttributeData) float64 {
				// Value is percentage remaining (100=best), invert to wearout
				return clamp(float64(100-a.Value), 0, 100)
			},
		},
		{
			AttrID: AttrNVMeDataUnitsWritten, Name: "Data Written vs TBW", Weight: 0.20,
			Desc: "Write endurance consumed based on data units written vs TBW rating",
			Calc: func(a AttributeData) float64 {
				// NVMe data units = 512-byte units * 1000
				bytesWritten := float64(a.RawValue) * 512 * 1000
				return safeDiv(bytesWritten, tbwBytes) * 100
			},
		},
		{
			AttrID: AttrNVMeMediaErrors, Name: "Media Errors", Weight: 0.15,
			Desc: "Unrecoverable media errors reported by controller (10 = full concern)",
			Calc: func(a AttributeData) float64 {
				return clamp(safeDiv(float64(a.RawValue), 10)*100, 0, 100)
			},
		},
	}

	return calculateWeighted(input, factors)
}
