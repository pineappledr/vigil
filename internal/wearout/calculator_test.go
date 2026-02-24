// internal/wearout/calculator_test.go
package wearout

import (
	"math"
	"testing"
)

// ── Shared test helpers ─────────────────────────────────────────────────────

func ssdInput(attrs map[int]AttributeData) CalculationInput {
	return CalculationInput{
		Hostname:     "test-host",
		SerialNumber: "SSD-001",
		DriveType:    "SSD",
		Capacity:     500_000_000_000, // 500 GB
		Attributes:   attrs,
	}
}

func hddInput(attrs map[int]AttributeData) CalculationInput {
	return CalculationInput{
		Hostname:     "test-host",
		SerialNumber: "HDD-001",
		DriveType:    "HDD",
		RotationRate: 7200,
		Attributes:   attrs,
	}
}

func floatPtr(v float64) *float64 { return &v }
func int64Ptr(v int64) *int64     { return &v }

func assertInRange(t *testing.T, name string, got, min, max float64) {
	t.Helper()
	if got < min || got > max {
		t.Errorf("%s = %.4f, want [%.4f, %.4f]", name, got, min, max)
	}
}

func assertApprox(t *testing.T, name string, got, want, tolerance float64) {
	t.Helper()
	if math.Abs(got-want) > tolerance {
		t.Errorf("%s = %.4f, want ~%.4f (tolerance %.4f)", name, got, want, tolerance)
	}
}

// ── SSD Strategy Tests ──────────────────────────────────────────────────────

func TestSSDStrategy_DriveType(t *testing.T) {
	s := &SSDStrategy{}
	if s.DriveType() != "SSD" {
		t.Errorf("DriveType() = %q, want %q", s.DriveType(), "SSD")
	}
}

func TestSSDStrategy_AllAttributes(t *testing.T) {
	s := &SSDStrategy{}
	input := ssdInput(map[int]AttributeData{
		AttrTotalLBAsWritten:   {RawValue: 0},     // 0 bytes written = 0%
		AttrMediaWearout:       {Value: 100},       // 100-100 = 0%
		AttrWearLeveling:       {Value: 100},       // 100-100 = 0%
		AttrReallocatedSectors: {RawValue: 0},      // 0 sectors = 0%
	})

	result := s.Calculate(input)

	// Brand new drive: all factors at 0%
	assertApprox(t, "Percentage", result.Percentage, 0, 0.01)
	if len(result.Factors) != 4 {
		t.Fatalf("expected 4 factors, got %d", len(result.Factors))
	}
	if result.DriveType != "SSD" {
		t.Errorf("DriveType = %q, want %q", result.DriveType, "SSD")
	}
}

func TestSSDStrategy_WornDrive(t *testing.T) {
	s := &SSDStrategy{}
	tbw := 150.0 // 150 TBW rated
	input := ssdInput(map[int]AttributeData{
		AttrTotalLBAsWritten:   {RawValue: 146484375000}, // ~75 TB written (75e12/512)
		AttrMediaWearout:       {Value: 50},               // 50% worn
		AttrWearLeveling:       {Value: 60},               // 40% worn
		AttrReallocatedSectors: {RawValue: 10},            // 10% concern
	})
	input.RatedTBW = &tbw

	result := s.Calculate(input)

	// Expected: 50*0.35 + 50*0.30 + 40*0.20 + 10*0.15 = 17.5 + 15 + 8 + 1.5 = 42%
	assertApprox(t, "WornSSD Percentage", result.Percentage, 42.0, 1.0)
}

