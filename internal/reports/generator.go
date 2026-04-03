package reports

import (
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"math"
	"time"

	agentsmart "vigil/cmd/agent/smart"
	"vigil/internal/health"
	"vigil/internal/notify"
	"vigil/internal/smart"
	"vigil/internal/wearout"
	"vigil/internal/zfs"
)

// ReportData is the full dataset passed to the HTML template.
type ReportData struct {
	GeneratedAt   time.Time
	Score         health.HealthScore
	Drives        []DriveRow
	Pools         []zfs.ZFSPool
	Notifications []notify.NotificationRecord
}

// DriveRow combines SMART health and wearout into one table row.
type DriveRow struct {
	Hostname      string
	SerialNumber  string
	ModelName     string
	DriveType     string
	OverallHealth string
	WearoutPct    float64
	HasWearout    bool
	Prediction    string
	Issues        []agentsmart.HealthIssue
}

// GenerateHealthReport builds a self-contained HTML health report.
func GenerateHealthReport(db *sql.DB) ([]byte, error) {
	smartData, err := smart.GetAllDrivesHealthSummary(db)
	if err != nil {
		return nil, err
	}

	wearoutData, err := wearout.GetAllLatestSnapshots(db)
	if err != nil {
		return nil, err
	}

	zfsData, err := zfs.GetAllZFSPools(db)
	if err != nil {
		return nil, err
	}

	recentNotifs, err := notify.RecentHistory(db, 50)
	if err != nil {
		recentNotifs = nil
	}

	score := health.ComputeScore(smartData, wearoutData, zfsData)

	// Index wearout by serial for fast lookup.
	wearoutBySerial := make(map[string]wearout.WearoutSnapshot, len(wearoutData))
	for _, w := range wearoutData {
		wearoutBySerial[w.SerialNumber] = w
	}

	var drives []DriveRow
	for _, s := range smartData {
		row := DriveRow{
			Hostname:      s.Hostname,
			SerialNumber:  s.SerialNumber,
			ModelName:     s.ModelName,
			DriveType:     s.DriveType,
			OverallHealth: s.OverallHealth,
			Issues:        s.Issues,
		}
		if w, ok := wearoutBySerial[s.SerialNumber]; ok {
			row.HasWearout = true
			row.WearoutPct = math.Round(w.Percentage*10) / 10
			row.Prediction = wearoutPrediction(db, s.Hostname, s.SerialNumber)
		}
		drives = append(drives, row)
	}

	// Filter notifications to last 7 days.
	sevenDaysAgo := time.Now().UTC().AddDate(0, 0, -7)
	var recentWeek []notify.NotificationRecord
	for _, n := range recentNotifs {
		if n.CreatedAt.After(sevenDaysAgo) {
			recentWeek = append(recentWeek, n)
		}
	}

	data := ReportData{
		GeneratedAt:   time.Now().UTC(),
		Score:         score,
		Drives:        drives,
		Pools:         zfsData,
		Notifications: recentWeek,
	}

	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"gradeColor":   gradeColor,
		"healthColor":  healthColor,
		"poolColor":    poolColor,
		"wearoutColor": wearoutColor,
		"formatTime":   func(t time.Time) string { return t.Format("2006-01-02 15:04 UTC") },
		"sub":          func(a, b int) int { return a - b },
		"pct":          func(v float64) string { return fmt.Sprintf("%.1f", v) },
		"scoreBarPct":  func(max, val int) float64 {
			if max == 0 {
				return 0
			}
			return float64(val) * 100.0 / float64(max)
		},
		"concat": func(a, b, c []string) []string {
			out := make([]string, 0, len(a)+len(b)+len(c))
			out = append(out, a...)
			out = append(out, b...)
			out = append(out, c...)
			return out
		},
	}).Parse(reportTemplate)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func wearoutPrediction(db *sql.DB, hostname, serial string) string {
	history, err := wearout.GetSnapshotHistory(db, hostname, serial, 365)
	if err != nil || len(history) < 3 {
		return ""
	}
	pred := wearout.PredictTrend(history)
	if pred == nil || pred.MonthsRemaining == nil {
		return ""
	}
	m := *pred.MonthsRemaining
	if m < 1 {
		return "<1 month"
	}
	return fmt.Sprintf("~%.0f months", math.Round(m))
}

func gradeColor(grade string) string {
	switch grade {
	case "Excellent":
		return "#22c55e"
	case "Good":
		return "#84cc16"
	case "Fair":
		return "#eab308"
	case "Warning":
		return "#f97316"
	default:
		return "#ef4444"
	}
}

