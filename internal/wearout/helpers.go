package wearout

import "time"

const timeFormat = "2006-01-02 15:04:05"

func nowString() string {
	return time.Now().UTC().Format(timeFormat)
}

// parseDBTime parses a timestamp read from SQLite, tolerating both the bare
// `2006-01-02 15:04:05` layout and RFC3339 (`2006-01-02T15:04:05Z`). Parsing with
// only the bare layout silently failed on RFC3339 values and left the zero value
// (the `0001-01-01T00:00:00Z` bug, same class fixed for agents in #36).
func parseDBTime(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	for _, layout := range []string{time.RFC3339, timeFormat, "2006-01-02 15:04:05.999999999-07:00"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}

func clamp(val, min, max float64) float64 {
	if val < min {
		return min
	}
	if val > max {
		return max
	}
	return val
}

func safeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	return a / b
}
