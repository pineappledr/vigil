package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vigil/internal/config"
	"vigil/internal/db"
	"vigil/internal/handlers"
	"vigil/internal/middleware"
	"vigil/internal/models"
)

// version is set at build time via -ldflags
var version = "dev"

func main() {
	log.SetFlags(log.Ltime | log.Ldate)
	log.Printf("🚀 Vigil Server v%s starting...", version)

	// Set version for handlers
	handlers.Version = version

	cfg := config.Load()

	if err := db.Init(cfg.DBPath, cfg); err != nil {
		log.Fatalf("❌ Database error: %v", err)
	}
	defer db.DB.Close()
	log.Printf("✓ Database: %s", cfg.DBPath)

	// Run SMART attributes migration
	if err := db.MigrateSmartAttributes(db.DB); err != nil {
		log.Printf("⚠️  SMART migration warning: %v", err)
	}

	// Run extended schema migrations
	if err := db.MigrateSchemaExtensions(db.DB); err != nil {
		log.Printf("⚠️  Schema migration warning: %v", err)
	}

	if cfg.AuthEnabled {
		log.Printf("✓ Authentication: enabled")
	} else {
		log.Printf("⚠️  Authentication: disabled (set AUTH_ENABLED=true to enable)")
	}

	mux := setupRoutes(cfg)
	handler := middleware.Logging(middleware.CORS(mux))

	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go gracefulShutdown(server)

	log.Printf("✓ Listening on port %s", cfg.Port)
	log.Printf("🌐 Dashboard: http://localhost:%s", cfg.Port)

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("❌ Server error: %v", err)
	}

	log.Println("👋 Server stopped")
}

func setupRoutes(cfg models.Config) *http.ServeMux {
	mux := http.NewServeMux()
	auth := func(h http.HandlerFunc) http.HandlerFunc {
		return middleware.Auth(cfg, h)
	}

	// Public endpoints
	mux.HandleFunc("GET /health", handlers.Health)
	mux.HandleFunc("GET /api/version", handlers.GetVersion)
	mux.HandleFunc("GET /api/auth/status", handlers.AuthStatus(cfg))

	// Agent endpoint (no auth)
	mux.HandleFunc("POST /api/report", handlers.Report)

	// Auth endpoints
	mux.HandleFunc("POST /api/auth/login", handlers.Login(cfg))
	mux.HandleFunc("POST /api/auth/logout", handlers.Logout)

	// Protected endpoints
	mux.HandleFunc("GET /api/history", auth(handlers.History))
	mux.HandleFunc("GET /api/hosts", auth(handlers.Hosts))
	mux.HandleFunc("DELETE /api/hosts/{hostname}", auth(handlers.DeleteHost))
	mux.HandleFunc("GET /api/hosts/{hostname}/history", auth(handlers.HostHistory))

	// Alias endpoints
	mux.HandleFunc("GET /api/aliases", auth(handlers.GetAliases))
	mux.HandleFunc("POST /api/aliases", auth(handlers.SetAlias))
	mux.HandleFunc("DELETE /api/aliases/{id}", auth(handlers.DeleteAlias))

	// User endpoints
	mux.HandleFunc("GET /api/users/me", auth(handlers.GetCurrentUser))
	mux.HandleFunc("POST /api/users/password", auth(handlers.ChangePassword))
	mux.HandleFunc("POST /api/users/username", auth(handlers.ChangeUsername))

	// ─── SMART Attributes API (Phase 1.2) ────────────────────────────
	// Get latest SMART attributes for a drive
	mux.HandleFunc("GET /api/smart/attributes", auth(handlers.GetSmartAttributes))

	// Get historical data for a specific attribute
	mux.HandleFunc("GET /api/smart/attributes/history", auth(handlers.GetSmartAttributeHistory))

	// Get trend analysis for an attribute
	mux.HandleFunc("GET /api/smart/attributes/trend", auth(handlers.GetSmartAttributeTrend))

	// Get health summary for a drive
	mux.HandleFunc("GET /api/smart/health/summary", auth(handlers.GetDriveHealthSummary))

	// Get health summaries for all drives
	mux.HandleFunc("GET /api/smart/health/all", auth(handlers.GetAllDrivesHealthSummary))

	// Get drives with issues (warnings/critical)
	mux.HandleFunc("GET /api/smart/health/issues", auth(handlers.GetDrivesWithIssues))

	// Get critical attribute definitions
	mux.HandleFunc("GET /api/smart/critical-attributes", auth(handlers.GetCriticalAttributes))

	// Get temperature history
	mux.HandleFunc("GET /api/smart/temperature/history", auth(handlers.GetTemperatureHistory))

	// Admin: cleanup old SMART data
	mux.HandleFunc("POST /api/smart/cleanup", auth(handlers.CleanupOldSmartData))

	// Static files
	mux.HandleFunc("/", handlers.StaticFiles(cfg))

	return mux
}

func gracefulShutdown(server *http.Server) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("\n⏹️  Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("⚠️  Shutdown error: %v", err)
	}
}
