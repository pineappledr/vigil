package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"vigil/internal/audit"
	"vigil/internal/auth"
	"vigil/internal/db"
	"vigil/internal/events"
	"vigil/internal/notify"
	"vigil/internal/validate"
)

// NotifySender is set from main.go to enable test-fire.
// It uses the same Sender interface as the dispatcher.
var NotifySender notify.Sender

// ─── Provider Definitions ───────────────────────────────────────────────

// GetNotificationProviders returns the provider field schemas for the frontend wizard.
// GET /api/notifications/providers
func GetNotificationProviders(w http.ResponseWriter, r *http.Request) {
	JSONResponse(w, notify.GetProviderDefs())
}

// GetEventTypes returns all known event types with category metadata.
// GET /api/notifications/event-types
func GetEventTypes(w http.ResponseWriter, r *http.Request) {
	JSONResponse(w, events.AllEventTypeMeta)
}

// ─── Service CRUD ────────────────────────────────────────────────────────

// ListNotificationServices returns all configured services.
// GET /api/notifications/services
func ListNotificationServices(w http.ResponseWriter, r *http.Request) {
	services, err := notify.ListServices(db.DB)
	if err != nil {
		log.Printf("❌ List notification services: %v", err)
		JSONError(w, "Failed to list services", http.StatusInternalServerError)
		return
	}
	if services == nil {
		services = []notify.NotificationService{}
	}
	JSONResponse(w, services)
}

// GetNotificationService returns a single service with its rules, quiet
// hours, and digest config. Password fields in config are masked.
// GET /api/notifications/services/{id}
func GetNotificationService(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	svc, err := notify.GetService(db.DB, id)
	if err != nil {
		log.Printf("❌ Get notification service: %v", err)
		JSONError(w, "Failed to get service", http.StatusInternalServerError)
		return
	}
	if svc == nil {
		JSONError(w, "Service not found", http.StatusNotFound)
		return
	}

	rules, _ := notify.GetEventRules(db.DB, id)
	qh, _ := notify.GetQuietHours(db.DB, id)
	digest, _ := notify.GetDigestConfig(db.DB, id)

	if rules == nil {
		rules = []notify.EventRule{}
	}

	// Mask secrets in config_json before returning
	svc.ConfigJSON = maskConfigSecrets(svc.ServiceType, svc.ConfigJSON)

	JSONResponse(w, map[string]interface{}{
		"service":     svc,
		"event_rules": rules,
		"quiet_hours": qh,
		"digest":      digest,
	})
}

