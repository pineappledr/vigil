package metrics

import (
	"math"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

// Collector gathers lightweight in-process metrics.
// Counters use sync/atomic; the latency ring buffer uses a mutex.
type Collector struct {
	ReportsProcessed    atomic.Int64
	ReportsDropped      atomic.Int64
	NotificationsSent   atomic.Int64
	NotificationsFailed atomic.Int64

	mu              sync.Mutex
	reportLatencies []float64 // ring buffer, last 100 entries (ms)
	latencyIdx      int
	startTime       time.Time
}

// New creates a new Collector.
func New() *Collector {
	return &Collector{
		reportLatencies: make([]float64, 0, 100),
		startTime:       time.Now(),
	}
}

// RecordReportLatency adds a report processing duration to the ring buffer.
func (c *Collector) RecordReportLatency(d time.Duration) {
	ms := float64(d.Microseconds()) / 1000.0
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.reportLatencies) < 100 {
		c.reportLatencies = append(c.reportLatencies, ms)
	} else {
		c.reportLatencies[c.latencyIdx%100] = ms
	}
	c.latencyIdx++
}

// Snapshot returns current metrics as a JSON-friendly map.
func (c *Collector) Snapshot(queueDepth int, activeAgents int, dbSizeBytes int64) map[string]interface{} {
	c.mu.Lock()
	latencies := make([]float64, len(c.reportLatencies))
	copy(latencies, c.reportLatencies)
	c.mu.Unlock()

	stats := map[string]interface{}{
		"uptime_seconds":            int(time.Since(c.startTime).Seconds()),
		"report_queue_depth":        queueDepth,
		"reports_processed_total":   c.ReportsProcessed.Load(),
		"reports_dropped_total":     c.ReportsDropped.Load(),
		"active_agents":             activeAgents,
		"notifications_sent_total":  c.NotificationsSent.Load(),
		"notifications_failed_total": c.NotificationsFailed.Load(),
		"db_size_bytes":             dbSizeBytes,
	}

	if len(latencies) > 0 {
		sort.Float64s(latencies)
		sum := 0.0
		for _, v := range latencies {
			sum += v
		}
		p95Idx := int(math.Ceil(float64(len(latencies))*0.95)) - 1
		if p95Idx < 0 {
			p95Idx = 0
		}
		stats["report_processing_ms"] = map[string]interface{}{
			"min":     math.Round(latencies[0]*100) / 100,
			"max":     math.Round(latencies[len(latencies)-1]*100) / 100,
			"avg":     math.Round(sum/float64(len(latencies))*100) / 100,
			"p95":     math.Round(latencies[p95Idx]*100) / 100,
			"samples": len(latencies),
		}
	}

	return stats
}
