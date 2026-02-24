package wearout

import "time"

const timeFormat = "2006-01-02 15:04:05"

func nowString() string {
	return time.Now().UTC().Format(timeFormat)
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