func TestSSDStrategy_MissingAttributes_WeightRedistribution(t *testing.T) {
	s := &SSDStrategy{}
	// Only provide media wearout (weight 0.30) — should become 100% of weight
	input := ssdInput(map[int]AttributeData{
		AttrMediaWearout: {Value: 50}, // 50% worn
	})

	result := s.Calculate(input)

	if len(result.Factors) != 1 {
		t.Fatalf("expected 1 factor, got %d", len(result.Factors))
	}
	// Single factor gets normalized weight of 1.0
	assertApprox(t, "Factor weight", result.Factors[0].Weight, 1.0, 0.001)
	// Overall should equal the single factor's percentage
	assertApprox(t, "Percentage", result.Percentage, 50.0, 0.01)
}

func TestSSDStrategy_NoAttributes(t *testing.T) {
	s := &SSDStrategy{}
	input := ssdInput(map[int]AttributeData{})

	result := s.Calculate(input)

	if result.Percentage != 0 {
		t.Errorf("expected 0%% for no attributes, got %.2f%%", result.Percentage)
	}
	if len(result.Factors) != 0 {
		t.Errorf("expected 0 factors, got %d", len(result.Factors))
	}
}

func TestSSDStrategy_ClampAt100(t *testing.T) {
	s := &SSDStrategy{}
	input := ssdInput(map[int]AttributeData{
		AttrReallocatedSectors: {RawValue: 999}, // way over 100 sectors → clamped to 100%
		AttrMediaWearout:       {Value: -50},     // negative value → 100-(-50)=150 → clamped to 100%
		AttrWearLeveling:       {Value: -10},     // clamped to 100%
		AttrTotalLBAsWritten:   {RawValue: math.MaxInt64},
	})

	result := s.Calculate(input)

	assertInRange(t, "Clamped percentage", result.Percentage, 0, 100)
	for _, f := range result.Factors {
		assertInRange(t, f.Name, f.Percentage, 0, 100)
	}
}

func TestSSDStrategy_DefaultTBW(t *testing.T) {
	// With 0 capacity, should use fallback of 500GB → 150 TBW
	got := defaultTBWBytes(0)
	want := 500.0 * 0.3 * 1e12
	assertApprox(t, "defaultTBW(0)", got, want, 1)

	// With 1TB capacity → 300 TBW
	got = defaultTBWBytes(1_000_000_000_000)
	want = 1000.0 * 0.3 * 1e12
	assertApprox(t, "defaultTBW(1TB)", got, want, 1)
}

func TestSSDStrategy_CustomTBW(t *testing.T) {
	s := &SSDStrategy{}
	tbw := 600.0 // 600 TBW enterprise drive
	// Write exactly 300 TBW = 50%
	lbas := int64(300e12 / 512)
	input := ssdInput(map[int]AttributeData{
		AttrTotalLBAsWritten: {RawValue: lbas},
	})
	input.RatedTBW = &tbw

	result := s.Calculate(input)

	assertApprox(t, "CustomTBW 50%", result.Percentage, 50.0, 0.5)
}

// ── HDD Strategy Tests ──────────────────────────────────────────────────────

func TestHDDStrategy_DriveType(t *testing.T) {
	h := &HDDStrategy{}
	if h.DriveType() != "HDD" {
		t.Errorf("DriveType() = %q, want %q", h.DriveType(), "HDD")
	}
}

func TestHDDStrategy_AllAttributes(t *testing.T) {
	h := &HDDStrategy{}
	input := hddInput(map[int]AttributeData{
		AttrPowerOnHours:       {RawValue: 0},
		AttrReallocatedSectors: {RawValue: 0},
		AttrLoadCycleCount:     {RawValue: 0},
		AttrPendingSectors:     {RawValue: 0},
		AttrUncorrectable:      {RawValue: 0},
	})

	result := h.Calculate(input)

	assertApprox(t, "New HDD percentage", result.Percentage, 0, 0.01)
	if len(result.Factors) != 4 {
		t.Fatalf("expected 4 factors, got %d", len(result.Factors))
	}
}

