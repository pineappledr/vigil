package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
)

func main() {
	mux := http.NewServeMux()

	// 1. Health Check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Vigil Server is Online 👁️"))
	})

	// 2. The Collector Endpoint
	mux.HandleFunc("POST /api/report", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		fmt.Println("------------- NEW REPORT RECEIVED -------------")
		fmt.Printf("Agent ID: %v\n", payload["hostname"])
		fmt.Println("-----------------------------------------------")

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Ack"))
	})

	// 3. Start Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}
	fmt.Printf("Vigil Server listening on port %s...\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}