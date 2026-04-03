package health

import (
	"fmt"

	agentsmart "vigil/cmd/agent/smart"
	"vigil/internal/wearout"
	"vigil/internal/zfs"
)

// HealthScore is the aggregated system health score for all monitored resources.
type HealthScore struct {
	Score      int             `json:"score"`      // 0–100
	Grade      string          `json:"grade"`      // Excellent / Good / Fair / Warning / Critical
	Components ScoreComponents `json:"components"`
}

// ScoreComponents breaks the score down by subsystem.
type ScoreComponents struct {
	Smart   ComponentScore `json:"smart"`
	Wearout ComponentScore `json:"wearout"`
	ZFS     ComponentScore `json:"zfs"`
}

// ComponentScore represents one subsystem's contribution to the overall score.
type ComponentScore struct {
	Deduction int      `json:"deduction"` // points lost (positive number)
	Max       int      `json:"max"`       // maximum possible deduction for this component
	Notes     []string `json:"notes"`     // human-readable reasons for deductions
}

// ComputeScore calculates the system health score from SMART, wearout, and ZFS data.
//
// Scoring (100 points — deductions applied per component, capped):
//
//	SMART  (max −40): CRITICAL drive −10 (cap −30), WARNING drive −5 (cap −20)
//	Wearout(max −30): ≥80% −10 (cap −20), 60–79% −5 (cap −15), 30–59% −2 (cap −10)
//	ZFS    (max −30): FAULTED pool −15, DEGRADED pool −10, errors −5 (cap −10)
func ComputeScore(
	smartData []*agentsmart.DriveHealthAnalysis,
	wearoutData []wearout.WearoutSnapshot,
	zfsData []zfs.ZFSPool,
) HealthScore {
	smartComp := scoreSmart(smartData)
	wearoutComp := scoreWearout(wearoutData)
	zfsComp := scoreZFS(zfsData)

	total := 100 - smartComp.Deduction - wearoutComp.Deduction - zfsComp.Deduction
	if total < 0 {
		total = 0
	}

	return HealthScore{
		Score: total,
		Grade: grade(total),
		Components: ScoreComponents{
			Smart:   smartComp,
			Wearout: wearoutComp,
			ZFS:     zfsComp,
		},
	}
}

func scoreSmart(drives []*agentsmart.DriveHealthAnalysis) ComponentScore {
	const maxDeduction = 40
	deduction := 0
	var notes []string

	for _, d := range drives {
		switch d.OverallHealth {
		case "CRITICAL":
			if deduction < 30 {
				deduction += 10
				notes = append(notes, fmt.Sprintf("%s (%s): SMART critical", d.ModelName, d.SerialNumber))
			}
		case "WARNING":
			if deduction < 20 {
				deduction += 5
				notes = append(notes, fmt.Sprintf("%s (%s): SMART warning", d.ModelName, d.SerialNumber))
			}
		}
	}

	if deduction > maxDeduction {
		deduction = maxDeduction
	}
	return ComponentScore{Deduction: deduction, Max: maxDeduction, Notes: notes}
}

func scoreWearout(snapshots []wearout.WearoutSnapshot) ComponentScore {
	const maxDeduction = 30
	deduction := 0
	var notes []string

	for _, s := range snapshots {
		switch {
		case s.Percentage >= 80:
			if deduction < 20 {
				deduction += 10
				notes = append(notes, fmt.Sprintf("%s: wearout %.0f%% (critical)", s.SerialNumber, s.Percentage))
			}
		case s.Percentage >= 60:
			if deduction < 15 {
				deduction += 5
				notes = append(notes, fmt.Sprintf("%s: wearout %.0f%% (warning)", s.SerialNumber, s.Percentage))
			}
		case s.Percentage >= 30:
			if deduction < 10 {
				deduction += 2
				notes = append(notes, fmt.Sprintf("%s: wearout %.0f%%", s.SerialNumber, s.Percentage))
			}
		}
	}

	if deduction > maxDeduction {
		deduction = maxDeduction
	}
	return ComponentScore{Deduction: deduction, Max: maxDeduction, Notes: notes}
}

func scoreZFS(pools []zfs.ZFSPool) ComponentScore {
	const maxDeduction = 30
	deduction := 0
	errorDeduction := 0
	var notes []string

	for _, p := range pools {
		switch p.Status {
		case "FAULTED":
			deduction += 15
			notes = append(notes, fmt.Sprintf("Pool %s: FAULTED", p.PoolName))
		case "DEGRADED":
			deduction += 10
			notes = append(notes, fmt.Sprintf("Pool %s: DEGRADED", p.PoolName))
		}

		if (p.ReadErrors + p.WriteErrors + p.ChecksumErrors) > 0 && errorDeduction < 10 {
			errorDeduction += 5
			notes = append(notes, fmt.Sprintf("Pool %s: I/O errors detected", p.PoolName))
		}
	}

	total := deduction + errorDeduction
	if total > maxDeduction {
		total = maxDeduction
	}
	return ComponentScore{Deduction: total, Max: maxDeduction, Notes: notes}
}

func grade(score int) string {
	switch {
	case score >= 90:
		return "Excellent"
	case score >= 75:
		return "Good"
	case score >= 60:
		return "Fair"
	case score >= 40:
		return "Warning"
	default:
		return "Critical"
	}
}
