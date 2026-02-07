//go:build ignore

// Simple static file server for Telegram Mini App
// Run: go run webapp/server.go
package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	port := "3000"

	// Serve static files from webapp directory
	fs := http.FileServer(http.Dir("webapp"))

	// Add CORS headers for local development
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		fs.ServeHTTP(w, r)
	})

	fmt.Printf("Serving Mini App at http://localhost:%s\n", port)
	fmt.Println("Use ngrok to expose: ngrok http " + port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}
