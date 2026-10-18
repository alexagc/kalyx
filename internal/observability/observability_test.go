package observability

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexagc/kalyx/internal/core"
	"github.com/alexagc/kalyx/internal/storage"
)

type failingStorage struct {
	*storage.MemoryStorage
}

func (f *failingStorage) Ping(ctx context.Context) error {
	return errors.New("database connection refused")
}

func TestLivenessHandler(t *testing.T) {
	handler := LivenessHandler()
	req := httptest.NewRequest(http.MethodGet, "/livez", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed decoding json: %v", err)
	}

	if res["status"] != "alive" {
		t.Errorf("expected status=alive, got %v", res["status"])
	}
}

func TestReadinessHandler(t *testing.T) {
	// 1. Healthy storage -> 200 OK
	store := storage.NewMemoryStorage()
	handlerHealthy := ReadinessHandler(store)

	reqHealthy := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	recHealthy := httptest.NewRecorder()
	handlerHealthy.ServeHTTP(recHealthy, reqHealthy)

	if recHealthy.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", recHealthy.Code)
	}

	var resHealthy map[string]any
	_ = json.Unmarshal(recHealthy.Body.Bytes(), &resHealthy)
	if resHealthy["status"] != "ready" {
		t.Errorf("expected status=ready, got %v", resHealthy["status"])
	}

	// 2. Nil storage -> 503
	handlerNil := ReadinessHandler(nil)
	reqNil := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	recNil := httptest.NewRecorder()
	handlerNil.ServeHTTP(recNil, reqNil)

	if recNil.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 Service Unavailable, got %d", recNil.Code)
	}

	// 3. Failing storage Ping -> 503
	failingStore := &failingStorage{MemoryStorage: storage.NewMemoryStorage()}
	handlerFailing := ReadinessHandler(failingStore)
	reqFailing := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	recFailing := httptest.NewRecorder()
	handlerFailing.ServeHTTP(recFailing, reqFailing)

	if recFailing.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", recFailing.Code)
	}
}

func TestMetricsRegistry_PrometheusExport(t *testing.T) {
	reg := NewRegistry()

	// 1. Record HTTP requests
	reg.IncHTTPRequests(http.MethodGet, "/api/v1/events", http.StatusOK)
	reg.IncHTTPRequests(http.MethodPost, "/api/v1/events", http.StatusCreated)
	reg.IncHTTPRequests(http.MethodGet, "/api/v1/events/ev_123", http.StatusOK) // should normalize to :id

	// 2. Record validation observations
	reg.ObserveValidationDuration(0.002) // 2ms
	reg.ObserveValidationDuration(0.004) // 4ms

	// 3. Record contracts total
	reg.SetContractsTotal("Orders", "approved", 5)
	reg.SetContractsTotal("Payments", "draft", 2)

	// 4. Test HTTP Handler
	handler := reg.Handler()
	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	contentType := rec.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/plain") {
		t.Errorf("expected text/plain Content-Type, got %s", contentType)
	}

	body := rec.Body.String()

	// Verify http_requests_total
	if !strings.Contains(body, "# TYPE http_requests_total counter") {
		t.Errorf("missing http_requests_total counter declaration")
	}
	if !strings.Contains(body, `http_requests_total{code="200",method="GET",path="/api/v1/events"} 1`) {
		t.Errorf("missing GET /api/v1/events counter line in:\n%s", body)
	}
	if !strings.Contains(body, `http_requests_total{code="200",method="GET",path="/api/v1/events/:id"} 1`) {
		t.Errorf("missing normalized path counter line in:\n%s", body)
	}

	// Verify validation_duration_seconds
	if !strings.Contains(body, "# TYPE validation_duration_seconds histogram") {
		t.Errorf("missing validation_duration_seconds histogram declaration")
	}
	if !strings.Contains(body, "validation_duration_seconds_count 2") {
		t.Errorf("missing validation_duration_seconds_count 2 in:\n%s", body)
	}

	// Verify contracts_total
	if !strings.Contains(body, "# TYPE contracts_total gauge") {
		t.Errorf("missing contracts_total gauge declaration")
	}
	if !strings.Contains(body, `contracts_total{domain="Orders",status="approved"} 5`) {
		t.Errorf("missing Orders approved gauge line in:\n%s", body)
	}
}

func TestMetrics_SyncContractsFromStorage(t *testing.T) {
	store := storage.NewMemoryStorage()
	ctx := context.Background()

	_ = store.CreateEvent(ctx, core.EventDefinition{
		ID:        "ev_sync_test",
		Domain:    "Checkout",
		Name:      "CartCreated",
		Type:      "com.test.cart.created",
		OwnerTeam: "Checkout",
	})
	_ = store.CreateEventVersion(ctx, core.EventVersion{
		ID:      "v_1",
		EventID: "ev_sync_test",
		SemVer:  "1.0.0",
		Status:  core.StatusApproved,
	})

	reg := NewRegistry()
	if err := reg.SyncContractsFromStorage(ctx, store); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	rec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if !strings.Contains(rec.Body.String(), `contracts_total{domain="Checkout",status="approved"}`) {
		t.Errorf("expected Checkout approved contract in metrics:\n%s", rec.Body.String())
	}
}

func TestMetrics_HTTPMiddleware(t *testing.T) {
	reg := NewRegistry()
	baseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	instrumented := reg.HTTPMiddleware(baseHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/events", nil)
	rec := httptest.NewRecorder()
	instrumented.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Errorf("expected 202, got %d", rec.Code)
	}

	metricsRec := httptest.NewRecorder()
	reg.Handler().ServeHTTP(metricsRec, httptest.NewRequest(http.MethodGet, "/metrics", nil))

	if !strings.Contains(metricsRec.Body.String(), `http_requests_total{code="202",method="POST",path="/api/v1/events"} 1`) {
		t.Errorf("expected 202 count in metrics output:\n%s", metricsRec.Body.String())
	}
}
