package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alexagc/kalyx/internal/core"
	"github.com/alexagc/kalyx/internal/httputil"
	"github.com/alexagc/kalyx/internal/storage"
)

func TestPATEngine_GenerateAndAuthenticate(t *testing.T) {
	store := storage.NewMemoryStorage()
	engine := NewPATEngine(store)
	ctx := context.Background()

	// 1. Generate PAT
	expiresAt := time.Now().UTC().Add(24 * time.Hour)
	rawToken, patToken, err := engine.GeneratePAT(ctx, "CI Token", "ci@company.com", "checkout", &expiresAt)
	if err != nil {
		t.Fatalf("failed to generate PAT: %v", err)
	}

	// Verify prefix
	if !strings.HasPrefix(rawToken, PATPrefix) {
		t.Errorf("expected raw token to start with %s, got: %s", PATPrefix, rawToken)
	}

	// Token must have 10 chars ("kalyx_sec_") + 64 hex chars (32 bytes) = 74 chars
	if len(rawToken) != 74 {
		t.Errorf("expected token length 74, got %d", len(rawToken))
	}

	// Token hash must match SHA-256 of rawToken
	expectedHash := HashToken(rawToken)
	if patToken.TokenHash != expectedHash {
		t.Errorf("expected stored token hash %s, got %s", expectedHash, patToken.TokenHash)
	}

	// Stored token must not store plaintext token
	if strings.Contains(patToken.TokenHash, PATPrefix) {
		t.Errorf("token hash appears to contain plaintext token!")
	}

	// 2. Authenticate with valid token
	user, err := engine.AuthenticatePAT(ctx, rawToken)
	if err != nil {
		t.Fatalf("failed to authenticate valid PAT: %v", err)
	}

	if user.Email != "ci@company.com" {
		t.Errorf("expected email ci@company.com, got %s", user.Email)
	}
	if !user.IsInTeam("checkout") {
		t.Errorf("expected user to be in checkout team, got %v", user.Teams)
	}
	if !user.IsMachine {
		t.Errorf("expected PAT user to have IsMachine=true")
	}

	// Verify last_used_at was touched
	retrieved, err := store.GetPATByHash(ctx, expectedHash)
	if err != nil {
		t.Fatalf("failed to retrieve stored token: %v", err)
	}
	if retrieved.LastUsedAt == nil {
		t.Errorf("expected LastUsedAt to be updated after authentication")
	}

	// 3. Test invalid / altered token
	alteredToken := rawToken[:len(rawToken)-2] + "ff"
	_, err = engine.AuthenticatePAT(ctx, alteredToken)
	if err == nil {
		t.Fatalf("expected error for altered token, got nil")
	}

	// 4. Test expired token
	past := time.Now().UTC().Add(-1 * time.Hour)
	expiredRawToken, _, err := engine.GeneratePAT(ctx, "Expired Token", "ci@company.com", "checkout", &past)
	if err != nil {
		t.Fatalf("failed to generate expired PAT: %v", err)
	}
	_, err = engine.AuthenticatePAT(ctx, expiredRawToken)
	if err != ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired, got %v", err)
	}

	// 5. Test token without prefix
	_, err = engine.AuthenticatePAT(ctx, "invalid_prefix_token_12345")
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken for invalid prefix, got %v", err)
	}
}

