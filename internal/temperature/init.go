package temperature

import (
	"database/sql"
	"log"

	"vigil/internal/db"
	"vigil/internal/settings"
)

// InitializeTables creates all temperature-related database tables
func InitializeTables(database *sql.DB) error {
	log.Println("[Temperature] Initializing database tables...")

	// Initialize settings table (if not already done)
	if err := settings.InitSettingsTable(database); err != nil {
		return err
	}

	// Initialize temperature spikes table
	if err := db.InitTemperatureSpikesTable(database); err != nil {
		return err
	}

	// Initialize temperature alerts table
	if err := db.InitTemperatureAlertsTable(database); err != nil {
		return err
	}

	// Create additional indexes for temperature_history if needed
	_, err := database.Exec(`
		CREATE INDEX IF NOT EXISTS idx_temp_hist_combined 
			ON temperature_history(hostname, serial_number, timestamp);
	`)
	if err != nil {
		log.Printf("[Temperature] Warning: Could not create combined index: %v", err)
		// Non-fatal, continue
	}

	log.Println("[Temperature] Database tables initialized")
	return nil
}

// ExtractTemperatureFromSMART extracts temperature from SMART attributes
// Works with both ATA and NVMe drives
func ExtractTemperatureFromSMART(attrs map[string]interface{}) int {
	// Try common temperature attribute names
	tempKeys := []string{
		"Temperature_Celsius",     // Common ATA
		"temperature",             // NVMe
		"Airflow_Temperature_Cel", // Some drives
		"Temperature_Case",        // Some enterprise drives
		"Temperature_Internal",    // Some SSDs
	}

	for _, key := range tempKeys {
		if val, ok := attrs[key]; ok {
			switch v := val.(type) {
			case int:
				return v
			case int64:
				return int(v)
			case float64:
				return int(v)
			case map[string]interface{}:
				// Handle nested structure like {"raw": {"value": 35}}
				if raw, ok := v["raw"].(map[string]interface{}); ok {
					if rawVal, ok := raw["value"]; ok {
						switch rv := rawVal.(type) {
						case int:
							return rv
						case int64:
							return int(rv)
						case float64:
							return int(rv)
						}
					}
				}
				// Handle {"value": 35} structure
				if rawVal, ok := v["value"]; ok {
					switch rv := rawVal.(type) {
					case int:
						return rv
					case int64:
						return int(rv)
					case float64:
						return int(rv)
					}
				}
			}
		}
	}

	// Try NVMe temperature sensors
	if tempSensors, ok := attrs["temperature_sensors"].([]interface{}); ok {
		if len(tempSensors) > 0 {
			if temp, ok := tempSensors[0].(float64); ok {
				return int(temp)
			}
		}
	}

	return 0 // No temperature found
}

// ValidateTemperature checks if a temperature reading is reasonable
func ValidateTemperature(temp int) bool {
	// Reasonable temperature range: -40°C to 100°C
	return temp >= -40 && temp <= 100
}

// ProcessDriveTemperature is a convenience function to process a single drive's temperature
// Call this from your report handler after storing SMART data
func ProcessDriveTemperature(database *sql.DB, hostname, serial string, temperature int) error {
	if !ValidateTemperature(temperature) {
		return nil // Invalid temperature, skip silently
	}

	alerts, err := db.ProcessTemperatureReading(database, hostname, serial, temperature)
	if err != nil {
		return err
	}

	for _, alert := range alerts {
		log.Printf("[Temperature] Alert generated: %s - %s (%s/%s)",
			alert.AlertType, alert.Message, hostname, serial)
	}

	return nil
}
