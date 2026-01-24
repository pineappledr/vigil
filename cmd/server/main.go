package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

var db *sql.DB

func initDB() {
	var err error
	// Create/Open the database file
	db, err = sql.Open("sqlite", "vigil.db")
	if err != nil {
		log.Fatal(err)
	}

	// Create table if it doesn't exist
	query := `
	CREATE TABLE IF NOT EXISTS reports (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		hostname TEXT,
		timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
		data JSON
	);`
	
	if _, err := db.Exec(query); err != nil {
		log.Fatal("Failed to create table:", err)
	}
	fmt.Println("✅ Database connected (vigil.db)")
}

func main() {
	initDB()
	defer db.Close()

	mux := http.NewServeMux()

	// 1. Health Check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Vigil Server is Online 👁️"))
	})

	// 2. The Collector Endpoint (Saves to DB)
	mux.HandleFunc("POST /api/report", func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		hostname := fmt.Sprintf("%v", payload["hostname"])
		jsonData, _ := json.Marshal(payload)

		// Insert into DB
		_, err := db.Exec("INSERT INTO reports (hostname, data) VALUES (?, ?)", hostname, string(jsonData))
		if err != nil {
			log.Printf("❌ DB Error: %v", err)
			http.Error(w, "Database Error", http.StatusInternalServerError)
			return
		}

		fmt.Printf("💾 Saved report from %s at %s\n", hostname, time.Now().Format(time.RFC3339))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Saved"))
	})

	// 3. History Endpoint (To test if saving works)
	mux.HandleFunc("GET /api/history", func(w http.ResponseWriter, r *http.Request) {
		rows, err := db.Query("SELECT id, hostname, timestamp FROM reports ORDER BY id DESC LIMIT 10")
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		defer rows.Close()

		var history []string
		for rows.Next() {
			var id int
			var host, time string
			rows.Scan(&id, &host, &time)
			history = append(history, fmt.Sprintf("ID: %d | Host: %s | Time: %s", id, host, time))
		}
		
		json.NewEncoder(w).Encode(history)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	fmt.Printf("Vigil Server listening on port %s...\n", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}