// CreateNotificationService adds a new service.
// Accepts either legacy config_json or structured config_fields.
// POST /api/notifications/services
func CreateNotificationService(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name             string            `json:"name"`
		ServiceType      string            `json:"service_type"`
		ConfigJSON       string            `json:"config_json"`
		ConfigFields     map[string]string `json:"config_fields"`
		Enabled          bool              `json:"enabled"`
		NotifyOnCritical bool              `json:"notify_on_critical"`
		NotifyOnWarning  bool              `json:"notify_on_warning"`
		NotifyOnHealthy  bool              `json:"notify_on_healthy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.ServiceType == "" {
		JSONError(w, "service_type is required", http.StatusBadRequest)
		return
	}
	if err := validate.Name(req.Name, 128); err != nil {
		JSONError(w, err.Error(), http.StatusBadRequest)
		return
	}

	configJSON := req.ConfigJSON

	// If structured fields provided, build the Shoutrrr URL server-side
	if req.ConfigFields != nil {
		built, err := buildConfigJSON(req.ServiceType, req.ConfigFields)
		if err != nil {
			JSONError(w, err.Error(), http.StatusBadRequest)
			return
		}
		configJSON = built
	}

	if configJSON == "" {
		JSONError(w, "config_json or config_fields is required", http.StatusBadRequest)
		return
	}

	svc := &notify.NotificationService{
		Name:             req.Name,
		ServiceType:      req.ServiceType,
		ConfigJSON:       configJSON,
		Enabled:          req.Enabled,
		NotifyOnCritical: req.NotifyOnCritical,
		NotifyOnWarning:  req.NotifyOnWarning,
		NotifyOnHealthy:  req.NotifyOnHealthy,
	}

	id, err := notify.CreateService(db.DB, svc)
	if err != nil {
		log.Printf("❌ Create notification service: %v", err)
		JSONError(w, "Failed to create service", http.StatusInternalServerError)
		return
	}

	svc.ID = id

	// Auto-populate event rules with sensible defaults so users can
	// immediately toggle individual event types on/off.
	for _, meta := range events.AllEventTypeMeta {
		rule := &notify.EventRule{
			ServiceID: id,
			EventType: string(meta.Type),
			Enabled:   meta.DefaultEnabled,
			Cooldown:  meta.DefaultCooldown,
		}
		if err := notify.UpsertEventRule(db.DB, rule); err != nil {
			log.Printf("notify: seed rule %s for service %d: %v", meta.Type, id, err)
		}
	}

	log.Printf("🔔 Notification service created: %s (%s)", svc.Name, svc.ServiceType)
	if s := auth.GetSessionFromContext(r); s != nil {
		audit.LogEvent(db.DB, r, s.UserID, s.Username, "notification_service_create", "notification_service", strconv.FormatInt(id, 10), svc.Name, "success")
	}
	w.WriteHeader(http.StatusCreated)
	JSONResponse(w, svc)
}

// UpdateNotificationService modifies a service.
// Accepts either legacy config_json or structured config_fields.
// PUT /api/notifications/services/{id}
func UpdateNotificationService(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Name             string            `json:"name"`
		ServiceType      string            `json:"service_type"`
		ConfigJSON       string            `json:"config_json"`
		ConfigFields     map[string]string `json:"config_fields"`
		Enabled          bool              `json:"enabled"`
		NotifyOnCritical bool              `json:"notify_on_critical"`
		NotifyOnWarning  bool              `json:"notify_on_warning"`
		NotifyOnHealthy  bool              `json:"notify_on_healthy"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	configJSON := req.ConfigJSON

	if req.ConfigFields != nil && req.ServiceType != "" {
		// Recover masked secrets from existing config
		existing, _ := notify.GetService(db.DB, id)
		if existing != nil {
			mergeExistingSecrets(req.ServiceType, req.ConfigFields, existing.ConfigJSON)
		}

		built, err := buildConfigJSON(req.ServiceType, req.ConfigFields)
		if err != nil {
			JSONError(w, err.Error(), http.StatusBadRequest)
			return
		}
		configJSON = built
	}

	svc := &notify.NotificationService{
		ID:               id,
		Name:             req.Name,
		ServiceType:      req.ServiceType,
		ConfigJSON:       configJSON,
		Enabled:          req.Enabled,
		NotifyOnCritical: req.NotifyOnCritical,
		NotifyOnWarning:  req.NotifyOnWarning,
		NotifyOnHealthy:  req.NotifyOnHealthy,
	}

	if err := notify.UpdateService(db.DB, svc); err != nil {
		log.Printf("❌ Update notification service: %v", err)
		JSONError(w, "Failed to update service", http.StatusInternalServerError)
		return
	}

	if s := auth.GetSessionFromContext(r); s != nil {
		audit.LogEvent(db.DB, r, s.UserID, s.Username, "notification_service_update", "notification_service", strconv.FormatInt(id, 10), "", "success")
	}
	JSONResponse(w, map[string]string{"status": "updated"})
}

// DeleteNotificationService removes a service.
// DELETE /api/notifications/services/{id}
func DeleteNotificationService(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	if err := notify.DeleteService(db.DB, id); err != nil {
		log.Printf("❌ Delete notification service: %v", err)
		JSONError(w, "Failed to delete service", http.StatusInternalServerError)
		return
	}

	log.Printf("🔔 Notification service deleted: id=%d", id)
	if s := auth.GetSessionFromContext(r); s != nil {
		audit.LogEvent(db.DB, r, s.UserID, s.Username, "notification_service_delete", "notification_service", strconv.FormatInt(id, 10), "", "success")
	}
	JSONResponse(w, map[string]string{"status": "deleted"})
}

// ─── Event Rules ─────────────────────────────────────────────────────────

// UpdateEventRules replaces event rules for a service.
// PUT /api/notifications/services/{id}/rules
func UpdateEventRules(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	var rules []notify.EventRule
	if err := json.NewDecoder(r.Body).Decode(&rules); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	for i := range rules {
		rules[i].ServiceID = id
		if err := notify.UpsertEventRule(db.DB, &rules[i]); err != nil {
			log.Printf("❌ Upsert event rule: %v", err)
			JSONError(w, "Failed to update rules", http.StatusInternalServerError)
			return
		}
	}

	if s := auth.GetSessionFromContext(r); s != nil {
		audit.LogEvent(db.DB, r, s.UserID, s.Username, "notification_rules_update", "notification_service", strconv.FormatInt(id, 10), "", "success")
	}
	JSONResponse(w, map[string]string{"status": "updated"})
}

