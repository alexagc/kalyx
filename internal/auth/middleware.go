package auth

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/alexagc/kalyx/internal/httputil"
	"github.com/alexagc/kalyx/internal/storage"
)

type userContextKeyType struct{}

var userCtxKey = userContextKeyType{}

// ContextWithUser associates a UserContext with the request context.
func ContextWithUser(ctx context.Context, user *UserContext) context.Context {
	return context.WithValue(ctx, userCtxKey, user)
}

// UserFromContext extracts the authenticated UserContext from the request context.
func UserFromContext(ctx context.Context) (*UserContext, bool) {
	if ctx == nil {
		return nil, false
	}
	u, ok := ctx.Value(userCtxKey).(*UserContext)
	return u, ok && u != nil
}

// Middleware enforces Bearer token authentication (PAT or OIDC JWT),
// injects UserContext into requests, and supports local dev bypass.
type Middleware struct {
	patEngine  *PATEngine
	oidcEngine *OIDCEngine
	bypass     bool
}

// NewMiddleware instantiates a new HTTP authentication middleware.
func NewMiddleware(store storage.Storage, oidc *OIDCEngine) *Middleware {
	return &Middleware{
		patEngine:  NewPATEngine(store),
		oidcEngine: oidc,
		bypass:     false,
	}
}

// SetBypass programmatically enables or disables auth enforcement.
func (m *Middleware) SetBypass(bypass bool) {
	m.bypass = bypass
}

// IsBypassed checks if auth is explicitly disabled via option or KALYX_AUTH_DISABLED env var.
func (m *Middleware) IsBypassed() bool {
	if m.bypass {
		return true
	}
	return strings.EqualFold(os.Getenv("KALYX_AUTH_DISABLED"), "true")
}

// Wrap wraps an http.Handler with Bearer authentication and UserContext injection.
func (m *Middleware) Wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Check if auth is bypassed for local development or testing
		if m.IsBypassed() {
			devUser := &UserContext{
				Subject:   "dev_user",
				Email:     "developer@kalyx.local",
				Teams:     []string{"*"},
				Roles:     []string{RoleDataSteward, RoleLead, RoleMember},
				IsMachine: false,
			}
			next.ServeHTTP(w, r.WithContext(ContextWithUser(r.Context(), devUser)))
			return
		}

		// 2. Extract Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			httputil.WriteProblemSimple(w, http.StatusUnauthorized, "Unauthorized", "missing Authorization header")
			return
		}

		parts := strings.SplitN(strings.TrimSpace(authHeader), " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			httputil.WriteProblemSimple(w, http.StatusUnauthorized, "Unauthorized", "invalid Authorization header format, expected 'Bearer <token>'")
			return
		}

		token := strings.TrimSpace(parts[1])
		if token == "" {
			httputil.WriteProblemSimple(w, http.StatusUnauthorized, "Unauthorized", "bearer token is empty")
			return
		}

		// 3. Differentiate between PAT ('kalyx_sec_...') and JWT tokens
		var user *UserContext
		var err error

		if strings.HasPrefix(token, PATPrefix) {
			user, err = m.patEngine.AuthenticatePAT(r.Context(), token)
		} else {
			if m.oidcEngine == nil {
				httputil.WriteProblemSimple(w, http.StatusUnauthorized, "Unauthorized", "OIDC identity provider is not configured")
				return
			}
			user, err = m.oidcEngine.ValidateToken(r.Context(), token)
		}

		if err != nil {
			httputil.WriteProblemSimple(w, http.StatusUnauthorized, "Unauthorized", err.Error())
			return
		}

		// 4. Attach UserContext and proceed
		next.ServeHTTP(w, r.WithContext(ContextWithUser(r.Context(), user)))
	})
}

// RequireRole creates middleware that ensures the authenticated user has the specified role.
func RequireRole(role string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			user, ok := UserFromContext(r.Context())
			if !ok || !user.HasRole(role) {
				httputil.WriteProblemSimple(w, http.StatusForbidden, "Forbidden", fmt.Sprintf("role '%s' is required to access this resource", role))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// RequireDataSteward is a convenience middleware requiring the 'data_steward' role.
func RequireDataSteward(next http.Handler) http.Handler {
	return RequireRole(RoleDataSteward)(next)
}