func TestOIDCEngine_ValidateToken(t *testing.T) {
	secret := []byte("test-super-secret-key-1234567890")
	issuer := "https://auth.company.com"
	audience := "kalyx-api"

	engine := NewOIDCEngine(OIDCConfig{
		Issuer:    issuer,
		Audience:  audience,
		Secret:    secret,
		ClockSkew: 5 * time.Second,
	})

	ctx := context.Background()
	now := time.Now().Unix()

	// 1. Valid Token
	claims := &JWTClaims{
		Issuer:    issuer,
		Audience:  audience,
		Subject:   "usr_123",
		Email:     "alice@company.com",
		ExpiresAt: now + 3600,
		NotBefore: now - 10,
		IssuedAt:  now,
		Groups:    []any{"checkout-lead", "data_steward"},
	}

	tokenStr, err := SignTestHS256Token(claims, secret)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	user, err := engine.ValidateToken(ctx, tokenStr)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if user.Subject != "usr_123" {
		t.Errorf("expected sub usr_123, got %s", user.Subject)
	}
	if user.Email != "alice@company.com" {
		t.Errorf("expected email alice@company.com, got %s", user.Email)
	}
	if !user.IsInTeam("checkout") {
		t.Errorf("expected user to be in checkout team, got %v", user.Teams)
	}
	if !user.HasRole(RoleLead) {
		t.Errorf("expected user to have lead role, got %v", user.Roles)
	}
	if !user.HasRole(RoleDataSteward) {
		t.Errorf("expected user to have data_steward role, got %v", user.Roles)
	}

	// 2. Expired Token
	expiredClaims := &JWTClaims{
		Issuer:    issuer,
		Audience:  audience,
		Subject:   "usr_expired",
		ExpiresAt: now - 3600,
	}
	expiredToken, _ := SignTestHS256Token(expiredClaims, secret)
	_, err = engine.ValidateToken(ctx, expiredToken)
	if err != ErrJWTExpired {
		t.Errorf("expected ErrJWTExpired, got %v", err)
	}

	// 3. Invalid Issuer
	badIssuerClaims := &JWTClaims{
		Issuer:    "https://wrong-issuer.com",
		Audience:  audience,
		Subject:   "usr_bad_iss",
		ExpiresAt: now + 3600,
	}
	badIssToken, _ := SignTestHS256Token(badIssuerClaims, secret)
	_, err = engine.ValidateToken(ctx, badIssToken)
	if err == nil || !strings.Contains(err.Error(), "jwt issuer does not match") {
		t.Errorf("expected invalid issuer error, got %v", err)
	}

	// 4. Invalid Audience
	badAudClaims := &JWTClaims{
		Issuer:    issuer,
		Audience:  "wrong-api",
		Subject:   "usr_bad_aud",
		ExpiresAt: now + 3600,
	}
	badAudToken, _ := SignTestHS256Token(badAudClaims, secret)
	_, err = engine.ValidateToken(ctx, badAudToken)
	if err == nil || !strings.Contains(err.Error(), "jwt audience does not match") {
		t.Errorf("expected invalid audience error, got %v", err)
	}

	// 5. Invalid Signature
	wrongSecret := []byte("wrong-secret-9876543210")
	tamperedToken, _ := SignTestHS256Token(claims, wrongSecret)
	_, err = engine.ValidateToken(ctx, tamperedToken)
	if err == nil || !strings.Contains(err.Error(), "invalid jwt signature") {
		t.Errorf("expected invalid signature error, got %v", err)
	}
}

func TestMapGroupsToTeamsAndRoles(t *testing.T) {
	groups := []string{
		"data_steward",
		"checkout-lead",
		"orders-member",
		"billing",
	}

	teams, roles := MapGroupsToTeamsAndRoles(groups, nil, nil)

	teamSet := make(map[string]bool)
	for _, t := range teams {
		teamSet[t] = true
	}
	roleSet := make(map[string]bool)
	for _, r := range roles {
		roleSet[r] = true
	}

	if !teamSet["checkout"] {
		t.Errorf("missing team 'checkout'")
	}
	if !teamSet["orders"] {
		t.Errorf("missing team 'orders'")
	}
	if !teamSet["billing"] {
		t.Errorf("missing team 'billing'")
	}

	if !roleSet[RoleDataSteward] {
		t.Errorf("missing role 'data_steward'")
	}
	if !roleSet[RoleLead] {
		t.Errorf("missing role 'lead'")
	}
	if !roleSet[RoleMember] {
		t.Errorf("missing role 'member'")
	}
}

