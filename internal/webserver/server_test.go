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

	"github.com/alexagc/kalyx/internal/auth"
	"github.com/alexagc/kalyx/internal/core"
	"github.com/alexagc/kalyx/internal/httputil"
	"github.com/alexagc/kalyx/internal/storage"
)

func setupTestServer(t *testing.T) (*Server, storage.Storage) {
	t.Helper()
	t.Setenv("KALYX_AUTH_DISABLED", "true")
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

func TestServer_Observability_Probes(t *testing.T) {
	server, _ := setupTestServer(t)

	// 1. /livez probe
	reqLive := httptest.NewRequest(http.MethodGet, "/livez", nil)
	recLive := httptest.NewRecorder()
	server.ServeHTTP(recLive, reqLive)

	if recLive.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /livez, got %d: %s", recLive.Code, recLive.Body.String())
	}
	var liveRes map[string]any
	_ = json.Unmarshal(recLive.Body.Bytes(), &liveRes)
	if liveRes["status"] != "alive" {
		t.Errorf("expected status=alive, got %v", liveRes["status"])
	}

	// 2. /readyz probe
	reqReady := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	recReady := httptest.NewRecorder()
	server.ServeHTTP(recReady, reqReady)

	if recReady.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /readyz, got %d: %s", recReady.Code, recReady.Body.String())
	}
	var readyRes map[string]any
	_ = json.Unmarshal(recReady.Body.Bytes(), &readyRes)
	if readyRes["status"] != "ready" {
		t.Errorf("expected status=ready, got %v", readyRes["status"])
	}
}

func TestServer_Observability_Metrics(t *testing.T) {
	server, _ := setupTestServer(t)

	// Trigger a validate request to record validation_duration_seconds
	validEvent := `{
		"specversion": "1.0",
		"id": "evt-metrics",
		"source": "/checkout",
		"type": "com.company.orders.completed.v1",
		"data": {
			"order_id": "ord-1",
			"customer_email": "test@example.com",
			"amount": 50.0,
			"country": "US"
		}
	}`
	reqVal := httptest.NewRequest(http.MethodPost, "/api/v1/validate", strings.NewReader(validEvent))
	recVal := httptest.NewRecorder()
	server.ServeHTTP(recVal, reqVal)

	// Scrape /metrics
	reqMetrics := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recMetrics := httptest.NewRecorder()
	server.ServeHTTP(recMetrics, reqMetrics)

	if recMetrics.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on /metrics, got %d", recMetrics.Code)
	}

	body := recMetrics.Body.String()
	if !strings.Contains(body, "# TYPE http_requests_total counter") {
		t.Errorf("missing http_requests_total in /metrics:\n%s", body)
	}
	if !strings.Contains(body, "# TYPE validation_duration_seconds histogram") {
		t.Errorf("missing validation_duration_seconds in /metrics:\n%s", body)
	}
	if !strings.Contains(body, "# TYPE contracts_total gauge") {
		t.Errorf("missing contracts_total in /metrics:\n%s", body)
	}
}

func TestServer_Auth_Protection_And_PAT(t *testing.T) {
	t.Setenv("KALYX_AUTH_DISABLED", "false")
	store := storage.NewMemoryStorage()
	server := NewServer(store, WithAuthDisabled(false))
	ctx := context.Background()

	// 1. Missing Authorization header -> 401 Unauthorized RFC 7807
	reqNoAuth := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	recNoAuth := httptest.NewRecorder()
	server.ServeHTTP(recNoAuth, reqNoAuth)

	if recNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", recNoAuth.Code)
	}
	if recNoAuth.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("expected application/problem+json, got %s", recNoAuth.Header().Get("Content-Type"))
	}
	var prob httputil.ProblemDetails
	_ = json.Unmarshal(recNoAuth.Body.Bytes(), &prob)
	if prob.Type != "urn:kalyx:error:unauthorized" {
		t.Errorf("expected urn:kalyx:error:unauthorized, got %s", prob.Type)
	}

	// 2. Generate valid PAT
	patEngine := auth.NewPATEngine(store)
	rawToken, _, err := patEngine.GeneratePAT(ctx, "Test PAT", "ci@company.com", "checkout", nil)
	if err != nil {
		t.Fatalf("failed to generate PAT: %v", err)
	}

	// 3. Request with valid PAT -> 200 OK
	reqPAT := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	reqPAT.Header.Set("Authorization", "Bearer "+rawToken)
	recPAT := httptest.NewRecorder()
	server.ServeHTTP(recPAT, reqPAT)

	if recPAT.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with valid PAT, got %d: %s", recPAT.Code, recPAT.Body.String())
	}
}

