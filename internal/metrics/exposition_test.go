package metrics

import (
	"math"
	"strings"
	"testing"
)

func TestExpositionFormat(t *testing.T) {
	e := NewExposition()
	g := e.Gauge("vigil_drive_temperature_celsius", "Current drive temperature.")
	g.Add(34, "host", "brain", "serial", "ABC")
	g.Add(41.5, "host", "friday", "serial", "XYZ")
	c := e.Counter("vigil_reports_processed_total", "Reports processed.")
	c.Add(7)
	e.Gauge("vigil_empty", "Never written.")

	var b strings.Builder
	if _, err := e.WriteTo(&b); err != nil {
		t.Fatal(err)
	}
	want := `# HELP vigil_drive_temperature_celsius Current drive temperature.
# TYPE vigil_drive_temperature_celsius gauge
vigil_drive_temperature_celsius{host="brain",serial="ABC"} 34
vigil_drive_temperature_celsius{host="friday",serial="XYZ"} 41.5
# HELP vigil_reports_processed_total Reports processed.
# TYPE vigil_reports_processed_total counter
vigil_reports_processed_total 7
`
	if b.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", b.String(), want)
	}
}

func TestExpositionEscapesLabelValues(t *testing.T) {
	e := NewExposition()
	e.Gauge("m", "help with \\ and\nnewline").Add(1, "alias", "a \"quoted\" \\ name\nx")

	var b strings.Builder
	e.WriteTo(&b)
	out := b.String()
	if !strings.Contains(out, `m{alias="a \"quoted\" \\ name\nx"} 1`) {
		t.Errorf("label value not escaped:\n%s", out)
	}
	if !strings.Contains(out, `# HELP m help with \\ and\nnewline`) {
		t.Errorf("help not escaped:\n%s", out)
	}
}

func TestExpositionDropsDuplicateLabelSets(t *testing.T) {
	e := NewExposition()
	g := e.Gauge("m", "h")
	g.Add(1, "host", "a")
	g.Add(2, "host", "a") // a duplicate series would make Prometheus reject the scrape
	if g.Len() != 1 {
		t.Fatalf("Len = %d, want 1", g.Len())
	}
	var b strings.Builder
	e.WriteTo(&b)
	if strings.Count(b.String(), `m{host="a"}`) != 1 {
		t.Errorf("duplicate series written:\n%s", b.String())
	}
}

func TestExpositionOmitsEmptyLabels(t *testing.T) {
	e := NewExposition()
	e.Gauge("m", "h").Add(1, "host", "a", "alias", "")
	e.Gauge("n", "h").Add(1, "alias", "")
	var b strings.Builder
	e.WriteTo(&b)
	if !strings.Contains(b.String(), "m{host=\"a\"} 1\n") || !strings.Contains(b.String(), "\nn 1\n") {
		t.Errorf("empty labels not dropped:\n%s", b.String())
	}
}

func TestFormatValueSpecials(t *testing.T) {
	cases := map[float64]string{
		math.NaN():    "NaN",
		math.Inf(1):   "+Inf",
		math.Inf(-1):  "-Inf",
		1e21:          "1e+21",
		8001563222016: "8001563222016",
		41.5:          "41.5",
		-3:            "-3",
	}
	for in, want := range cases {
		if got := formatValue(in); got != want {
			t.Errorf("formatValue(%v) = %q, want %q", in, got, want)
		}
	}
}
