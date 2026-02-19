package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vigil/internal/auth"
	"vigil/internal/config"
	"vigil/internal/db"
	"vigil/internal/handlers"
	"vigil/internal/middleware"
	"vigil/internal/models"
	"vigil/internal/smart"
)

// version is set at build time via -ldflags
var version = "dev"

func main() {
	log.SetFlags(log.Ltime | log.Ldate)
	log.Printf("🚀 Vigil Server v%s starting...", version)

	// Set version for handlers
	handlers.Version = version

	cfg := config.Load()

	if err := db.Init(cfg.DBPath); err != nil {
		log.Fatalf("❌ Database error: %v", err)
	}
	defer db.DB.Close()
	log.Printf("✓ Database: %s", cfg.DBPath)

	// Run SMART attributes migration
	if err := smart.MigrateSmartAttributes(db.DB); err != nil {
		log.Printf("⚠️  SMART migration warning: %v", err)
	}

	// Run extended schema migrations
	if err := db.MigrateSchemaExtensions(db.DB); err != nil {
		log.Printf("⚠️  Schema migration warning: %v", err)
	}

	// Auth initialisation
	if cfg.AuthEnabled {
		auth.CreateDefaultAdmin(cfg)
		log.Printf("✓ Authentication: enabled")
	} else {
		log.Printf("⚠️  Authentication: disabled (set AUTH_ENABLED=true to enable)")
	}
	auth.CleanupExpiredSessions()

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
	protect := func(h http.HandlerFunc) http.HandlerFunc {
		return auth.Middleware(cfg, h)
	}

	// Public endpoints
	mux.HandleFunc("GET /health", handlers.Health)
	mux.HandleFunc("GET /api/version", handlers.GetVersion)
	mux.HandleFunc("GET /api/auth/status", auth.Status(cfg))

	// Agent endpoint (no auth)
	mux.HandleFunc("POST /api/report", handlers.Report)

	// Auth endpoints
	mux.HandleFunc("POST /api/auth/login", auth.Login(cfg))
	mux.HandleFunc("POST /api/auth/logout", auth.Logout)

	// Protected endpoints
	mux.HandleFunc("GET /api/history", protect(handlers.History))
	mux.HandleFunc("GET /api/hosts", protect(handlers.Hosts))
	mux.HandleFunc("DELETE /api/hosts/{hostname}", protect(handlers.DeleteHost))
	mux.HandleFunc("GET /api/hosts/{hostname}/history", protect(handlers.HostHistory))

	// Alias endpoints
	mux.HandleFunc("GET /api/aliases", protect(handlers.GetAliases))
	mux.HandleFunc("POST /api/aliases", protect(handlers.SetAlias))
	mux.HandleFunc("DELETE /api/aliases/{id}", protect(handlers.DeleteAlias))

	// User endpoints
	mux.HandleFunc("GET /api/users/me", protect(auth.GetCurrentUser))
	mux.HandleFunc("POST /api/users/password", protect(auth.ChangePassword))
	mux.HandleFunc("POST /api/users/username", protect(auth.ChangeUsername))

	// ─── SMART Attributes API ─────────────────────────────────────────────
	mux.HandleFunc("GET /api/smart/attributes", protect(handlers.GetSmartAttributes))
	mux.HandleFunc("GET /api/smart/attributes/history", protect(handlers.GetSmartAttributeHistory))
	mux.HandleFunc("GET /api/smart/attributes/trend", protect(handlers.GetSmartAttributeTrend))
	mux.HandleFunc("GET /api/smart/health/summary", protect(handlers.GetDriveHealthSummary))
	mux.HandleFunc("GET /api/smart/health/all", protect(handlers.GetAllDrivesHealthSummary))
	mux.HandleFunc("GET /api/smart/health/issues", protect(handlers.GetDrivesWithIssues))
	mux.HandleFunc("GET /api/smart/critical-attributes", protect(handlers.GetCriticalAttributes))
	mux.HandleFunc("GET /api/smart/temperature/history", protect(handlers.GetTemperatureHistory))
	mux.HandleFunc("POST /api/smart/cleanup", protect(handlers.CleanupOldSmartData))

	// ─── ZFS Endpoints ────────────────────────────────────────────────────
	handlers.RegisterZFSRoutes(mux, protect)

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
