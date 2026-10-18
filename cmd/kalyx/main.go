package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/alexagc/kalyx/internal/storage"
	"github.com/alexagc/kalyx/internal/webserver"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	store := storage.NewMemoryStorage()
	server := webserver.NewServer(store)

	addr := fmt.Sprintf(":%s", port)
	log.Printf("🌸 Kalyx Server starting on http://localhost:%s (Zero-Dependency Mode)", port)
	log.Printf("📖 CloudEvents Contract & Schema Registry ready")

	if err := http.ListenAndServe(addr, server); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
