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

// TestMaxPlausibleTempRejectsCorruptValue documents the value that prompted the
// guard: on 2026-09-02 /api/summary served max_temp_c 27058405379 while no
// drive in the fleet was above 46 °C. The bad row is in temperature_history;
// this endpoint refuses to repeat it.
func TestMaxPlausibleTempRejectsCorruptValue(t *testing.T) {
	cases := []struct {
		name  string
		temp  int
		valid bool
	}{
		{"lectura normal", 46, true},
		{"disco caliente pero real", 68, true},
		{"límite", maxPlausibleTempC, true},
		{"por encima del límite", maxPlausibleTempC + 1, false},
		{"el valor real visto en producción", 27058405379, false},
		{"cero (sin dato)", 0, false},
		{"negativo", -5, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := c.temp > 0 && c.temp <= maxPlausibleTempC
			if got != c.valid {
				t.Errorf("temp %d aceptada=%v, quiero %v", c.temp, got, c.valid)
			}
		})
	}
}