func TestServer_Auth_Protection_And_JWT(t *testing.T) {
	t.Setenv("KALYX_AUTH_DISABLED", "false")
	store := storage.NewMemoryStorage()
	secret := []byte("secret-jwt-key-for-webserver-test!")
	oidc := auth.NewOIDCEngine(auth.OIDCConfig{
		Secret: secret,
	})
	server := NewServer(store, WithOIDCEngine(oidc), WithAuthDisabled(false))

	// 1. Create valid JWT
	claims := &auth.JWTClaims{
		Subject:   "usr_lead_1",
		Email:     "lead@company.com",
		Groups:    []any{"checkout-lead"},
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	tokenStr, err := auth.SignTestHS256Token(claims, secret)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	// 2. Request with valid JWT -> 200 OK
	req := httptest.NewRequest(http.MethodGet, "/api/v1/events", nil)
	req.Header.Set("Authorization", "Bearer "+tokenStr)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with valid JWT, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestServer_RBAC_CanPropose(t *testing.T) {
	t.Setenv("KALYX_AUTH_DISABLED", "false")
	store := storage.NewMemoryStorage()
	secret := []byte("secret-rbac-key-for-test-32bytes!")
	oidc := auth.NewOIDCEngine(auth.OIDCConfig{
		Secret: secret,
	})
	server := NewServer(store, WithOIDCEngine(oidc), WithAuthDisabled(false))

	// 1. User from "orders" team trying to create event for "checkout" team -> 403 Forbidden
	ordersClaims := &auth.JWTClaims{
		Subject:   "usr_orders",
		Email:     "orders_member@company.com",
		Groups:    []any{"orders-member"},
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	ordersToken, _ := auth.SignTestHS256Token(ordersClaims, secret)

	eventPayload := `{
		"id": "ev_checkout_new",
		"domain": "Checkout",
		"name": "CheckoutInitiated",
		"type": "com.company.checkout.initiated.v1",
		"owner_team": "checkout"
	}`

	reqForbidden := httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(eventPayload))
	reqForbidden.Header.Set("Authorization", "Bearer "+ordersToken)
	recForbidden := httptest.NewRecorder()
	server.ServeHTTP(recForbidden, reqForbidden)

	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for cross-team propose, got %d: %s", recForbidden.Code, recForbidden.Body.String())
	}
	if recForbidden.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("expected application/problem+json, got %s", recForbidden.Header().Get("Content-Type"))
	}

	// 2. User from "checkout" team creating event for "checkout" team -> 201 Created
	checkoutClaims := &auth.JWTClaims{
		Subject:   "usr_checkout",
		Email:     "checkout_member@company.com",
		Groups:    []any{"checkout-member"},
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	checkoutToken, _ := auth.SignTestHS256Token(checkoutClaims, secret)

	reqAllowed := httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(eventPayload))
	reqAllowed.Header.Set("Authorization", "Bearer "+checkoutToken)
	recAllowed := httptest.NewRecorder()
	server.ServeHTTP(recAllowed, reqAllowed)

	if recAllowed.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for team member, got %d: %s", recAllowed.Code, recAllowed.Body.String())
	}
}

func TestServer_WebDashboard_EmbeddedUI(t *testing.T) {
	server, _ := setupTestServer(t)

	// Test 1: Root path / serves embedded HTML
	reqRoot := httptest.NewRequest(http.MethodGet, "/", nil)
	recRoot := httptest.NewRecorder()
	server.ServeHTTP(recRoot, reqRoot)

	if recRoot.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /, got %d: %s", recRoot.Code, recRoot.Body.String())
	}
	if !strings.Contains(recRoot.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html Content-Type, got %s", recRoot.Header().Get("Content-Type"))
	}
	if !strings.Contains(recRoot.Body.String(), "Kalyx") {
		t.Errorf("expected body to contain 'Kalyx', got: %s", recRoot.Body.String()[:100])
	}

	// Test 2: SPA client route /events serves index.html fallback
	reqEvents := httptest.NewRequest(http.MethodGet, "/events", nil)
	recEvents := httptest.NewRecorder()
	server.ServeHTTP(recEvents, reqEvents)

	if recEvents.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for SPA route /events, got %d", recEvents.Code)
	}
	if !strings.Contains(recEvents.Header().Get("Content-Type"), "text/html") {
		t.Errorf("expected text/html for /events SPA fallback, got %s", recEvents.Header().Get("Content-Type"))
	}

	// Test 3: SPA client route /events/orders.order.completed/propose serves index.html
	reqPropose := httptest.NewRequest(http.MethodGet, "/events/orders.order.completed/propose", nil)
	recPropose := httptest.NewRecorder()
	server.ServeHTTP(recPropose, reqPropose)

	if recPropose.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for SPA route /events/.../propose, got %d", recPropose.Code)
	}
}

