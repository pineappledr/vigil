package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"vigil/internal/db"
)

// TestGetSummaryIsUnauthenticated is the point of the endpoint: Homepage's
// customapi widget cannot do Vigil's cookie login, so this must answer without
// one. If someone later wraps it in protect(), this fails.
func TestGetSummaryIsUnauthenticated(t *testing.T) {
	if err := db.Init(filepath.Join(t.TempDir(), "t.db")); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	GetSummary(rec, httptest.NewRequest(http.MethodGet, "/api/summary", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (no auth)", rec.Code)
	}

	var s Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &s); err != nil {
		t.Fatalf("body is not a Summary: %v", err)
	}
	if s.Status == "" {
		t.Error("status is empty; a dashboard has nothing to colour on")
	}
}

// TestGetSummaryLeaksNoIdentifiers guards the tradeoff that made it acceptable
// to serve this without auth: counts only. A hostname, serial or model showing
// up here would turn an unauthenticated endpoint into an inventory disclosure.
func TestGetSummaryLeaksNoIdentifiers(t *testing.T) {
	if err := db.Init(filepath.Join(t.TempDir(), "t.db")); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	GetSummary(rec, httptest.NewRequest(http.MethodGet, "/api/summary", nil))

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}

	// Walk every key at every depth; none may name a machine or a disk.
	forbidden := []string{"hostname", "serial", "model", "dataset", "pool_name", "device"}
	var walk func(prefix string, v any)
	walk = func(prefix string, v any) {
		m, ok := v.(map[string]any)
		if !ok {
			return
		}
		for k, sub := range m {
			for _, bad := range forbidden {
				if strings.Contains(strings.ToLower(k), bad) {
					t.Errorf("key %q%s exposes an identifier on an unauthenticated endpoint", k, prefix)
				}
			}
			walk(prefix+"."+k, sub)
		}
	}
	walk("", raw)
}
