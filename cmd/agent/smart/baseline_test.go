package smart

import "testing"

// Brain's two Toshibas: UDMA_CRC frozen at 1633/2335 since the cables were
// replaced (2026-09-24). Judged on the total they are CRITICAL forever.
func crcDrive(raw int64) *DriveSmartData {
	return &DriveSmartData{
		Hostname: "brain", SerialNumber: "41L0A06SFBEG", SmartPassed: true,
		Attributes: []SmartAttribute{{ID: 199, Name: "UDMA_CRC_Error_Count", RawValue: raw}},
	}
}

func TestBaseline_WithoutOneTheTotalIsCritical(t *testing.T) {
	if got := AnalyzeDriveHealthWithBaseline(crcDrive(2335), nil).OverallHealth; got != SeverityCritical {
		t.Fatalf("got %s, want critical", got)
	}
}

func TestBaseline_AcknowledgedAndFrozenIsHealthy(t *testing.T) {
	a := AnalyzeDriveHealthWithBaseline(crcDrive(2335), Baseline{199: 2335})
	if a.OverallHealth != SeverityHealthy || len(a.Issues) != 0 {
		t.Fatalf("got %s with %d issues, want healthy", a.OverallHealth, len(a.Issues))
	}
}

// The whole point: a counter that GROWS past its baseline is news again.
func TestBaseline_OneMoreErrorIsCriticalAgain(t *testing.T) {
	if got := AnalyzeDriveHealthWithBaseline(crcDrive(2336), Baseline{199: 2335}).OverallHealth; got != SeverityCritical {
		t.Fatalf("got %s, want critical", got)
	}
}

// Temperature is current state, not history: a baseline must not hide it.
func TestBaseline_TemperatureCannotBeAcknowledged(t *testing.T) {
	d := &DriveSmartData{SmartPassed: true, Attributes: []SmartAttribute{{ID: 194, RawValue: 70}}}
	if got := AnalyzeDriveHealthWithBaseline(d, Baseline{194: 70}).OverallHealth; got != SeverityCritical {
		t.Fatalf("got %s, want critical", got)
	}
}

func TestBaseline_FailedSmartVerdictIsNeverHidden(t *testing.T) {
	d := crcDrive(2335)
	d.SmartPassed = false
	if got := AnalyzeDriveHealthWithBaseline(d, Baseline{199: 2335}).OverallHealth; got != SeverityCritical {
		t.Fatalf("got %s, want critical", got)
	}
}

// Acknowledging one counter says nothing about the others.
func TestBaseline_OnlyTheAcknowledgedCounterIsSilenced(t *testing.T) {
	d := crcDrive(2335)
	d.Attributes = append(d.Attributes, SmartAttribute{ID: 5, Name: "Reallocated_Sector_Ct", RawValue: 8})
	a := AnalyzeDriveHealthWithBaseline(d, Baseline{199: 2335})
	if len(a.Issues) != 1 || a.Issues[0].AttributeID != 5 {
		t.Fatalf("want exactly the reallocated-sectors issue, got %+v", a.Issues)
	}
}
