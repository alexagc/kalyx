package webserver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexagc/kalyx/internal/core"
	"github.com/alexagc/kalyx/internal/httputil"
	"github.com/alexagc/kalyx/internal/storage"
)

func setupTestServer(t *testing.T) (*Server, storage.Storage) {
	t.Helper()
	store := storage.NewMemoryStorage()
	server := NewServer(store)
	return server, store
}

func TestServer_Validate_Success(t *testing.T) {
	server, store := setupTestServer(t)

	// Create test event and version
	ctx := context.Background()
	eventID := "orders.validate.success"
	eventType := "com.test.orders.validate.v1"
	_ = store.CreateEvent(ctx, core.EventDefinition{
		ID:        eventID,
		Domain:    "orders",
		Name:      "orders_completed",
		Type:      eventType,
		OwnerTeam: "checkout",
	})

	schemaJSON := `{
		"title": "OrderCompleted",
		"properties": {
			"order_id": {"type": "string"},
			"amount": {"type": "number", "minimum": 1.0}
		},
		"required": ["order_id", "amount"]
	}`

	_ = store.CreateEventVersion(ctx, core.EventVersion{
		ID:         "v-1",
		EventID:    eventID,
		SemVer:     "1.0.0",
		Status:     core.StatusApproved,
		JSONSchema: schemaJSON,
	})

	validEvent := fmt.Sprintf(`{
		"specversion": "1.0",
		"id": "evt-abc-123",
		"source": "/checkout-service",
		"type": "%s",
		"data": {
			"order_id": "ord-456",
			"amount": 99.95
		}
	}`, eventType)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/validate", strings.NewReader(validEvent))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed decoding json: %v", err)
	}

	if res["valid"] != true {
		t.Errorf("expected valid=true, got %v", res["valid"])
	}
	if res["event_id"] != eventID {
		t.Errorf("expected event_id=%s, got %v", eventID, res["event_id"])
	}
}

func TestServer_Validate_UnprocessableEntity(t *testing.T) {
	server, store := setupTestServer(t)

	ctx := context.Background()
	eventID := "orders.validate.invalid"
	eventType := "com.test.orders.invalid.v1"
	_ = store.CreateEvent(ctx, core.EventDefinition{
		ID:     eventID,
		Domain: "orders",
		Name:   "orders_completed",
		Type:   eventType,
	})

	schemaJSON := `{
		"title": "OrderCompleted",
		"properties": {
			"order_id": {"type": "string"},
			"amount": {"type": "number", "minimum": 1.0}
		},
		"required": ["order_id", "amount"]
	}`

	_ = store.CreateEventVersion(ctx, core.EventVersion{
		ID:         "v-1",
		EventID:    eventID,
		SemVer:     "1.0.0",
		Status:     core.StatusApproved,
		JSONSchema: schemaJSON,
	})

	// Missing order_id and amount is below minimum
	invalidEvent := fmt.Sprintf(`{
		"specversion": "1.0",
		"id": "evt-invalid",
		"source": "/checkout",
		"type": "%s",
		"data": {
			"amount": 0.5
		}
	}`, eventType)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/validate", strings.NewReader(invalidEvent))
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 Unprocessable Entity, got %d: %s", rec.Code, rec.Body.String())
	}

	if rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("expected application/problem+json Content-Type, got %s", rec.Header().Get("Content-Type"))
	}

	var prob httputil.ProblemDetails
	if err := json.Unmarshal(rec.Body.Bytes(), &prob); err != nil {
		t.Fatalf("failed decoding problem details: %v", err)
	}

	if prob.Type != "urn:kalyx:error:validation-failed" {
		t.Errorf("expected urn:kalyx:error:validation-failed, got %s", prob.Type)
	}
	if len(prob.InvalidParams) == 0 {
		t.Errorf("expected invalid_params in RFC 7807 problem details")
	}
}

