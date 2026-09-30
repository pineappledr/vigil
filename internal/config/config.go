package config

import (
	"os"
	"strings"

	"vigil/internal/models"
)

// Load returns the server configuration from environment variables
func Load() models.Config {
	return models.Config{
		Port:        getEnv("PORT", "9080"),
		DBPath:      getEnv("DB_PATH", "vigil.db"),
		AdminUser:   getEnv("ADMIN_USER", "admin"),
		AdminPass:   getEnv("ADMIN_PASS", ""),
		AuthEnabled: getEnv("AUTH_ENABLED", "true") == "true",
		// Trimmed: a token pasted into an env file often carries a newline.
		MetricsToken: strings.TrimSpace(getEnv("VIGIL_METRICS_TOKEN", "")),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
