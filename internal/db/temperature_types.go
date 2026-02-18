package db

import (
	"time"
)

// TemperaturePeriod represents a time period for statistics
type TemperaturePeriod string

const (
	Period24Hours TemperaturePeriod = "24h"
	Period7Days   TemperaturePeriod = "7d"
	Period30Days  TemperaturePeriod = "30d"
	PeriodAllTime TemperaturePeriod = "all"
)

// ParsePeriod converts a string to TemperaturePeriod with validation
func ParsePeriod(s string) TemperaturePeriod {
	switch s {
	case "24h", "1d":
		return Period24Hours
	case "7d", "1w":
		return Period7Days
	case "30d", "1m":
		return Period30Days
	case "all", "":
		return PeriodAllTime
	default:
		return Period24Hours // Default to 24h
	}
}

// PeriodToDuration converts a period to time.Duration
func PeriodToDuration(p TemperaturePeriod) time.Duration {
	switch p {
	case Period24Hours:
		return 24 * time.Hour
	case Period7Days:
		return 7 * 24 * time.Hour
	case Period30Days:
		return 30 * 24 * time.Hour
	case PeriodAllTime:
		return 365 * 24 * time.Hour * 10 // 10 years
	default:
		return 24 * time.Hour
	}
}

// AggregationInterval represents time intervals for data aggregation
type AggregationInterval string

const (
	IntervalHourly  AggregationInterval = "1h"
	Interval6Hours  AggregationInterval = "6h"
	IntervalDaily   AggregationInterval = "1d"
	IntervalWeekly  AggregationInterval = "1w"
	IntervalMonthly AggregationInterval = "1m"
)

// ParseInterval converts a string to AggregationInterval
func ParseInterval(s string) AggregationInterval {
	switch s {
	case "1h", "hour", "hourly":
		return IntervalHourly
	case "6h":
		return Interval6Hours
	case "1d", "day", "daily":
		return IntervalDaily
	case "1w", "week", "weekly":
		return IntervalWeekly
	case "1m", "month", "monthly":
		return IntervalMonthly
	default:
		return IntervalHourly
	}
}

// IntervalToSQLite returns SQLite strftime format for grouping
func IntervalToSQLite(i AggregationInterval) string {
	switch i {
	case IntervalHourly:
		return "%Y-%m-%d %H:00:00"
	case Interval6Hours:
		return "%Y-%m-%d %H:00:00" // Will need post-processing
	case IntervalDaily:
		return "%Y-%m-%d 00:00:00"
	case IntervalWeekly:
		return "%Y-%W" // Year-Week
	case IntervalMonthly:
		return "%Y-%m-01 00:00:00"
	default:
		return "%Y-%m-%d %H:00:00"
	}
}

// TemperatureStats holds statistical data for a drive's temperature
type TemperatureStats struct {
	Hostname     string    `json:"hostname"`
	SerialNumber string    `json:"serial_number"`
	DeviceName   string    `json:"device_name,omitempty"`
	Model        string    `json:"model,omitempty"`
	Period       string    `json:"period"`
	MinTemp      int       `json:"min_temp"`
	MaxTemp      int       `json:"max_temp"`
	AvgTemp      float64   `json:"avg_temp"`
	CurrentTemp  int       `json:"current_temp"`
	StdDev       float64   `json:"std_dev"`
	Variance     float64   `json:"variance"`
	DataPoints   int       `json:"data_points"`
	FirstReading time.Time `json:"first_reading"`
	LastReading  time.Time `json:"last_reading"`
	TrendSlope   float64   `json:"trend_slope"` // Positive = heating, negative = cooling
	TrendDesc    string    `json:"trend_desc"`  // "heating", "cooling", "stable"
}

// TempReading represents a single temperature reading from the database
// Note: If TemperatureRecord already exists in smart_db.go, use that instead
type TempReading struct {
	ID           int64     `json:"id,omitempty"`
	Hostname     string    `json:"hostname"`
	SerialNumber string    `json:"serial_number"`
	Temperature  int       `json:"temperature"`
	Timestamp    time.Time `json:"timestamp"`
}