// ─── Quiet Hours ─────────────────────────────────────────────────────────

// UpdateQuietHours sets quiet hours for a service.
// PUT /api/notifications/services/{id}/quiet-hours
func UpdateQuietHours(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	var qh notify.QuietHours
	if err := json.NewDecoder(r.Body).Decode(&qh); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	qh.ServiceID = id

	if err := notify.UpsertQuietHours(db.DB, &qh); err != nil {
		log.Printf("❌ Upsert quiet hours: %v", err)
		JSONError(w, "Failed to update quiet hours", http.StatusInternalServerError)
		return
	}

	if s := auth.GetSessionFromContext(r); s != nil {
		audit.LogEvent(db.DB, r, s.UserID, s.Username, "notification_quiet_hours_update", "notification_service", strconv.FormatInt(id, 10), "", "success")
	}
	JSONResponse(w, map[string]string{"status": "updated"})
}

// ─── Digest Config ───────────────────────────────────────────────────────

// UpdateDigestConfig sets digest config for a service.
// PUT /api/notifications/services/{id}/digest
func UpdateDigestConfig(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	var dc notify.DigestConfig
	if err := json.NewDecoder(r.Body).Decode(&dc); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	dc.ServiceID = id

	if err := notify.UpsertDigestConfig(db.DB, &dc); err != nil {
		log.Printf("❌ Upsert digest config: %v", err)
		JSONError(w, "Failed to update digest config", http.StatusInternalServerError)
		return
	}

	if s := auth.GetSessionFromContext(r); s != nil {
		audit.LogEvent(db.DB, r, s.UserID, s.Username, "notification_digest_update", "notification_service", strconv.FormatInt(id, 10), "", "success")
	}
	JSONResponse(w, map[string]string{"status": "updated"})
}

// ─── Test Fire ───────────────────────────────────────────────────────────

// TestFireNotification sends a test message through the given service.
// POST /api/notifications/test
func TestFireNotification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ServiceID int64  `json:"service_id"`
		Message   string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.ServiceID == 0 {
		JSONError(w, "service_id is required", http.StatusBadRequest)
		return
	}

	svc, err := notify.GetService(db.DB, req.ServiceID)
	if err != nil || svc == nil {
		JSONError(w, "Service not found", http.StatusNotFound)
		return
	}

	// Extract Shoutrrr URL from config
	var cfg struct {
		ShoutrrrURL string `json:"shoutrrr_url"`
	}
	if err := json.Unmarshal([]byte(svc.ConfigJSON), &cfg); err != nil || cfg.ShoutrrrURL == "" {
		JSONError(w, "Service config missing shoutrrr_url", http.StatusBadRequest)
		return
	}

	msg := req.Message
	if msg == "" {
		msg = "Vigil test notification from " + svc.Name
	}

	sender := NotifySender
	if sender == nil {
		sender = notify.ShoutrrrSender{}
	}

	if err := sender.Send(cfg.ShoutrrrURL, msg); err != nil {
		log.Printf("🔔 Test fire failed for %s: %v", svc.Name, err)
		JSONResponse(w, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	log.Printf("🔔 Test fire sent via %s", svc.Name)
	now := time.Now()
	notify.RecordNotification(db.DB, &notify.NotificationRecord{ //nolint:errcheck
		SettingID: svc.ID,
		EventType: "test",
		Message:   msg,
		Status:    "sent",
		SentAt:    now,
	})
	JSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "Test notification sent",
	})
}