func TestHDDStrategy_AgingDrive(t *testing.T) {
	h := &HDDStrategy{}
	input := hddInput(map[int]AttributeData{
		AttrPowerOnHours:       {RawValue: 500_000}, // 50% of default 1M MTBF
		AttrReallocatedSectors: {RawValue: 25},      // 25/50 = 50%
		AttrLoadCycleCount:     {RawValue: 300_000},  // 300k/600k = 50%
		AttrPendingSectors:     {RawValue: 5},        // combined 5+5=10, 10/20 = 50%
		AttrUncorrectable:      {RawValue: 5},
	})

	result := h.Calculate(input)

	// All factors at ~50% → overall ~50%
	assertApprox(t, "Aging HDD percentage", result.Percentage, 50.0, 1.0)
}

func TestHDDStrategy_CustomMTBF(t *testing.T) {
	h := &HDDStrategy{}
	mtbf := int64(500_000)
	input := hddInput(map[int]AttributeData{
		AttrPowerOnHours: {RawValue: 250_000}, // 250k/500k = 50%
	})
	input.RatedMTBFHours = &mtbf

	result := h.Calculate(input)

	assertApprox(t, "Custom MTBF", result.Percentage, 50.0, 0.5)
}

func TestHDDStrategy_CustomLoadCycles(t *testing.T) {
	h := &HDDStrategy{}
	loadCycles := int64(300_000)
	input := hddInput(map[int]AttributeData{
		AttrLoadCycleCount: {RawValue: 150_000}, // 150k/300k = 50%
	})
	input.RatedLoadCycles = &loadCycles

	result := h.Calculate(input)

	assertApprox(t, "Custom load cycles", result.Percentage, 50.0, 0.5)
}

func TestHDDStrategy_PendingUncorrectableCombined(t *testing.T) {
	h := &HDDStrategy{}
	input := hddInput(map[int]AttributeData{
		AttrPendingSectors: {RawValue: 10},
		AttrUncorrectable:  {RawValue: 10},
	})

	result := h.Calculate(input)

	// Combined = 20, threshold = 20 → 100% for that factor
	// Single factor present → normalized to 100% weight → overall 100%
	assertApprox(t, "Combined pending+uncorrectable", result.Percentage, 100.0, 0.5)
}

func TestHDDStrategy_MissingAttributes(t *testing.T) {
	h := &HDDStrategy{}
	// Only power-on hours
	input := hddInput(map[int]AttributeData{
		AttrPowerOnHours: {RawValue: 100_000}, // 10% of 1M default
	})

	result := h.Calculate(input)

	if len(result.Factors) != 1 {
		t.Fatalf("expected 1 factor, got %d", len(result.Factors))
	}
	assertApprox(t, "Single factor weight", result.Factors[0].Weight, 1.0, 0.001)
	assertApprox(t, "POH 10%", result.Percentage, 10.0, 0.5)
}

func TestHDDStrategy_ClampExtremeValues(t *testing.T) {
	h := &HDDStrategy{}
	input := hddInput(map[int]AttributeData{
		AttrPowerOnHours:       {RawValue: 10_000_000},  // 1000% of MTBF → clamped
		AttrReallocatedSectors: {RawValue: 500},          // 1000% → clamped to 100
		AttrLoadCycleCount:     {RawValue: 6_000_000},    // 1000% → clamped
		AttrPendingSectors:     {RawValue: 200},           // 1000% → clamped
	})

	result := h.Calculate(input)

	// All factors clamped to 100% → overall 100%
	assertApprox(t, "All clamped", result.Percentage, 100.0, 0.01)
}

// ── Strategy Selection Tests ────────────────────────────────────────────────

func TestStrategyRegistry(t *testing.T) {
	tests := []struct {
		driveType string
		wantType  string
		wantOk    bool
	}{
		{"SSD", "SSD", true},
		{"HDD", "HDD", true},
		{"NVMe", "", false},
		{"unknown", "", false},
	}

	for _, tt := range tests {
		s, ok := strategies[tt.driveType]
		if ok != tt.wantOk {
			t.Errorf("strategies[%q] ok = %v, want %v", tt.driveType, ok, tt.wantOk)
			continue
		}
		if ok && s.DriveType() != tt.wantType {
			t.Errorf("strategies[%q].DriveType() = %q, want %q", tt.driveType, s.DriveType(), tt.wantType)
		}
	}
}

