// internal/wearout/trend_test.go
package wearout

import (
	"math"
	"testing"
	"time"
)

// ── Test helpers ────────────────────────────────────────────────────────────

func makeSnapshots(startPct float64, dailyIncrease float64, days int, startTime time.Time) []WearoutSnapshot {
	snapshots := make([]WearoutSnapshot, days)
	for i := range snapshots {
		snapshots[i] = WearoutSnapshot{
			Hostname:     "test-host",
			SerialNumber: "TEST-001",
			Percentage:   startPct + dailyIncrease*float64(i),
			Timestamp:    startTime.Add(time.Duration(i) * 24 * time.Hour),
		}
	}
	return snapshots
}

// ── PredictTrend Tests ──────────────────────────────────────────────────────

func TestPredictTrend_InsufficientData(t *testing.T) {
	tests := []struct {
		name  string
		count int
	}{
		{"empty", 0},
		{"one point", 1},
		{"two points", 2},
	}

	start := time.Now()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshots := makeSnapshots(10, 0.1, tt.count, start)
			pred := PredictTrend(snapshots)
			if pred != nil {
				t.Errorf("expected nil for %d data points, got %+v", tt.count, pred)
			}
		})
	}
}

func TestPredictTrend_StableDrive(t *testing.T) {
	// All snapshots at same percentage → slope ≈ 0
	start := time.Now().AddDate(0, 0, -100)
	snapshots := makeSnapshots(25.0, 0.0, 100, start)

	pred := PredictTrend(snapshots)

	if pred == nil {
		t.Fatal("expected prediction, got nil")
	}
	if pred.MonthsRemaining != nil {
		t.Errorf("stable drive should have nil MonthsRemaining, got %.2f", *pred.MonthsRemaining)
	}
	if math.Abs(pred.DailyRate) > 0.001 {
		t.Errorf("stable drive daily rate = %.6f, want ~0", pred.DailyRate)
	}
	if pred.Confidence != "high" {
		t.Errorf("100 days of data should be high confidence, got %q", pred.Confidence)
	}
}

func TestPredictTrend_IncreasingWearout(t *testing.T) {
	// 0.5% per day starting at 10%, 60 days of data
	start := time.Now().AddDate(0, 0, -60)
	snapshots := makeSnapshots(10.0, 0.5, 60, start)

	pred := PredictTrend(snapshots)

	if pred == nil {
		t.Fatal("expected prediction, got nil")
	}
	// Daily rate should be ~0.5
	assertApprox(t, "DailyRate", pred.DailyRate, 0.5, 0.01)

	// Current pct = 10 + 0.5*59 = 39.5. Remaining = 60.5. Days = 60.5/0.5 = 121. Months ≈ 3.97
	if pred.MonthsRemaining == nil {
		t.Fatal("expected MonthsRemaining, got nil")
	}
	assertApprox(t, "MonthsRemaining", *pred.MonthsRemaining, 3.97, 0.2)
	if pred.Confidence != "medium" {
		t.Errorf("60 days should be medium confidence, got %q", pred.Confidence)
	}
}

func TestPredictTrend_ImprovingDrive(t *testing.T) {
	// Negative slope: wearout decreasing (e.g. after replacing bad sectors)
	start := time.Now().AddDate(0, 0, -30)
	snapshots := makeSnapshots(50.0, -0.5, 30, start)

	pred := PredictTrend(snapshots)

	if pred == nil {
		t.Fatal("expected prediction, got nil")
	}
	if pred.MonthsRemaining != nil {
		t.Errorf("improving drive should have nil MonthsRemaining, got %.2f", *pred.MonthsRemaining)
	}
	if pred.DailyRate >= 0 {
		t.Errorf("improving drive should have negative daily rate, got %.4f", pred.DailyRate)
	}
}

func TestPredictTrend_NearFailure(t *testing.T) {
	// 95% wearout, increasing at 1%/day → ~5 days remaining
	start := time.Now().AddDate(0, 0, -10)
	snapshots := makeSnapshots(85.0, 1.0, 10, start)

	pred := PredictTrend(snapshots)

	if pred == nil {
		t.Fatal("expected prediction, got nil")
	}
	if pred.MonthsRemaining == nil {
		t.Fatal("expected MonthsRemaining, got nil")
	}
	// Current = 85 + 1*9 = 94. Remaining = 6. Days = 6. Months ≈ 0.2
	if *pred.MonthsRemaining > 1.0 {
		t.Errorf("near-failure drive should have < 1 month remaining, got %.2f", *pred.MonthsRemaining)
	}
}

// ── Confidence Tier Tests ───────────────────────────────────────────────────

func TestTrendConfidence(t *testing.T) {
	tests := []struct {
		days float64
		want string
	}{
		{5, "low"},
		{29, "low"},
		{30, "medium"},
		{60, "medium"},
		{89, "medium"},
		{90, "high"},
		{365, "high"},
	}

	for _, tt := range tests {
		got := trendConfidence(tt.days)
		if got != tt.want {
			t.Errorf("trendConfidence(%.0f) = %q, want %q", tt.days, got, tt.want)
		}
	}
}

// ── Linear Regression Tests ─────────────────────────────────────────────────

func TestLinearRegression_PerfectLine(t *testing.T) {
	// y = 2x + 5
	xs := []float64{0, 1, 2, 3, 4}
	ys := []float64{5, 7, 9, 11, 13}

	slope, intercept := linearRegression(xs, ys)

	assertApprox(t, "slope", slope, 2.0, 0.0001)
	assertApprox(t, "intercept", intercept, 5.0, 0.0001)
}

func TestLinearRegression_Flat(t *testing.T) {
	xs := []float64{0, 1, 2, 3}
	ys := []float64{10, 10, 10, 10}

	slope, intercept := linearRegression(xs, ys)

	assertApprox(t, "flat slope", slope, 0.0, 0.0001)
	assertApprox(t, "flat intercept", intercept, 10.0, 0.0001)
}

func TestLinearRegression_NegativeSlope(t *testing.T) {
	// y = -0.5x + 20
	xs := []float64{0, 10, 20, 30}
	ys := []float64{20, 15, 10, 5}

	slope, intercept := linearRegression(xs, ys)

	assertApprox(t, "negative slope", slope, -0.5, 0.0001)
	assertApprox(t, "negative intercept", intercept, 20.0, 0.0001)
}

func TestLinearRegression_Empty(t *testing.T) {
	slope, intercept := linearRegression([]float64{}, []float64{})

	if slope != 0 || intercept != 0 {
		t.Errorf("empty data: slope=%.4f, intercept=%.4f, want 0,0", slope, intercept)
	}
}

func TestLinearRegression_SinglePoint(t *testing.T) {
	slope, intercept := linearRegression([]float64{5}, []float64{42})

	// Single point: slope=0, intercept=value
	assertApprox(t, "single slope", slope, 0.0, 0.0001)
	assertApprox(t, "single intercept", intercept, 42.0, 0.0001)
}

func TestLinearRegression_NoisyData(t *testing.T) {
	// Roughly y = 1x + 0, with some noise
	xs := []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9}
	ys := []float64{0.5, 1.2, 1.8, 3.1, 3.9, 5.2, 5.8, 7.1, 8.3, 9.0}

	slope, _ := linearRegression(xs, ys)

	// Should be approximately 1.0
	assertApprox(t, "noisy slope", slope, 1.0, 0.1)
}
