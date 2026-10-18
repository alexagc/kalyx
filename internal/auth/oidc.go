package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Kalyx standard authorization roles.
const (
	RoleMember      = "member"
	RoleLead        = "lead"
	RoleDataSteward = "data_steward"
)

var (
	ErrInvalidJWTFormat    = errors.New("invalid jwt format")
	ErrInvalidJWTSignature = errors.New("invalid jwt signature")
	ErrInvalidIssuer       = errors.New("jwt issuer does not match")
	ErrInvalidAudience     = errors.New("jwt audience does not match")
	ErrJWTExpired          = errors.New("jwt token has expired")
	ErrJWTNotYetValid      = errors.New("jwt token is not yet valid")
)

// UserContext carries the authenticated identity, assigned teams, and resolved roles.
type UserContext struct {
	Subject   string   `json:"sub"`
	Email     string   `json:"email"`
	Teams     []string `json:"teams"`
	Roles     []string `json:"roles"`
	IsMachine bool     `json:"is_machine"`
}

// HasRole returns true if the user possesses the specified role.
func (u *UserContext) HasRole(role string) bool {
	if u == nil {
		return false
	}
	for _, r := range u.Roles {
		if strings.EqualFold(r, role) {
			return true
		}
	}
	return false
}

// IsInTeam returns true if the user is a member of the specified team.
func (u *UserContext) IsInTeam(team string) bool {
	if u == nil || team == "" {
		return false
	}
	for _, t := range u.Teams {
		if strings.EqualFold(t, team) {
			return true
		}
	}
	return false
}

// JWTHeader represents the standard JWT JOSE header.
type JWTHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

// JWTClaims represents standard and custom OIDC claims.
type JWTClaims struct {
	Issuer        string `json:"iss,omitempty"`
	Subject       string `json:"sub,omitempty"`
	Audience      any    `json:"aud,omitempty"`
	ExpiresAt     int64  `json:"exp,omitempty"`
	NotBefore     int64  `json:"nbf,omitempty"`
	IssuedAt      int64  `json:"iat,omitempty"`
	Email         string `json:"email,omitempty"`
	Groups        any    `json:"groups,omitempty"`
	Roles         any    `json:"roles,omitempty"`
	CognitoGroups any    `json:"cognito:groups,omitempty"`
}

// Audiences extracts all audience strings from the claims.
func (c *JWTClaims) Audiences() []string {
	if c.Audience == nil {
		return nil
	}
	switch v := c.Audience.(type) {
	case string:
		return []string{v}
	case []any:
		res := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				res = append(res, s)
			}
		}
		return res
	case []string:
		return v
	default:
		return nil
	}
}

// ExtractGroups resolves raw groups from groups, roles, or cognito:groups claims.
func (c *JWTClaims) ExtractGroups() []string {
	var rawList []any

	if c.Groups != nil {
		switch v := c.Groups.(type) {
		case []any:
			rawList = append(rawList, v...)
		case []string:
			return v
		case string:
			return strings.Split(v, ",")
		}
	}

	if c.Roles != nil {
		if rList, ok := c.Roles.([]any); ok {
			rawList = append(rawList, rList...)
		}
	}

	if c.CognitoGroups != nil {
		if cgList, ok := c.CognitoGroups.([]any); ok {
			rawList = append(rawList, cgList...)
		}
	}

	groups := make([]string, 0, len(rawList))
	for _, item := range rawList {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			groups = append(groups, strings.TrimSpace(s))
		}
	}
	return groups
}

// OIDCConfig defines validation and identity-mapping parameters for OIDC JWTs.
type OIDCConfig struct {
	Issuer                  string
	Audience                string
	Secret                  []byte        // HMAC secret for HS256 tokens
	SkipSignatureValidation bool          // Useful for dev or behind API gateway
	ClockSkew               time.Duration // Allowance for clock skew (default: 1 min)
	GroupRoleOverrides      map[string]string
	GroupTeamOverrides      map[string]string
}

// OIDCEngine handles OIDC token parsing, cryptographic verification, and claims mapping.
type OIDCEngine struct {
	config OIDCConfig
}

