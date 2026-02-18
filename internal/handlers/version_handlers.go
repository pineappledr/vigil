package handlers

import (
	"encoding/json"
	"net/http"

	"vigil/internal/version"
)

// VersionHandler handles version-related API requests
type VersionHandler struct {
	checker *version.Checker
}

// NewVersionHandler creates a new version handler
func NewVersionHandler(currentVersion, owner, repo string) *VersionHandler {
	return &VersionHandler{
		checker: version.NewChecker(currentVersion, owner, repo),
	}
}

// CheckVersion handles GET /api/version/check
// Returns information about available updates
func (h *VersionHandler) CheckVersion(w http.ResponseWriter, r *http.Request) {
	// Only allow GET
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Check for force refresh parameter
	var info *version.ReleaseInfo
	var err error

	if r.URL.Query().Get("force") == "true" {
		info, err = h.checker.ForceCheck()
	} else {
		info, err = h.checker.Check()
	}

	if err != nil {
		// Return a graceful response even on error
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"current_version":  h.checker.GetCurrentVersion(),
			"update_available": false,
			"error":            err.Error(),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

// GetCurrentVersion handles GET /api/version
// Returns just the current version (existing endpoint, enhanced)
func (h *VersionHandler) GetCurrentVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"version": h.checker.GetCurrentVersion(),
	})
}
