package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"vigil/internal/agents"
	"vigil/internal/auth"
	"vigil/internal/config"
	"vigil/internal/crypto"
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

	// Initialize version checker for update notifications
	handlers.VersionChecker = handlers.NewVersionHandler(version, "pineappledr", "vigil")

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

	// Run agent authentication migration
	if err := agents.Migrate(db.DB); err != nil {
		log.Printf("⚠️  Agent auth migration warning: %v", err)
	}

	// Load or generate server Ed25519 key pair
	dataDir := filepath.Dir(cfg.DBPath)
	if dataDir == "." {
		if cwd, err := os.Getwd(); err == nil {
			dataDir = cwd
		}
	}
	keys, err := crypto.LoadOrGenerate(dataDir)
	if err != nil {
		log.Fatalf("❌ Failed to initialise server keys: %v", err)
	}
	handlers.ServerKeys = keys
	log.Printf("✓ Server keys: %s", filepath.Join(dataDir, "vigil.key"))

	// Auth initialisation
	if cfg.AuthEnabled {
		auth.CreateDefaultAdmin(cfg)
		log.Printf("✓ Authentication: enabled")
	} else {
		log.Printf("⚠️  Authentication: disabled (set AUTH_ENABLED=true to enable)")
	}
	auth.CleanupExpiredSessions()
	agents.CleanupExpiredAgentSessions(db.DB)

	// Periodic session cleanup
	go func() {
		ticker := time.NewTicker(1 * time.Hour)
		for range ticker.C {
			auth.CleanupExpiredSessions()
			agents.CleanupExpiredAgentSessions(db.DB)
		}
	}()

	// Periodic update checking (every 12 hours)
	go func() {
		// Check immediately on startup
		if handlers.VersionChecker != nil {
			log.Printf("🔄 Checking for updates...")
			// The checker has internal caching, so frequent checks from clients won't hit GitHub API
			handlers.VersionChecker.SetCacheTTL(12 * time.Hour)
			info, err := handlers.VersionChecker.Check()
			if err != nil {
				log.Printf("⚠️  Update check failed: %v", err)
			} else if info.UpdateAvailable {
				log.Printf("📦 Update available: v%s → v%s", info.CurrentVersion, info.LatestVersion)
			} else {
				log.Printf("✓ Running latest version: v%s", info.CurrentVersion)
			}
		}

		// Then check every 12 hours
		ticker := time.NewTicker(12 * time.Hour)
		for range ticker.C {
			if handlers.VersionChecker != nil {
				log.Printf("🔄 Checking for updates...")
				info, err := handlers.VersionChecker.Check()
				if err != nil {
					log.Printf("⚠️  Update check failed: %v", err)
				} else if info.UpdateAvailable {
					log.Printf("📦 Update available: v%s → v%s", info.CurrentVersion, info.LatestVersion)
				} else {
					log.Printf("✓ Running latest version: v%s", info.CurrentVersion)
				}
			}
		}
	}()

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

	// Rate limiters for public auth endpoints
	loginLimiter := middleware.NewRateLimiter(5, time.Minute)
	agentLimiter := middleware.NewRateLimiter(10, time.Minute)

	// Public endpoints
	mux.HandleFunc("GET /health", handlers.Health)
	mux.HandleFunc("GET /api/version", handlers.GetVersion)
	mux.HandleFunc("GET /api/version/check", handlers.VersionChecker.CheckVersion)
	mux.HandleFunc("GET /api/auth/status", auth.Status(cfg))

	// Auth endpoints (rate limited)
	mux.HandleFunc("POST /api/auth/login", loginLimiter.Limit(auth.Login(cfg)))
	mux.HandleFunc("POST /api/auth/logout", auth.Logout)

	// ─── Agent authentication (public, rate limited) ─────────────────────
	mux.HandleFunc("GET /api/v1/server/pubkey", handlers.GetServerPublicKey)
	mux.HandleFunc("POST /api/v1/agents/register", agentLimiter.Limit(handlers.RegisterAgent))
	mux.HandleFunc("POST /api/v1/agents/auth", agentLimiter.Limit(handlers.AuthAgent))

	// Agent report endpoint — requires valid agent session token
	mux.HandleFunc("POST /api/report", handlers.Report)

	// ─── Agent management (admin-protected) ───────────────────────────────
	mux.HandleFunc("GET /api/v1/agents", protect(handlers.ListAgents))
	mux.HandleFunc("DELETE /api/v1/agents/{id}", protect(handlers.DeleteRegisteredAgent))
	mux.HandleFunc("POST /api/v1/tokens", protect(handlers.CreateToken))
	mux.HandleFunc("GET /api/v1/tokens", protect(handlers.ListTokens))
	mux.HandleFunc("DELETE /api/v1/tokens/{id}", protect(handlers.DeleteToken))

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
