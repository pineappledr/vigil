package handlers

import (
	"encoding/json"
	"net/http"

	"vigil/internal/db"
	"vigil/internal/settings"
)

// GetSettingsByCategory returns all settings for a given category.
// GET /api/settings/{category}
func GetSettingsByCategory(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	if category == "" {
		JSONError(w, "Missing category", http.StatusBadRequest)
		return
	}

	items, err := settings.GetSettingsByCategory(db.DB, category)
	if err != nil {
		JSONError(w, "Failed to get settings", http.StatusInternalServerError)
		return
	}
	if items == nil {
		items = []settings.Setting{}
	}

	JSONResponse(w, items)
}

// UpdateSettingValue updates a single setting value.
// PUT /api/settings/{category}/{key}
func UpdateSettingValue(w http.ResponseWriter, r *http.Request) {
	category := r.PathValue("category")
	key := r.PathValue("key")
	if category == "" || key == "" {
		JSONError(w, "Missing category or key", http.StatusBadRequest)
		return
	}

	var req struct {
		Value string `json:"value"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := settings.UpdateSetting(db.DB, category, key, req.Value); err != nil {
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	JSONResponse(w, map[string]string{"status": "updated"})
}

// RegisterSettingsRoutes registers settings API routes.
func RegisterSettingsRoutes(mux *http.ServeMux, protect func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /api/settings/{category}", protect(GetSettingsByCategory))
	mux.HandleFunc("PUT /api/settings/{category}/{key}", protect(UpdateSettingValue))
}
