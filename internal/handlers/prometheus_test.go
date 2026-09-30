package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"vigil/internal/agents"
	"vigil/internal/db"
	"vigil/internal/metrics"
	"vigil/internal/wearout"
)

const testMetricsToken = "0123456789abcdef0123456789abcdef"

func initMetricsTestDB(t *testing.T) {
	t.Helper()
	if err := db.Init(filepath.Join(t.TempDir(), "t.db")); err != nil {
		t.Fatal(err)
	}
	if err := db.MigrateSchemaExtensions(db.DB); err != nil {
		t.Fatal(err)
	}
	if err := agents.Migrate(db.DB); err != nil {
		t.Fatal(err)
	}
	if err := wearout.MigrateWearoutTables(db.DB); err != nil {
		t.Fatal(err)
	}
	// zfs_pools is created by MigrateSchemaExtensions; the ALTERs in db.Init's
	// migrateSchema() ran before it existed (see e2e_zfs_test.go).
	db.DB.Exec("ALTER TABLE zfs_pools ADD COLUMN compress_ratio REAL DEFAULT 1.0")
	db.DB.Exec("ALTER TABLE zfs_pools ADD COLUMN scan_speed INTEGER DEFAULT 0")
	db.DB.Exec("ALTER TABLE zfs_pools ADD COLUMN scan_errors INTEGER DEFAULT 0")
	db.DB.Exec("ALTER TABLE zfs_pools ADD COLUMN scan_time_remaining INTEGER DEFAULT 0")
}

func scrape(t *testing.T, token, auth string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	if auth != "" {
		req.Header.Set("Authorization", auth)
	}
	rec := httptest.NewRecorder()
	PrometheusMetrics(token)(rec, req)
	return rec
}

func TestPrometheusMetricsDisabledWithoutToken(t *testing.T) {
	initMetricsTestDB(t)
	if rec := scrape(t, "", "Bearer "+testMetricsToken); rec.Code != http.StatusNotFound {
		t.Errorf("no token configured: status = %d, want 404", rec.Code)
	}
	// A short token must not enable the endpoint: it is not rate limited.
	if rec := scrape(t, "short", "Bearer short"); rec.Code != http.StatusNotFound {
		t.Errorf("short token configured: status = %d, want 404", rec.Code)
	}
}

func TestPrometheusMetricsRejectsBadToken(t *testing.T) {
	initMetricsTestDB(t)
	for name, auth := range map[string]string{
		"missing":     "",
		"wrong":       "Bearer " + strings.Repeat("x", len(testMetricsToken)),
		"prefix only": "Bearer ",
		"not bearer":  "Basic " + testMetricsToken,
		"truncated":   "Bearer " + testMetricsToken[:20],
	} {
		rec := scrape(t, testMetricsToken, auth)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s: status = %d, want 401", name, rec.Code)
		}
		if rec.Header().Get("WWW-Authenticate") == "" {
			t.Errorf("%s: missing WWW-Authenticate", name)
		}
		if strings.Contains(rec.Body.String(), "vigil_") {
			t.Errorf("%s: metrics leaked on a 401", name)
		}
	}
}

