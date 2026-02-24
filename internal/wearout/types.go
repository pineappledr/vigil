package wearout

import "time"

// WearoutStrategy calculates wearout for a specific drive type.
// Open/Closed: add new strategies (NVMe, vendor-specific) without modifying the calculator.
type WearoutStrategy interface {
	Calculate(input CalculationInput) WearoutResult
	DriveType() string
}

// CalculationInput is the data needed to compute wearout.
type CalculationInput struct {
	Hostname     string
	SerialNumber string
	ModelName    string
	DriveType    string // "SSD", "HDD"
	Capacity     int64  // bytes
	RotationRate int

	// SMART attribute values keyed by attribute ID
	Attributes map[int]AttributeData

	// Optional manufacturer specs (nil = use defaults)
	RatedTBW        *float64
	RatedMTBFHours  *int64
	RatedLoadCycles *int64
}

// AttributeData holds the relevant fields from a SMART attribute.
type AttributeData struct {
	Value    int
	RawValue int64
}

// WearoutResult is the output of a wearout calculation.
type WearoutResult struct {
	Hostname     string               `json:"hostname"`
	SerialNumber string               `json:"serial_number"`
	DriveType    string               `json:"drive_type"`
	Percentage   float64              `json:"percentage"`
	Factors      []ContributingFactor `json:"factors"`
	Prediction   *TrendPrediction     `json:"prediction,omitempty"`
}

// ContributingFactor describes one component of the wearout score.
type ContributingFactor struct {
	Name        string  `json:"name"`
	Percentage  float64 `json:"percentage"`
	Weight      float64 `json:"weight"`
	Description string  `json:"description"`
}

// WearoutSnapshot is a point-in-time wearout record stored in the database.
type WearoutSnapshot struct {
	ID           int       `json:"id,omitempty"`
	Hostname     string    `json:"hostname"`
	SerialNumber string    `json:"serial_number"`
	DriveType    string    `json:"drive_type"`
	Percentage   float64   `json:"percentage"`
	FactorsJSON  string    `json:"factors_json,omitempty"`
	Timestamp    time.Time `json:"timestamp"`
}

// TrendPrediction estimates future wearout based on historical data.
type TrendPrediction struct {
	MonthsRemaining *float64 `json:"months_remaining,omitempty"`
	DailyRate       float64  `json:"daily_rate"`
	Confidence      string   `json:"confidence"` // "low", "medium", "high"
}

// DriveSpec holds manufacturer ratings for a drive model.
type DriveSpec struct {
	ID              int     `json:"id,omitempty"`
	ModelPattern    string  `json:"model_pattern"`
	RatedTBW        *float64 `json:"rated_tbw,omitempty"`
	RatedMTBFHours  *int64   `json:"rated_mtbf_hours,omitempty"`
	RatedLoadCycles *int64   `json:"rated_load_cycles,omitempty"`
}

// SMART attribute IDs referenced by strategies.
const (
	AttrReallocatedSectors = 5
	AttrPowerOnHours       = 9
	AttrLoadCycleCount     = 193
	AttrPendingSectors     = 197
	AttrUncorrectable      = 198
	AttrWearLeveling       = 177
	AttrMediaWearout       = 233
	AttrTotalLBAsWritten   = 241
)
