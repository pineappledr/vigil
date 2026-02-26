package wearout

import "math"

// PredictTrend computes a linear regression on wearout snapshots and projects
// time remaining until 100% wearout. Returns nil if insufficient data.
func PredictTrend(snapshots []WearoutSnapshot) *TrendPrediction {
	if len(snapshots) < 3 {
		return nil
	}

	// Build x (days since first snapshot) and y (percentage) arrays
	first := snapshots[0].Timestamp
	xs := make([]float64, len(snapshots))
	ys := make([]float64, len(snapshots))
	for i, s := range snapshots {
		xs[i] = s.Timestamp.Sub(first).Hours() / 24.0
		ys[i] = s.Percentage
	}

	slope, _ := linearRegression(xs, ys)

	dataSpanDays := xs[len(xs)-1]
	confidence := trendConfidence(dataSpanDays)
	currentPct := snapshots[len(snapshots)-1].Percentage

	pred := &TrendPrediction{
		DailyRate:  slope,
		Confidence: confidence,
	}

	// Only project remaining time if wearout is actually increasing
	if slope > 0.0001 {
		daysRemaining := (100.0 - currentPct) / slope
		months := daysRemaining / 30.44
		if months > 0 {
			pred.MonthsRemaining = &months
		}
	}

	return pred
}

// linearRegression computes slope and intercept for y = slope*x + intercept.
func linearRegression(xs, ys []float64) (slope, intercept float64) {
	n := float64(len(xs))
	if n == 0 {
		return 0, 0
	}

	var sumX, sumY, sumXY, sumX2 float64
	for i := range xs {
		sumX += xs[i]
		sumY += ys[i]
		sumXY += xs[i] * ys[i]
		sumX2 += xs[i] * xs[i]
	}

	denom := n*sumX2 - sumX*sumX
	if math.Abs(denom) < 1e-10 {
		return 0, sumY / n
	}

	slope = (n*sumXY - sumX*sumY) / denom
	intercept = (sumY - slope*sumX) / n
	return slope, intercept
}

func trendConfidence(dataSpanDays float64) string {
	switch {
	case dataSpanDays >= 90:
		return "high"
	case dataSpanDays >= 30:
		return "medium"
	default:
		return "low"
	}
}
