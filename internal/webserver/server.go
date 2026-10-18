package webserver

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/alexagc/kalyx/internal/auth"
	"github.com/alexagc/kalyx/internal/compatibility"
	"github.com/alexagc/kalyx/internal/core"
	"github.com/alexagc/kalyx/internal/datalake"
	"github.com/alexagc/kalyx/internal/httputil"
	"github.com/alexagc/kalyx/internal/observability"
	"github.com/alexagc/kalyx/internal/storage"
	"github.com/alexagc/kalyx/internal/validator"
)

//go:embed dist/*
var embeddedDist embed.FS

// Server handles HTTP API requests and the embedded web frontend.
type Server struct {
	store     storage.Storage
	validator validator.Validator
	auth      *auth.Middleware
	metrics   *observability.Registry
	mux       *http.ServeMux
}

// ServerOption configures optional behaviors and dependencies for Server.
type ServerOption func(*Server)

// WithAuthMiddleware injects a custom auth middleware instance into the Server.
func WithAuthMiddleware(m *auth.Middleware) ServerOption {
	return func(s *Server) {
		s.auth = m
	}
}

// WithAuthDisabled programmatically configures whether auth is bypassed.
func WithAuthDisabled(disabled bool) ServerOption {
	return func(s *Server) {
		if s.auth != nil {
			s.auth.SetBypass(disabled)
		}
	}
}

// WithOIDCEngine injects an OIDC identity engine for JWT validation.
func WithOIDCEngine(oidc *auth.OIDCEngine) ServerOption {
	return func(s *Server) {
		s.auth = auth.NewMiddleware(s.store, oidc)
	}
}

// WithMetricsRegistry injects a custom Prometheus metrics registry into the Server.
func WithMetricsRegistry(reg *observability.Registry) ServerOption {
	return func(s *Server) {
		s.metrics = reg
	}
}

// NewServer initializes the HTTP server routes and dependencies.
func NewServer(store storage.Storage, opts ...ServerOption) *Server {
	s := &Server{
		store:     store,
		validator: validator.New(),
		mux:       http.NewServeMux(),
		metrics:   observability.DefaultRegistry,
		auth:      auth.NewMiddleware(store, nil),
	}

	for _, opt := range opts {
		opt(s)
	}

	// Sync initial contract count metrics from persistent storage
	_ = s.metrics.SyncContractsFromStorage(context.Background(), store)

	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.metrics.HTTPMiddleware(s.mux).ServeHTTP(w, r)
}

