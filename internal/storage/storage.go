package storage

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/alexagc/kalyx/internal/core"
)

var (
	ErrEventNotFound    = errors.New("event not found")
	ErrVersionNotFound  = errors.New("event version not found")
	ErrProposalNotFound = errors.New("change proposal not found")
	ErrTokenNotFound    = errors.New("access token not found")
	ErrConflict         = errors.New("resource already exists")
)

// Storage defines the persistence contract for all database backends.
type Storage interface {
	// Lifecycle & Health
	Ping(ctx context.Context) error
	Close() error

	// Event Operations
	ListEvents(ctx context.Context) ([]core.EventDefinition, error)
	GetEvent(ctx context.Context, id string) (*core.EventDefinition, error)
	CreateEvent(ctx context.Context, event core.EventDefinition) error

	// Version Operations
	ListVersions(ctx context.Context, eventID string) ([]core.EventVersion, error)
	GetEventVersion(ctx context.Context, eventID, semver string) (*core.EventVersion, error)
	CreateEventVersion(ctx context.Context, version core.EventVersion) error
	UpdateEventVersionStatus(ctx context.Context, eventID, semver string, status core.EventStatus) error

	// Governance Proposal Operations
	ListProposals(ctx context.Context, status *core.EventStatus) ([]core.ChangeProposal, error)
	GetProposal(ctx context.Context, id string) (*core.ChangeProposal, error)
	CreateProposal(ctx context.Context, proposal core.ChangeProposal) error
	UpdateProposalStatus(ctx context.Context, id string, status core.EventStatus, reviewer, notes string) error

	// Security & Token Operations
	SavePAT(ctx context.Context, token core.PATToken) error
	GetPATByHash(ctx context.Context, tokenHash string) (*core.PATToken, error)
	ListPATs(ctx context.Context, ownerEmail string) ([]core.PATToken, error)
	ListPATTokens(ctx context.Context, ownerEmail string) ([]*core.PATToken, error)
	RevokePAT(ctx context.Context, id string) error
	DeletePATToken(ctx context.Context, id string) error
	TouchPAT(ctx context.Context, id string) error
}

// MemoryStorage is a thread-safe zero-dependency in-memory store for quickstart, unit testing, and fakes.
type MemoryStorage struct {
	mu        sync.RWMutex
	events    map[string]core.EventDefinition
	versions  map[string][]core.EventVersion
	proposals map[string]core.ChangeProposal
	tokens    map[string]core.PATToken
}

// NewMemoryStorage initializes an in-memory storage instance with initial sample data.
func NewMemoryStorage() *MemoryStorage {
	s := &MemoryStorage{
		events:    make(map[string]core.EventDefinition),
		versions:  make(map[string][]core.EventVersion),
		proposals: make(map[string]core.ChangeProposal),
		tokens:    make(map[string]core.PATToken),
	}

	// Seed sample CloudEvent contract
	sampleEvent := core.EventDefinition{
		ID:          "ev_orders_completed",
		Domain:      "Orders",
		Name:        "OrderCompleted",
		Type:        "com.company.orders.completed.v1",
		Description: "Emitted when a customer checkout is confirmed and paid.",
		OwnerTeam:   "Checkout Core",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}
	s.events[sampleEvent.ID] = sampleEvent

	sampleVersion := core.EventVersion{
		ID:         "ver_1_0_0",
		EventID:    sampleEvent.ID,
		SemVer:     "1.0.0",
		Status:     core.StatusApproved,
		CreatedAt:  time.Now().UTC(),
		CreatedBy:  "alice@company.com",
		ApprovedBy: "data-steward@company.com",
		DataLakeMetadata: core.DataLakeMetadata{
			TargetTable: "lake_orders_completed",
			PartitionBy: []string{"country"},
			PIIFields:   []string{"customer_email"},
		},
		JSONSchema: `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "title": "OrderCompletedPayload",
  "type": "object",
  "properties": {
    "order_id": { "type": "string", "description": "Unique order identifier" },
    "customer_email": { "type": "string", "description": "Customer contact email" },
    "amount": { "type": "number", "description": "Total purchase amount" },
    "country": { "type": "string", "description": "ISO 2-letter country code" }
  },
  "required": ["order_id", "customer_email", "amount", "country"]
}`,
	}
	s.versions[sampleEvent.ID] = []core.EventVersion{sampleVersion}

	return s
}

// Ping checks storage responsiveness.
func (s *MemoryStorage) Ping(ctx context.Context) error {
	return nil
}

// Close releases any allocated resources.
func (s *MemoryStorage) Close() error {
	return nil
}

// ListEvents retrieves all registered event definitions.
func (s *MemoryStorage) ListEvents(ctx context.Context) ([]core.EventDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]core.EventDefinition, 0, len(s.events))
	for _, ev := range s.events {
		list = append(list, ev)
	}
	return list, nil
}

// GetEvent fetches a single event definition by ID.
func (s *MemoryStorage) GetEvent(ctx context.Context, id string) (*core.EventDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ev, exists := s.events[id]
	if !exists {
		return nil, ErrEventNotFound
	}
	return &ev, nil
}

// CreateEvent persists a new event definition.
func (s *MemoryStorage) CreateEvent(ctx context.Context, event core.EventDefinition) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.events[event.ID]; exists {
		return ErrConflict
	}
	s.events[event.ID] = event
	return nil
}

