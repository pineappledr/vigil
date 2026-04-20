package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"

	"vigil/internal/auth"
	"vigil/internal/db"
	"vigil/internal/health"
	"vigil/internal/models"
	"vigil/internal/wearout"
	"vigil/internal/zfs"
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

// indexHTML caches the raw index.html template and the injection marker position.
var indexHTML struct {
	once     sync.Once
	raw      []byte   // original file bytes
	marker   int      // byte offset of </head>
	loadErr  error
}

func loadIndex() {
	indexHTML.raw, indexHTML.loadErr = os.ReadFile("./web/index.html")
	if indexHTML.loadErr != nil {
		return
	}
	indexHTML.marker = strings.Index(string(indexHTML.raw), "</head>")
	if indexHTML.marker < 0 {
		indexHTML.marker = 0 // fallback: prepend (harmless)
	}
}

// preloadJSON builds a JSON blob with the same data the dashboard would
// fetch via separate API calls, so the first paint has data immediately.
func preloadJSON() string {
	type preload struct {
		History []map[string]interface{} `json:"history"`
		ZFS     interface{}              `json:"zfs,omitempty"`
		Wearout interface{}              `json:"wearout,omitempty"`
		Health  interface{}              `json:"health,omitempty"`
	}
	p := preload{History: make([]map[string]interface{}, 0)}

	// ── History ─────────────────────────────────────────────────────────
	aliases := loadAliases()
	rows, err := db.DB.Query(`
		SELECT r.hostname, r.timestamp, r.data,
		       COALESCE(ag.last_seen, r.timestamp) AS last_seen
		FROM reports r
		INNER JOIN (
			SELECT hostname, MAX(id) AS max_id
			FROM reports
			GROUP BY hostname
		) latest ON r.id = latest.max_id
		LEFT JOIN (
			SELECT hostname, MAX(last_seen_at) AS last_seen
			FROM agent_registry
			WHERE enabled = 1
			GROUP BY hostname
		) ag ON LOWER(ag.hostname) = LOWER(r.hostname)
		ORDER BY r.timestamp DESC`)
	if err != nil {
		log.Printf("[preload] history query: %v", err)
		b, _ := json.Marshal(p)
		return string(b)
	}
	defer rows.Close()

	for rows.Next() {
		var host, ts, lastSeen string
		var dataRaw []byte
		if err := rows.Scan(&host, &ts, &dataRaw, &lastSeen); err != nil {
			continue
		}
		var dataMap map[string]interface{}
		if err := json.Unmarshal(dataRaw, &dataMap); err != nil {
			continue
		}
		enrichDrivesWithAliases(dataMap, host, aliases)
		p.History = append(p.History, map[string]interface{}{
			"hostname":  host,
			"timestamp": ts,
			"last_seen": lastSeen,
			"details":   dataMap,
		})
	}

	// ── ZFS Pools ───────────────────────────────────────────────────────
	if pools, err := zfs.GetAllZFSPools(db.DB); err == nil && len(pools) > 0 {
		response := make([]ZFSPoolWithCount, len(pools))
		for i, pool := range pools {
			response[i].ZFSPool = pool
			count, err := zfs.CountZFSDisks(db.DB, pool.ID)
			if err != nil {
				count = 0
			}
			response[i].DeviceCount = count
		}
		p.ZFS = response
	}

	// ── Wearout ─────────────────────────────────────────────────────────
	if snapshots, err := wearout.GetAllLatestSnapshots(db.DB); err == nil {
		p.Wearout = map[string]interface{}{
			"drives": snapshots,
			"count":  len(snapshots),
		}
	}

	// ── Health Score ────────────────────────────────────────────────────
	if score, err := health.Calculate(db.DB); err == nil {
		p.Health = score
	}

	b, _ := json.Marshal(p)
	return string(b)
}

// serveIndex injects preloaded data into index.html so the first paint is instant.
func serveIndex(w http.ResponseWriter, _ *http.Request) {
	indexHTML.once.Do(loadIndex)
	if indexHTML.loadErr != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}

	data := preloadJSON()
	script := "<script>window.__VIGIL_PRELOAD__=" + data + ";</script>"

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(indexHTML.raw[:indexHTML.marker])
	w.Write([]byte(script))
	w.Write(indexHTML.raw[indexHTML.marker:])
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
		if config.AuthEnabled && !auth.IsAuthenticated(r) {
			http.Redirect(w, r, "/login.html", http.StatusFound)
			return
		}

		// Serve index.html with preloaded data for instant first paint
		if r.URL.Path == "/" || r.URL.Path == "/index.html" {
			serveIndex(w, r)
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