func (s *Server) routes() {
	// Health & Observability probes (unprotected)
	s.mux.HandleFunc("/healthz", s.handleHealth)
	s.mux.HandleFunc("/livez", observability.LivenessHandler())
	s.mux.HandleFunc("/readyz", observability.ReadinessHandler(s.store))
	s.mux.HandleFunc("/metrics", s.metrics.Handler())

	// Embedded Web Dashboard
	s.mux.HandleFunc("/", s.handleWebDashboard)

	// Protected API routes wrapped with AuthMiddleware
	apiWrap := s.auth.Wrap
	s.mux.Handle("/api/v1/events", apiWrap(http.HandlerFunc(s.handleEvents)))
	s.mux.Handle("/api/v1/compatibility/check", apiWrap(http.HandlerFunc(s.handleCompatibilityCheck)))
	s.mux.Handle("/api/v1/validate", apiWrap(http.HandlerFunc(s.handleValidate)))
	s.mux.Handle("/api/v1/events/", apiWrap(http.HandlerFunc(s.handleEventSubroutes)))
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
	switch r.Method {
	case http.MethodGet:
		events, err := s.store.ListEvents(r.Context())
		if err != nil {
			httputil.WriteProblemSimple(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(events)

	case http.MethodPost:
		var ev core.EventDefinition
		if err := json.NewDecoder(r.Body).Decode(&ev); err != nil {
			httputil.WriteProblemSimple(w, http.StatusBadRequest, "Invalid Request Body", "failed to parse event JSON body")
			return
		}
		var invalidParams []httputil.InvalidParam
		if ev.ID == "" {
			invalidParams = append(invalidParams, httputil.InvalidParam{Name: "id", Reason: "field is required"})
		}
		if ev.Name == "" {
			invalidParams = append(invalidParams, httputil.InvalidParam{Name: "name", Reason: "field is required"})
		}
		if ev.Type == "" {
			invalidParams = append(invalidParams, httputil.InvalidParam{Name: "type", Reason: "field is required"})
		}
		if len(invalidParams) > 0 {
			httputil.WriteProblem(w, http.StatusBadRequest, "Missing Required Fields", "one or more required fields are missing", invalidParams)
			return
		}

		// Enforce RBAC CanPropose check
		if user, ok := auth.UserFromContext(r.Context()); ok {
			if !auth.CanPropose(user, &ev) {
				httputil.WriteProblemSimple(w, http.StatusForbidden, "Forbidden", "user is not authorized to create events for this team")
				return
			}
		}

		if err := s.store.CreateEvent(r.Context(), ev); err != nil {
			if errors.Is(err, storage.ErrConflict) {
				httputil.WriteProblemSimple(w, http.StatusConflict, "Event Already Exists", err.Error())
				return
			}
			httputil.WriteProblemSimple(w, http.StatusInternalServerError, "Failed to Create Event", err.Error())
			return
		}

		s.metrics.SetContractsTotal(ev.Domain, "registered", 1)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(ev)

	default:
		httputil.WriteProblemSimple(w, http.StatusMethodNotAllowed, "Method Not Allowed", fmt.Sprintf("method %s is not allowed", r.Method))
	}
}

func (s *Server) handleEventSubroutes(w http.ResponseWriter, r *http.Request) {
	// Parse /api/v1/events/{id}...
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/events/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		httputil.WriteProblemSimple(w, http.StatusNotFound, "Not Found", "endpoint not found")
		return
	}

	eventID := parts[0]

	// GET /api/v1/events/{id}
	if len(parts) == 1 {
		ev, err := s.store.GetEvent(r.Context(), eventID)
		if err != nil {
			if errors.Is(err, storage.ErrEventNotFound) {
				httputil.WriteProblemSimple(w, http.StatusNotFound, "Event Not Found", fmt.Sprintf("event '%s' not found", eventID))
				return
			}
			httputil.WriteProblemSimple(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ev)
		return
	}

	// Subroutes: /api/v1/events/{id}/versions...
	if parts[1] == "versions" {
		if len(parts) == 2 {
			versions, err := s.store.ListVersions(r.Context(), eventID)
			if err != nil {
				httputil.WriteProblemSimple(w, http.StatusInternalServerError, "Failed to List Versions", err.Error())
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(versions)
			return
		}

		semver := parts[2]
		version, err := s.store.GetEventVersion(r.Context(), eventID, semver)
		if err != nil {
			if errors.Is(err, storage.ErrVersionNotFound) {
				httputil.WriteProblemSimple(w, http.StatusNotFound, "Version Not Found", fmt.Sprintf("version '%s' for event '%s' not found", semver, eventID))
				return
			}
			httputil.WriteProblemSimple(w, http.StatusInternalServerError, "Internal Server Error", err.Error())
			return
		}

		// Apply RFC 8594 deprecation and sunset headers if contract status is StatusDeprecated
		if version.Status == core.StatusDeprecated {
			s.applyDeprecationHeaders(w, r, eventID, semver, version)
		}

		// Check for /ddl endpoint: /api/v1/events/{id}/versions/{semver}/ddl
		if len(parts) >= 4 && parts[3] == "ddl" {
			tableName := version.DataLakeMetadata.TargetTable
			if tableName == "" {
				tableName = strings.ReplaceAll(eventID, "-", "_")
			}

			format := r.URL.Query().Get("format")
			generator, err := datalake.GetGenerator(format)
			if err != nil {
				httputil.WriteProblemSimple(w, http.StatusBadRequest, "Invalid DDL Format", err.Error())
				return
			}

			ddl, err := generator.GenerateDDL(tableName, version.JSONSchema, version.DataLakeMetadata.PartitionBy)
			if err != nil {
				httputil.WriteProblemSimple(w, http.StatusInternalServerError, "DDL Generation Failed", fmt.Sprintf("failed to generate DDL: %v", err))
				return
			}

			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{
				"table":  tableName,
				"format": generator.Format(),
				"ddl":    ddl,
			})
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(version)
		return
	}

	httputil.WriteProblemSimple(w, http.StatusNotFound, "Not Found", "endpoint not found")
}

func (s *Server) applyDeprecationHeaders(w http.ResponseWriter, r *http.Request, eventID, semver string, version *core.EventVersion) {
	// RFC 8594 standard headers
	w.Header().Set("Deprecation", "true")

	// Sunset HTTP-date header (default 6 months after approval or creation)
	sunsetTime := version.CreatedAt.AddDate(0, 6, 0).UTC()
	if version.ApprovedAt != nil {
		sunsetTime = version.ApprovedAt.AddDate(0, 6, 0).UTC()
	}
	w.Header().Set("Sunset", sunsetTime.Format(http.TimeFormat))

	// Link header pointing to successor version
	successorURL := fmt.Sprintf("/api/v1/events/%s/versions", eventID)
	if versions, err := s.store.ListVersions(r.Context(), eventID); err == nil {
		for _, v := range versions {
			if v.SemVer != semver && v.Status == core.StatusApproved {
				successorURL = fmt.Sprintf("/api/v1/events/%s/versions/%s", eventID, v.SemVer)
				break
			}
		}
	}
	w.Header().Set("Link", fmt.Sprintf("<%s>; rel=\"successor-version\"", successorURL))
}

func (s *Server) handleValidate(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	defer func() {
		s.metrics.ObserveValidationDuration(time.Since(start).Seconds())
	}()

	if r.Method != http.MethodPost {
		httputil.WriteProblemSimple(w, http.StatusMethodNotAllowed, "Method Not Allowed", fmt.Sprintf("method %s is not allowed, use POST", r.Method))
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		httputil.WriteProblemSimple(w, http.StatusBadRequest, "Bad Request", "failed to read request body")
		return
	}

	if len(bodyBytes) == 0 {
		httputil.WriteProblemSimple(w, http.StatusBadRequest, "Bad Request", "request body is empty")
		return
	}

	// Support both wrapped validation requests and raw CloudEvents
	var wrapper struct {
		EventID string          `json:"event_id"`
		SemVer  string          `json:"semver"`
		Schema  string          `json:"schema"`
		Event   json.RawMessage `json:"event"`
	}

	var eventBytes []byte
	var eventID string
	var semver string
	var targetSchema string

	if err := json.Unmarshal(bodyBytes, &wrapper); err == nil && len(wrapper.Event) > 0 {
		eventBytes = wrapper.Event
		eventID = wrapper.EventID
		semver = wrapper.SemVer
		targetSchema = wrapper.Schema
	} else {
		eventBytes = bodyBytes
	}

	// Query parameters can provide or override event_id / semver
	if qEventID := r.URL.Query().Get("event_id"); qEventID != "" {
		eventID = qEventID
	}
	if qSemVer := r.URL.Query().Get("semver"); qSemVer != "" {
		semver = qSemVer
	}

	// Resolve schema from storage if not directly provided
	if targetSchema == "" {
		// Try inspecting CloudEvent envelope for 'type'
		var env struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(eventBytes, &env)

		if eventID == "" && env.Type != "" {
			events, err := s.store.ListEvents(r.Context())
			if err == nil {
				for _, e := range events {
					if e.Type == env.Type || e.ID == env.Type {
						eventID = e.ID
						break
					}
				}
			}
		}

		if eventID == "" {
			httputil.WriteProblemSimple(w, http.StatusNotFound, "Contract Not Found", "could not determine event contract; specify 'event_id' query param or matching CloudEvent type")
			return
		}

		var version *core.EventVersion
		if semver != "" {
			version, err = s.store.GetEventVersion(r.Context(), eventID, semver)
			if err != nil {
				httputil.WriteProblemSimple(w, http.StatusNotFound, "Version Not Found", fmt.Sprintf("contract version '%s' not found for event '%s'", semver, eventID))
				return
			}
		} else {
			versions, err := s.store.ListVersions(r.Context(), eventID)
			if err != nil || len(versions) == 0 {
				httputil.WriteProblemSimple(w, http.StatusNotFound, "No Versions Found", fmt.Sprintf("no contract versions registered for event '%s'", eventID))
				return
			}
			// Prefer latest approved version
			for i := len(versions) - 1; i >= 0; i-- {
				if versions[i].Status == core.StatusApproved {
					version = &versions[i]
					semver = version.SemVer
					break
				}
			}
			if version == nil {
				version = &versions[len(versions)-1]
				semver = version.SemVer
			}
		}

		targetSchema = version.JSONSchema
	}

	res, err := s.validator.ValidateCloudEvent(eventBytes, targetSchema)
	if err != nil {
		httputil.WriteProblemSimple(w, http.StatusInternalServerError, "Validation Error", fmt.Sprintf("schema validation failed internally: %v", err))
		return
	}

	if !res.IsValid {
		httputil.WriteProblem(
			w,
			http.StatusUnprocessableEntity,
			"CloudEvent Validation Failed",
			"The provided CloudEvent does not satisfy the contract schema",
			res.ToInvalidParams(),
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"valid":    true,
		"event_id": eventID,
		"semver":   semver,
		"status":   "valid",
	})
}

func (s *Server) handleCompatibilityCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		httputil.WriteProblemSimple(w, http.StatusMethodNotAllowed, "Method Not Allowed", "only POST method is allowed")
		return
	}

	var req struct {
		OldSchema string `json:"old_schema"`
		NewSchema string `json:"new_schema"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.WriteProblemSimple(w, http.StatusBadRequest, "Invalid Request Body", "failed to parse json schema comparison request")
		return
	}

	res, err := compatibility.CheckCompatibility(req.OldSchema, req.NewSchema)
	if err != nil {
		httputil.WriteProblemSimple(w, http.StatusBadRequest, "Compatibility Check Failed", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(res)
}

func (s *Server) handleWebDashboard(w http.ResponseWriter, r *http.Request) {
	// Sub-filesystem rooted in dist
	distFS, err := fs.Sub(embeddedDist, "dist")
	if err != nil {
		httputil.WriteProblemSimple(w, http.StatusInternalServerError, "Internal Server Error", "embedded web assets failed to initialize")
		return
	}

	cleanPath := strings.TrimPrefix(r.URL.Path, "/")
	if cleanPath == "" {
		cleanPath = "index.html"
	}

	// Try reading the file from distFS if it's a static file (e.g. assets, favicon)
	if file, err := distFS.Open(cleanPath); err == nil {
		stat, err := file.Stat()
		_ = file.Close()
		if err == nil && !stat.IsDir() {
			http.FileServer(http.FS(distFS)).ServeHTTP(w, r)
			return
		}
	}

	// Fallback to index.html for SPA client-side routing
	indexFile, err := distFS.Open("index.html")
	if err != nil {
		httputil.WriteProblemSimple(w, http.StatusNotFound, "Not Found", "dashboard index.html not found")
		return
	}
	defer indexFile.Close()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, indexFile)
}
