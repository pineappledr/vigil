package temperature

import (
	"database/sql"
	"log"
	"sync"
	"time"

	"vigil/internal/settings"
)

// Processor handles temperature data processing and alert generation
type Processor struct {
	DB              *sql.DB
	mu              sync.Mutex
	running         bool
	stopChan        chan struct{}
	processingQueue chan processingRequest
}

type processingRequest struct {
	Hostname     string
	SerialNumber string
	Temperature  int
	Timestamp    time.Time
}

// NewProcessor creates a new temperature processor
func NewProcessor(database *sql.DB) *Processor {
	return &Processor{
		DB:              database,
		stopChan:        make(chan struct{}),
		processingQueue: make(chan processingRequest, 100),
	}
}

// Start begins background processing of temperature data
func (p *Processor) Start() {
	p.mu.Lock()
	if p.running {
		p.mu.Unlock()
		return
	}
	p.running = true
	p.mu.Unlock()

	// Start the processing worker
	go p.processWorker()

	// Start periodic tasks
	go p.periodicTasks()

	log.Println("[Temperature] Processor started")
}

// Stop halts the temperature processor
func (p *Processor) Stop() {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return
	}
	p.running = false
	p.mu.Unlock()

	close(p.stopChan)
	log.Println("[Temperature] Processor stopped")
}

// ProcessReading queues a temperature reading for processing
func (p *Processor) ProcessReading(hostname, serial string, temperature int) {
	select {
	case p.processingQueue <- processingRequest{
		Hostname:     hostname,
		SerialNumber: serial,
		Temperature:  temperature,
		Timestamp:    time.Now(),
	}:
	default:
		// Queue full, process synchronously
		p.processReadingSync(hostname, serial, temperature)
	}
}

// processWorker handles queued temperature readings
func (p *Processor) processWorker() {
	for {
		select {
		case <-p.stopChan:
			return
		case req := <-p.processingQueue:
			p.processReadingSync(req.Hostname, req.SerialNumber, req.Temperature)
		}
	}
}

// processReadingSync processes a single temperature reading
func (p *Processor) processReadingSync(hostname, serial string, temperature int) {
	alerts, err := ProcessTemperatureReading(p.DB, hostname, serial, temperature)
	if err != nil {
		log.Printf("[Temperature] Processing error for %s/%s: %v", hostname, serial, err)
		return
	}

	for _, alert := range alerts {
		log.Printf("[Temperature] Alert: %s - %s (%s/%s)",
			alert.AlertType, alert.Message, hostname, serial)
	}
}

// periodicTasks runs scheduled maintenance tasks
func (p *Processor) periodicTasks() {
	// Run cleanup daily at startup, then every 24 hours
	cleanupTicker := time.NewTicker(24 * time.Hour)
	defer cleanupTicker.Stop()

	// Run spike detection every 15 minutes
	spikeTicker := time.NewTicker(15 * time.Minute)
	defer spikeTicker.Stop()

	// Initial cleanup on startup (delayed by 1 minute)
	time.AfterFunc(1*time.Minute, func() {
		p.runCleanup()
	})

	for {
		select {
		case <-p.stopChan:
			return
		case <-cleanupTicker.C:
			p.runCleanup()
		case <-spikeTicker.C:
			p.runSpikeDetection()
		}
	}
}

// runCleanup removes old temperature data based on retention settings
func (p *Processor) runCleanup() {
	retentionDays := settings.GetIntSettingWithDefault(p.DB, "temperature", "retention_days", 90)

	// Cleanup temperature history
	deleted, err := CleanupOldTemperatureData(p.DB, retentionDays)
	if err != nil {
		log.Printf("[Temperature] Cleanup error: %v", err)
	} else if deleted > 0 {
		log.Printf("[Temperature] Cleaned up %d old temperature records", deleted)
	}

	// Cleanup old spikes
	deleted, err = CleanupOldSpikes(p.DB, retentionDays)
	if err != nil {
		log.Printf("[Temperature] Spike cleanup error: %v", err)
	} else if deleted > 0 {
		log.Printf("[Temperature] Cleaned up %d old spike records", deleted)
	}

	// Cleanup old alerts
	alertRetention := settings.GetIntSettingWithDefault(p.DB, "system", "data_retention_days", 365)
	deleted, err = CleanupOldAlerts(p.DB, alertRetention)
	if err != nil {
		log.Printf("[Temperature] Alert cleanup error: %v", err)
	} else if deleted > 0 {
		log.Printf("[Temperature] Cleaned up %d old alert records", deleted)
	}
}

// runSpikeDetection runs spike detection for all drives
func (p *Processor) runSpikeDetection() {
	spikes, err := DetectAllDrivesSpikes(p.DB)
	if err != nil {
		log.Printf("[Temperature] Spike detection error: %v", err)
		return
	}

	if len(spikes) > 0 {
		log.Printf("[Temperature] Detected %d new spikes", len(spikes))

		// Create alerts for detected spikes
		for i := range spikes {
			_, err := CreateSpikeAlert(p.DB, &spikes[i])
			if err != nil {
				log.Printf("[Temperature] Failed to create spike alert: %v", err)
			}
		}
	}
}

// GetStatus returns the current processor status
func (p *Processor) GetStatus() map[string]interface{} {
	p.mu.Lock()
	running := p.running
	queueLen := len(p.processingQueue)
	p.mu.Unlock()

	return map[string]interface{}{
		"running":        running,
		"queue_length":   queueLen,
		"queue_capacity": cap(p.processingQueue),
	}
}
