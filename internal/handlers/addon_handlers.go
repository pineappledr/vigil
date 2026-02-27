package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"vigil/internal/addons"
	"vigil/internal/db"
)

// TelemetryBroker is set from main.go during startup.
var TelemetryBroker *addons.TelemetryBroker

// WebSocketHub is set from main.go during startup.
var WebSocketHub *addons.WebSocketHub

// ─── Add-on CRUD ─────────────────────────────────────────────────────────

// RegisterAddon validates the manifest and stores the add-on.
// POST /api/addons
func RegisterAddon(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ManifestJSON json.RawMessage `json:"manifest"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if len(req.ManifestJSON) == 0 {
		JSONError(w, "manifest is required", http.StatusBadRequest)
		return
	}

	manifest, err := addons.ValidateManifest(req.ManifestJSON)
	if err != nil {
		JSONError(w, fmt.Sprintf("Invalid manifest: %v", err), http.StatusBadRequest)
		return
	}

	id, err := addons.Register(db.DB, manifest.Name, manifest.Version,
		manifest.Description, string(req.ManifestJSON))
	if err != nil {
		log.Printf("❌ Register addon: %v", err)
		JSONError(w, "Failed to register add-on", http.StatusInternalServerError)
		return
	}

	addon, _ := addons.Get(db.DB, id)
	log.Printf("📦 Add-on registered: %s v%s (id=%d)", manifest.Name, manifest.Version, id)
	w.WriteHeader(http.StatusCreated)
	JSONResponse(w, addon)
}

// ListAddons returns all registered add-ons.
// GET /api/addons
func ListAddons(w http.ResponseWriter, r *http.Request) {
	list, err := addons.List(db.DB)
	if err != nil {
		log.Printf("❌ List addons: %v", err)
		JSONError(w, "Failed to list add-ons", http.StatusInternalServerError)
		return
	}
	if list == nil {
		list = []addons.Addon{}
	}
	JSONResponse(w, list)
}

// GetAddon returns a single add-on with its full manifest.
// GET /api/addons/{id}
func GetAddon(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid add-on ID", http.StatusBadRequest)
		return
	}

	addon, err := addons.Get(db.DB, id)
	if err != nil {
		log.Printf("❌ Get addon: %v", err)
		JSONError(w, "Failed to get add-on", http.StatusInternalServerError)
		return
	}
	if addon == nil {
		JSONError(w, "Add-on not found", http.StatusNotFound)
		return
	}

	// Return the addon with its parsed manifest
	var manifest json.RawMessage
	if err := json.Unmarshal([]byte(addon.ManifestJSON), &manifest); err != nil {
		manifest = json.RawMessage(addon.ManifestJSON)
	}

	JSONResponse(w, map[string]interface{}{
		"addon":    addon,
		"manifest": manifest,
	})
}

// DeregisterAddon removes an add-on.
// DELETE /api/addons/{id}
func DeregisterAddon(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid add-on ID", http.StatusBadRequest)
		return
	}

	addon, _ := addons.Get(db.DB, id)
	if addon == nil {
		JSONError(w, "Add-on not found", http.StatusNotFound)
		return
	}

	if err := addons.Deregister(db.DB, id); err != nil {
		log.Printf("❌ Deregister addon: %v", err)
		JSONError(w, "Failed to deregister add-on", http.StatusInternalServerError)
		return
	}

	log.Printf("📦 Add-on deregistered: %s (id=%d)", addon.Name, id)
	JSONResponse(w, map[string]string{"status": "deregistered"})
}

// SetAddonEnabled enables or disables an add-on.
// PUT /api/addons/{id}/enabled
func SetAddonEnabled(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid add-on ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	if err := addons.SetEnabled(db.DB, id, req.Enabled); err != nil {
		log.Printf("❌ Set addon enabled: %v", err)
		JSONError(w, "Failed to update add-on", http.StatusInternalServerError)
		return
	}

	JSONResponse(w, map[string]interface{}{
		"status":  "updated",
		"enabled": req.Enabled,
	})
}

// ─── SSE Telemetry Stream ────────────────────────────────────────────────

// AddonTelemetrySSE streams telemetry events for a specific add-on.
// GET /api/addons/{id}/telemetry
func AddonTelemetrySSE(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid add-on ID", http.StatusBadRequest)
		return
	}

	addon, _ := addons.Get(db.DB, id)
	if addon == nil {
		JSONError(w, "Add-on not found", http.StatusNotFound)
		return
	}

	if TelemetryBroker == nil {
		JSONError(w, "Telemetry not available", http.StatusServiceUnavailable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		JSONError(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no") // Nginx buffering bypass

	ch := TelemetryBroker.Subscribe(id)
	defer TelemetryBroker.Unsubscribe(id, ch)

	// Send initial connection event
	fmt.Fprintf(w, "event: connected\ndata: {\"addon_id\":%d}\n\n", id)
	flusher.Flush()

	ctx := r.Context()
	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return
			}
			data, err := json.Marshal(evt)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", evt.Type, data)
			flusher.Flush()

		case <-ctx.Done():
			return
		}
	}
}

// ─── Route Registration ──────────────────────────────────────────────────

// RegisterAddonRoutes registers all add-on API routes.
func RegisterAddonRoutes(mux *http.ServeMux, protect func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /api/addons", protect(RegisterAddon))
	mux.HandleFunc("GET /api/addons", protect(ListAddons))
	mux.HandleFunc("GET /api/addons/{id}", protect(GetAddon))
	mux.HandleFunc("DELETE /api/addons/{id}", protect(DeregisterAddon))
	mux.HandleFunc("PUT /api/addons/{id}/enabled", protect(SetAddonEnabled))
	mux.HandleFunc("GET /api/addons/{id}/telemetry", protect(AddonTelemetrySSE))

	// WebSocket telemetry ingestion — add-ons connect here to stream data
	if WebSocketHub != nil {
		mux.HandleFunc("GET /api/addons/ws", WebSocketHub.HandleConnection)
	}
}
