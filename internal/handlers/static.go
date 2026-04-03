package handlers

import (
	"net/http"
	"strings"

	"vigil/internal/auth"
	"vigil/internal/models"
)

// Version is set at build time
var Version = "dev"

// VersionChecker handles version update checking
var VersionChecker *VersionHandler

// Health returns server health status
func Health(w http.ResponseWriter, r *http.Request) {
	JSONResponse(w, map[string]string{
		"status":  "healthy",
		"version": Version,
	})
}

// GetVersion returns server version
func GetVersion(w http.ResponseWriter, r *http.Request) {
	JSONResponse(w, map[string]string{"version": Version})
}

// StaticFiles serves static files with auth check
func StaticFiles(config models.Config) http.HandlerFunc {
	fs := http.FileServer(http.Dir("./web"))

	// Extensions that don't require auth
	publicExtensions := []string{".css", ".js", ".ico", ".png", ".svg"}

	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Force revalidation for JS and CSS so browsers always pick up
		// new deployments without requiring a hard refresh.
		if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".css") {
			w.Header().Set("Cache-Control", "no-cache, must-revalidate")
		}

		// Always allow login page and static assets
		if path == "/login.html" || hasPublicExtension(path, publicExtensions) {
			fs.ServeHTTP(w, r)
			return
		}

		// Check auth for protected pages
		if config.AuthEnabled && !auth.IsAuthenticated(r) {
			http.Redirect(w, r, "/login.html", http.StatusFound)
			return
		}

		fs.ServeHTTP(w, r)
	}
}

func hasPublicExtension(path string, extensions []string) bool {
	for _, ext := range extensions {
		if strings.HasSuffix(path, ext) {
			return true
		}
	}
	return false
}