// ── calculateWeighted Tests ─────────────────────────────────────────────────

func TestCalculateWeighted_EqualWeights(t *testing.T) {
	input := CalculationInput{
		Hostname:     "h",
		SerialNumber: "s",
		DriveType:    "TEST",
		Attributes: map[int]AttributeData{
			1: {RawValue: 50},
			2: {RawValue: 100},
		},
	}

	defs := []FactorDef{
		{AttrID: 1, Name: "A", Weight: 0.5, Calc: func(a AttributeData) float64 { return float64(a.RawValue) }},
		{AttrID: 2, Name: "B", Weight: 0.5, Calc: func(a AttributeData) float64 { return float64(a.RawValue) }},
	}

	result := calculateWeighted(input, defs)

	// (50*0.5 + 100*0.5) = 75
	assertApprox(t, "Equal weights", result.Percentage, 75.0, 0.01)
}

func TestCalculateWeighted_SkipsMissingAttributes(t *testing.T) {
	input := CalculationInput{
		Hostname:     "h",
		SerialNumber: "s",
		DriveType:    "TEST",
		Attributes: map[int]AttributeData{
			1: {RawValue: 80},
			// AttrID 2 is missing
		},
	}

	defs := []FactorDef{
		{AttrID: 1, Name: "A", Weight: 0.3, Calc: func(a AttributeData) float64 { return float64(a.RawValue) }},
		{AttrID: 2, Name: "B", Weight: 0.7, Calc: func(a AttributeData) float64 { return float64(a.RawValue) }},
	}

	result := calculateWeighted(input, defs)

	// Only factor A present, normalized weight = 1.0, so overall = 80%
	assertApprox(t, "Missing attr redistribution", result.Percentage, 80.0, 0.01)
	if len(result.Factors) != 1 {
		t.Fatalf("expected 1 factor, got %d", len(result.Factors))
	}
}

func TestCalculateWeighted_EmptyAttributes(t *testing.T) {
	input := CalculationInput{
		Attributes: map[int]AttributeData{},
	}
	defs := []FactorDef{
		{AttrID: 1, Name: "A", Weight: 1.0, Calc: func(a AttributeData) float64 { return 50 }},
	}

	result := calculateWeighted(input, defs)

	if result.Percentage != 0 {
		t.Errorf("expected 0%% for empty attrs, got %.2f%%", result.Percentage)
	}
}

// ── Helper Tests ────────────────────────────────────────────────────────────

func TestClamp(t *testing.T) {
	tests := []struct {
		val, min, max, want float64
	}{
		{50, 0, 100, 50},
		{-10, 0, 100, 0},
		{150, 0, 100, 100},
		{0, 0, 100, 0},
		{100, 0, 100, 100},
	}

	for _, tt := range tests {
		got := clamp(tt.val, tt.min, tt.max)
		if got != tt.want {
			t.Errorf("clamp(%.0f, %.0f, %.0f) = %.0f, want %.0f", tt.val, tt.min, tt.max, got, tt.want)
		}
	}
}

func TestSafeDiv(t *testing.T) {
	tests := []struct {
		a, b, want float64
	}{
		{10, 2, 5},
		{10, 0, 0},
		{0, 5, 0},
		{100, 3, 33.3333},
	}

	for _, tt := range tests {
		got := safeDiv(tt.a, tt.b)
		if math.Abs(got-tt.want) > 0.001 {
			t.Errorf("safeDiv(%.0f, %.0f) = %.4f, want %.4f", tt.a, tt.b, got, tt.want)
		}
	}
}
