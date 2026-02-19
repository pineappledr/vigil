package settings

import (
	"database/sql"
	"encoding/json"
	"net/http"
)

// Handler handles settings-related API requests
type Handler struct {
	DB *sql.DB
}

// NewHandler creates a new settings handler
func NewHandler(database *sql.DB) *Handler {
	return &Handler{DB: database}
}

// GetAllSettings handles GET /api/settings
func (h *Handler) GetAllSettings(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("grouped") == "true" {
		settings, err := GetSettingsGrouped(h.DB)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		respondJSON(w, settings)
		return
	}

	settings, err := GetAllSettings(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, settings)
}

// GetSettingsByCategory handles GET /api/settings/{category}
func (h *Handler) GetSettingsByCategory(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	if category == "" {
		http.Error(w, "category is required", http.StatusBadRequest)
		return
	}

	settings, err := GetSettingsByCategory(h.DB, category)
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
func (h *Handler) GetSetting(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	key := r.PathValue("key")

	if category == "" || key == "" {
		http.Error(w, "category and key are required", http.StatusBadRequest)
		return
	}

	setting, err := GetSetting(h.DB, category, key)
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
func (h *Handler) UpdateSetting(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	key := r.PathValue("key")

	if category == "" || key == "" {
		http.Error(w, "category and key are required", http.StatusBadRequest)
		return
	}

	var update SettingUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := UpdateSetting(h.DB, category, key, update.Value); err != nil {
		if err.Error() == "setting "+category+"."+key+" not found" {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	setting, err := GetSetting(h.DB, category, key)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, setting)
}

// ResetCategory handles POST /api/settings/reset/{category}
func (h *Handler) ResetCategory(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	if category == "" {
		http.Error(w, "category is required", http.StatusBadRequest)
		return
	}

	if err := ResetCategoryToDefaults(h.DB, category); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	settings, err := GetSettingsByCategory(h.DB, category)
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
func (h *Handler) ResetAll(w http.ResponseWriter, r *http.Request) {
	if err := ResetAllToDefaults(h.DB); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	settings, err := GetSettingsGrouped(h.DB)
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
func (h *Handler) GetCategories(w http.ResponseWriter, r *http.Request) {
	categories, err := GetCategories(h.DB)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, map[string]interface{}{
		"categories": categories,
	})
}

func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