// NewOIDCEngine initializes an OIDC engine with the provided configuration.
func NewOIDCEngine(config OIDCConfig) *OIDCEngine {
	if config.ClockSkew == 0 {
		config.ClockSkew = 1 * time.Minute
	}
	return &OIDCEngine{config: config}
}

// ValidateToken parses a Bearer JWT, validates standard claims (iss, aud, exp, nbf),
// verifies HMAC signature if configured, maps groups to Kalyx Teams and Roles, and returns *UserContext.
func (e *OIDCEngine) ValidateToken(ctx context.Context, tokenString string) (*UserContext, error) {
	tokenString = strings.TrimSpace(tokenString)
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidJWTFormat
	}

	// 1. Decode Header
	headerBytes, err := decodeBase64URL(parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decode header: %v", ErrInvalidJWTFormat, err)
	}
	var header JWTHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("%w: failed to parse header json: %v", ErrInvalidJWTFormat, err)
	}

	// 2. Signature verification
	if !e.config.SkipSignatureValidation && len(e.config.Secret) > 0 {
		if header.Algorithm != "HS256" {
			return nil, fmt.Errorf("%w: unsupported algorithm %s", ErrInvalidJWTSignature, header.Algorithm)
		}
		expectedSig := signHMACSHA256([]byte(parts[0]+"."+parts[1]), e.config.Secret)
		sigBytes, err := decodeBase64URL(parts[2])
		if err != nil {
			return nil, fmt.Errorf("%w: failed to decode signature: %v", ErrInvalidJWTSignature, err)
		}
		if subtle.ConstantTimeCompare(sigBytes, expectedSig) != 1 {
			return nil, ErrInvalidJWTSignature
		}
	}

	// 3. Decode Claims
	payloadBytes, err := decodeBase64URL(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decode payload: %v", ErrInvalidJWTFormat, err)
	}
	var claims JWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("%w: failed to parse payload json: %v", ErrInvalidJWTFormat, err)
	}

	// 4. Validate Standard Claims
	now := time.Now().Unix()
	skewSec := int64(e.config.ClockSkew.Seconds())

	// Expiration check (exp)
	if claims.ExpiresAt > 0 {
		if now > claims.ExpiresAt+skewSec {
			return nil, ErrJWTExpired
		}
	}

	// Not Before check (nbf)
	if claims.NotBefore > 0 {
		if now < claims.NotBefore-skewSec {
			return nil, ErrJWTNotYetValid
		}
	}

	// Issuer check (iss)
	if e.config.Issuer != "" && claims.Issuer != e.config.Issuer {
		return nil, fmt.Errorf("%w: expected %s, got %s", ErrInvalidIssuer, e.config.Issuer, claims.Issuer)
	}

	// Audience check (aud)
	if e.config.Audience != "" {
		audList := claims.Audiences()
		matched := false
		for _, aud := range audList {
			if aud == e.config.Audience {
				matched = true
				break
			}
		}
		if !matched {
			return nil, fmt.Errorf("%w: expected audience %s", ErrInvalidAudience, e.config.Audience)
		}
	}

	// 5. Map IdP groups claim to Kalyx Teams and Roles
	rawGroups := claims.ExtractGroups()
	teams, roles := MapGroupsToTeamsAndRoles(rawGroups, e.config.GroupRoleOverrides, e.config.GroupTeamOverrides)

	email := claims.Email
	if email == "" {
		email = claims.Subject
	}

	return &UserContext{
		Subject:   claims.Subject,
		Email:     email,
		Teams:     teams,
		Roles:     roles,
		IsMachine: false,
	}, nil
}

