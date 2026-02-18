package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"vigil/internal/db"
)

// SettingsHandler handles settings-related API requests
type SettingsHandler struct {
	DB *sql.DB
}

// NewSettingsHandler creates a new settings handler
func NewSettingsHandler(database *sql.DB) *SettingsHandler {
	return &SettingsHandler{DB: database}
}

// GetAllSettings handles GET /api/settings
// Returns all settings, optionally grouped by category
func (h *SettingsHandler) GetAllSettings(w http.ResponseWriter, r *http.Request) {
	// Check if grouped format requested
	grouped := r.URL.Query().Get("grouped") == "true"

	if grouped {
		settings, err := db.GetSettingsGrouped(h.DB)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		respondJSON(w, settings)
		return
	}

	settings, err := db.GetAllSettings(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, settings)
}

// GetSettingsByCategory handles GET /api/settings/{category}
// Returns all settings for a specific category
func (h *SettingsHandler) GetSettingsByCategory(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	if category == "" {
		http.Error(w, "category is required", http.StatusBadRequest)
		return
	}

	settings, err := db.GetSettingsByCategory(h.DB, category)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(settings) == 0 {
		http.Error(w, "category not found", http.StatusNotFound)
		return
	}

	respondJSON(w, settings)
}

// GetSetting handles GET /api/settings/{category}/{key}
// Returns a single setting
func (h *SettingsHandler) GetSetting(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	key := r.PathValue("key")

	if category == "" || key == "" {
		http.Error(w, "category and key are required", http.StatusBadRequest)
		return
	}

	setting, err := db.GetSetting(h.DB, category, key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if setting == nil {
		http.Error(w, "setting not found", http.StatusNotFound)
		return
	}

	respondJSON(w, setting)
}

// UpdateSetting handles PUT /api/settings/{category}/{key}
// Updates a single setting value
func (h *SettingsHandler) UpdateSetting(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	key := r.PathValue("key")

	if category == "" || key == "" {
		http.Error(w, "category and key are required", http.StatusBadRequest)
		return
	}

	// Parse request body
	var update db.SettingUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	// Update the setting
	if err := db.UpdateSetting(h.DB, category, key, update.Value); err != nil {
		// Check if it's a validation error vs not found
		if err.Error() == "setting "+category+"."+key+" not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Return the updated setting
	setting, err := db.GetSetting(h.DB, category, key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, setting)
}

// ResetCategory handles POST /api/settings/reset/{category}
// Resets all settings in a category to defaults
func (h *SettingsHandler) ResetCategory(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")

	if category == "" {
		http.Error(w, "category is required", http.StatusBadRequest)
		return
	}

	if err := db.ResetCategoryToDefaults(h.DB, category); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Return the reset settings
	settings, err := db.GetSettingsByCategory(h.DB, category)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
		"message":  "settings reset to defaults",
		"category": category,
		"settings": settings,
	})
}

// ResetAll handles POST /api/settings/reset
// Resets all settings to defaults
func (h *SettingsHandler) ResetAll(w http.ResponseWriter, r *http.Request) {
	if err := db.ResetAllToDefaults(h.DB); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Return all reset settings
	settings, err := db.GetSettingsGrouped(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
		"message":  "all settings reset to defaults",
		"settings": settings,
	})
}

// GetCategories handles GET /api/settings/categories
// Returns list of all setting categories
func (h *SettingsHandler) GetCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := db.GetCategories(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
		"categories": categories,
	})
}

// respondJSON is a helper to write JSON responses
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
