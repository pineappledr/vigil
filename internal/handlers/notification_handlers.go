package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"vigil/internal/db"
	"vigil/internal/notify"
)

// NotifySender is set from main.go to enable test-fire.
// It uses the same Sender interface as the dispatcher.
var NotifySender notify.Sender

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
// hours, and digest config.
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

	JSONResponse(w, map[string]interface{}{
		"service":      svc,
		"event_rules":  rules,
		"quiet_hours":  qh,
		"digest":       digest,
	})
}

// CreateNotificationService adds a new service.
// POST /api/notifications/services
func CreateNotificationService(w http.ResponseWriter, r *http.Request) {
	var svc notify.NotificationService
	if err := json.NewDecoder(r.Body).Decode(&svc); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if svc.Name == "" || svc.ServiceType == "" || svc.ConfigJSON == "" {
		JSONError(w, "name, service_type, and config_json are required", http.StatusBadRequest)
		return
	}

	id, err := notify.CreateService(db.DB, &svc)
	if err != nil {
		log.Printf("❌ Create notification service: %v", err)
		JSONError(w, "Failed to create service", http.StatusInternalServerError)
		return
	}

	svc.ID = id
	log.Printf("🔔 Notification service created: %s (%s)", svc.Name, svc.ServiceType)
	w.WriteHeader(http.StatusCreated)
	JSONResponse(w, svc)
}

// UpdateNotificationService modifies a service.
// PUT /api/notifications/services/{id}
func UpdateNotificationService(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid service ID", http.StatusBadRequest)
		return
	}

	var svc notify.NotificationService
	if err := json.NewDecoder(r.Body).Decode(&svc); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	svc.ID = id

	if err := notify.UpdateService(db.DB, &svc); err != nil {
		log.Printf("❌ Update notification service: %v", err)
		JSONError(w, "Failed to update service", http.StatusInternalServerError)
		return
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
	JSONResponse(w, map[string]interface{}{
		"success": true,
		"message": "Test notification sent",
	})
}

// TestNotificationURL sends a test message to a raw Shoutrrr URL (no saved service needed).
// POST /api/notifications/test-url
func TestNotificationURL(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL     string `json:"url"`
		Message string `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	if req.URL == "" {
		JSONError(w, "url is required", http.StatusBadRequest)
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

	if err := sender.Send(req.URL, msg); err != nil {
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

func parseID(r *http.Request, name string) (int64, error) {
	return strconv.ParseInt(r.PathValue(name), 10, 64)
}
