package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"vigil/internal/db"
)

// ─── ZFS Pool Endpoints ──────────────────────────────────────────────────────

// ZFSPools returns all ZFS pools or pools for a specific hostname
// GET /api/zfs/pools
// GET /api/zfs/pools?hostname=server1
func ZFSPools(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")

	var pools []db.ZFSPool
	var err error

	if hostname != "" {
		pools, err = db.GetZFSPoolsByHostname(hostname)
	} else {
		pools, err = db.GetAllZFSPools()
	}

	if err != nil {
		log.Printf("❌ Failed to get ZFS pools: %v", err)
		JSONError(w, "Failed to retrieve ZFS pools", http.StatusInternalServerError)
		return
	}

	if pools == nil {
		pools = []db.ZFSPool{}
	}

	JSONResponse(w, pools)
}

// ZFSPool returns a single ZFS pool with its devices
// GET /api/zfs/pools/{hostname}/{poolname}
func ZFSPool(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	poolName := r.PathValue("poolname")

	if hostname == "" || poolName == "" {
		JSONError(w, "Missing hostname or pool name", http.StatusBadRequest)
		return
	}

	pool, err := db.GetZFSPool(hostname, poolName)
	if err != nil {
		log.Printf("❌ Failed to get ZFS pool: %v", err)
		JSONError(w, "Failed to retrieve ZFS pool", http.StatusInternalServerError)
		return
	}

	if pool == nil {
		JSONError(w, "Pool not found", http.StatusNotFound)
		return
	}

	// Get devices for this pool
	devices, err := db.GetZFSPoolDevices(pool.ID)
	if err != nil {
		log.Printf("⚠️  Failed to get pool devices: %v", err)
		devices = []db.ZFSPoolDevice{}
	}

	// Get recent scrub history
	scrubHistory, err := db.GetZFSScrubHistory(pool.ID, 5)
	if err != nil {
		log.Printf("⚠️  Failed to get scrub history: %v", err)
		scrubHistory = []db.ZFSScrubHistory{}
	}

	response := map[string]interface{}{
		"pool":          pool,
		"devices":       devices,
		"scrub_history": scrubHistory,
	}

	JSONResponse(w, response)
}

// ZFSPoolSummary returns aggregate ZFS stats
// GET /api/zfs/summary
// GET /api/zfs/summary?hostname=server1
func ZFSPoolSummary(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")

	var summary *db.ZFSPoolSummary
	var err error

	if hostname != "" {
		summary, err = db.GetZFSPoolSummary(hostname)
	} else {
		summary, err = db.GetGlobalZFSSummary()
	}

	if err != nil {
		log.Printf("❌ Failed to get ZFS summary: %v", err)
		JSONError(w, "Failed to retrieve ZFS summary", http.StatusInternalServerError)
		return
	}

	JSONResponse(w, summary)
}

// ─── ZFS Device Endpoints ────────────────────────────────────────────────────

// ZFSPoolDevices returns devices for a specific pool
// GET /api/zfs/pools/{hostname}/{poolname}/devices
func ZFSPoolDevices(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	poolName := r.PathValue("poolname")

	if hostname == "" || poolName == "" {
		JSONError(w, "Missing hostname or pool name", http.StatusBadRequest)
		return
	}

	pool, err := db.GetZFSPool(hostname, poolName)
	if err != nil {
		log.Printf("❌ Failed to get ZFS pool: %v", err)
		JSONError(w, "Failed to retrieve ZFS pool", http.StatusInternalServerError)
		return
	}

	if pool == nil {
		JSONError(w, "Pool not found", http.StatusNotFound)
		return
	}

	devices, err := db.GetZFSPoolDevices(pool.ID)
	if err != nil {
		log.Printf("❌ Failed to get pool devices: %v", err)
		JSONError(w, "Failed to retrieve pool devices", http.StatusInternalServerError)
		return
	}

	if devices == nil {
		devices = []db.ZFSPoolDevice{}
	}

	JSONResponse(w, devices)
}

// ZFSDeviceBySerial returns a ZFS device by its serial number
// GET /api/zfs/devices/serial/{hostname}/{serial}
func ZFSDeviceBySerial(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	serial := r.PathValue("serial")

	if hostname == "" || serial == "" {
		JSONError(w, "Missing hostname or serial number", http.StatusBadRequest)
		return
	}

	device, err := db.GetZFSDeviceBySerial(hostname, serial)
	if err != nil {
		log.Printf("❌ Failed to get ZFS device: %v", err)
		JSONError(w, "Failed to retrieve ZFS device", http.StatusInternalServerError)
		return
	}

	if device == nil {
		JSONError(w, "Device not found", http.StatusNotFound)
		return
	}

	// Also get the pool info for context
	pool, _ := db.GetZFSPoolByID(device.PoolID)

	response := map[string]interface{}{
		"device": device,
		"pool":   pool,
	}

	JSONResponse(w, response)
}

// ─── ZFS Scrub History Endpoints ─────────────────────────────────────────────