// MapGroupsToTeamsAndRoles converts IdP groups into Kalyx Teams and Roles.
// Rules:
// - 'data_steward', 'data-steward', 'steward' -> role 'data_steward'
// - 'admin', 'administrator' -> roles 'data_steward', 'lead', 'member'
// - 'lead', 'team-lead' -> role 'lead'
// - '<team>-lead' or '<team>_lead' -> team '<team>', roles 'lead', 'member'
// - '<team>-member' or '<team>_member' -> team '<team>', role 'member'
// - '<team>' -> team '<team>', role 'member'
func MapGroupsToTeamsAndRoles(rawGroups []string, roleOverrides, teamOverrides map[string]string) ([]string, []string) {
	teamMap := make(map[string]struct{})
	roleMap := make(map[string]struct{})

	for _, g := range rawGroups {
		trimmed := strings.TrimSpace(g)
		if trimmed == "" {
			continue
		}
		lower := strings.ToLower(trimmed)

		// Check custom overrides first
		if roleOverrides != nil {
			if r, ok := roleOverrides[lower]; ok {
				roleMap[r] = struct{}{}
			}
		}
		if teamOverrides != nil {
			if t, ok := teamOverrides[lower]; ok {
				teamMap[t] = struct{}{}
			}
		}

		// Built-in mapping conventions
		switch lower {
		case "data_steward", "data-steward", "datasteward", "steward", "stewards":
			roleMap[RoleDataSteward] = struct{}{}
			roleMap[RoleMember] = struct{}{}
			continue
		case "admin", "administrator":
			roleMap[RoleDataSteward] = struct{}{}
			roleMap[RoleLead] = struct{}{}
			roleMap[RoleMember] = struct{}{}
			continue
		case "lead", "team-lead", "team_lead":
			roleMap[RoleLead] = struct{}{}
			roleMap[RoleMember] = struct{}{}
			continue
		}

		// Pattern: <team>-lead or <team>_lead
		if strings.HasSuffix(lower, "-lead") {
			t := strings.TrimSuffix(lower, "-lead")
			if t != "" {
				teamMap[t] = struct{}{}
				roleMap[RoleLead] = struct{}{}
				roleMap[RoleMember] = struct{}{}
				continue
			}
		}
		if strings.HasSuffix(lower, "_lead") {
			t := strings.TrimSuffix(lower, "_lead")
			if t != "" {
				teamMap[t] = struct{}{}
				roleMap[RoleLead] = struct{}{}
				roleMap[RoleMember] = struct{}{}
				continue
			}
		}

		// Pattern: <team>-member or <team>_member
		if strings.HasSuffix(lower, "-member") {
			t := strings.TrimSuffix(lower, "-member")
			if t != "" {
				teamMap[t] = struct{}{}
				roleMap[RoleMember] = struct{}{}
				continue
			}
		}
		if strings.HasSuffix(lower, "_member") {
			t := strings.TrimSuffix(lower, "_member")
			if t != "" {
				teamMap[t] = struct{}{}
				roleMap[RoleMember] = struct{}{}
				continue
			}
		}

		// Prefix pattern: team:<name> or org:<name>
		if strings.HasPrefix(lower, "team:") {
			t := strings.TrimPrefix(lower, "team:")
			if t != "" {
				teamMap[t] = struct{}{}
				roleMap[RoleMember] = struct{}{}
				continue
			}
		}

		// Default: treat unmapped group as team name
		teamMap[lower] = struct{}{}
		roleMap[RoleMember] = struct{}{}
	}

	// Always ensure at least member role if no role was assigned
	if len(roleMap) == 0 {
		roleMap[RoleMember] = struct{}{}
	}

	teams := make([]string, 0, len(teamMap))
	for t := range teamMap {
		teams = append(teams, t)
	}

	roles := make([]string, 0, len(roleMap))
	for r := range roleMap {
		roles = append(roles, r)
	}

	return teams, roles
}

// Helper functions for base64url and HMAC-SHA256

func decodeBase64URL(s string) ([]byte, error) {
	// Support unpadded or padded base64url
	if m := len(s) % 4; m != 0 {
		s += strings.Repeat("=", 4-m)
	}
	return base64.URLEncoding.DecodeString(s)
}

func encodeBase64URL(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

func signHMACSHA256(data, secret []byte) []byte {
	h := hmac.New(sha256.New, secret)
	h.Write(data)
	return h.Sum(nil)
}

// SignTestHS256Token produces a signed JWT string for testing.
func SignTestHS256Token(claims *JWTClaims, secret []byte) (string, error) {
	header := JWTHeader{
		Algorithm: "HS256",
		Type:      "JWT",
	}
	hBytes, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	cBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedHeader := encodeBase64URL(hBytes)
	encodedPayload := encodeBase64URL(cBytes)
	unsignedToken := encodedHeader + "." + encodedPayload

	sig := signHMACSHA256([]byte(unsignedToken), secret)
	encodedSig := encodeBase64URL(sig)

	return unsignedToken + "." + encodedSig, nil
}