// ListVersions returns all immutable contract versions for an event.
func (s *MemoryStorage) ListVersions(ctx context.Context, eventID string) ([]core.EventVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, exists := s.versions[eventID]
	if !exists {
		return []core.EventVersion{}, nil
	}
	res := make([]core.EventVersion, len(versions))
	copy(res, versions)
	return res, nil
}

// GetEventVersion retrieves a specific version of an event contract.
func (s *MemoryStorage) GetEventVersion(ctx context.Context, eventID, semver string) (*core.EventVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, exists := s.versions[eventID]
	if !exists {
		return nil, ErrVersionNotFound
	}
	for _, v := range versions {
		if v.SemVer == semver {
			res := v
			return &res, nil
		}
	}
	return nil, ErrVersionNotFound
}

// CreateEventVersion records a new contract version.
func (s *MemoryStorage) CreateEventVersion(ctx context.Context, version core.EventVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	existing := s.versions[version.EventID]
	for _, v := range existing {
		if v.SemVer == version.SemVer {
			return ErrConflict
		}
	}

	s.versions[version.EventID] = append(existing, version)
	return nil
}

// UpdateEventVersionStatus updates the lifecycle state of a contract version.
func (s *MemoryStorage) UpdateEventVersionStatus(ctx context.Context, eventID, semver string, status core.EventStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	versions, exists := s.versions[eventID]
	if !exists {
		return ErrVersionNotFound
	}

	for i, v := range versions {
		if v.SemVer == semver {
			s.versions[eventID][i].Status = status
			return nil
		}
	}
	return ErrVersionNotFound
}

// ListProposals retrieves change proposals, optionally filtered by status.
func (s *MemoryStorage) ListProposals(ctx context.Context, status *core.EventStatus) ([]core.ChangeProposal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]core.ChangeProposal, 0, len(s.proposals))
	for _, p := range s.proposals {
		if status == nil || p.Status == *status {
			list = append(list, p)
		}
	}
	return list, nil
}

// GetProposal retrieves a single change proposal by ID.
func (s *MemoryStorage) GetProposal(ctx context.Context, id string) (*core.ChangeProposal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	prop, exists := s.proposals[id]
	if !exists {
		return nil, ErrProposalNotFound
	}
	return &prop, nil
}

// CreateProposal creates a new schema change proposal.
func (s *MemoryStorage) CreateProposal(ctx context.Context, proposal core.ChangeProposal) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.proposals[proposal.ID]; exists {
		return ErrConflict
	}
	s.proposals[proposal.ID] = proposal
	return nil
}

// UpdateProposalStatus updates the review status and notes of a proposal.
func (s *MemoryStorage) UpdateProposalStatus(ctx context.Context, id string, status core.EventStatus, reviewer, notes string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	prop, exists := s.proposals[id]
	if !exists {
		return ErrProposalNotFound
	}

	prop.Status = status
	if notes != "" {
		prop.ReviewNotes = notes
	}
	s.proposals[id] = prop
	return nil
}

// SavePAT securely stores a hashed Personal Access Token.
func (s *MemoryStorage) SavePAT(ctx context.Context, token core.PATToken) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tokens[token.ID]; exists {
		return ErrConflict
	}
	for _, t := range s.tokens {
		if t.TokenHash == token.TokenHash {
			return ErrConflict
		}
	}

	s.tokens[token.ID] = token
	return nil
}

// GetPATByHash finds a Personal Access Token record by its SHA-256 hash.
func (s *MemoryStorage) GetPATByHash(ctx context.Context, tokenHash string) (*core.PATToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, token := range s.tokens {
		if token.TokenHash == tokenHash {
			res := token
			return &res, nil
		}
	}
	return nil, ErrTokenNotFound
}

// ListPATs returns tokens belonging to an owner (or all if ownerEmail is empty).
func (s *MemoryStorage) ListPATs(ctx context.Context, ownerEmail string) ([]core.PATToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]core.PATToken, 0)
	for _, token := range s.tokens {
		if ownerEmail == "" || token.OwnerEmail == ownerEmail {
			list = append(list, token)
		}
	}
	return list, nil
}

// ListPATTokens returns tokens belonging to an owner as pointers (or all if ownerEmail is empty).
func (s *MemoryStorage) ListPATTokens(ctx context.Context, ownerEmail string) ([]*core.PATToken, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]*core.PATToken, 0)
	for _, token := range s.tokens {
		if ownerEmail == "" || token.OwnerEmail == ownerEmail {
			t := token
			list = append(list, &t)
		}
	}
	return list, nil
}

// RevokePAT deletes a token by its ID.
func (s *MemoryStorage) RevokePAT(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.tokens[id]; !exists {
		return ErrTokenNotFound
	}
	delete(s.tokens, id)
	return nil
}

// DeletePATToken deletes a token by its ID.
func (s *MemoryStorage) DeletePATToken(ctx context.Context, id string) error {
	return s.RevokePAT(ctx, id)
}

// TouchPAT updates the last used timestamp of a token.
func (s *MemoryStorage) TouchPAT(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	token, exists := s.tokens[id]
	if !exists {
		return ErrTokenNotFound
	}

	now := time.Now().UTC()
	token.LastUsedAt = &now
	s.tokens[id] = token
	return nil
}