func TestRBAC_CanPropose(t *testing.T) {
	eventCheckout := &core.EventDefinition{
		ID:        "ev_checkout",
		OwnerTeam: "checkout",
	}

	userCheckoutMember := &UserContext{
		Email: "alice@company.com",
		Teams: []string{"checkout"},
		Roles: []string{RoleMember},
	}

	userOrdersMember := &UserContext{
		Email: "bob@company.com",
		Teams: []string{"orders"},
		Roles: []string{RoleMember},
	}

	userLead := &UserContext{
		Email: "lead@company.com",
		Teams: []string{"orders"},
		Roles: []string{RoleLead},
	}

	userSteward := &UserContext{
		Email: "steward@company.com",
		Teams: []string{"governance"},
		Roles: []string{RoleDataSteward},
	}

	// 1. Team member proposing for own team -> true
	if !CanPropose(userCheckoutMember, eventCheckout) {
		t.Errorf("checkout member should be allowed to propose for checkout event")
	}

	// 2. Team member proposing for other team -> false
	if CanPropose(userOrdersMember, eventCheckout) {
		t.Errorf("orders member should NOT be allowed to propose for checkout event")
	}

	// 3. Lead proposing for any team -> true
	if !CanPropose(userLead, eventCheckout) {
		t.Errorf("lead should be allowed to propose for any event")
	}

	// 4. Data Steward proposing for any team -> true
	if !CanPropose(userSteward, eventCheckout) {
		t.Errorf("data steward should be allowed to propose for any event")
	}

	// 5. Nil user -> false
	if CanPropose(nil, eventCheckout) {
		t.Errorf("nil user must not be allowed to propose")
	}
}

func TestRBAC_CanApprove(t *testing.T) {
	eventCheckout := &core.EventDefinition{
		ID:        "ev_checkout",
		OwnerTeam: "checkout",
	}

	userCheckoutMember := &UserContext{
		Email: "alice@company.com",
		Teams: []string{"checkout"},
		Roles: []string{RoleMember},
	}

	userCheckoutLead := &UserContext{
		Email: "lead_checkout@company.com",
		Teams: []string{"checkout"},
		Roles: []string{RoleLead},
	}

	userOrdersLead := &UserContext{
		Email: "lead_orders@company.com",
		Teams: []string{"orders"},
		Roles: []string{RoleLead},
	}

	userDataSteward := &UserContext{
		Email: "steward@company.com",
		Teams: []string{"governance"},
		Roles: []string{RoleDataSteward},
	}

	// Case 1: Non-Breaking Change (isBreaking = false)
	// - Checkout Lead: allowed
	if !CanApprove(userCheckoutLead, eventCheckout, false) {
		t.Errorf("checkout lead should be allowed to approve non-breaking changes for checkout event")
	}
	// - Orders Lead: NOT allowed
	if CanApprove(userOrdersLead, eventCheckout, false) {
		t.Errorf("orders lead should NOT be allowed to approve changes for checkout event")
	}
	// - Checkout Member: NOT allowed
	if CanApprove(userCheckoutMember, eventCheckout, false) {
		t.Errorf("regular member should NOT be allowed to approve changes")
	}
	// - Data Steward: allowed
	if !CanApprove(userDataSteward, eventCheckout, false) {
		t.Errorf("data steward should be allowed to approve non-breaking changes")
	}

	// Case 2: Breaking Change (isBreaking = true)
	// - Checkout Lead: NOT allowed (MUST be Data Steward!)
	if CanApprove(userCheckoutLead, eventCheckout, true) {
		t.Errorf("checkout lead must NOT be allowed to approve breaking changes")
	}
	// - Regular Member: NOT allowed
	if CanApprove(userCheckoutMember, eventCheckout, true) {
		t.Errorf("regular member must NOT be allowed to approve breaking changes")
	}
	// - Data Steward: strictly allowed
	if !CanApprove(userDataSteward, eventCheckout, true) {
		t.Errorf("data steward MUST be allowed to approve breaking changes")
	}
}