// TestNotificationURL sends a test message to a Shoutrrr URL.
// Accepts either a raw URL or structured config_fields.
// POST /api/notifications/test-url
func TestNotificationURL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL          string            `json:"url"`
		ServiceType  string            `json:"service_type"`
		ConfigFields map[string]string `json:"config_fields"`
		Message      string            `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	testURL := req.URL

	// Build URL from structured fields if provided
	if req.ConfigFields != nil && req.ServiceType != "" {
		built, err := notify.BuildShoutrrrURL(req.ServiceType, req.ConfigFields)
		if err != nil {
			JSONResponse(w, map[string]interface{}{
				"success": false,
				"error":   "Invalid configuration: " + err.Error(),
			})
			return
		}
		testURL = built
	}

	if testURL == "" {
		JSONError(w, "url or (service_type + config_fields) required", http.StatusBadRequest)
		return
	}

	msg := req.Message
	if msg == "" {
		msg = "Vigil test notification"
	}

	sender := NotifySender
	if sender == nil {
		sender = notify.ShoutrrrSender{}
	}

	if err := sender.Send(testURL, msg); err != nil {
		log.Printf("🔔 Test URL fire failed: %v", err)
		JSONResponse(w, map[string]interface{}{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	JSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "Test notification sent",
	})
}

// ─── History ─────────────────────────────────────────────────────────────

// GetNotificationHistory returns recent notification records.
// GET /api/notifications/history?limit=50
func GetNotificationHistory(w http.ResponseWriter, r *http.Request) {
	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	history, err := notify.RecentHistory(db.DB, limit)
	if err != nil {
		log.Printf("❌ Notification history: %v", err)
		JSONError(w, "Failed to get history", http.StatusInternalServerError)
		return
	}
	if history == nil {
		history = []notify.NotificationRecord{}
	}

	JSONResponse(w, history)
}

// ─── Route Registration ──────────────────────────────────────────────────

// RegisterNotificationRoutes registers all notification API routes.
func RegisterNotificationRoutes(mux *http.ServeMux, protect func(http.HandlerFunc) http.HandlerFunc) {
	// Provider definitions (for dynamic form wizard)
	mux.HandleFunc("GET /api/notifications/providers", protect(GetNotificationProviders))
	mux.HandleFunc("GET /api/notifications/event-types", protect(GetEventTypes))

	mux.HandleFunc("GET /api/notifications/services", protect(ListNotificationServices))
	mux.HandleFunc("GET /api/notifications/services/{id}", protect(GetNotificationService))
	mux.HandleFunc("POST /api/notifications/services", protect(CreateNotificationService))
	mux.HandleFunc("PUT /api/notifications/services/{id}", protect(UpdateNotificationService))
	mux.HandleFunc("DELETE /api/notifications/services/{id}", protect(DeleteNotificationService))

	mux.HandleFunc("PUT /api/notifications/services/{id}/rules", protect(UpdateEventRules))
	mux.HandleFunc("PUT /api/notifications/services/{id}/quiet-hours", protect(UpdateQuietHours))
	mux.HandleFunc("PUT /api/notifications/services/{id}/digest", protect(UpdateDigestConfig))

	mux.HandleFunc("POST /api/notifications/test", protect(TestFireNotification))
	mux.HandleFunc("POST /api/notifications/test-url", protect(TestNotificationURL))
	mux.HandleFunc("GET /api/notifications/history", protect(GetNotificationHistory))
}

// ── helpers ──────────────────────────────────────────────────────────────

// buildConfigJSON validates fields, builds the Shoutrrr URL, and returns
// the combined JSON string for config_json storage.
func buildConfigJSON(serviceType string, fields map[string]string) (string, error) {
	if err := notify.ValidateFields(serviceType, fields); err != nil {
		return "", err
	}
	shoutrrrURL, err := notify.BuildShoutrrrURL(serviceType, fields)
	if err != nil {
		return "", err
	}
	cfgData, _ := json.Marshal(map[string]interface{}{
		"shoutrrr_url": shoutrrrURL,
		"fields":       fields,
	})
	return string(cfgData), nil
}

// mergeExistingSecrets replaces masked password placeholder values in fields
// with the actual secrets from the stored config.
func mergeExistingSecrets(serviceType string, fields map[string]string, existingConfigJSON string) {
	var oldCfg struct {
		Fields map[string]string `json:"fields"`
	}
	if err := json.Unmarshal([]byte(existingConfigJSON), &oldCfg); err != nil || oldCfg.Fields == nil {
		return
	}

	def, ok := notify.GetProviderDef(serviceType)
	if !ok {
		return
	}
	for _, f := range def.Fields {
		if f.Type == notify.FieldPassword && fields[f.Key] == notify.SecretMask {
			if original, exists := oldCfg.Fields[f.Key]; exists {
				fields[f.Key] = original
			}
		}
	}
}

// maskConfigSecrets masks password fields in a config_json string for API responses.
func maskConfigSecrets(serviceType, configJSON string) string {
	var cfg struct {
		ShoutrrrURL string            `json:"shoutrrr_url"`
		Fields      map[string]string `json:"fields"`
	}
	if err := json.Unmarshal([]byte(configJSON), &cfg); err != nil || cfg.Fields == nil {
		return configJSON // legacy config without fields — return as-is
	}

	masked := notify.MaskSecrets(serviceType, cfg.Fields)
	newCfg, err := json.Marshal(map[string]interface{}{
		"shoutrrr_url": notify.SecretMask,
		"fields":       masked,
	})
	if err != nil {
		return configJSON
	}
	return string(newCfg)
}