// ZFSScrubHistory returns scrub history for a pool
// GET /api/zfs/pools/{hostname}/{poolname}/scrubs
// GET /api/zfs/pools/{hostname}/{poolname}/scrubs?limit=10
func ZFSScrubHistory(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	poolName := r.PathValue("poolname")

	if hostname == "" || poolName == "" {
		JSONError(w, "Missing hostname or pool name", http.StatusBadRequest)
		return
	}

	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}

	pool, err := db.GetZFSPool(hostname, poolName)
	if err != nil {
		log.Printf("❌ Failed to get ZFS pool: %v", err)
		JSONError(w, "Failed to retrieve ZFS pool", http.StatusInternalServerError)
		return
	}

	if pool == nil {
		JSONError(w, "Pool not found", http.StatusNotFound)
		return
	}

	history, err := db.GetZFSScrubHistory(pool.ID, limit)
	if err != nil {
		log.Printf("❌ Failed to get scrub history: %v", err)
		JSONError(w, "Failed to retrieve scrub history", http.StatusInternalServerError)
		return
	}

	if history == nil {
		history = []db.ZFSScrubHistory{}
	}

	JSONResponse(w, history)
}

// ZFSLastScrub returns the most recent scrub for a pool
// GET /api/zfs/pools/{hostname}/{poolname}/scrubs/last
func ZFSLastScrub(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	poolName := r.PathValue("poolname")

	if hostname == "" || poolName == "" {
		JSONError(w, "Missing hostname or pool name", http.StatusBadRequest)
		return
	}

	pool, err := db.GetZFSPool(hostname, poolName)
	if err != nil {
		log.Printf("❌ Failed to get ZFS pool: %v", err)
		JSONError(w, "Failed to retrieve ZFS pool", http.StatusInternalServerError)
		return
	}

	if pool == nil {
		JSONError(w, "Pool not found", http.StatusNotFound)
		return
	}

	lastScrub, err := db.GetLastScrub(pool.ID)
	if err != nil {
		log.Printf("❌ Failed to get last scrub: %v", err)
		JSONError(w, "Failed to retrieve last scrub", http.StatusInternalServerError)
		return
	}

	if lastScrub == nil {
		JSONResponse(w, map[string]interface{}{
			"message": "No scrub history available",
		})
		return
	}

	JSONResponse(w, lastScrub)
}

// ─── ZFS Pool Management Endpoints ───────────────────────────────────────────

// DeleteZFSPool removes a ZFS pool from the database
// DELETE /api/zfs/pools/{hostname}/{poolname}
func DeleteZFSPool(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	poolName := r.PathValue("poolname")

	if hostname == "" || poolName == "" {
		JSONError(w, "Missing hostname or pool name", http.StatusBadRequest)
		return
	}

	// Check if pool exists
	pool, err := db.GetZFSPool(hostname, poolName)
	if err != nil {
		log.Printf("❌ Failed to check ZFS pool: %v", err)
		JSONError(w, "Database error", http.StatusInternalServerError)
		return
	}

	if pool == nil {
		JSONError(w, "Pool not found", http.StatusNotFound)
		return
	}

	// Delete the pool (cascades to devices and scrub history)
	if err := db.DeleteZFSPool(hostname, poolName); err != nil {
		log.Printf("❌ Failed to delete ZFS pool: %v", err)
		JSONError(w, "Failed to delete pool", http.StatusInternalServerError)
		return
	}

	log.Printf("🗑️  Deleted ZFS pool: %s/%s", hostname, poolName)
	JSONResponse(w, map[string]string{
		"status":   "deleted",
		"pool":     poolName,
		"hostname": hostname,
	})
}

// ─── ZFS Health Check Endpoint ───────────────────────────────────────────────

// ZFSHealthCheck returns pools that need attention (degraded, faulted, errors)
// GET /api/zfs/health
// GET /api/zfs/health?hostname=server1
func ZFSHealthCheck(w http.ResponseWriter, r *http.Request) {
	hostname := r.URL.Query().Get("hostname")

	var pools []db.ZFSPool
	var err error

	if hostname != "" {
		pools, err = db.GetZFSPoolsByHostname(hostname)
	} else {
		pools, err = db.GetAllZFSPools()
	}

	if err != nil {
		log.Printf("❌ Failed to get ZFS pools: %v", err)
		JSONError(w, "Failed to retrieve ZFS pools", http.StatusInternalServerError)
		return
	}

	// Filter to pools that need attention
	var needsAttention []map[string]interface{}
	for _, pool := range pools {
		issues := []string{}

		// Check health status
		if pool.Health == "DEGRADED" {
			issues = append(issues, "Pool is degraded")
		} else if pool.Health == "FAULTED" {
			issues = append(issues, "Pool is faulted")
		} else if pool.Health != "ONLINE" {
			issues = append(issues, "Pool status: "+pool.Health)
		}

		// Check for errors
		if pool.ReadErrors > 0 {
			issues = append(issues, "Read errors detected")
		}
		if pool.WriteErrors > 0 {
			issues = append(issues, "Write errors detected")
		}
		if pool.ChecksumErrors > 0 {
			issues = append(issues, "Checksum errors detected")
		}

		// Check capacity
		if pool.CapacityPct >= 90 {
			issues = append(issues, "Capacity above 90%")
		} else if pool.CapacityPct >= 80 {
			issues = append(issues, "Capacity above 80%")
		}

		// Check fragmentation
		if pool.Fragmentation >= 75 {
			issues = append(issues, "High fragmentation")
		}

		if len(issues) > 0 {
			needsAttention = append(needsAttention, map[string]interface{}{
				"hostname":     pool.Hostname,
				"pool_name":    pool.PoolName,
				"health":       pool.Health,
				"capacity":     pool.CapacityPct,
				"issues":       issues,
				"total_errors": pool.ReadErrors + pool.WriteErrors + pool.ChecksumErrors,
			})
		}
	}

	if needsAttention == nil {
		needsAttention = []map[string]interface{}{}
	}

	response := map[string]interface{}{
		"total_pools":     len(pools),
		"needs_attention": len(needsAttention),
		"pools":           needsAttention,
	}

	JSONResponse(w, response)
}

