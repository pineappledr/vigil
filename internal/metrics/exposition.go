package metrics

import (
	"bufio"
	"io"
	"math"
	"strconv"
	"strings"
)

// Exposition builds a Prometheus text exposition (format 0.0.4) by hand.
//
// Vigil only needs gauges and a few counters with static label sets, so a
// ~100-line writer is cheaper than pulling in prometheus/client_golang and its
// dependency tree. It guarantees the two things a scraper rejects a page for:
// every family is declared once (HELP + TYPE before its samples), and no
// family contains the same label set twice.
type Exposition struct {
	families []*Family
}

// ContentType is the media type Prometheus expects for this format.
const ContentType = "text/plain; version=0.0.4; charset=utf-8"

// Family is one metric name with its samples.
type Family struct {
	name    string
	help    string
	typ     string
	samples []sample
	seen    map[string]bool
}

type sample struct {
	labels string // already rendered: {k="v",...} or ""
	value  float64
}

// NewExposition returns an empty exposition.
func NewExposition() *Exposition { return &Exposition{} }

// Gauge declares a gauge family. Families are written in declaration order.
func (e *Exposition) Gauge(name, help string) *Family { return e.family(name, help, "gauge") }

// Counter declares a counter family.
func (e *Exposition) Counter(name, help string) *Family { return e.family(name, help, "counter") }

func (e *Exposition) family(name, help, typ string) *Family {
	f := &Family{name: name, help: help, typ: typ, seen: map[string]bool{}}
	e.families = append(e.families, f)
	return f
}

// Add appends a sample. labels are key/value pairs: Add(1, "host", "brain").
// A label set already present in the family is ignored (first one wins), so a
// duplicate row in the database can never produce an unscrapable page.
// Labels with an empty value are dropped, as Prometheus treats them as absent.
func (f *Family) Add(value float64, labels ...string) {
	rendered := renderLabels(labels)
	if f.seen[rendered] {
		return
	}
	f.seen[rendered] = true
	f.samples = append(f.samples, sample{labels: rendered, value: value})
}

// Len returns the number of samples in the family.
func (f *Family) Len() int { return len(f.samples) }

// WriteTo writes the exposition. Families without samples are omitted.
func (e *Exposition) WriteTo(w io.Writer) (int64, error) {
	bw := bufio.NewWriter(w)
	cw := &countingWriter{w: bw}
	for _, f := range e.families {
		if len(f.samples) == 0 {
			continue
		}
		cw.str("# HELP " + f.name + " " + escapeHelp(f.help) + "\n")
		cw.str("# TYPE " + f.name + " " + f.typ + "\n")
		for _, s := range f.samples {
			cw.str(f.name + s.labels + " " + formatValue(s.value) + "\n")
		}
	}
	if cw.err != nil {
		return cw.n, cw.err
	}
	return cw.n, bw.Flush()
}

func renderLabels(kv []string) string {
	if len(kv) < 2 {
		return ""
	}
	var b strings.Builder
	first := true
	for i := 0; i+1 < len(kv); i += 2 {
		if kv[i+1] == "" {
			continue
		}
		if first {
			b.WriteByte('{')
			first = false
		} else {
			b.WriteByte(',')
		}
		b.WriteString(kv[i])
		b.WriteString(`="`)
		b.WriteString(escapeLabelValue(kv[i+1]))
		b.WriteByte('"')
	}
	if first {
		return ""
	}
	b.WriteByte('}')
	return b.String()
}

var labelEscaper = strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`)
var helpEscaper = strings.NewReplacer(`\`, `\\`, "\n", `\n`)

func escapeLabelValue(s string) string { return labelEscaper.Replace(s) }
func escapeHelp(s string) string       { return helpEscaper.Replace(s) }

func formatValue(v float64) string {
	switch {
	case math.IsNaN(v):
		return "NaN"
	case math.IsInf(v, 1):
		return "+Inf"
	case math.IsInf(v, -1):
		return "-Inf"
	}
	// Whole numbers (counters, bytes, Unix timestamps) print as integers:
	// "7988639170560" reads better than "7.98863917056e+12" and parses the same.
	if v == math.Trunc(v) && math.Abs(v) < 1e15 {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'g', -1, 64)
}

type countingWriter struct {
	w   io.Writer
	n   int64
	err error
}

func (c *countingWriter) str(s string) {
	if c.err != nil {
		return
	}
	n, err := io.WriteString(c.w, s)
	c.n += int64(n)
	c.err = err
}
