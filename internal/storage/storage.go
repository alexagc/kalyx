package storage

import (
	"errors"
	"sync"
	"time"

	"github.com/alexagc/kalyx/internal/core"
)

var (
	ErrEventNotFound   = errors.New("event not found")
	ErrVersionNotFound = errors.New("event version not found")
)

// Storage defines the persistence interface for Kalyx.
type Storage interface {
	ListEvents() ([]core.EventDefinition, error)
	GetEvent(id string) (*core.EventDefinition, error)
	CreateEvent(event core.EventDefinition) error
	GetEventVersion(eventID, semver string) (*core.EventVersion, error)
	CreateEventVersion(version core.EventVersion) error
	ListVersions(eventID string) ([]core.EventVersion, error)
}

// MemoryStorage is a zero-dependency in-memory store for quickstart and testing.
type MemoryStorage struct {
	mu       sync.RWMutex
	events   map[string]core.EventDefinition
	versions map[string][]core.EventVersion
}

// NewMemoryStorage initializes an in-memory storage instance with initial sample data.
func NewMemoryStorage() *MemoryStorage {
	s := &MemoryStorage{
		events:   make(map[string]core.EventDefinition),
		versions: make(map[string][]core.EventVersion),
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

func (s *MemoryStorage) ListEvents() ([]core.EventDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	list := make([]core.EventDefinition, 0, len(s.events))
	for _, ev := range s.events {
		list = append(list, ev)
	}
	return list, nil
}

func (s *MemoryStorage) GetEvent(id string) (*core.EventDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ev, exists := s.events[id]
	if !exists {
		return nil, ErrEventNotFound
	}
	return &ev, nil
}

func (s *MemoryStorage) CreateEvent(event core.EventDefinition) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events[event.ID] = event
	return nil
}

func (s *MemoryStorage) GetEventVersion(eventID, semver string) (*core.EventVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, exists := s.versions[eventID]
	if !exists {
		return nil, ErrVersionNotFound
	}
	for _, v := range versions {
		if v.SemVer == semver {
			return &v, nil
		}
	}
	return nil, ErrVersionNotFound
}

func (s *MemoryStorage) CreateEventVersion(version core.EventVersion) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.versions[version.EventID] = append(s.versions[version.EventID], version)
	return nil
}

func (s *MemoryStorage) ListVersions(eventID string) ([]core.EventVersion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	versions, exists := s.versions[eventID]
	if !exists {
		return []core.EventVersion{}, nil
	}
	return versions, nil
}