func healthColor(h string) string {
	switch h {
	case "HEALTHY":
		return "#22c55e"
	case "WARNING":
		return "#eab308"
	default:
		return "#ef4444"
	}
}

func poolColor(status string) string {
	switch status {
	case "ONLINE":
		return "#22c55e"
	case "DEGRADED":
		return "#eab308"
	default:
		return "#ef4444"
	}
}

func wearoutColor(pct float64) string {
	switch {
	case pct >= 80:
		return "#ef4444"
	case pct >= 60:
		return "#f97316"
	case pct >= 30:
		return "#eab308"
	default:
		return "#22c55e"
	}
}

const reportTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Vigil Health Report</title>
<style>
  * { box-sizing: border-box; margin: 0; padding: 0; }
  body { font-family: system-ui, -apple-system, sans-serif; background: #0f1117; color: #e2e8f0; line-height: 1.5; }
  .container { max-width: 1000px; margin: 0 auto; padding: 32px 20px; }
  h1 { font-size: 1.6rem; font-weight: 700; color: #f1f5f9; margin-bottom: 4px; }
  h2 { font-size: 1.1rem; font-weight: 600; color: #94a3b8; text-transform: uppercase; letter-spacing: .05em; margin: 32px 0 12px; }
  .meta { color: #64748b; font-size: .85rem; margin-bottom: 32px; }
  .score-card { background: #1e2330; border: 1px solid #2d3748; border-radius: 12px; padding: 24px; display: flex; align-items: flex-start; gap: 32px; margin-bottom: 32px; }
  .score-number { font-size: 4rem; font-weight: 800; line-height: 1; }
  .score-grade { font-size: 1.1rem; font-weight: 600; margin-top: 4px; }
  .components { flex: 1; display: flex; flex-direction: column; gap: 10px; }
  .comp-row { display: flex; align-items: center; gap: 10px; font-size: .85rem; }
  .comp-label { width: 70px; color: #94a3b8; }
  .bar-track { flex: 1; background: #2d3748; border-radius: 4px; height: 8px; overflow: hidden; }
  .bar-fill { height: 8px; border-radius: 4px; background: #3b82f6; }
  .comp-val { width: 50px; text-align: right; color: #cbd5e1; font-size: .8rem; }
  .notes { margin-top: 8px; padding-top: 8px; border-top: 1px solid #2d3748; }
  .note-item { font-size: .8rem; color: #94a3b8; padding: 2px 0; }
  table { width: 100%; border-collapse: collapse; font-size: .85rem; }
  th { text-align: left; padding: 8px 12px; color: #64748b; font-weight: 600; border-bottom: 1px solid #2d3748; text-transform: uppercase; font-size: .75rem; letter-spacing: .05em; }
  td { padding: 10px 12px; border-bottom: 1px solid #1e2330; }
  tr:last-child td { border-bottom: none; }
  tr:hover td { background: #1a1f2e; }
  .badge { display: inline-block; padding: 2px 8px; border-radius: 99px; font-size: .75rem; font-weight: 600; }
  .wearout-bar { display: flex; align-items: center; gap: 8px; }
  .wearout-track { width: 60px; background: #2d3748; border-radius: 4px; height: 6px; overflow: hidden; }
  .wearout-fill { height: 6px; border-radius: 4px; }
  .empty { color: #4a5568; font-size: .85rem; padding: 20px 0; text-align: center; }
</style>
</head>
<body>
<div class="container">
  <h1>Vigil Health Report</h1>
  <div class="meta">Generated {{formatTime .GeneratedAt}}</div>

  <div class="score-card">
    <div>
      <div class="score-number" style="color:{{gradeColor .Score.Grade}}">{{.Score.Score}}</div>
      <div class="score-grade" style="color:{{gradeColor .Score.Grade}}">{{.Score.Grade}}</div>
    </div>
    <div class="components">
      <div class="comp-row">
        <span class="comp-label">SMART</span>
        <div class="bar-track"><div class="bar-fill" style="width:{{sub .Score.Components.Smart.Max .Score.Components.Smart.Deduction | scoreBarPct .Score.Components.Smart.Max}}%"></div></div>
        <span class="comp-val">{{sub .Score.Components.Smart.Max .Score.Components.Smart.Deduction}}/{{.Score.Components.Smart.Max}}</span>
      </div>
      <div class="comp-row">
        <span class="comp-label">Wearout</span>
        <div class="bar-track"><div class="bar-fill" style="width:{{sub .Score.Components.Wearout.Max .Score.Components.Wearout.Deduction | scoreBarPct .Score.Components.Wearout.Max}}%"></div></div>
        <span class="comp-val">{{sub .Score.Components.Wearout.Max .Score.Components.Wearout.Deduction}}/{{.Score.Components.Wearout.Max}}</span>
      </div>
      <div class="comp-row">
        <span class="comp-label">ZFS</span>
        <div class="bar-track"><div class="bar-fill" style="width:{{sub .Score.Components.ZFS.Max .Score.Components.ZFS.Deduction | scoreBarPct .Score.Components.ZFS.Max}}%"></div></div>
        <span class="comp-val">{{sub .Score.Components.ZFS.Max .Score.Components.ZFS.Deduction}}/{{.Score.Components.ZFS.Max}}</span>
      </div>
      {{$allNotes := concat .Score.Components.Smart.Notes .Score.Components.Wearout.Notes .Score.Components.ZFS.Notes}}
      {{if $allNotes}}<div class="notes">{{range $allNotes}}<div class="note-item">⚠ {{.}}</div>{{end}}</div>{{end}}
    </div>
  </div>

  <h2>Drives ({{len .Drives}})</h2>
  {{if .Drives}}
  <table>
    <thead><tr><th>Host</th><th>Model</th><th>Serial</th><th>Type</th><th>SMART</th><th>Wearout</th><th>Prediction</th></tr></thead>
    <tbody>
    {{range .Drives}}
    <tr>
      <td>{{.Hostname}}</td>
      <td>{{.ModelName}}</td>
      <td style="font-family:monospace;font-size:.8rem">{{.SerialNumber}}</td>
      <td>{{.DriveType}}</td>
      <td><span class="badge" style="background:{{healthColor .OverallHealth}}22;color:{{healthColor .OverallHealth}}">{{.OverallHealth}}</span></td>
      <td>{{if .HasWearout}}<div class="wearout-bar"><div class="wearout-track"><div class="wearout-fill" style="width:{{.WearoutPct}}%;background:{{wearoutColor .WearoutPct}}"></div></div><span style="color:{{wearoutColor .WearoutPct}};font-size:.85rem">{{pct .WearoutPct}}%</span></div>{{else}}—{{end}}</td>
      <td style="color:#94a3b8">{{if .Prediction}}{{.Prediction}}{{else}}—{{end}}</td>
    </tr>
    {{end}}
    </tbody>
  </table>
  {{else}}<div class="empty">No drive data available.</div>{{end}}

  {{if .Pools}}
  <h2>ZFS Pools ({{len .Pools}})</h2>
  <table>
    <thead><tr><th>Host</th><th>Pool</th><th>Status</th><th>Capacity</th><th>Read Err</th><th>Write Err</th><th>Checksum Err</th></tr></thead>
    <tbody>
    {{range .Pools}}
    <tr>
      <td>{{.Hostname}}</td>
      <td>{{.PoolName}}</td>
      <td><span class="badge" style="background:{{poolColor .Status}}22;color:{{poolColor .Status}}">{{.Status}}</span></td>
      <td><div class="wearout-bar"><div class="wearout-track"><div class="wearout-fill" style="width:{{pct .CapacityPct}}%;background:{{wearoutColor .CapacityPct}}"></div></div><span style="font-size:.85rem">{{pct .CapacityPct}}%</span></div></td>
      <td>{{.ReadErrors}}</td>
      <td>{{.WriteErrors}}</td>
      <td>{{.ChecksumErrors}}</td>
    </tr>
    {{end}}
    </tbody>
  </table>
  {{end}}

  <h2>Alerts — Last 7 Days ({{len .Notifications}})</h2>
  {{if .Notifications}}
  <table>
    <thead><tr><th>Time</th><th>Type</th><th>Host</th><th>Message</th><th>Status</th></tr></thead>
    <tbody>
    {{range .Notifications}}
    <tr>
      <td style="white-space:nowrap;color:#64748b;font-size:.8rem">{{formatTime .CreatedAt}}</td>
      <td style="font-family:monospace;font-size:.78rem">{{.EventType}}</td>
      <td>{{.Hostname}}</td>
      <td style="max-width:380px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;font-size:.82rem">{{.Message}}</td>
      <td><span class="badge" style="background:#1e2330;color:#94a3b8">{{.Status}}</span></td>
    </tr>
    {{end}}
    </tbody>
  </table>
  {{else}}<div class="empty">No alerts in the last 7 days.</div>{{end}}

</div>
</body>
</html>`
