package observability

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/alexagc/kalyx/internal/storage"
)

// LivenessHandler provides the Kubernetes /livez probe.
// Returns HTTP 200 OK as long as the server process is alive.
func LivenessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    "alive",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}

// ReadinessHandler provides the Kubernetes /readyz probe.
// Verifies persistent storage availability via store.Ping(ctx).
// Returns HTTP 200 OK when ready, or HTTP 503 Service Unavailable when storage ping fails.
func ReadinessHandler(store storage.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if store == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "not ready",
				"error":  "storage engine is uninitialized",
			})
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()

		if err := store.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "not ready",
				"error":  err.Error(),
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":    "ready",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
		})
	}
}
