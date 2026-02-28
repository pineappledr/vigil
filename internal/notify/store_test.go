package notify

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	// Enable foreign keys
	db.Exec("PRAGMA foreign_keys = ON")

	// Create the base table from migrations.go
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS notification_settings (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			name            TEXT    NOT NULL,
			service_type    TEXT    NOT NULL,
			config_json     TEXT    NOT NULL,
			enabled         INTEGER DEFAULT 1,
			notify_on_critical INTEGER DEFAULT 1,
			notify_on_warning  INTEGER DEFAULT 0,
			notify_on_healthy  INTEGER DEFAULT 0,
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE TABLE IF NOT EXISTS notification_history (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			setting_id      INTEGER,
			event_type      TEXT    NOT NULL,
			hostname        TEXT,
			serial_number   TEXT,
			message         TEXT    NOT NULL,
			status          TEXT    NOT NULL DEFAULT 'pending',
			error_message   TEXT,
			sent_at         DATETIME,
			created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (setting_id) REFERENCES notification_settings(id) ON DELETE SET NULL
		);`)
	if err != nil {
		t.Fatal(err)
	}

	// Run our extension migration
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func createTestService(t *testing.T, db *sql.DB) int64 {
	t.Helper()
	svc := &NotificationService{
		Name:             "test-discord",
		ServiceType:      "discord",
		ConfigJSON:       `{"webhook_url":"https://discord.com/api/webhooks/test"}`,
		Enabled:          true,
		NotifyOnCritical: true,
		NotifyOnWarning:  true,
	}
	id, err := CreateService(db, svc)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func TestCreateAndGetService(t *testing.T) {
	db := setupTestDB(t)
	id := createTestService(t, db)

	svc, err := GetService(db, id)
	if err != nil {
		t.Fatal(err)
	}
	if svc == nil {
		t.Fatal("expected service, got nil")
	}
	if svc.Name != "test-discord" {
		t.Errorf("name = %q, want %q", svc.Name, "test-discord")
	}
	if !svc.Enabled {
		t.Error("expected enabled")
	}
	if !svc.NotifyOnCritical {
		t.Error("expected notify_on_critical")
	}
	if !svc.NotifyOnWarning {
		t.Error("expected notify_on_warning")
	}
	if svc.NotifyOnHealthy {
		t.Error("expected no notify_on_healthy")
	}
}

func TestListServices(t *testing.T) {
	db := setupTestDB(t)
	createTestService(t, db)
	CreateService(db, &NotificationService{
		Name: "slack", ServiceType: "slack", ConfigJSON: `{}`, Enabled: true,
	})

	list, err := ListServices(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 services, got %d", len(list))
	}
}

func TestListEnabledServices(t *testing.T) {
	db := setupTestDB(t)
	id := createTestService(t, db)

	// Disable the service
	svc, _ := GetService(db, id)
	svc.Enabled = false
	UpdateService(db, svc)

	CreateService(db, &NotificationService{
		Name: "enabled-one", ServiceType: "email", ConfigJSON: `{}`, Enabled: true,
	})

	list, err := ListEnabledServices(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 enabled service, got %d", len(list))
	}
}

func TestUpdateService(t *testing.T) {
	db := setupTestDB(t)
	id := createTestService(t, db)

	svc, _ := GetService(db, id)
	svc.Name = "renamed"
	svc.NotifyOnHealthy = true

	if err := UpdateService(db, svc); err != nil {
		t.Fatal(err)
	}

	updated, _ := GetService(db, id)
	if updated.Name != "renamed" {
		t.Errorf("name = %q, want %q", updated.Name, "renamed")
	}
	if !updated.NotifyOnHealthy {
		t.Error("expected notify_on_healthy after update")
	}
}

func TestDeleteService(t *testing.T) {
	db := setupTestDB(t)
	id := createTestService(t, db)

	if err := DeleteService(db, id); err != nil {
		t.Fatal(err)
	}

	svc, _ := GetService(db, id)
	if svc != nil {
		t.Error("expected nil after delete")
	}
}

func TestEventRuleCRUD(t *testing.T) {
	db := setupTestDB(t)
	svcID := createTestService(t, db)

	rule := &EventRule{
		ServiceID: svcID,
		EventType: "smart_warning",
		Enabled:   true,
		Cooldown:  600,
	}
	if err := UpsertEventRule(db, rule); err != nil {
		t.Fatal(err)
	}

	rules, err := GetEventRules(db, svcID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(rules))
	}
	if rules[0].Cooldown != 600 {
		t.Errorf("cooldown = %d, want 600", rules[0].Cooldown)
	}

	// Upsert update
	rule.Cooldown = 120
	if err := UpsertEventRule(db, rule); err != nil {
		t.Fatal(err)
	}
	rules, _ = GetEventRules(db, svcID)
	if rules[0].Cooldown != 120 {
		t.Errorf("cooldown after upsert = %d, want 120", rules[0].Cooldown)
	}
}

func TestQuietHoursCRUD(t *testing.T) {
	db := setupTestDB(t)
	svcID := createTestService(t, db)

	qh := &QuietHours{
		ServiceID: svcID,
		StartTime: "23:00",
		EndTime:   "06:00",
		Enabled:   true,
	}
	if err := UpsertQuietHours(db, qh); err != nil {
		t.Fatal(err)
	}

	got, err := GetQuietHours(db, svcID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected quiet hours")
	}
	if got.StartTime != "23:00" {
		t.Errorf("start = %q, want %q", got.StartTime, "23:00")
	}
	if !got.Enabled {
		t.Error("expected enabled")
	}
}

func TestDigestConfigCRUD(t *testing.T) {
	db := setupTestDB(t)
	svcID := createTestService(t, db)

	dc := &DigestConfig{
		ServiceID: svcID,
		Enabled:   true,
		SendAt:    "09:00",
	}
	if err := UpsertDigestConfig(db, dc); err != nil {
		t.Fatal(err)
	}

	got, err := GetDigestConfig(db, svcID)
	if err != nil {
		t.Fatal(err)
	}
	if got == nil {
		t.Fatal("expected digest config")
	}
	if got.SendAt != "09:00" {
		t.Errorf("send_at = %q, want %q", got.SendAt, "09:00")
	}
}

func TestRecordAndRecentHistory(t *testing.T) {
	db := setupTestDB(t)
	svcID := createTestService(t, db)

	rec := &NotificationRecord{
		SettingID: svcID,
		EventType: "smart_warning",
		Hostname:  "host1",
		Message:   "SMART warning on sda",
		Status:    "sent",
	}
	_, err := RecordNotification(db, rec)
	if err != nil {
		t.Fatal(err)
	}

	history, err := RecentHistory(db, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 record, got %d", len(history))
	}
	if history[0].Hostname != "host1" {
		t.Errorf("hostname = %q, want %q", history[0].Hostname, "host1")
	}
}