func TestMiddleware_AuthenticationAndRFC7807(t *testing.T) {
	store := storage.NewMemoryStorage()
	secret := []byte("secret-key-32-bytes-long-for-test!")
	oidc := NewOIDCEngine(OIDCConfig{
		Secret: secret,
	})

	middleware := NewMiddleware(store, oidc)
	ctx := context.Background()

	// Create test handler
	handler := middleware.Wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u, ok := UserFromContext(r.Context())
		if !ok {
			http.Error(w, "missing context user", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(u)
	}))

	// 1. Missing Authorization header -> 401 Unauthorized RFC 7807
	reqNoAuth := httptest.NewRequest(http.MethodGet, "/test", nil)
	recNoAuth := httptest.NewRecorder()
	handler.ServeHTTP(recNoAuth, reqNoAuth)

	if recNoAuth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", recNoAuth.Code)
	}
	if recNoAuth.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("expected application/problem+json, got %s", recNoAuth.Header().Get("Content-Type"))
	}
	var prob httputil.ProblemDetails
	_ = json.Unmarshal(recNoAuth.Body.Bytes(), &prob)
	if prob.Type != "urn:kalyx:error:unauthorized" {
		t.Errorf("expected urn:kalyx:error:unauthorized, got %s", prob.Type)
	}

	// 2. Malformed Authorization header -> 401
	reqBadAuth := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqBadAuth.Header.Set("Authorization", "Basic 12345")
	recBadAuth := httptest.NewRecorder()
	handler.ServeHTTP(recBadAuth, reqBadAuth)

	if recBadAuth.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for malformed auth header, got %d", recBadAuth.Code)
	}

	// 3. Valid PAT token -> 200 OK
	rawPAT, _, err := middleware.patEngine.GeneratePAT(ctx, "Test PAT", "ci@company.com", "checkout", nil)
	if err != nil {
		t.Fatalf("failed to generate PAT: %v", err)
	}

	reqPAT := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqPAT.Header.Set("Authorization", "Bearer "+rawPAT)
	recPAT := httptest.NewRecorder()
	handler.ServeHTTP(recPAT, reqPAT)

	if recPAT.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid PAT, got %d: %s", recPAT.Code, recPAT.Body.String())
	}
	var patUser UserContext
	_ = json.Unmarshal(recPAT.Body.Bytes(), &patUser)
	if patUser.Email != "ci@company.com" || !patUser.IsMachine {
		t.Errorf("unexpected user from PAT: %+v", patUser)
	}

	// 4. Valid JWT token -> 200 OK
	claims := &JWTClaims{
		Subject:   "usr_jwt",
		Email:     "jwt_user@company.com",
		Groups:    []any{"data_steward"},
		ExpiresAt: time.Now().Add(time.Hour).Unix(),
	}
	tokenStr, _ := SignTestHS256Token(claims, secret)

	reqJWT := httptest.NewRequest(http.MethodGet, "/test", nil)
	reqJWT.Header.Set("Authorization", "Bearer "+tokenStr)
	recJWT := httptest.NewRecorder()
	handler.ServeHTTP(recJWT, reqJWT)

	if recJWT.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid JWT, got %d: %s", recJWT.Code, recJWT.Body.String())
	}
	var jwtUser UserContext
	_ = json.Unmarshal(recJWT.Body.Bytes(), &jwtUser)
	if jwtUser.Email != "jwt_user@company.com" || !jwtUser.HasRole(RoleDataSteward) {
		t.Errorf("unexpected user from JWT: %+v", jwtUser)
	}

	// 5. Bypass via KALYX_AUTH_DISABLED=true -> 200 OK without token
	t.Setenv("KALYX_AUTH_DISABLED", "true")
	reqBypass := httptest.NewRequest(http.MethodGet, "/test", nil)
	recBypass := httptest.NewRecorder()
	handler.ServeHTTP(recBypass, reqBypass)

	if recBypass.Code != http.StatusOK {
		t.Fatalf("expected 200 with auth disabled, got %d: %s", recBypass.Code, recBypass.Body.String())
	}
	var devUser UserContext
	_ = json.Unmarshal(recBypass.Body.Bytes(), &devUser)
	if !devUser.HasRole(RoleDataSteward) {
		t.Errorf("expected dev user to have Data Steward role")
	}
}

