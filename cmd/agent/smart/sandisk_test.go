package smart

import "testing"

// Veronica's SanDisk SD9SB8W-256G-1006 (2026-09-30): healthy (SMART passed,
// 0 reallocated, 93 % life left), but its 181 raw is a vendor-packed total that
// grows with use; each report re-fired a critical/warning alert.
func sandisk(raw181 int64, withReal bool) *DriveSmartData {
	attrs := []SmartAttribute{
		{ID: 5, Name: "Reallocated_Sector_Ct", Value: 100, Threshold: 5, RawValue: 0},
		{ID: 181, Name: "Program_Fail_Cnt_Total", Value: 100, RawValue: raw181},
	}
	if withReal {
		attrs = append(attrs,
			SmartAttribute{ID: 171, Name: "Program_Fail_Count", Value: 100, RawValue: 0},
			SmartAttribute{ID: 172, Name: "Erase_Fail_Count", Value: 100, RawValue: 0})
	}
	return &DriveSmartData{Hostname: "veronica", SerialNumber: "180584807115", SmartPassed: true, Attributes: attrs}
}

func TestSanDisk_PackedTotalIgnoredWhenRealCounterPresent(t *testing.T) {
	for _, raw := range []int64{204581822, 204637411} {
		a := AnalyzeDriveHealthWithBaseline(sandisk(raw, true), nil)
		if a.OverallHealth != SeverityHealthy {
			t.Fatalf("raw181=%d: got %s with %v, want healthy", raw, a.OverallHealth, a.Issues)
		}
	}
}

// Without 171/172 the drive gives no better source, so 181 is still judged.
func TestSanDisk_PackedTotalStillJudgedWithoutRealCounter(t *testing.T) {
	if got := AnalyzeDriveHealthWithBaseline(sandisk(500, false), nil).OverallHealth; got != SeverityCritical {
		t.Fatalf("got %s, want critical", got)
	}
}

// A real program failure in 171 is still news.
func TestSanDisk_RealProgramFailureStillAlerts(t *testing.T) {
	d := sandisk(204581822, true)
	d.Attributes[2].RawValue = 3
	d.Attributes[2].Threshold = 0
	if got := AnalyzeDriveHealthWithBaseline(d, nil).OverallHealth; got == SeverityHealthy {
		t.Fatalf("got %s, want a non-healthy verdict for 3 program failures", got)
	}
}