func TestPrometheusMetricsServesExposition(t *testing.T) {
	initMetricsTestDB(t)
	Version = "9.9.9"
	Metrics = metrics.New()
	Metrics.ReportsProcessed.Add(5)
	t.Cleanup(func() { Version = "dev"; Metrics = nil })

	// Brain-like report: one HDD with a frozen CRC counter (acknowledged), one
	// NVMe, and a drive without serial (must be skipped).
	report := `{"hostname":"brain","drives":[
	  {"serial_number":"41L0A046FBEG","model_name":"TOSHIBA HDWG180","rotation_rate":7200,
	   "device":{"name":"/dev/sdb","protocol":"ATA"},
	   "smart_status":{"passed":true},
	   "temperature":{"current":36},
	   "power_on_time":{"hours":12000},
	   "ata_smart_attributes":{"table":[
	     {"id":5,"name":"Reallocated_Sector_Ct","value":100,"thresh":50,"raw":{"value":0}},
	     {"id":9,"name":"Power_On_Hours","value":70,"thresh":0,"raw":{"value":12000}},
	     {"id":194,"name":"Temperature_Celsius","value":100,"thresh":0,"raw":{"value":36}},
	     {"id":199,"name":"UDMA_CRC_Error_Count","value":200,"thresh":0,"raw":{"value":1633}},
	     {"id":241,"name":"Total_LBAs_Written","value":100,"thresh":0,"raw":{"value":999}}
	   ]}},
	  {"serial_number":"S4EWNX0N","model_name":"Samsung SSD 970 EVO Plus 250GB",
	   "device":{"name":"/dev/nvme0","protocol":"NVMe"},
	   "smart_status":{"passed":true},
	   "nvme_smart_health_information_log":{"temperature":41,"percentage_used":3,
	     "media_errors":0,"unsafe_shutdowns":27,"critical_warning":0,"available_spare":100,
	     "power_on_hours":8000}},
	  {"model_name":"sipeed NanoKVM"}
	],"zfs":{"zfs_available":true,"pools":[{"name":"Storage","health":"ONLINE","status":"ONLINE",
	  "size_bytes":7988639170560,"allocated_bytes":3806516764672,"free_bytes":4182122405888,
	  "read_errors":0,"write_errors":0,"checksum_errors":0,
	  "devices":[{"name":"sdb2","state":"ONLINE","serial_number":"41L0A046FBEG","vdev_type":"disk"}]}]}}`

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(report), &payload); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec("INSERT INTO reports (hostname, timestamp, data) VALUES (?, ?, ?)",
		"brain", "2026-09-30 12:00:00", report); err != nil {
		t.Fatal(err)
	}
	ProcessZFSFromReport("brain", payload)
	if _, err := db.DB.Exec(`INSERT INTO drive_aliases (hostname, serial_number, alias) VALUES ('brain','41L0A046FBEG','Toshiba "A"')`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.DB.Exec(`INSERT INTO drive_health_baselines (hostname, serial_number, attribute_id, raw_value)
		VALUES ('brain','41L0A046FBEG',199,1633)`); err != nil {
		t.Fatal(err)
	}

	rec := scrape(t, testMetricsToken, "Bearer "+testMetricsToken)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain; version=0.0.4") {
		t.Errorf("Content-Type = %q", ct)
	}
	body := rec.Body.String()

	for _, want := range []string{
		`vigil_build_info{version="9.9.9"} 1`,
		`vigil_host_last_report_timestamp_seconds{host="brain"} 1790769600`,
		`vigil_host_drives{host="brain"} 3`,
		`vigil_drive_info{host="brain",serial="41L0A046FBEG",model="TOSHIBA HDWG180",alias="Toshiba \"A\"",device="/dev/sdb",type="HDD"} 1`,
		// CRC 1633 is acknowledged, so Vigil does not call the drive critical.
		`vigil_drive_health{host="brain",serial="41L0A046FBEG"} 0`,
		`vigil_drive_health_issues{host="brain",serial="41L0A046FBEG"} 0`,
		`vigil_drive_smart_passed{host="brain",serial="41L0A046FBEG"} 1`,
		`vigil_drive_temperature_celsius{host="brain",serial="41L0A046FBEG"} 36`,
		`vigil_drive_power_on_seconds{host="brain",serial="41L0A046FBEG"} 43200000`,
		`vigil_drive_smart_attribute_raw{host="brain",serial="41L0A046FBEG",id="199",name="UDMA_CRC_Error_Count"} 1633`,
		`vigil_drive_smart_attribute_raw{host="brain",serial="41L0A046FBEG",id="5",name="Reallocated_Sector_Ct"} 0`,
		`vigil_drive_baseline_acknowledged{host="brain",serial="41L0A046FBEG"} 1`,
		`vigil_drive_baseline_acknowledged_counters{host="brain",serial="41L0A046FBEG"} 1`,
		`vigil_drive_baseline_acknowledged{host="brain",serial="S4EWNX0N"} 0`,
		`vigil_drive_temperature_celsius{host="brain",serial="S4EWNX0N"} 41`,
		`vigil_drive_nvme_percentage_used{host="brain",serial="S4EWNX0N"} 3`,
		`vigil_drive_nvme_unsafe_shutdowns{host="brain",serial="S4EWNX0N"} 27`,
		`vigil_zfs_pool_health{host="brain",pool="Storage",state="ONLINE"} 1`,
		`vigil_zfs_pool_online{host="brain",pool="Storage"} 1`,
		`vigil_zfs_pool_size_bytes{host="brain",pool="Storage"} 7988639170560`,
		`vigil_zfs_pool_errors{host="brain",pool="Storage",type="checksum"} 0`,
		`vigil_zfs_device_errors{host="brain",pool="Storage",device="sdb2",vdev_type="disk",serial="41L0A046FBEG",type="write"} 0`,
		"# TYPE vigil_reports_processed_total counter",
		"vigil_reports_processed_total 5",
	} {
		if !strings.Contains(body, want+"\n") && !strings.Contains(body, want) {
			t.Errorf("missing line:\n  %s", want)
		}
	}

	for _, notWant := range []string{
		`id="241"`, // not a key attribute: kept out to bound cardinality
		`id="187"`, // the NVMe pseudo-id must not leak into the ATA family
		`NanoKVM`,  // no serial, no series
	} {
		if strings.Contains(body, notWant) {
			t.Errorf("unexpected %q in output", notWant)
		}
	}
	if t.Failed() {
		t.Logf("body:\n%s", body)
	}
}