// ─── ZFS Drive Cross-Reference Endpoint ──────────────────────────────────────

// ZFSDriveInfo returns ZFS info for a drive by serial number
// GET /api/zfs/drive/{hostname}/{serial}
// This endpoint allows cross-referencing SMART data with ZFS pool membership
func ZFSDriveInfo(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	serial := r.PathValue("serial")

	if hostname == "" || serial == "" {
		JSONError(w, "Missing hostname or serial number", http.StatusBadRequest)
		return
	}

	device, err := db.GetZFSDeviceBySerial(hostname, serial)
	if err != nil {
		log.Printf("❌ Failed to get ZFS device: %v", err)
		JSONError(w, "Failed to retrieve ZFS device info", http.StatusInternalServerError)
		return
	}

	if device == nil {
		// Drive is not part of any ZFS pool
		JSONResponse(w, map[string]interface{}{
			"in_zfs_pool": false,
			"hostname":    hostname,
			"serial":      serial,
		})
		return
	}

	// Get pool info
	pool, _ := db.GetZFSPoolByID(device.PoolID)

	response := map[string]interface{}{
		"in_zfs_pool":     true,
		"hostname":        hostname,
		"serial":          serial,
		"pool_name":       device.PoolName,
		"device_name":     device.DeviceName,
		"vdev_type":       device.VdevType,
		"device_state":    device.State,
		"read_errors":     device.ReadErrors,
		"write_errors":    device.WriteErrors,
		"checksum_errors": device.ChecksumErrors,
	}

	if pool != nil {
		response["pool_health"] = pool.Health
		response["pool_status"] = pool.Status
	}

	JSONResponse(w, response)
}

// ─── Report Handler Update ───────────────────────────────────────────────────

// ProcessZFSFromReport extracts and processes ZFS data from an agent report
// This should be called from the main Report handler
func ProcessZFSFromReport(hostname string, payload map[string]interface{}) {
	zfsData, ok := payload["zfs"]
	if !ok || zfsData == nil {
		return
	}

	// Convert to JSON for processing
	zfsJSON, err := json.Marshal(zfsData)
	if err != nil {
		log.Printf("⚠️  Failed to marshal ZFS data: %v", err)
		return
	}

	if err := db.ProcessZFSReport(hostname, zfsJSON); err != nil {
		log.Printf("⚠️  Failed to process ZFS report: %v", err)
	}
}

// ─── Route Registration Helper ───────────────────────────────────────────────

// RegisterZFSRoutes registers all ZFS API routes
// Call this from your main router setup
func RegisterZFSRoutes(mux *http.ServeMux, authMiddleware func(http.HandlerFunc) http.HandlerFunc) {
	// Pool endpoints
	mux.HandleFunc("GET /api/zfs/pools", authMiddleware(ZFSPools))
	mux.HandleFunc("GET /api/zfs/pools/{hostname}/{poolname}", authMiddleware(ZFSPool))
	mux.HandleFunc("DELETE /api/zfs/pools/{hostname}/{poolname}", authMiddleware(DeleteZFSPool))

	// Device endpoints
	mux.HandleFunc("GET /api/zfs/pools/{hostname}/{poolname}/devices", authMiddleware(ZFSPoolDevices))
	mux.HandleFunc("GET /api/zfs/devices/serial/{hostname}/{serial}", authMiddleware(ZFSDeviceBySerial))

	// Scrub history endpoints
	mux.HandleFunc("GET /api/zfs/pools/{hostname}/{poolname}/scrubs", authMiddleware(ZFSScrubHistory))
	mux.HandleFunc("GET /api/zfs/pools/{hostname}/{poolname}/scrubs/last", authMiddleware(ZFSLastScrub))

	// Summary and health endpoints
	mux.HandleFunc("GET /api/zfs/summary", authMiddleware(ZFSPoolSummary))
	mux.HandleFunc("GET /api/zfs/health", authMiddleware(ZFSHealthCheck))

	// Drive cross-reference endpoint
	mux.HandleFunc("GET /api/zfs/drive/{hostname}/{serial}", authMiddleware(ZFSDriveInfo))
}
