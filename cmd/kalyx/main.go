package main

import (
	"context"
	"fmt"
	"log/slog"
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

	ctx := context.Background()
	store, err := storage.NewFromEnv(ctx)
	if err != nil {
		slog.Error("Failed to initialize storage", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	server := webserver.NewServer(store)

	addr := fmt.Sprintf(":%s", port)
	slog.Info("🌸 Kalyx Server starting", "url", fmt.Sprintf("http://localhost:%s", port), "mode", "Zero-Dependency")
	slog.Info("📖 CloudEvents Contract & Schema Registry ready")

	if err := http.ListenAndServe(addr, server); err != nil {
		slog.Error("Server failed to start", "error", err)
		os.Exit(1)
	}
}

