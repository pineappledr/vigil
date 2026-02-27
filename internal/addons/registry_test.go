package addons

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
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestRegisterAndGet(t *testing.T) {
	db := setupTestDB(t)

	id, err := Register(db, "burnin-hub", "1.0.0", "Drive burn-in", `{"pages":[]}`)
	if err != nil {
		t.Fatal(err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	a, err := Get(db, id)
	if err != nil {
		t.Fatal(err)
	}
	if a == nil {
		t.Fatal("expected addon, got nil")
	}
	if a.Name != "burnin-hub" {
		t.Errorf("name = %q, want %q", a.Name, "burnin-hub")
	}
	if a.Version != "1.0.0" {
		t.Errorf("version = %q, want %q", a.Version, "1.0.0")
	}
	if a.Status != StatusOnline {
		t.Errorf("status = %q, want %q", a.Status, StatusOnline)
	}
	if !a.Enabled {
		t.Error("expected enabled=true")
	}
}

func TestRegisterUpsert(t *testing.T) {
	db := setupTestDB(t)

	id1, _ := Register(db, "test-addon", "1.0.0", "v1", `{}`)
	id2, _ := Register(db, "test-addon", "2.0.0", "v2", `{"updated":true}`)

	if id1 != id2 {
		t.Errorf("upsert should return same id: %d != %d", id1, id2)
	}

	a, _ := Get(db, id1)
	if a.Version != "2.0.0" {
		t.Errorf("version not updated: %q", a.Version)
	}
}

func TestGetByName(t *testing.T) {
	db := setupTestDB(t)
	Register(db, "my-addon", "1.0.0", "", `{}`)

	a, err := GetByName(db, "my-addon")
	if err != nil {
		t.Fatal(err)
	}
	if a == nil || a.Name != "my-addon" {
		t.Fatal("expected addon by name")
	}

	missing, err := GetByName(db, "no-such")
	if err != nil {
		t.Fatal(err)
	}
	if missing != nil {
		t.Error("expected nil for missing addon")
	}
}

func TestList(t *testing.T) {
	db := setupTestDB(t)
	Register(db, "alpha", "1.0.0", "", `{}`)
	Register(db, "beta", "1.0.0", "", `{}`)

	list, err := List(db)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 addons, got %d", len(list))
	}
	if list[0].Name != "alpha" {
		t.Errorf("expected alpha first, got %q", list[0].Name)
	}
}

func TestUpdateStatusAndSetEnabled(t *testing.T) {
	db := setupTestDB(t)
	id, _ := Register(db, "test", "1.0.0", "", `{}`)

	if err := UpdateStatus(db, id, StatusDegraded); err != nil {
		t.Fatal(err)
	}
	a, _ := Get(db, id)
	if a.Status != StatusDegraded {
		t.Errorf("status = %q, want %q", a.Status, StatusDegraded)
	}

	if err := SetEnabled(db, id, false); err != nil {
		t.Fatal(err)
	}
	a, _ = Get(db, id)
	if a.Enabled {
		t.Error("expected enabled=false")
	}
}

func TestDeregister(t *testing.T) {
	db := setupTestDB(t)
	id, _ := Register(db, "to-delete", "1.0.0", "", `{}`)

	if err := Deregister(db, id); err != nil {
		t.Fatal(err)
	}

	a, _ := Get(db, id)
	if a != nil {
		t.Error("expected nil after deregister")
	}
}

func TestDeregisterNotFound(t *testing.T) {
	db := setupTestDB(t)

	err := Deregister(db, 9999)
	if err == nil {
		t.Error("expected error for missing addon")
	}
}
