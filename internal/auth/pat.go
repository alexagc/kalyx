package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alexagc/kalyx/internal/core"
	"github.com/alexagc/kalyx/internal/storage"
)

const (
	// PATPrefix is the required prefix for all Kalyx Personal Access Tokens.
	PATPrefix = "kalyx_sec_"
)

var (
	ErrInvalidToken  = errors.New("invalid personal access token")
	ErrTokenExpired  = errors.New("personal access token has expired")
	ErrTokenNotFound = errors.New("personal access token not found")
)

// PATEngine provides secure generation, hashing, timing-safe validation,
// and lifecycle management for Personal Access Tokens.
type PATEngine struct {
	store storage.Storage
}

// NewPATEngine instantiates a new PATEngine with backing persistence.
func NewPATEngine(store storage.Storage) *PATEngine {
	return &PATEngine{store: store}
}

// HashToken computes a SHA-256 hex string hash of a raw token.
func HashToken(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return hex.EncodeToString(sum[:])
}

// GeneratePAT creates a new Personal Access Token with format:
// 'kalyx_sec_' prefix + 32-byte cryptographically secure random hex string.
// Computes and stores the SHA-256 hash; the plain token is returned only once.
func (e *PATEngine) GeneratePAT(ctx context.Context, name, ownerEmail, team string, expiresAt *time.Time) (string, *core.PATToken, error) {
	if strings.TrimSpace(name) == "" {
		return "", nil, fmt.Errorf("token name is required")
	}
	if strings.TrimSpace(ownerEmail) == "" {
		return "", nil, fmt.Errorf("token owner email is required")
	}

	// 32 cryptographically secure random bytes = 64 hex characters
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", nil, fmt.Errorf("failed to generate random bytes for token: %w", err)
	}

	rawToken := PATPrefix + hex.EncodeToString(tokenBytes)
	tokenHash := HashToken(rawToken)

	// Unique ID generation for token entity
	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		return "", nil, fmt.Errorf("failed to generate random ID: %w", err)
	}
	tokenID := "pat_" + hex.EncodeToString(idBytes)

	now := time.Now().UTC()
	token := core.PATToken{
		ID:         tokenID,
		Name:       strings.TrimSpace(name),
		TokenHash:  tokenHash,
		OwnerEmail: strings.TrimSpace(ownerEmail),
		Team:       strings.TrimSpace(team),
		ExpiresAt:  expiresAt,
		CreatedAt:  now,
	}

	if err := e.store.SavePAT(ctx, token); err != nil {
		return "", nil, fmt.Errorf("failed to save PAT: %w", err)
	}

	return rawToken, &token, nil
}

// AuthenticatePAT validates a raw token string:
// 1. Verifies 'kalyx_sec_' prefix.
// 2. Computes SHA-256 hash.
// 3. Retrieves PAT from storage.
// 4. Performs timing-safe comparison with crypto/subtle.ConstantTimeCompare.
// 5. Verifies token is not expired.
// 6. Touches last_used_at timestamp.
// 7. Returns the associated UserContext.
func (e *PATEngine) AuthenticatePAT(ctx context.Context, rawToken string) (*UserContext, error) {
	rawToken = strings.TrimSpace(rawToken)
	if !strings.HasPrefix(rawToken, PATPrefix) {
		return nil, ErrInvalidToken
	}

	computedHash := HashToken(rawToken)
	token, err := e.store.GetPATByHash(ctx, computedHash)
	if err != nil {
		if errors.Is(err, storage.ErrTokenNotFound) {
			return nil, ErrInvalidToken
		}
		return nil, fmt.Errorf("failed to retrieve PAT: %w", err)
	}

	// Constant-time comparison to prevent timing attacks
	if subtle.ConstantTimeCompare([]byte(token.TokenHash), []byte(computedHash)) != 1 {
		return nil, ErrInvalidToken
	}

	// Expiration check
	if token.ExpiresAt != nil && token.ExpiresAt.Before(time.Now().UTC()) {
		return nil, ErrTokenExpired
	}

	// Update last_used_at asynchronously or synchronously in storage
	_ = e.store.TouchPAT(ctx, token.ID)

	teams := make([]string, 0)
	if token.Team != "" {
		teams = append(teams, token.Team)
	}

	return &UserContext{
		Subject:   token.ID,
		Email:     token.OwnerEmail,
		Teams:     teams,
		Roles:     []string{RoleMember},
		IsMachine: true,
	}, nil
}
