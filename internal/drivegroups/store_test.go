package drivegroups

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
	db.Exec("PRAGMA foreign_keys = ON")

	// Create dependency table for foreign keys
	db.Exec(`CREATE TABLE IF NOT EXISTS notification_settings (
		id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, service_type TEXT, config_json TEXT)`)

	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestGroupCRUD(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Create
	g := &DriveGroup{Name: "Production", Color: "#00ff00"}
	id, err := CreateGroup(db, g)
	if err != nil {
		t.Fatalf("CreateGroup: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero ID")
	}

	// List
	groups, err := ListGroups(db)
	if err != nil {
		t.Fatalf("ListGroups: %v", err)
	}
	if len(groups) != 1 || groups[0].Name != "Production" {
		t.Fatalf("expected 1 group 'Production', got %v", groups)
	}

	// Get
	got, err := GetGroup(db, id)
	if err != nil {
		t.Fatalf("GetGroup: %v", err)
	}
	if got.Color != "#00ff00" {
		t.Fatalf("expected color #00ff00, got %s", got.Color)
	}

	// Update
	got.Name = "Critical"
	got.Color = "#ff0000"
	if err := UpdateGroup(db, got); err != nil {
		t.Fatalf("UpdateGroup: %v", err)
	}
	got2, _ := GetGroup(db, id)
	if got2.Name != "Critical" {
		t.Fatalf("expected name Critical, got %s", got2.Name)
	}

	// Delete
	if err := DeleteGroup(db, id); err != nil {
		t.Fatalf("DeleteGroup: %v", err)
	}
	groups, _ = ListGroups(db)
	if len(groups) != 0 {
		t.Fatalf("expected 0 groups after delete, got %d", len(groups))
	}
}

func TestDuplicateGroupName(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	CreateGroup(db, &DriveGroup{Name: "Backup"})
	_, err := CreateGroup(db, &DriveGroup{Name: "Backup"})
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
}

func TestMemberAssignment(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	gid, _ := CreateGroup(db, &DriveGroup{Name: "Production"})

	// Assign drive
	if err := AssignDrive(db, gid, "server1", "ABC123"); err != nil {
		t.Fatalf("AssignDrive: %v", err)
	}

	// Check membership
	groupID, err := GetDriveGroup(db, "server1", "ABC123")
	if err != nil {
		t.Fatalf("GetDriveGroup: %v", err)
	}
	if groupID == nil || *groupID != gid {
		t.Fatalf("expected group %d, got %v", gid, groupID)
	}

	// List members
	members, err := ListGroupMembers(db, gid)
	if err != nil {
		t.Fatalf("ListGroupMembers: %v", err)
	}
	if len(members) != 1 || members[0].SerialNumber != "ABC123" {
		t.Fatalf("expected 1 member, got %v", members)
	}

	// Member count in ListGroups
	groups, _ := ListGroups(db)
	if groups[0].MemberCount != 1 {
		t.Fatalf("expected member_count=1, got %d", groups[0].MemberCount)
	}

	// Unassign
	if err := UnassignDrive(db, "server1", "ABC123"); err != nil {
		t.Fatalf("UnassignDrive: %v", err)
	}
	groupID, _ = GetDriveGroup(db, "server1", "ABC123")
	if groupID != nil {
		t.Fatal("expected nil after unassign")
	}
}

func TestDriveMovesBetweenGroups(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	g1, _ := CreateGroup(db, &DriveGroup{Name: "Group1"})
	g2, _ := CreateGroup(db, &DriveGroup{Name: "Group2"})

	AssignDrive(db, g1, "host", "serial1")
	// Move to g2 (upsert)
	AssignDrive(db, g2, "host", "serial1")

	groupID, _ := GetDriveGroup(db, "host", "serial1")
	if groupID == nil || *groupID != g2 {
		t.Fatalf("expected group %d after move, got %v", g2, groupID)
	}
}

func TestCascadeDeleteGroup(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	gid, _ := CreateGroup(db, &DriveGroup{Name: "ToDelete"})
	AssignDrive(db, gid, "h1", "s1")

	// Add a notification service for group rules FK
	db.Exec(`INSERT INTO notification_settings (name, service_type, config_json) VALUES ('test', 'discord', '{}')`)
	UpsertGroupEventRule(db, &GroupEventRule{ServiceID: 1, GroupID: gid, EventType: "smart_critical", Enabled: true, Cooldown: 3600})

	// Delete group — should cascade
	DeleteGroup(db, gid)

	members, _ := ListGroupMembers(db, gid)
	if len(members) != 0 {
		t.Fatal("expected members cascade-deleted")
	}

	rules, _ := GetGroupEventRules(db, 1, gid)
	if len(rules) != 0 {
		t.Fatal("expected group rules cascade-deleted")
	}
}

func TestListAllAssignments(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	g1, _ := CreateGroup(db, &DriveGroup{Name: "A"})
	g2, _ := CreateGroup(db, &DriveGroup{Name: "B"})
	AssignDrive(db, g1, "h1", "s1")
	AssignDrive(db, g2, "h2", "s2")

	m, err := ListAllAssignments(db)
	if err != nil {
		t.Fatalf("ListAllAssignments: %v", err)
	}
	if len(m) != 2 {
		t.Fatalf("expected 2 assignments, got %d", len(m))
	}
	if m["h1:s1"] != g1 || m["h2:s2"] != g2 {
		t.Fatalf("unexpected assignments: %v", m)
	}
}

func TestGroupEventRules(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	gid, _ := CreateGroup(db, &DriveGroup{Name: "Prod"})
	db.Exec(`INSERT INTO notification_settings (name, service_type, config_json) VALUES ('telegram', 'telegram', '{}')`)

	// Upsert rule
	r := &GroupEventRule{ServiceID: 1, GroupID: gid, EventType: "smart_critical", Enabled: true, Cooldown: 3600}
	if err := UpsertGroupEventRule(db, r); err != nil {
		t.Fatalf("UpsertGroupEventRule: %v", err)
	}

	// Get rules
	rules, err := GetGroupEventRules(db, 1, gid)
	if err != nil {
		t.Fatalf("GetGroupEventRules: %v", err)
	}
	if len(rules) != 1 || rules[0].Cooldown != 3600 {
		t.Fatalf("expected 1 rule with 3600s cooldown, got %v", rules)
	}

	// Update via upsert
	r.Cooldown = 86400
	UpsertGroupEventRule(db, r)
	rules, _ = GetGroupEventRules(db, 1, gid)
	if rules[0].Cooldown != 86400 {
		t.Fatalf("expected 86400 after upsert, got %d", rules[0].Cooldown)
	}

	// GetAll for service
	all, err := GetAllGroupRulesForService(db, 1)
	if err != nil {
		t.Fatalf("GetAllGroupRulesForService: %v", err)
	}
	if len(all[gid]) != 1 {
		t.Fatalf("expected 1 rule for group, got %v", all)
	}

	// Delete group rules
	DeleteGroupRules(db, 1, gid)
	rules, _ = GetGroupEventRules(db, 1, gid)
	if len(rules) != 0 {
		t.Fatal("expected 0 rules after delete")
	}
}

func TestUnassignedDriveReturnsNil(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	gid, _ := GetDriveGroup(db, "nonexistent", "drive")
	if gid != nil {
		t.Fatal("expected nil for unassigned drive")
	}
}