func TestServer_Tokens(t *testing.T) {
	server, _ := setupTestServer(t)

	// POST /api/v1/tokens - create new token
	body := `{"name": "GitLab CI Runner", "expires_in_days": 30}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/tokens", strings.NewReader(body))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed decoding json: %v", err)
	}

	tokenStr, ok := res["token"].(string)
	if !ok || !strings.HasPrefix(tokenStr, "kalyx_sec_") {
		t.Errorf("expected token starting with kalyx_sec_, got %v", res["token"])
	}
	if res["name"] != "GitLab CI Runner" {
		t.Errorf("expected name 'GitLab CI Runner', got %v", res["name"])
	}

	// GET /api/v1/tokens - list tokens
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/tokens", nil)
	recList := httptest.NewRecorder()
	server.ServeHTTP(recList, reqList)

	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", recList.Code, recList.Body.String())
	}
}

func TestServer_WebDashboard_HTMLIntegrity(t *testing.T) {
	server, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	html := rec.Body.String()

	// Verify key dynamic features are present
	requiredSnippets := []string{
		"loadLiveCatalog()",
		"getAuthHeaders()",
		"handleDraftInput(",
		"updateSemVerDiagnostic(",
		"generateTokenPrompt()",
		"updateSessionTokenUI()",
		"saveSessionToken()",
		"clearSessionToken()",
		"session-pat-input",
		"active-token-badge",
		"/api/v1/events",
		"/api/v1/compatibility/check",
		"/api/v1/tokens",
	}

	for _, snippet := range requiredSnippets {
		if !strings.Contains(html, snippet) {
			t.Errorf("expected HTML to contain snippet %q", snippet)
		}
	}

	// Verify crucial opening and closing tag counts match
	tags := []string{"html", "body", "head", "main", "aside"}
	for _, tag := range tags {
		openCount := strings.Count(html, "<"+tag+">") + strings.Count(html, "<"+tag+" ")
		closeCount := strings.Count(html, "</"+tag+">")
		if openCount != closeCount {
			t.Errorf("mismatched tag %s: %d open vs %d close", tag, openCount, closeCount)
		}
	}
}

func TestServer_Me(t *testing.T) {
	// 1. With auth disabled (dev mode) -> returns devUser
	server, _ := setupTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for /api/v1/me, got %d: %s", rec.Code, rec.Body.String())
	}

	var res map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed decoding json: %v", err)
	}

	if res["email"] != "developer@kalyx.local" {
		t.Errorf("expected developer@kalyx.local, got %v", res["email"])
	}
	if res["subject"] != "dev_user" {
		t.Errorf("expected subject dev_user, got %v", res["subject"])
	}

	// 2. With auth enabled without Authorization header -> 401 Unauthorized
	t.Setenv("KALYX_AUTH_DISABLED", "false")
	store := storage.NewMemoryStorage()
	authServer := NewServer(store, WithAuthDisabled(false))
	reqNoAuth := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	recNoAuth := httptest.NewRecorder()
	authServer.ServeHTTP(recNoAuth, reqNoAuth)

	if recNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d: %s", recNoAuth.Code, recNoAuth.Body.String())
	}
	if recNoAuth.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("expected application/problem+json, got %s", recNoAuth.Header().Get("Content-Type"))
	}

	// 3. With valid PAT -> 200 OK with PAT identity
	patEngine := auth.NewPATEngine(store)
	rawToken, _, err := patEngine.GeneratePAT(context.Background(), "Me PAT", "pat_owner@company.com", "platform", nil)
	if err != nil {
		t.Fatalf("failed to generate PAT: %v", err)
	}

	reqAuth := httptest.NewRequest(http.MethodGet, "/api/v1/me", nil)
	reqAuth.Header.Set("Authorization", "Bearer "+rawToken)
	recAuth := httptest.NewRecorder()
	authServer.ServeHTTP(recAuth, reqAuth)

	if recAuth.Code != http.StatusOK {
		t.Fatalf("expected 200 OK with valid PAT, got %d: %s", recAuth.Code, recAuth.Body.String())
	}
	var resAuth map[string]any
	_ = json.Unmarshal(recAuth.Body.Bytes(), &resAuth)
	if resAuth["email"] != "pat_owner@company.com" {
		t.Errorf("expected email pat_owner@company.com, got %v", resAuth["email"])
	}
}

func TestServer_Tokens_FullLifecycle(t *testing.T) {
	server, _ := setupTestServer(t)

	// 1. POST /api/v1/tokens with missing name -> 400 Bad Request
	reqMissing := httptest.NewRequest(http.MethodPost, "/api/v1/tokens", strings.NewReader(`{"expires_in_days": 30}`))
	recMissing := httptest.NewRecorder()
	server.ServeHTTP(recMissing, reqMissing)

	if recMissing.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for missing name, got %d: %s", recMissing.Code, recMissing.Body.String())
	}

	// 2. POST /api/v1/tokens valid -> 201 Created
	reqCreate := httptest.NewRequest(http.MethodPost, "/api/v1/tokens", strings.NewReader(`{"name": "Deploy Agent", "expires_in_days": 14}`))
	recCreate := httptest.NewRecorder()
	server.ServeHTTP(recCreate, reqCreate)

	if recCreate.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", recCreate.Code, recCreate.Body.String())
	}

	var created map[string]any
	if err := json.Unmarshal(recCreate.Body.Bytes(), &created); err != nil {
		t.Fatalf("failed decoding json: %v", err)
	}

	tokenID, ok := created["id"].(string)
	if !ok || tokenID == "" {
		t.Fatalf("expected non-empty token id, got %v", created["id"])
	}
	rawToken, ok := created["token"].(string)
	if !ok || !strings.HasPrefix(rawToken, "kalyx_sec_") {
		t.Fatalf("expected raw token starting with kalyx_sec_, got %v", created["token"])
	}

	// 3. GET /api/v1/tokens -> list tokens
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/tokens", nil)
	recList := httptest.NewRecorder()
	server.ServeHTTP(recList, reqList)

	if recList.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", recList.Code, recList.Body.String())
	}

	var tokenList []core.PATToken
	if err := json.Unmarshal(recList.Body.Bytes(), &tokenList); err != nil {
		t.Fatalf("failed decoding tokens list: %v", err)
	}
	found := false
	for _, tk := range tokenList {
		if tk.ID == tokenID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("created token %s not found in list response", tokenID)
	}

	// 4. DELETE /api/v1/tokens/{id} -> 204 No Content
	reqDelete := httptest.NewRequest(http.MethodDelete, "/api/v1/tokens/"+tokenID, nil)
	recDelete := httptest.NewRecorder()
	server.ServeHTTP(recDelete, reqDelete)

	if recDelete.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content, got %d: %s", recDelete.Code, recDelete.Body.String())
	}

	// 5. DELETE non-existent token -> 404 Not Found
	reqDeleteAgain := httptest.NewRequest(http.MethodDelete, "/api/v1/tokens/"+tokenID, nil)
	recDeleteAgain := httptest.NewRecorder()
	server.ServeHTTP(recDeleteAgain, reqDeleteAgain)

	if recDeleteAgain.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found on already deleted token, got %d: %s", recDeleteAgain.Code, recDeleteAgain.Body.String())
	}
}

func TestServer_CreateEventVersion(t *testing.T) {
	server, store := setupTestServer(t)
	ctx := context.Background()

	// Register test event
	eventID := "orders.created"
	_ = store.CreateEvent(ctx, core.EventDefinition{
		ID:        eventID,
		Domain:    "Orders",
		Name:      "OrderCreated",
		Type:      "com.company.orders.created.v1",
		OwnerTeam: "checkout",
	})

	validSchema := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"properties": {
			"order_id": {"type": "string"},
			"total": {"type": "number"}
		},
		"required": ["order_id", "total"]
	}`

	// 1. Success -> 201 Created
	payload := fmt.Sprintf(`{
		"semver": "1.0.0",
		"json_schema": %q,
		"status": "approved"
	}`, validSchema)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/events/"+eventID+"/versions", strings.NewReader(payload))
	rec := httptest.NewRecorder()
	server.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d: %s", rec.Code, rec.Body.String())
	}

	var saved core.EventVersion
	if err := json.Unmarshal(rec.Body.Bytes(), &saved); err != nil {
		t.Fatalf("failed decoding event version: %v", err)
	}
	if saved.EventID != eventID {
		t.Errorf("expected EventID %s, got %s", eventID, saved.EventID)
	}
	if saved.SemVer != "1.0.0" {
		t.Errorf("expected SemVer 1.0.0, got %s", saved.SemVer)
	}
	if saved.CreatedBy == "" {
		t.Errorf("expected CreatedBy to be populated")
	}

	// 2. Duplicate semver -> 409 Conflict
	reqDup := httptest.NewRequest(http.MethodPost, "/api/v1/events/"+eventID+"/versions", strings.NewReader(payload))
	recDup := httptest.NewRecorder()
	server.ServeHTTP(recDup, reqDup)

	if recDup.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for duplicate version, got %d: %s", recDup.Code, recDup.Body.String())
	}

	// 3. Missing required fields (semver or json_schema) -> 400 Bad Request
	reqMissing := httptest.NewRequest(http.MethodPost, "/api/v1/events/"+eventID+"/versions", strings.NewReader(`{"semver": ""}`))
	recMissing := httptest.NewRecorder()
	server.ServeHTTP(recMissing, reqMissing)

	if recMissing.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for missing fields, got %d: %s", recMissing.Code, recMissing.Body.String())
	}

	// 4. Invalid json_schema syntax -> 400 Bad Request
	badSchemaPayload := `{"semver": "1.1.0", "json_schema": "not a valid json"}`
	reqBadSchema := httptest.NewRequest(http.MethodPost, "/api/v1/events/"+eventID+"/versions", strings.NewReader(badSchemaPayload))
	recBadSchema := httptest.NewRecorder()
	server.ServeHTTP(recBadSchema, reqBadSchema)

	if recBadSchema.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid schema syntax, got %d: %s", recBadSchema.Code, recBadSchema.Body.String())
	}

	// 5. Non-existent event -> 404 Not Found
	reqNotFound := httptest.NewRequest(http.MethodPost, "/api/v1/events/non.existent/versions", strings.NewReader(payload))
	recNotFound := httptest.NewRecorder()
	server.ServeHTTP(recNotFound, reqNotFound)

	if recNotFound.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for missing event, got %d: %s", recNotFound.Code, recNotFound.Body.String())
	}
}

