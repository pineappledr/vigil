package config

import (
	"os"

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
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
