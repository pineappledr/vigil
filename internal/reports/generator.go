package reports

import (
	"bytes"
	"database/sql"
	"fmt"
	"html/template"
	"strings"
	"time"

	"vigil/internal/health"
	"vigil/internal/notify"
	"vigil/internal/smart"
	"vigil/internal/wearout"
	"vigil/internal/zfs"
)

// ReportData holds all data needed to render the health report.
type ReportData struct {
	GeneratedAt string
	Hostname    string // empty = all hosts
	Score       *health.HealthScore
	Drives      []DriveRow
	Pools       []PoolRow
	Alerts      []AlertRow
}

// DriveRow is a single row in the drive table.
type DriveRow struct {
	Hostname     string
	SerialNumber string
	ModelName    string
	DriveType    string
	Health       string
	WearoutPct   string
	Issues       int
}

// PoolRow is a single row in the ZFS pool table.
type PoolRow struct {
	Hostname   string
	PoolName   string
	Health     string
	Size       string
	Used       string
	Free       string
	Errors     int64
}

// AlertRow is a single row in the recent alerts table.
type AlertRow struct {
	Time      string
	EventType string
	Hostname  string
	Message   string
	Status    string
}

// GenerateHealthReport builds a self-contained HTML health report.
func GenerateHealthReport(db *sql.DB, hostname string) ([]byte, error) {
	score, err := health.Calculate(db)
	if err != nil {
		return nil, fmt.Errorf("calculate health score: %w", err)
	}

	drives, err := buildDriveRows(db, hostname)
	if err != nil {
		return nil, fmt.Errorf("build drive rows: %w", err)
	}

	pools, err := buildPoolRows(db, hostname)
	if err != nil {
		return nil, fmt.Errorf("build pool rows: %w", err)
	}

	alerts, err := buildAlertRows(db)
	if err != nil {
		return nil, fmt.Errorf("build alert rows: %w", err)
	}

	data := ReportData{
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04 UTC"),
		Hostname:    hostname,
		Score:       score,
		Drives:      drives,
		Pools:       pools,
		Alerts:      alerts,
	}

	tmpl, err := template.New("report").Funcs(template.FuncMap{
		"gradeColor": gradeColor,
		"healthColor": healthColor,
		"lower": strings.ToLower,
	}).Parse(reportTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	return buf.Bytes(), nil
}

// GenerateHealthReportJSON returns the report data as a struct (for JSON serialization).
func GenerateHealthReportJSON(db *sql.DB, hostname string) (*ReportData, error) {
	score, err := health.Calculate(db)
	if err != nil {
		return nil, err
	}
	drives, err := buildDriveRows(db, hostname)
	if err != nil {
		return nil, err
	}
	pools, err := buildPoolRows(db, hostname)
	if err != nil {
		return nil, err
	}
	alerts, err := buildAlertRows(db)
	if err != nil {
		return nil, err
	}
	return &ReportData{
		GeneratedAt: time.Now().UTC().Format("2006-01-02 15:04 UTC"),
		Hostname:    hostname,
		Score:       score,
		Drives:      drives,
		Pools:       pools,
		Alerts:      alerts,
	}, nil
}

func buildDriveRows(db *sql.DB, hostname string) ([]DriveRow, error) {
	summaries, err := smart.GetAllDrivesHealthSummary(db)
	if err != nil {
		return nil, err
	}

	snapshots, err := wearout.GetAllLatestSnapshots(db)
	if err != nil {
		return nil, err
	}
	wearMap := make(map[string]float64, len(snapshots))
	for _, s := range snapshots {
		wearMap[s.Hostname+":"+s.SerialNumber] = s.Percentage
	}

	var rows []DriveRow
	for _, s := range summaries {
		if hostname != "" && s.Hostname != hostname {
			continue
		}
		pct := "--"
		if w, ok := wearMap[s.Hostname+":"+s.SerialNumber]; ok {
			pct = fmt.Sprintf("%.1f%%", w)
		}
		rows = append(rows, DriveRow{
			Hostname:     s.Hostname,
			SerialNumber: s.SerialNumber,
			ModelName:    s.ModelName,
			DriveType:    s.DriveType,
			Health:       s.OverallHealth,
			WearoutPct:   pct,
			Issues:       s.CriticalCount + s.WarningCount,
		})
	}
	return rows, nil
}

func buildPoolRows(db *sql.DB, hostname string) ([]PoolRow, error) {
	var pools []zfs.ZFSPool
	var err error
	if hostname != "" {
		pools, err = zfs.GetZFSPoolsByHostname(db, hostname)
	} else {
		pools, err = zfs.GetAllZFSPools(db)
	}
	if err != nil {
		return nil, err
	}

	rows := make([]PoolRow, len(pools))
	for i, p := range pools {
		rows[i] = PoolRow{
			Hostname: p.Hostname,
			PoolName: p.PoolName,
			Health:   p.Health,
			Size:     formatBytes(p.SizeBytes),
			Used:     formatBytes(p.AllocatedBytes),
			Free:     formatBytes(p.FreeBytes),
			Errors:   p.ReadErrors + p.WriteErrors + p.ChecksumErrors,
		}
	}
	return rows, nil
}

func buildAlertRows(db *sql.DB) ([]AlertRow, error) {
	records, err := notify.RecentHistory(db, 20)
	if err != nil {
		return nil, err
	}
	rows := make([]AlertRow, len(records))
	for i, r := range records {
		rows[i] = AlertRow{
			Time:      r.CreatedAt.Format("Jan 2, 15:04"),
			EventType: r.EventType,
			Hostname:  r.Hostname,
			Message:   r.Message,
			Status:    r.Status,
		}
	}
	return rows, nil
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	suffixes := []string{"KB", "MB", "GB", "TB", "PB"}
	return fmt.Sprintf("%.1f %s", float64(b)/float64(div), suffixes[exp])
}

func gradeColor(grade string) string {
	switch grade {
	case "Excellent", "Good":
		return "#10b981"
	case "Fair":
		return "#f59e0b"
	case "Warning":
		return "#f97316"
	default:
		return "#ef4444"
	}
}

func healthColor(h string) string {
	switch strings.ToUpper(h) {
	case "HEALTHY", "ONLINE":
		return "#10b981"
	case "WARNING", "DEGRADED":
		return "#f59e0b"
	default:
		return "#ef4444"
	}
}

const reportTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Vigil Health Report{{if .Hostname}} — {{.Hostname}}{{end}}</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#0a0e17;color:#f1f5f9;line-height:1.6;padding:32px;max-width:1200px;margin:0 auto}
h1{font-size:1.5rem;margin-bottom:4px}
h2{font-size:1.1rem;color:#94a3b8;margin:32px 0 12px;border-bottom:1px solid rgba(148,163,184,.1);padding-bottom:8px}
.meta{color:#64748b;font-size:.85rem;margin-bottom:24px}
.score-card{display:inline-flex;align-items:center;gap:20px;background:#111827;border:1px solid rgba(148,163,184,.1);border-radius:12px;padding:20px 28px;margin-bottom:8px}
.score-num{font-size:3rem;font-weight:700;font-family:'JetBrains Mono',monospace;line-height:1}
.score-grade{font-size:1.1rem;font-weight:600}
.score-details{font-size:.85rem;color:#94a3b8;margin-top:4px}
.components{display:flex;gap:12px;margin:12px 0 0}
.comp{background:#1a2234;border-radius:8px;padding:10px 16px;font-size:.85rem}
.comp-label{color:#64748b;margin-bottom:2px}
.comp-val{font-family:'JetBrains Mono',monospace;font-weight:600}
table{width:100%;border-collapse:collapse;font-size:.85rem;margin-bottom:24px}
th{text-align:left;color:#64748b;font-weight:600;padding:8px 12px;border-bottom:1px solid rgba(148,163,184,.15)}
td{padding:8px 12px;border-bottom:1px solid rgba(148,163,184,.06)}
tr:hover td{background:rgba(148,163,184,.04)}
.badge{display:inline-block;padding:2px 8px;border-radius:4px;font-size:.75rem;font-weight:600;text-transform:uppercase}
.badge-healthy,.badge-online{background:rgba(16,185,129,.15);color:#10b981}
.badge-warning,.badge-degraded{background:rgba(245,158,11,.15);color:#f59e0b}
.badge-critical,.badge-faulted{background:rgba(239,68,68,.15);color:#ef4444}
.badge-sent{background:rgba(59,130,246,.15);color:#3b82f6}
.badge-failed{background:rgba(239,68,68,.15);color:#ef4444}
.empty{color:#64748b;font-style:italic;padding:16px 0}
footer{margin-top:40px;padding-top:16px;border-top:1px solid rgba(148,163,184,.1);color:#64748b;font-size:.8rem}
</style>
</head>
<body>
<h1>Vigil Health Report</h1>
<p class="meta">Generated {{.GeneratedAt}}{{if .Hostname}} &middot; Filtered: {{.Hostname}}{{end}}</p>

<div class="score-card">
  <div class="score-num" style="color:{{gradeColor .Score.Grade}}">{{.Score.Score}}</div>
  <div>
    <div class="score-grade" style="color:{{gradeColor .Score.Grade}}">{{.Score.Grade}}</div>
    <div class="score-details">out of 100</div>
  </div>
</div>
<div class="components">
  {{range $k, $v := .Score.Components}}
  <div class="comp">
    <div class="comp-label">{{$k}}</div>
    <div class="comp-val">−{{printf "%.0f" $v.Deduction}}</div>
    <div class="score-details">{{$v.Details}}</div>
  </div>
  {{end}}
</div>

<h2>Drives</h2>
{{if .Drives}}
<table>
<thead><tr><th>Host</th><th>Serial</th><th>Model</th><th>Type</th><th>Health</th><th>Wearout</th><th>Issues</th></tr></thead>
<tbody>
{{range .Drives}}
<tr>
  <td>{{.Hostname}}</td>
  <td><code>{{.SerialNumber}}</code></td>
  <td>{{.ModelName}}</td>
  <td>{{.DriveType}}</td>
  <td><span class="badge badge-{{lower .Health}}">{{.Health}}</span></td>
  <td>{{.WearoutPct}}</td>
  <td>{{.Issues}}</td>
</tr>
{{end}}
</tbody>
</table>
{{else}}
<p class="empty">No drive data available.</p>
{{end}}

<h2>ZFS Pools</h2>
{{if .Pools}}
<table>
<thead><tr><th>Host</th><th>Pool</th><th>Health</th><th>Size</th><th>Used</th><th>Free</th><th>Errors</th></tr></thead>
<tbody>
{{range .Pools}}
<tr>
  <td>{{.Hostname}}</td>
  <td>{{.PoolName}}</td>
  <td><span class="badge badge-{{lower .Health}}">{{.Health}}</span></td>
  <td>{{.Size}}</td>
  <td>{{.Used}}</td>
  <td>{{.Free}}</td>
  <td>{{.Errors}}</td>
</tr>
{{end}}
</tbody>
</table>
{{else}}
<p class="empty">No ZFS pools configured.</p>
{{end}}

<h2>Recent Alerts</h2>
{{if .Alerts}}
<table>
<thead><tr><th>Time</th><th>Event</th><th>Host</th><th>Message</th><th>Status</th></tr></thead>
<tbody>
{{range .Alerts}}
<tr>
  <td style="white-space:nowrap">{{.Time}}</td>
  <td><code>{{.EventType}}</code></td>
  <td>{{.Hostname}}</td>
  <td>{{.Message}}</td>
  <td><span class="badge badge-{{lower .Status}}">{{.Status}}</span></td>
</tr>
{{end}}
</tbody>
</table>
{{else}}
<p class="empty">No recent alerts.</p>
{{end}}

<footer>Vigil &mdash; Infrastructure Health Monitor</footer>
</body>
</html>`