func TestRequireRole_Forbidden(t *testing.T) {
	guard := RequireDataSteward(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// User lacking Data Steward role
	user := &UserContext{
		Email: "member@company.com",
		Roles: []string{RoleMember},
	}

	req := httptest.NewRequest(http.MethodPost, "/approve", nil)
	req = req.WithContext(ContextWithUser(req.Context(), user))
	rec := httptest.NewRecorder()

	guard.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d", rec.Code)
	}

	if rec.Header().Get("Content-Type") != "application/problem+json" {
		t.Errorf("expected application/problem+json, got %s", rec.Header().Get("Content-Type"))
	}

	var prob httputil.ProblemDetails
	_ = json.Unmarshal(rec.Body.Bytes(), &prob)
	if prob.Type != "urn:kalyx:error:forbidden" {
		t.Errorf("expected urn:kalyx:error:forbidden, got %s", prob.Type)
	}
}

func TestRBAC_CanDeprecate(t *testing.T) {
	event := &core.EventDefinition{
		ID:        "ev_test",
		OwnerTeam: "checkout",
	}

	steward := &UserContext{Roles: []string{RoleDataSteward}}
	leadInTeam := &UserContext{Roles: []string{RoleLead}, Teams: []string{"checkout"}}
	leadOtherTeam := &UserContext{Roles: []string{RoleLead}, Teams: []string{"billing"}}
	member := &UserContext{Roles: []string{RoleMember}, Teams: []string{"checkout"}}

	if !CanDeprecate(steward, event) {
		t.Errorf("data steward should be able to deprecate")
	}
	if !CanDeprecate(leadInTeam, event) {
		t.Errorf("lead in team should be able to deprecate")
	}
	if CanDeprecate(leadOtherTeam, event) {
		t.Errorf("lead in other team should NOT be able to deprecate")
	}
	if CanDeprecate(member, event) {
		t.Errorf("regular member should NOT be able to deprecate")
	}
	if CanDeprecate(nil, event) {
		t.Errorf("nil user should NOT be able to deprecate")
	}
}

func TestOIDCEngine_EdgeCases(t *testing.T) {
	secret := []byte("secret-key-32-bytes-long-for-test!")
	engine := NewOIDCEngine(OIDCConfig{
		Secret: secret,
		GroupRoleOverrides: map[string]string{
			"custom-admin": RoleDataSteward,
		},
		GroupTeamOverrides: map[string]string{
			"custom-team": "special-ops",
		},
	})

	ctx := context.Background()
	now := time.Now().Unix()

	claims := &JWTClaims{
		Subject:   "usr_edge",
		ExpiresAt: now + 3600,
		NotBefore: now + 300, // Future not before
		Audience:  []string{"api1", "api2"},
		Groups:    "checkout,billing",
	}

	tokenStr, _ := SignTestHS256Token(claims, secret)
	_, err := engine.ValidateToken(ctx, tokenStr)
	if !errors.Is(err, ErrJWTNotYetValid) {
		t.Errorf("expected ErrJWTNotYetValid, got %v", err)
	}

	// Test overrides
	validClaims := &JWTClaims{
		Subject:   "usr_overrides",
		ExpiresAt: now + 3600,
		Groups:    []any{"custom-admin", "custom-team", "team:infra", "analytics-member"},
	}
	validToken, _ := SignTestHS256Token(validClaims, secret)
	u, err := engine.ValidateToken(ctx, validToken)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !u.HasRole(RoleDataSteward) {
		t.Errorf("expected role override to grant data_steward")
	}
	if !u.IsInTeam("special-ops") {
		t.Errorf("expected team override to grant special-ops")
	}
	if !u.IsInTeam("infra") {
		t.Errorf("expected team:infra to grant infra team")
	}
	if !u.IsInTeam("analytics") {
		t.Errorf("expected analytics-member to grant analytics team")
	}
}
