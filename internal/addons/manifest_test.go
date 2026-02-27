package addons

import (
	"strings"
	"testing"
)

func TestValidateManifest_Valid(t *testing.T) {
	raw := []byte(`{
		"name": "burnin-hub",
		"version": "1.0.0",
		"description": "Drive burn-in automation",
		"pages": [{
			"id": "overview",
			"title": "Overview",
			"components": [
				{"type": "progress", "id": "job-progress"},
				{"type": "log-viewer", "id": "job-log"}
			]
		}]
	}`)
	m, err := ValidateManifest(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Name != "burnin-hub" {
		t.Errorf("name = %q", m.Name)
	}
	if len(m.Pages) != 1 {
		t.Errorf("pages = %d", len(m.Pages))
	}
}

func TestValidateManifest_MissingName(t *testing.T) {
	raw := []byte(`{"version":"1.0","pages":[{"id":"p","title":"P","components":[]}]}`)
	_, err := ValidateManifest(raw)
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Errorf("expected name required error, got: %v", err)
	}
}

func TestValidateManifest_NoPages(t *testing.T) {
	raw := []byte(`{"name":"x","version":"1.0","pages":[]}`)
	_, err := ValidateManifest(raw)
	if err == nil || !strings.Contains(err.Error(), "at least one page") {
		t.Errorf("expected no pages error, got: %v", err)
	}
}

func TestValidateManifest_DuplicatePageID(t *testing.T) {
	raw := []byte(`{
		"name":"x","version":"1.0",
		"pages":[
			{"id":"dup","title":"A","components":[]},
			{"id":"dup","title":"B","components":[]}
		]
	}`)
	_, err := ValidateManifest(raw)
	if err == nil || !strings.Contains(err.Error(), "duplicate id") {
		t.Errorf("expected duplicate id error, got: %v", err)
	}
}

func TestValidateManifest_UnknownComponentType(t *testing.T) {
	raw := []byte(`{
		"name":"x","version":"1.0",
		"pages":[{"id":"p","title":"P","components":[
			{"type":"unknown","id":"c1"}
		]}]
	}`)
	_, err := ValidateManifest(raw)
	if err == nil || !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("expected unknown type error, got: %v", err)
	}
}

func TestValidateManifest_FormWithFields(t *testing.T) {
	raw := []byte(`{
		"name":"x","version":"1.0",
		"pages":[{"id":"p","title":"P","components":[
			{"type":"form","id":"f1","config":{"fields":[
				{"name":"target","label":"Target","type":"select"},
				{"name":"speed","label":"Speed","type":"number","live_calculation":"size * 1024"}
			]}}
		]}]
	}`)
	m, err := ValidateManifest(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m.Pages[0].Components) != 1 {
		t.Errorf("expected 1 component")
	}
}

func TestValidateManifest_UnsafeFormula(t *testing.T) {
	raw := []byte(`{
		"name":"x","version":"1.0",
		"pages":[{"id":"p","title":"P","components":[
			{"type":"form","id":"f1","config":{"fields":[
				{"name":"a","label":"A","type":"text","live_calculation":"eval('bad')"}
			]}}
		]}]
	}`)
	_, err := ValidateManifest(raw)
	if err == nil || !strings.Contains(err.Error(), "unsafe formula") {
		t.Errorf("expected unsafe formula error, got: %v", err)
	}
}

func TestValidateFormula_Safe(t *testing.T) {
	formulas := []string{
		"a + b",
		"size * 1024",
		"(a + b) / 2",
		"total_bytes * 100",
	}
	for _, f := range formulas {
		if err := ValidateFormula(f); err != nil {
			t.Errorf("ValidateFormula(%q) = %v", f, err)
		}
	}
}

func TestValidateFormula_Blocked(t *testing.T) {
	blocked := []string{
		"eval(x)",
		"window.location",
		"import('mod')",
		"constructor",
		"__proto__",
	}
	for _, f := range blocked {
		if err := ValidateFormula(f); err == nil {
			t.Errorf("ValidateFormula(%q) should fail", f)
		}
	}
}

func TestValidateFormula_BadChars(t *testing.T) {
	if err := ValidateFormula("a; rm -rf /"); err == nil {
		t.Error("expected error for semicolon")
	}
}

func TestEvalFormula(t *testing.T) {
	tests := []struct {
		expr string
		vars map[string]float64
		want float64
	}{
		{"a + b", map[string]float64{"a": 1, "b": 2}, 3},
		{"a * b + c", map[string]float64{"a": 3, "b": 4, "c": 5}, 17},
		{"(a + b) * 2", map[string]float64{"a": 3, "b": 7}, 20},
		{"100 / 4", nil, 25},
		{"size * 1024", map[string]float64{"size": 5}, 5120},
	}
	for _, tt := range tests {
		got, err := EvalFormula(tt.expr, tt.vars)
		if err != nil {
			t.Errorf("EvalFormula(%q) error: %v", tt.expr, err)
			continue
		}
		if got != tt.want {
			t.Errorf("EvalFormula(%q) = %f, want %f", tt.expr, got, tt.want)
		}
	}
}

func TestEvalFormula_DivByZero(t *testing.T) {
	_, err := EvalFormula("10 / 0", nil)
	if err == nil || !strings.Contains(err.Error(), "division by zero") {
		t.Errorf("expected division by zero, got: %v", err)
	}
}

func TestEvalFormula_UndefinedVar(t *testing.T) {
	_, err := EvalFormula("x + 1", nil)
	if err == nil || !strings.Contains(err.Error(), "undefined variable") {
		t.Errorf("expected undefined variable, got: %v", err)
	}
}

func TestValidateManifest_TooLarge(t *testing.T) {
	big := `{"name":"x","version":"1","pages":[{"id":"p","title":"t","components":[]}]}` + strings.Repeat(" ", maxManifestSize)
	_, err := ValidateManifest([]byte(big))
	if err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("expected size limit error, got: %v", err)
	}
}
