package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"vigil/internal/addons"
	"vigil/internal/auth"
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
// Requires the user's password for confirmation.
// DELETE /api/addons/{id}
func DeregisterAddon(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid add-on ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Verify the user's password
	session := auth.GetSessionFromContext(r)
	if session == nil {
		JSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var storedHash string
	if err := db.DB.QueryRow("SELECT password_hash FROM users WHERE id = ?", session.UserID).Scan(&storedHash); err != nil {
		JSONError(w, "Failed to verify password", http.StatusInternalServerError)
		return
	}
	if !auth.CheckPassword(storedHash, req.Password) {
		JSONError(w, "Incorrect password", http.StatusUnauthorized)
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

	log.Printf("📦 Add-on deregistered: %s (id=%d, by=%s)", addon.Name, id, session.Username)
	JSONResponse(w, map[string]string{"status": "deregistered"})
}

// SetAddonEnabled enables or disables an add-on.
// Requires the user's password for confirmation.
// PUT /api/addons/{id}/enabled
func SetAddonEnabled(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid add-on ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Enabled  bool   `json:"enabled"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Verify the user's password
	session := auth.GetSessionFromContext(r)
	if session == nil {
		JSONError(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	var storedHash string
	if err := db.DB.QueryRow("SELECT password_hash FROM users WHERE id = ?", session.UserID).Scan(&storedHash); err != nil {
		JSONError(w, "Failed to verify password", http.StatusInternalServerError)
		return
	}
	if !auth.CheckPassword(storedHash, req.Password) {
		JSONError(w, "Incorrect password", http.StatusUnauthorized)
		return
	}

	if err := addons.SetEnabled(db.DB, id, req.Enabled); err != nil {
		log.Printf("❌ Set addon enabled: %v", err)
		JSONError(w, "Failed to update add-on", http.StatusInternalServerError)
		return
	}

	action := "enabled"
	if !req.Enabled {
		action = "disabled"
	}
	addon, _ := addons.Get(db.DB, id)
	if addon != nil {
		log.Printf("📦 Add-on %s: %s (id=%d, by=%s)", action, addon.Name, id, session.Username)
	}

	JSONResponse(w, map[string]interface{}{
		"status":  "updated",
		"enabled": req.Enabled,
	})
}

// ─── Admin Add-on Registration (UI Flow) ─────────────────────────────────

// CreateAddonFromUI creates a new add-on record with name + URL and binds
// the pre-generated registration token to it.
// POST /api/addons/register
func CreateAddonFromUI(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name  string `json:"name"`
		URL   string `json:"url"`
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		JSONError(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// ── Validate inputs ──────────────────────────────────────────────
	if req.Name == "" {
		JSONError(w, "name is required", http.StatusBadRequest)
		return
	}
	if len(req.Name) > 128 {
		JSONError(w, "name must be 128 characters or fewer", http.StatusBadRequest)
		return
	}
	if req.Token == "" {
		JSONError(w, "token is required", http.StatusBadRequest)
		return
	}
	if len(req.URL) > 512 {
		JSONError(w, "URL must be 512 characters or fewer", http.StatusBadRequest)
		return
	}

	// ── Validate the pre-generated token ─────────────────────────────
	tok, err := addons.GetRegistrationToken(db.DB, req.Token)
	if err != nil {
		log.Printf("❌ Register addon — token lookup: %v", err)
		JSONError(w, "Failed to validate token", http.StatusInternalServerError)
		return
	}
	if tok == nil {
		JSONError(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}
	if tok.UsedAt != nil {
		JSONError(w, "Token has already been used", http.StatusConflict)
		return
	}

	// ── Create the add-on ────────────────────────────────────────────
	addonID, err := addons.RegisterWithURL(db.DB, req.Name, req.URL)
	if err != nil {
		log.Printf("❌ Register addon from UI: %v", err)
		JSONError(w, "Failed to register add-on", http.StatusInternalServerError)
		return
	}

	// Consume the pre-generated token — bind it to this addon
	if err := addons.ConsumeRegistrationToken(db.DB, req.Token, addonID); err != nil {
		log.Printf("⚠️  Could not bind token to addon: %v", err)
	}

	addon, _ := addons.Get(db.DB, addonID)
	log.Printf("📦 Add-on registered from UI: %s (id=%d, token=%.16s…)", req.Name, addonID, req.Token)

	w.WriteHeader(http.StatusCreated)
	JSONResponse(w, map[string]interface{}{
		"addon": addon,
		"token": req.Token,
	})
}

// ─── Add-on Self-Registration (Programmatic) ─────────────────────────────

// ConnectAddon allows an add-on to submit its manifest using a registration
// token. The token must have been previously bound to an add-on via the
// admin UI flow (POST /api/addons/register).
//
// This endpoint is NOT behind the user session middleware — it authenticates
// via the addon registration token in the Authorization header.
//
// POST /api/addons/connect
func ConnectAddon(w http.ResponseWriter, r *http.Request) {
	// Extract Bearer token.
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		JSONError(w, "Missing or invalid Authorization header", http.StatusUnauthorized)
		return
	}
	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Look up the registration token.
	tok, err := addons.GetRegistrationToken(db.DB, token)
	if err != nil {
		log.Printf("❌ ConnectAddon — token lookup: %v", err)
		JSONError(w, "Failed to validate token", http.StatusInternalServerError)
		return
	}
	if tok == nil {
		JSONError(w, "Invalid or expired token", http.StatusUnauthorized)
		return
	}
	if tok.UsedByAddonID == nil {
		// Token exists but hasn't been bound to an addon yet (admin hasn't
		// finished the "Register Add-on" step in the UI). Tell the addon to retry.
		JSONError(w, "Token not yet bound to an add-on — complete registration in the Vigil UI first", http.StatusPreconditionFailed)
		return
	}

	// Parse the manifest from the request body.
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

	// Update the addon's manifest and mark it online.
	addonID := *tok.UsedByAddonID

	// Check for version change before updating.
	oldAddon, _ := addons.Get(db.DB, addonID)
	var versionChanged bool
	if oldAddon != nil && oldAddon.Version != "" && oldAddon.Version != "0.0.0" && oldAddon.Version != manifest.Version {
		versionChanged = true
		log.Printf("📦 Add-on %s updated: v%s → v%s (id=%d)", manifest.Name, oldAddon.Version, manifest.Version, addonID)
	}

	if err := addons.UpdateManifest(db.DB, addonID, manifest.Version,
		manifest.Description, string(req.ManifestJSON)); err != nil {
		log.Printf("❌ ConnectAddon — update manifest: %v", err)
		JSONError(w, "Failed to update add-on manifest", http.StatusInternalServerError)
		return
	}

	addon, _ := addons.Get(db.DB, addonID)
	log.Printf("📦 Add-on connected: %s v%s (id=%d)", manifest.Name, manifest.Version, addonID)

	// Send version-change notification via telemetry if the version changed.
	if versionChanged && TelemetryBroker != nil {
		TelemetryBroker.Publish(addons.TelemetryEvent{
			AddonID: addonID,
			Type:    "notification",
			Payload: json.RawMessage(fmt.Sprintf(`{"message":"Add-on %s updated to v%s","severity":"info"}`, manifest.Name, manifest.Version)),
		})
	}

	resp := map[string]interface{}{
		"addon_id":   addonID,
		"session_id": tok.Token[:16],
		"addon":      addon,
	}
	if versionChanged && oldAddon != nil {
		resp["previous_version"] = oldAddon.Version
	}
	JSONResponse(w, resp)
}

// ─── Add-on Token CRUD ───────────────────────────────────────────────────

// CreateAddonToken generates a new registration token for an add-on.
// Tokens expire after 1 hour by default.
// POST /api/addons/tokens
func CreateAddonToken(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	expiry := 1 * time.Hour
	tok, err := addons.CreateRegistrationToken(db.DB, req.Name, &expiry)
	if err != nil {
		log.Printf("❌ Create addon token: %v", err)
		JSONError(w, "Failed to create token", http.StatusInternalServerError)
		return
	}

	log.Printf("🔑 Add-on registration token created: %.16s… expires=%v (name=%q)", tok.Token, tok.ExpiresAt, tok.Name)
	w.WriteHeader(http.StatusCreated)
	JSONResponse(w, tok)
}

// ListAddonTokens returns all add-on registration tokens.
// Full token values are masked — only the first 16 hex chars are returned.
// GET /api/addons/tokens
func ListAddonTokens(w http.ResponseWriter, r *http.Request) {
	tokens, err := addons.ListRegistrationTokens(db.DB)
	if err != nil {
		log.Printf("❌ List addon tokens: %v", err)
		JSONError(w, "Failed to list tokens", http.StatusInternalServerError)
		return
	}
	if tokens == nil {
		tokens = []addons.RegistrationToken{}
	}

	// Mask full token values — never expose secrets in list views
	for i := range tokens {
		if len(tokens[i].Token) > 16 {
			tokens[i].Token = tokens[i].Token[:16] + "…"
		}
	}

	JSONResponse(w, map[string]interface{}{"tokens": tokens})
}

// DeleteAddonToken removes a registration token.
// DELETE /api/addons/tokens/{id}
func DeleteAddonToken(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid token ID", http.StatusBadRequest)
		return
	}

	if err := addons.DeleteRegistrationToken(db.DB, id); err != nil {
		log.Printf("❌ Delete addon token: %v", err)
		JSONError(w, "Failed to delete token", http.StatusInternalServerError)
		return
	}
	JSONResponse(w, map[string]string{"status": "deleted"})
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

// ─── Addon Proxy ─────────────────────────────────────────────────────────

// ProxyAddonRequest proxies a request to the add-on's own API.
// This allows the frontend to interact with addon APIs without CORS issues.
// GET/DELETE /api/addons/{id}/proxy?path=/api/deploy-info
//
// Security: The target URL is constructed from the addon's registered URL
// (stored by an admin) combined with an allowlisted API path. Only paths
// starting with "/api/" are permitted to prevent path traversal.
func ProxyAddonRequest(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid add-on ID", http.StatusBadRequest)
		return
	}

	path := r.URL.Query().Get("path")
	if path == "" || !strings.HasPrefix(path, "/api/") {
		JSONError(w, "path must start with /api/", http.StatusBadRequest)
		return
	}
	// Block path traversal attempts.
	if strings.Contains(path, "..") {
		JSONError(w, "invalid path", http.StatusBadRequest)
		return
	}

	addon, err := addons.Get(db.DB, id)
	if err != nil {
		log.Printf("❌ Proxy addon lookup: %v", err)
		JSONError(w, "Failed to look up add-on", http.StatusInternalServerError)
		return
	}
	if addon == nil {
		JSONError(w, "Add-on not found", http.StatusNotFound)
		return
	}
	if addon.URL == "" {
		JSONError(w, "Add-on has no URL configured", http.StatusBadRequest)
		return
	}

	// Parse the admin-registered addon base URL (trusted).
	baseURL, err := url.Parse(addon.URL)
	if err != nil || (baseURL.Scheme != "http" && baseURL.Scheme != "https") {
		JSONError(w, "invalid addon URL", http.StatusBadRequest)
		return
	}

	// Build the target URL by resolving the path against the trusted base.
	targetURL, err := url.JoinPath(addon.URL, path)
	if err != nil {
		JSONError(w, "invalid addon URL", http.StatusBadRequest)
		return
	}
	parsed, err := url.Parse(targetURL)
	if err != nil {
		JSONError(w, "invalid target URL", http.StatusBadRequest)
		return
	}

	// SSRF guard: verify the resolved URL still points to the same host.
	if parsed.Host != baseURL.Host || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		JSONError(w, "proxy target does not match addon host", http.StatusBadRequest)
		return
	}

	// Allow overriding the upstream method via ?method=DELETE (etc.)
	// so the frontend can issue DELETE/PUT through a GET/POST proxy route.
	upstreamMethod := r.Method
	if m := r.URL.Query().Get("method"); m != "" {
		upstreamMethod = strings.ToUpper(m)
	}

	// Safe: target host verified against admin-registered addon URL.
	safeURL := parsed.String()
	req, err := http.NewRequestWithContext(r.Context(), upstreamMethod, safeURL, nil)
	if err != nil {
		JSONError(w, "failed to create proxy request", http.StatusInternalServerError)
		return
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ Proxy request to addon %d: %v", id, err)
		JSONError(w, "Failed to reach add-on", http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", resp.Header.Get("Content-Type"))
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, io.LimitReader(resp.Body, 64*1024)) // 64 KiB limit
}

// ─── Update Check ────────────────────────────────────────────────────────

// CheckAddonUpdates queries the container registry for newer image tags.
// It extracts the Docker image reference from the addon's manifest
// (deploy-wizard component) and checks for tags newer than the current one.
// GET /api/addons/{id}/check-updates
func CheckAddonUpdates(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		JSONError(w, "Invalid add-on ID", http.StatusBadRequest)
		return
	}

	addon, err := addons.Get(db.DB, id)
	if err != nil || addon == nil {
		JSONError(w, "Add-on not found", http.StatusNotFound)
		return
	}

	// Extract Docker image from manifest deploy-wizard config.
	image, currentTag := extractDockerImage(addon.ManifestJSON)
	if image == "" {
		JSONResponse(w, map[string]interface{}{
			"update_available": false,
			"message":          "No Docker image found in manifest",
		})
		return
	}

	// Query the container registry for available tags.
	tags, err := queryRegistryTags(r.Context(), image)
	if err != nil {
		log.Printf("⚠️  Registry tag check for %s: %v", image, err)
		JSONResponse(w, map[string]interface{}{
			"update_available": false,
			"current_tag":      currentTag,
			"image":            image,
			"error":            err.Error(),
		})
		return
	}

	// Find the latest semver tag.
	latestTag := findLatestTag(tags)

	JSONResponse(w, map[string]interface{}{
		"update_available": latestTag != "" && latestTag != currentTag,
		"current_tag":      currentTag,
		"latest_tag":       latestTag,
		"image":            image,
		"current_version":  addon.Version,
	})
}

// extractDockerImage parses the manifest JSON and returns the Docker image
// and default tag from the first deploy-wizard component found.
func extractDockerImage(manifestJSON string) (image, tag string) {
	var m addons.Manifest
	if err := json.Unmarshal([]byte(manifestJSON), &m); err != nil {
		return "", ""
	}

	for _, page := range m.Pages {
		for _, comp := range page.Components {
			if comp.Type != "deploy-wizard" || len(comp.Config) == 0 {
				continue
			}
			var cfg addons.DeployWizardConfig
			if err := json.Unmarshal(comp.Config, &cfg); err != nil {
				continue
			}
			if cfg.Docker != nil && cfg.Docker.Image != "" {
				t := cfg.Docker.DefaultTag
				if t == "" {
					t = "latest"
				}
				return cfg.Docker.Image, t
			}
		}
	}
	return "", ""
}

// queryRegistryTags fetches available tags from a container registry.
// Supports ghcr.io and Docker Hub via the OCI distribution API.
func queryRegistryTags(ctx context.Context, image string) ([]string, error) {
	// Parse image into registry + repository
	registry, repo := parseImageRef(image)

	var tagsURL string
	switch {
	case strings.Contains(registry, "ghcr.io"):
		tagsURL = fmt.Sprintf("https://ghcr.io/v2/%s/tags/list", repo)
	case registry == "docker.io" || registry == "":
		// Docker Hub
		if !strings.Contains(repo, "/") {
			repo = "library/" + repo
		}
		tagsURL = fmt.Sprintf("https://registry-1.docker.io/v2/%s/tags/list", repo)
	default:
		tagsURL = fmt.Sprintf("https://%s/v2/%s/tags/list", registry, repo)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, tagsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// For ghcr.io public images, anonymous access works with this Accept header.
	req.Header.Set("Accept", "application/json")

	// For Docker Hub, we may need a token. Try anonymous first.
	if strings.Contains(registry, "docker.io") || registry == "" {
		// Get anonymous token for Docker Hub
		tokenURL := fmt.Sprintf("https://auth.docker.io/token?service=registry.docker.io&scope=repository:%s:pull", repo)
		tokenReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, tokenURL, nil)
		tokenResp, err := client.Do(tokenReq)
		if err == nil {
			defer tokenResp.Body.Close()
			var tokenData struct {
				Token string `json:"token"`
			}
			if json.NewDecoder(tokenResp.Body).Decode(&tokenData) == nil && tokenData.Token != "" {
				req.Header.Set("Authorization", "Bearer "+tokenData.Token)
			}
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching tags: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned %d", resp.StatusCode)
	}

	var result struct {
		Tags []string `json:"tags"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decoding tags: %w", err)
	}

	return result.Tags, nil
}

// parseImageRef splits an image reference like "ghcr.io/org/repo" into
// registry ("ghcr.io") and repository ("org/repo").
func parseImageRef(image string) (registry, repo string) {
	// Remove tag if present
	if idx := strings.LastIndex(image, ":"); idx > 0 {
		image = image[:idx]
	}

	parts := strings.SplitN(image, "/", 2)
	if len(parts) == 1 {
		// e.g., "nginx" → Docker Hub library
		return "docker.io", parts[0]
	}

	// Check if first part looks like a registry (contains a dot or is localhost)
	if strings.Contains(parts[0], ".") || strings.Contains(parts[0], ":") || parts[0] == "localhost" {
		return parts[0], parts[1]
	}

	// e.g., "user/repo" → Docker Hub
	return "docker.io", image
}

// findLatestTag returns the highest semver tag from the list, or empty if
// the current tag is already the latest. Non-semver tags are ignored.
func findLatestTag(tags []string) string {
	type semver struct {
		major, minor, patch int
		original            string
	}

	parseSemver := func(tag string) (semver, bool) {
		t := strings.TrimPrefix(tag, "v")
		var s semver
		n, _ := fmt.Sscanf(t, "%d.%d.%d", &s.major, &s.minor, &s.patch)
		if n >= 2 {
			s.original = tag
			return s, true
		}
		return semver{}, false
	}

	compareSemver := func(a, b semver) int {
		if a.major != b.major {
			return a.major - b.major
		}
		if a.minor != b.minor {
			return a.minor - b.minor
		}
		return a.patch - b.patch
	}

	var best semver
	var found bool

	for _, tag := range tags {
		sv, ok := parseSemver(tag)
		if !ok {
			continue
		}
		if !found || compareSemver(sv, best) > 0 {
			best = sv
			found = true
		}
	}

	if !found {
		return ""
	}
	return best.original
}

// ─── Route Registration ──────────────────────────────────────────────────

// RegisterAddonRoutes registers all add-on API routes.
func RegisterAddonRoutes(mux *http.ServeMux, protect func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("POST /api/addons", protect(RegisterAddon))
	mux.HandleFunc("GET /api/addons", protect(ListAddons))
	mux.HandleFunc("GET /api/addons/{id}", protect(GetAddon))
	mux.HandleFunc("GET /api/addons/{id}/proxy", protect(ProxyAddonRequest))
	mux.HandleFunc("DELETE /api/addons/{id}", protect(DeregisterAddon))
	mux.HandleFunc("PUT /api/addons/{id}/enabled", protect(SetAddonEnabled))
	mux.HandleFunc("GET /api/addons/{id}/telemetry", protect(AddonTelemetrySSE))
	mux.HandleFunc("GET /api/addons/{id}/check-updates", protect(CheckAddonUpdates))

	// Admin UI registration flow
	mux.HandleFunc("POST /api/addons/register", protect(CreateAddonFromUI))

	// Token management
	mux.HandleFunc("POST /api/addons/tokens", protect(CreateAddonToken))
	mux.HandleFunc("GET /api/addons/tokens", protect(ListAddonTokens))
	mux.HandleFunc("DELETE /api/addons/tokens/{id}", protect(DeleteAddonToken))

	// Add-on self-registration — NOT behind protect (uses registration token auth)
	mux.HandleFunc("POST /api/addons/connect", ConnectAddon)

	// WebSocket telemetry ingestion — add-ons connect here to stream data
	if WebSocketHub != nil {
		mux.HandleFunc("GET /api/addons/ws", WebSocketHub.HandleConnection)
	}
}
