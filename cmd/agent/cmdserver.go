package main

import (
	"encoding/json"
	"log"
	"net/http"

	"vigil/cmd/agent/led"
)

// startCommandServer runs a minimal HTTP server that accepts commands
// from the Vigil server (proxied through the dashboard). This is
// optional — only started when --listen or AGENT_LISTEN is set.
func startCommandServer(addr string, ledCtrl *led.Controller) {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`)) //nolint:errcheck
	})

	mux.HandleFunc("POST /api/identify", handleIdentify(ledCtrl))

	server := &http.Server{Addr: addr, Handler: mux}
	if err := server.ListenAndServe(); err != nil {
		log.Printf("⚠️  Command server error: %v", err)
	}
}

func handleIdentify(ledCtrl *led.Controller) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !ledCtrl.Available() {
			http.Error(w, `{"error":"LED identification not available on this host"}`, http.StatusNotImplemented)
			return
		}

		var req struct {
			Device string `json:"device"`
			Mode   string `json:"mode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, `{"error":"invalid request body"}`, http.StatusBadRequest)
			return
		}
		if req.Device == "" {
			http.Error(w, `{"error":"device is required"}`, http.StatusBadRequest)
			return
		}
		if req.Mode == "" {
			req.Mode = "locate"
		}

		log.Printf("💡 LED identify: device=%s mode=%s", req.Device, req.Mode)
		output, err := ledCtrl.Identify(req.Device, req.Mode)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": err.Error(), "output": output}) //nolint:errcheck
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "output": output}) //nolint:errcheck
	}
}
