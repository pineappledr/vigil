package health

import (
	"database/sql"
	"strings"

	"vigil/internal/smart"
	"vigil/internal/wearout"
	"vigil/internal/zfs"
)

// Component holds the deduction details for one scoring dimension.
type Component struct {
	Deduction float64 `json:"deduction"`
	Details   string  `json:"details"`
}

// HealthScore is the aggregate health result.
type HealthScore struct {
	Score      int                  `json:"score"`
	Grade      string               `json:"grade"`
	Components map[string]Component `json:"components"`
}

// Grade thresholds.
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

// Calculate computes the overall health score from SMART, wearout, and ZFS data.
func Calculate(db *sql.DB) (*HealthScore, error) {
	smartComp, err := smartComponent(db)
	if err != nil {
		return nil, err
	}
	wearComp, err := wearoutComponent(db)
	if err != nil {
		return nil, err
	}
	zfsComp, err := zfsComponent(db)
	if err != nil {
		return nil, err
	}

	total := smartComp.Deduction + wearComp.Deduction + zfsComp.Deduction
	score := 100 - int(total)
	if score < 0 {
		score = 0
	}

	return &HealthScore{
		Score: score,
		Grade: grade(score),
		Components: map[string]Component{
			"smart":   smartComp,
			"wearout": wearComp,
			"zfs":     zfsComp,
		},
	}, nil
}

// smartComponent: −20 per CRITICAL drive, −5 per WARNING drive (max 40).
func smartComponent(db *sql.DB) (Component, error) {
	summaries, err := smart.GetAllDrivesHealthSummary(db)
	if err != nil {
		return Component{}, err
	}

	var critical, warning int
	for _, s := range summaries {
		switch strings.ToUpper(s.OverallHealth) {
		case "CRITICAL":
			critical++
		case "WARNING":
			warning++
		}
	}

	ded := float64(critical*20 + warning*5)
	if ded > 40 {
		ded = 40
	}

	return Component{
		Deduction: ded,
		Details:   smartDetails(len(summaries), critical, warning),
	}, nil
}

func smartDetails(total, critical, warning int) string {
	if critical == 0 && warning == 0 {
		return "all drives healthy"
	}
	parts := make([]string, 0, 2)
	if critical > 0 {
		parts = append(parts, pluralize(critical, "critical drive"))
	}
	if warning > 0 {
		parts = append(parts, pluralize(warning, "warning drive"))
	}
	return strings.Join(parts, ", ")
}

// wearoutComponent: average wearout % scaled to max 30 points.
func wearoutComponent(db *sql.DB) (Component, error) {
	snapshots, err := wearout.GetAllLatestSnapshots(db)
	if err != nil {
		return Component{}, err
	}
	if len(snapshots) == 0 {
		return Component{Deduction: 0, Details: "no wearout data"}, nil
	}

	var sum float64
	for _, s := range snapshots {
		sum += s.Percentage
	}
	avg := sum / float64(len(snapshots))
	ded := avg / 100 * 30
	if ded > 30 {
		ded = 30
	}

	return Component{
		Deduction: ded,
		Details:   wearoutDetails(avg, len(snapshots)),
	}, nil
}

func wearoutDetails(avg float64, count int) string {
	return pluralize(count, "drive") + ", avg " + formatPct(avg) + "% worn"
}

// zfsComponent: −30 per FAULTED, −10 per DEGRADED, −1 per 100 errors (max 30).
func zfsComponent(db *sql.DB) (Component, error) {
	pools, err := zfs.GetAllZFSPools(db)
	if err != nil {
		return Component{}, err
	}
	if len(pools) == 0 {
		return Component{Deduction: 0, Details: "no ZFS pools"}, nil
	}

	var ded float64
	var faulted, degraded int
	var totalErrors int64
	for _, p := range pools {
		h := strings.ToUpper(p.Health)
		switch h {
		case "FAULTED", "UNAVAIL":
			faulted++
			ded += 30
		case "DEGRADED":
			degraded++
			ded += 10
		}
		totalErrors += p.ReadErrors + p.WriteErrors + p.ChecksumErrors
	}
	ded += float64(totalErrors) / 100
	if ded > 30 {
		ded = 30
	}

	return Component{
		Deduction: ded,
		Details:   zfsDetails(len(pools), faulted, degraded, totalErrors),
	}, nil
}

func zfsDetails(total, faulted, degraded int, errors int64) string {
	if faulted == 0 && degraded == 0 && errors == 0 {
		return pluralize(total, "pool") + " healthy"
	}
	parts := make([]string, 0, 3)
	if faulted > 0 {
		parts = append(parts, pluralize(faulted, "faulted"))
	}
	if degraded > 0 {
		parts = append(parts, pluralize(degraded, "degraded"))
	}
	if errors > 0 {
		parts = append(parts, pluralize(int(errors), "error"))
	}
	return strings.Join(parts, ", ")
}

// ── helpers ─────────────────────────────────────────────────────────────────

func pluralize(n int, singular string) string {
	if n == 1 {
		return "1 " + singular
	}
	return itoa(n) + " " + singular + "s"
}

func itoa(n int) string {
	return strings.TrimRight(strings.TrimRight(formatInt(n), "0"), ".")
}

func formatInt(n int) string {
	buf := make([]byte, 0, 8)
	if n < 0 {
		buf = append(buf, '-')
		n = -n
	}
	buf = appendInt(buf, n)
	return string(buf)
}

func appendInt(buf []byte, n int) []byte {
	if n >= 10 {
		buf = appendInt(buf, n/10)
	}
	buf = append(buf, byte('0'+n%10))
	return buf
}

func formatPct(f float64) string {
	// one decimal place
	n := int(f*10 + 0.5)
	whole := n / 10
	frac := n % 10
	return formatInt(whole) + "." + string(byte('0'+frac))
}

