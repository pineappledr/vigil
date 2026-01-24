package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	// Use modern Go routing
	mux := http.NewServeMux()

	// 1. Health Check (To test if server is up)
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Vigil Server is Online 👁️"))
	})

	// 2. The Collector Endpoint (Where Proxmox sends data)
	mux.HandleFunc("POST /api/report", func(w http.ResponseWriter, r *http.Request) {
		// For now, just decode the raw JSON and print it to the console
		var payload map[string]interface{}
		
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		// Print what we received (Proof of Life)
		fmt.Println("------------- NEW REPORT RECEIVED -------------")
		fmt.Printf("Agent ID: %v\n", payload["hostname"])
		fmt.Printf("Data: %v\n", payload)
		fmt.Println("-----------------------------------------------")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ack"))
	})

	// 3. Start the Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	fmt.Printf("Vigil Server listening on port %s...\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}