package webserver

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/alexagc/kalyx/internal/compatibility"
	"github.com/alexagc/kalyx/internal/core"
	"github.com/alexagc/kalyx/internal/datalake"
	"github.com/alexagc/kalyx/internal/storage"
)

// Server handles HTTP API requests and the embedded web frontend.
type Server struct {
	store storage.Storage
	mux   *http.ServeMux
}

// NewServer initializes the HTTP server routes.
func NewServer(store storage.Storage) *Server {
	s := &Server{
		store: store,
		mux:   http.NewServeMux(),
	}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.HandleFunc("/api/v1/events", s.handleEvents)
	s.mux.HandleFunc("/api/v1/compatibility/check", s.handleCompatibilityCheck)
	s.mux.HandleFunc("/api/v1/events/", s.handleEventSubroutes)
	s.mux.HandleFunc("/", s.handleWebDashboard)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "healthy",
		"service": "kalyx-server",
		"version": "0.1.0",
	})
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	switch r.Method {
	case http.MethodGet:
		events, err := s.store.ListEvents(r.Context())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(events)

	case http.MethodPost:
		var ev core.EventDefinition
		if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if ev.ID == "" || ev.Name == "" || ev.Type == "" {
			http.Error(w, "missing required fields: id, name, type", http.StatusBadRequest)
			return
		}
		if err := s.store.CreateEvent(r.Context(), ev); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ev)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleEventSubroutes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Parse /api/v1/events/{id}...
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/events/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}

	eventID := parts[0]

	// GET /api/v1/events/{id}
	if len(parts) == 1 {
		ev, err := s.store.GetEvent(r.Context(), eventID)
		if err != nil {
			http.Error(w, "event not found", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(ev)
		return
	}

	// Subroutes: /api/v1/events/{id}/versions
	if parts[1] == "versions" {
		if len(parts) == 2 {
			versions, err := s.store.ListVersions(r.Context(), eventID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(versions)
			return
		}

		semver := parts[2]
		version, err := s.store.GetEventVersion(r.Context(), eventID, semver)
		if err != nil {
			http.Error(w, "version not found", http.StatusNotFound)
			return
		}

		// Check for /ddl endpoint: /api/v1/events/{id}/versions/{semver}/ddl
		if len(parts) >= 4 && parts[3] == "ddl" {
			tableName := version.DataLakeMetadata.TargetTable
			if tableName == "" {
				tableName = strings.ReplaceAll(eventID, "-", "_")
			}
			ddl, err := datalake.GenerateIcebergDDL(tableName, version.JSONSchema, version.DataLakeMetadata.PartitionBy)
			if err != nil {
				http.Error(w, fmt.Sprintf("failed to generate DDL: %v", err), http.StatusInternalServerError)
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]string{
				"table":  tableName,
				"format": "iceberg",
				"ddl":    ddl,
			})
			return
		}

		_ = json.NewEncoder(w).Encode(version)
		return
	}

	http.NotFound(w, r)
}

func (s *Server) handleCompatibilityCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		OldSchema string `json:"old_schema"`
		NewSchema string `json:"new_schema"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	res, err := compatibility.CheckCompatibility(req.OldSchema, req.NewSchema)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleWebDashboard(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	// Embedded modern dashboard HTML out of the box
	html := `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Kalyx 🌸 CloudEvents Contract Hub</title>
  <script src="https://cdn.tailwindcss.com"></script>
  <style>
    body { background-color: #0d1117; color: #c9d1d9; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; }
  </style>
</head>
<body class="p-8 max-w-6xl mx-auto">
  <header class="flex justify-between items-center pb-6 border-b border-gray-800">
    <div class="flex items-center space-x-3">
      <span class="text-3xl">🌸</span>
      <div>
        <h1 class="text-2xl font-bold text-white tracking-tight">Kalyx</h1>
        <p class="text-sm text-gray-400">CloudEvents Contract & Schema Registry for Data Lakes</p>
      </div>
    </div>
    <div class="flex space-x-3">
      <span class="inline-flex items-center px-3 py-1 rounded-full text-xs font-medium bg-emerald-950 text-emerald-400 border border-emerald-800">
        ● Zero Dependencies (Active)
      </span>
      <a href="https://github.com/alexagc/kalyx" target="_blank" class="px-4 py-1.5 rounded-lg text-sm bg-gray-800 hover:bg-gray-700 text-white font-medium border border-gray-700 transition">
        GitHub
      </a>
    </div>
  </header>

  <main class="mt-8 space-y-8">
    <section class="grid grid-cols-1 md:grid-cols-3 gap-6">
      <div class="p-5 rounded-xl bg-gray-900 border border-gray-800">
        <h3 class="text-sm font-semibold text-gray-400">Standard</h3>
        <p class="text-xl font-bold text-white mt-1">CNCF CloudEvents 1.0.2</p>
      </div>
      <div class="p-5 rounded-xl bg-gray-900 border border-gray-800">
        <h3 class="text-sm font-semibold text-gray-400">Schema Engine</h3>
        <p class="text-xl font-bold text-white mt-1">JSON Schema 2020-12</p>
      </div>
      <div class="p-5 rounded-xl bg-gray-900 border border-gray-800">
        <h3 class="text-sm font-semibold text-gray-400">Data Lake Bridge</h3>
        <p class="text-xl font-bold text-white mt-1">Apache Iceberg DDL</p>
      </div>
    </section>

    <section class="rounded-xl bg-gray-900 border border-gray-800 p-6">
      <div class="flex justify-between items-center mb-4">
        <h2 class="text-lg font-bold text-white">Event Contracts Catalog</h2>
        <span class="text-xs text-gray-400">Live API: <code class="text-sky-400">/api/v1/events</code></span>
      </div>
      <div class="overflow-x-auto">
        <table class="w-full text-left border-collapse text-sm">
          <thead>
            <tr class="border-b border-gray-800 text-gray-400">
              <th class="py-3 px-4">Event Name</th>
              <th class="py-3 px-4">Domain</th>
              <th class="py-3 px-4">CloudEvent Type</th>
              <th class="py-3 px-4">Owner</th>
              <th class="py-3 px-4">Status</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-800">
            <tr class="hover:bg-gray-800/50 transition">
              <td class="py-3 px-4 font-semibold text-white">OrderCompleted</td>
              <td class="py-3 px-4 text-gray-300"><span class="px-2 py-0.5 rounded bg-gray-800 text-xs">Orders</span></td>
              <td class="py-3 px-4 font-mono text-xs text-sky-400">com.company.orders.completed.v1</td>
              <td class="py-3 px-4 text-gray-400">Checkout Core</td>
              <td class="py-3 px-4"><span class="px-2 py-0.5 rounded-full text-xs font-semibold bg-emerald-950 text-emerald-400 border border-emerald-800">v1.0.0 Approved</span></td>
            </tr>
          </tbody>
        </table>
      </div>
    </section>
  </main>
</body>
</html>`
	_, _ = w.Write([]byte(html))
}