func TestServer_RFC8594_DeprecationHeaders(t *testing.T) {
	server, store := setupTestServer(t)
	ctx := context.Background()

	eventID := "orders.legacy"
	_ = store.CreateEvent(ctx, core.EventDefinition{
		ID:     eventID,
		Domain: "orders",
		Name:   "legacy_orders",
		Type:   "orders.legacy",
	})

	now := time.Now().UTC()
	_ = store.CreateEventVersion(ctx, core.EventVersion{
		ID:         "v-1-0",
		EventID:    eventID,
		SemVer:     "1.0.0",
		Status:     core.StatusDeprecated,
		JSONSchema: `{"title": "v1"}`,
		CreatedAt:  now.Add(-24 * time.Hour),
		ApprovedAt: &now,
	})

	_ = store.CreateEventVersion(ctx, core.EventVersion{
		ID:         "v-2-0",
		EventID:    eventID,
		SemVer:     "2.0.0",
		Status:     core.StatusApproved,
		JSONSchema: `{"title": "v2"}`,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/events/orders.legacy/versions/1.0.0", nil)
	rec := httptest.NewRecorder()

	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	deprecationHeader := rec.Header().Get("Deprecation")
	if deprecationHeader != "true" {
		t.Errorf("expected Deprecation: true, got '%s'", deprecationHeader)
	}

	sunsetHeader := rec.Header().Get("Sunset")
	if sunsetHeader == "" {
		t.Errorf("missing Sunset header")
	}

	linkHeader := rec.Header().Get("Link")
	expectedLink := `</api/v1/events/orders.legacy/versions/2.0.0>; rel="successor-version"`
	if linkHeader != expectedLink {
		t.Errorf("expected Link header '%s', got '%s'", expectedLink, linkHeader)
	}
}

func TestServer_DDL_DeltaAndIceberg(t *testing.T) {
	server, store := setupTestServer(t)
	ctx := context.Background()

	eventID := "payments.processed"
	_ = store.CreateEvent(ctx, core.EventDefinition{
		ID:     eventID,
		Domain: "payments",
		Name:   "processed",
		Type:   "payments.processed",
	})

	schemaJSON := `{
		"title": "PaymentProcessed",
		"properties": {
			"payment_id": {"type": "string"},
			"amount": {"type": "number"}
		},
		"required": ["payment_id"]
	}`

	_ = store.CreateEventVersion(ctx, core.EventVersion{
		ID:         "v-1",
		EventID:    eventID,
		SemVer:     "1.0.0",
		Status:     core.StatusApproved,
		JSONSchema: schemaJSON,
		DataLakeMetadata: core.DataLakeMetadata{
			TargetTable: "lake_payments",
		},
	})

	// 1. Iceberg format (default)
	reqIceberg := httptest.NewRequest(http.MethodGet, "/api/v1/events/payments.processed/versions/1.0.0/ddl", nil)
	recIceberg := httptest.NewRecorder()
	server.ServeHTTP(recIceberg, reqIceberg)

	if recIceberg.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", recIceberg.Code, recIceberg.Body.String())
	}
	var resIceberg map[string]string
	_ = json.Unmarshal(recIceberg.Body.Bytes(), &resIceberg)
	if resIceberg["format"] != "iceberg" {
		t.Errorf("expected format=iceberg, got %s", resIceberg["format"])
	}
	if !strings.Contains(resIceberg["ddl"], "USING iceberg") {
		t.Errorf("expected USING iceberg in DDL")
	}

	// 2. Delta Lake format via query param format=delta
	reqDelta := httptest.NewRequest(http.MethodGet, "/api/v1/events/payments.processed/versions/1.0.0/ddl?format=delta", nil)
	recDelta := httptest.NewRecorder()
	server.ServeHTTP(recDelta, reqDelta)

	if recDelta.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", recDelta.Code, recDelta.Body.String())
	}
	var resDelta map[string]string
	_ = json.Unmarshal(recDelta.Body.Bytes(), &resDelta)
	if resDelta["format"] != "delta" {
		t.Errorf("expected format=delta, got %s", resDelta["format"])
	}
	if !strings.Contains(resDelta["ddl"], "USING delta") {
		t.Errorf("expected USING delta in DDL")
	}

	// 3. Unsupported format returns 400 Bad Request with RFC 7807 problem details
	reqInvalid := httptest.NewRequest(http.MethodGet, "/api/v1/events/payments.processed/versions/1.0.0/ddl?format=unsupported", nil)
	recInvalid := httptest.NewRecorder()
	server.ServeHTTP(recInvalid, reqInvalid)

	if recInvalid.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request, got %d", recInvalid.Code)
	}
	if recInvalid.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("expected application/problem+json for error, got %s", recInvalid.Header().Get("Content-Type"))
	}
}

func TestServer_RFC7807_ErrorResponses(t *testing.T) {
	server, _ := setupTestServer(t)

	// Test 404 Not Found on unknown event
	reqNotFound := httptest.NewRequest(http.MethodGet, "/api/v1/events/non-existent-event", nil)
	recNotFound := httptest.NewRecorder()
	server.ServeHTTP(recNotFound, reqNotFound)

	if recNotFound.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", recNotFound.Code)
	}
	if recNotFound.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("expected application/problem+json, got %s", recNotFound.Header().Get("Content-Type"))
	}

	var prob404 httputil.ProblemDetails
	if err := json.Unmarshal(recNotFound.Body.Bytes(), &prob404); err != nil {
		t.Fatalf("failed decoding 404 problem details: %v", err)
	}
	if prob404.Type != "urn:kalyx:error:not-found" {
		t.Errorf("expected urn:kalyx:error:not-found, got %s", prob404.Type)
	}

	// Test 400 Bad Request on malformed event POST
	reqBadJSON := httptest.NewRequest(http.MethodPost, "/api/v1/events", bytes.NewReader([]byte("{invalid-json")))
	recBadJSON := httptest.NewRecorder()
	server.ServeHTTP(recBadJSON, reqBadJSON)

	if recBadJSON.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", recBadJSON.Code)
	}
	var prob400 httputil.ProblemDetails
	_ = json.Unmarshal(recBadJSON.Body.Bytes(), &prob400)
	if prob400.Type != "urn:kalyx:error:bad-request" {
		t.Errorf("expected urn:kalyx:error:bad-request, got %s", prob400.Type)
	}

	// Test 405 Method Not Allowed on /api/v1/validate GET
	req405 := httptest.NewRequest(http.MethodGet, "/api/v1/validate", nil)
	rec405 := httptest.NewRecorder()
	server.ServeHTTP(rec405, req405)

	if rec405.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec405.Code)
	}
	var prob405 httputil.ProblemDetails
	_ = json.Unmarshal(rec405.Body.Bytes(), &prob405)
	if prob405.Type != "urn:kalyx:error:method-not-allowed" {
		t.Errorf("expected urn:kalyx:error:method-not-allowed, got %s", prob405.Type)
	}
}
