package addons

import (
	"testing"
	"time"
)

// TestParseTime guards against the `0001-01-01T00:00:00Z` regression where
// addon created_at/updated_at were returned as zero-time because the parser only
// accepted the bare SQLite layout and silently failed on RFC3339 values.
func TestParseTime(t *testing.T) {
	cases := []struct {
		name  string
		in    string
		zero  bool
		check string // expected RFC3339 (UTC) when not zero
	}{
		{"bare sqlite layout", "2026-04-09 17:20:26", false, "2026-04-09T17:20:26Z"},
		{"rfc3339 with Z", "2026-04-09T17:20:26Z", false, "2026-04-09T17:20:26Z"},
		{"rfc3339 with offset", "2026-04-09T12:20:26-05:00", false, "2026-04-09T17:20:26Z"},
		{"empty string", "", true, ""},
		{"garbage", "not-a-date", true, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := parseTime(c.in)
			if c.zero {
				if !got.IsZero() {
					t.Fatalf("expected zero time, got %v", got)
				}
				return
			}
			if got.IsZero() {
				t.Fatalf("got zero time for %q (the 0001-01-01 bug)", c.in)
			}
			if got.UTC().Format(time.RFC3339) != c.check {
				t.Fatalf("got %s, want %s", got.UTC().Format(time.RFC3339), c.check)
			}
		})
	}
}