// TimeSeriesPoint represents a single point in a time series
type TimeSeriesPoint struct {
	Timestamp   time.Time `json:"timestamp"`
	Temperature int       `json:"temperature"`
	MinTemp     int       `json:"min_temp,omitempty"`
	MaxTemp     int       `json:"max_temp,omitempty"`
	AvgTemp     float64   `json:"avg_temp,omitempty"`
	DataPoints  int       `json:"data_points,omitempty"`
}

// TimeSeriesData holds time series data for charting
type TimeSeriesData struct {
	Hostname     string            `json:"hostname"`
	SerialNumber string            `json:"serial_number"`
	DeviceName   string            `json:"device_name,omitempty"`
	Model        string            `json:"model,omitempty"`
	Period       string            `json:"period"`
	Interval     string            `json:"interval"`
	Points       []TimeSeriesPoint `json:"points"`
}

// CurrentTemperature represents the current temperature of a drive
type CurrentTemperature struct {
	Hostname     string    `json:"hostname"`
	SerialNumber string    `json:"serial_number"`
	DeviceName   string    `json:"device_name,omitempty"`
	Model        string    `json:"model,omitempty"`
	Temperature  int       `json:"temperature"`
	Timestamp    time.Time `json:"timestamp"`
	Status       string    `json:"status"` // "normal", "warning", "critical"
}

// TemperatureThresholds holds threshold values for status determination
type TemperatureThresholds struct {
	Warning  int `json:"warning"`
	Critical int `json:"critical"`
}

// DefaultThresholds returns the default temperature thresholds
func DefaultThresholds() TemperatureThresholds {
	return TemperatureThresholds{
		Warning:  45,
		Critical: 55,
	}
}

// GetStatus returns the temperature status based on thresholds
func (t TemperatureThresholds) GetStatus(temp int) string {
	if temp >= t.Critical {
		return "critical"
	}
	if temp >= t.Warning {
		return "warning"
	}
	return "normal"
}

// TemperatureSummary provides an overview of all drive temperatures
type TemperatureSummary struct {
	TotalDrives    int                  `json:"total_drives"`
	DrivesNormal   int                  `json:"drives_normal"`
	DrivesWarning  int                  `json:"drives_warning"`
	DrivesCritical int                  `json:"drives_critical"`
	AvgTemperature float64              `json:"avg_temperature"`
	MinTemperature int                  `json:"min_temperature"`
	MaxTemperature int                  `json:"max_temperature"`
	HottestDrive   *CurrentTemperature  `json:"hottest_drive,omitempty"`
	CoolestDrive   *CurrentTemperature  `json:"coolest_drive,omitempty"`
	Drives         []CurrentTemperature `json:"drives,omitempty"`
}

// TemperatureSpike represents a rapid temperature change event
type TemperatureSpike struct {
	ID           int64     `json:"id"`
	Hostname     string    `json:"hostname"`
	SerialNumber string    `json:"serial_number"`
	DeviceName   string    `json:"device_name,omitempty"`
	Model        string    `json:"model,omitempty"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	StartTemp    int       `json:"start_temp"`
	EndTemp      int       `json:"end_temp"`
	Change       int       `json:"change"`
	RatePerMin   float64   `json:"rate_per_min"`
	Direction    string    `json:"direction"` // "heating" or "cooling"
	Acknowledged bool      `json:"acknowledged"`
	CreatedAt    time.Time `json:"created_at"`
}

// HeatmapData holds data for temperature heatmap visualization
type HeatmapData struct {
	Period    string         `json:"period"`
	Interval  string         `json:"interval"`
	Drives    []HeatmapDrive `json:"drives"`
	TimeSlots []time.Time    `json:"time_slots"`
}

// HeatmapDrive holds heatmap data for a single drive
type HeatmapDrive struct {
	Hostname     string           `json:"hostname"`
	SerialNumber string           `json:"serial_number"`
	DeviceName   string           `json:"device_name,omitempty"`
	Model        string           `json:"model,omitempty"`
	Readings     []HeatmapReading `json:"readings"`
}

// HeatmapReading is a single cell in the heatmap
type HeatmapReading struct {
	Timestamp   time.Time `json:"timestamp"`
	Temperature int       `json:"temperature"`
	Status      string    `json:"status"` // For color coding
}