func TestServer_CreateEventVersion_RBAC(t *testing.T) {
	t.Setenv("KALYX_AUTH_DISABLED", "false")
	store := storage.NewMemoryStorage()
	secret := []byte("test-secret-at-least-32-bytes-long!!")
	oidc := auth.NewOIDCEngine(auth.OIDCConfig{
		Secret: secret,
	})
	server := NewServer(store, WithOIDCEngine(oidc), WithAuthDisabled(false))

	// Event owned by "checkout"
	ctx := context.Background()
	_ = store.CreateEvent(ctx, core.EventDefinition{
		ID:        "orders.restricted",
		Domain:    "Orders",
		Name:      "RestrictedOrder",
		Type:      "com.company.orders.restricted.v1",
		OwnerTeam: "checkout",
	})

	validSchema := `{"$schema":"https://json-schema.org/draft/2020-12/schema","type":"object"}`
	versionPayload := fmt.Sprintf(`{"semver": "1.0.0", "json_schema": %q}`, validSchema)

	// User in "payments" team tries to propose for "checkout" event -> 403 Forbidden
	otherTeamClaims := &auth.JWTClaims{
		Subject:   "usr_payments",
		Email:     "payments_dev@company.com",
		Groups:    []any{"payments-member"},
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	otherToken, _ := auth.SignTestHS256Token(otherTeamClaims, secret)

	reqForbidden := httptest.NewRequest(http.MethodPost, "/api/v1/events/orders.restricted/versions", strings.NewReader(versionPayload))
	reqForbidden.Header.Set("Authorization", "Bearer "+otherToken)
	recForbidden := httptest.NewRecorder()
	server.ServeHTTP(recForbidden, reqForbidden)

	if recForbidden.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for cross-team propose, got %d: %s", recForbidden.Code, recForbidden.Body.String())
	}

	// User in "checkout" team proposes -> 201 Created
	checkoutClaims := &auth.JWTClaims{
		Subject:   "usr_checkout",
		Email:     "checkout_dev@company.com",
		Groups:    []any{"checkout-member"},
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	checkoutToken, _ := auth.SignTestHS256Token(checkoutClaims, secret)

	reqAllowed := httptest.NewRequest(http.MethodPost, "/api/v1/events/orders.restricted/versions", strings.NewReader(versionPayload))
	reqAllowed.Header.Set("Authorization", "Bearer "+checkoutToken)
	recAllowed := httptest.NewRecorder()
	server.ServeHTTP(recAllowed, reqAllowed)

	if recAllowed.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for team member, got %d: %s", recAllowed.Code, recAllowed.Body.String())
	}
}



