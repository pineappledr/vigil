package handlers

import (
	"net/http"
	"strings"

	"vigil/internal/middleware"
	"vigil/internal/models"
)

// Version is set at build time
var Version = "dev"

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
		// Always allow login page and static assets
		if r.URL.Path == "/login.html" || hasPublicExtension(r.URL.Path, publicExtensions) {
			fs.ServeHTTP(w, r)
			return
		}

		// Check auth for protected pages
		if config.AuthEnabled && !middleware.IsAuthenticated(r) {
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
