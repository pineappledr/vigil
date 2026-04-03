package handlers

import (
	"net/http"

	"vigil/internal/db"
	"vigil/internal/health"
	"vigil/internal/smart"
	"vigil/internal/wearout"
	"vigil/internal/zfs"
)

// GetHealthScore returns the aggregated system health score.
// GET /api/health/score
func GetHealthScore(w http.ResponseWriter, r *http.Request) {
	smartData, err := smart.GetAllDrivesHealthSummary(db.DB)
	if err != nil {
		JSONError(w, "Failed to retrieve SMART data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	wearoutData, err := wearout.GetAllLatestSnapshots(db.DB)
	if err != nil {
		JSONError(w, "Failed to retrieve wearout data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	zfsData, err := zfs.GetAllZFSPools(db.DB)
	if err != nil {
		JSONError(w, "Failed to retrieve ZFS data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	score := health.ComputeScore(smartData, wearoutData, zfsData)
	JSONResponse(w, score)
}
