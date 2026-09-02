package handlers

import (
	"net/http"

	"vigil/internal/agents"
	"vigil/internal/db"
	"vigil/internal/health"
	"vigil/internal/smart"
	"vigil/internal/temperature"
	"vigil/internal/zfs"
)

// Ningún disco funciona por encima de esto; un valor mayor es corrupción del
// dato, no un disco ardiendo. Los HDD se apagan solos mucho antes de 100 °C.
const maxPlausibleTempC = 120

// Summary is the compact, read-only view a dashboard needs: how many drives
// are fine, how many are not, how hot the hottest one runs, whether every
// agent is still reporting, and whether any ZFS pool is degraded.
type Summary struct {
	Score  int    `json:"score"`  // 0-100 aggregate health
	Grade  string `json:"grade"`  // Excellent | Good | Fair | Warning | Critical
	Status string `json:"status"` // ok | warning | critical — what a dashboard colours on

	Drives struct {
		Total    int `json:"total"`
		Healthy  int `json:"healthy"`
		Warning  int `json:"warning"`
		Critical int `json:"critical"`
	} `json:"drives"`

	Agents struct {
		Total   int `json:"total"`
		Enabled int `json:"enabled"`
	} `json:"agents"`

	Pools struct {
		Total    int `json:"total"`
		Degraded int `json:"degraded"` // anything not ONLINE
	} `json:"pools"`

	MaxTempC int    `json:"max_temp_c"`
	Version  string `json:"version"`
}

// GetSummary serves GET /api/summary.
//
// Deliberately UNAUTHENTICATED, and that is the whole point of the endpoint.
// Homepage's generic `customapi` widget cannot perform Vigil's cookie login,
// so the only alternatives were forking the Homepage image (which breaks
// Renovate on the official one) or writing a bespoke widget upstream. Serving
// the few aggregate numbers a dashboard needs is far cheaper than either.
//
// It exposes COUNTS ONLY — no hostnames, serials, models, dataset names or
// temperatures per drive. Someone who reaches this endpoint learns "17 drives,
// all healthy", never which machine or which disk. Vigil is LAN-only to begin
// with; this keeps the blast radius of that decision small.
func GetSummary(w http.ResponseWriter, r *http.Request) {
	var s Summary
	s.Version = Version

	if hs, err := health.Calculate(db.DB); err == nil && hs != nil {
		s.Score, s.Grade = hs.Score, hs.Grade
	}

	if summaries, err := smart.GetAllDrivesHealthSummary(db.DB); err == nil {
		s.Drives.Total = len(summaries)
		for _, d := range summaries {
			switch d.OverallHealth {
			case "HEALTHY":
				s.Drives.Healthy++
			case "WARNING":
				s.Drives.Warning++
			case "CRITICAL":
				s.Drives.Critical++
			}
		}
	}

	// La temperatura máxima ya la agrega el módulo de temperature; no hace
	// falta recorrer los discos otra vez.
	//
	// Se acota a un rango físicamente posible antes de publicarla. La tabla
	// temperature_history tiene al menos una fila con un valor absurdo
	// (27058405379 el 2026-09-02, cuando ningún disco pasaba de 46 °C), y un
	// dashboard que pinta ese número miente con aplomo. No se arregla el dato
	// aquí — eso es harina de otro costal — pero tampoco se propaga: fuera de
	// rango se reporta 0, que el widget muestra como "sin dato".
	if td, err := temperature.GetDashboardTemperatureData(db.DB, false); err == nil && td != nil {
		if t := td.MaxTemperature; t > 0 && t <= maxPlausibleTempC {
			s.MaxTempC = t
		}
	}

	if list, err := agents.ListAgents(db.DB); err == nil {
		s.Agents.Total = len(list)
		for _, a := range list {
			if a.Enabled {
				s.Agents.Enabled++
			}
		}
	}

	if pools, err := zfs.GetAllZFSPools(db.DB); err == nil {
		s.Pools.Total = len(pools)
		for _, p := range pools {
			if p.Health != "ONLINE" {
				s.Pools.Degraded++
			}
		}
	}

	// One field a dashboard can colour on without re-deriving the rules.
	switch {
	case s.Drives.Critical > 0 || s.Pools.Degraded > 0:
		s.Status = "critical"
	case s.Drives.Warning > 0 || s.Agents.Enabled < s.Agents.Total:
		s.Status = "warning"
	default:
		s.Status = "ok"
	}

	JSONResponse(w, s)
}
