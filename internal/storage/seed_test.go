package storage

import (
	"context"
	"testing"

	"github.com/alexagc/kalyx/internal/core"
)

func TestSeedDemoData(t *testing.T) {
	ctx := context.Background()
	store := &MemoryStorage{
		events:    make(map[string]core.EventDefinition),
		versions:  make(map[string][]core.EventVersion),
		proposals: make(map[string]core.ChangeProposal),
		tokens:    make(map[string]core.PATToken),
	}

	// 1. Initial seed
	if err := SeedDemoData(ctx, store); err != nil {
		t.Fatalf("failed to seed demo data: %v", err)
	}

	events, err := store.ListEvents(ctx)
	if err != nil {
		t.Fatalf("failed to list events: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 demo events, got %d", len(events))
	}

	expectedEvents := []string{
		"orders.order.completed",
		"payments.payment.processed",
		"users.user.registered",
	}

	for _, eventID := range expectedEvents {
		ev, err := store.GetEvent(ctx, eventID)
		if err != nil {
			t.Errorf("expected demo event %s to exist: %v", eventID, err)
			continue
		}
		if ev.Name == "" || ev.OwnerTeam == "" {
			t.Errorf("demo event %s missing required attributes", eventID)
		}

		// Verify 1.0.0 and 1.2.0 versions
		v100, err := store.GetEventVersion(ctx, eventID, "1.0.0")
		if err != nil {
			t.Errorf("expected version 1.0.0 for event %s: %v", eventID, err)
		} else if v100.JSONSchema == "" {
			t.Errorf("expected non-empty JSON schema for %s@1.0.0", eventID)
		}

		v120, err := store.GetEventVersion(ctx, eventID, "1.2.0")
		if err != nil {
			t.Errorf("expected version 1.2.0 for event %s: %v", eventID, err)
		} else if v120.JSONSchema == "" {
			t.Errorf("expected non-empty JSON schema for %s@1.2.0", eventID)
		}
	}

	// 2. Idempotency - seeding again should succeed with no errors or duplicates
	if err := SeedDemoData(ctx, store); err != nil {
		t.Fatalf("seeding again should be idempotent, got error: %v", err)
	}

	eventsAfter, _ := store.ListEvents(ctx)
	if len(eventsAfter) != 3 {
		t.Errorf("expected 3 events after re-seed, got %d", len(eventsAfter))
	}
}

func TestMaybeSeedDemoData(t *testing.T) {
	ctx := context.Background()

	t.Run("KALYX_SEED_DEMO=true forces seeding", func(t *testing.T) {
		t.Setenv("KALYX_SEED_DEMO", "true")
		store := &MemoryStorage{
			events:    make(map[string]core.EventDefinition),
			versions:  make(map[string][]core.EventVersion),
			proposals: make(map[string]core.ChangeProposal),
			tokens:    make(map[string]core.PATToken),
		}

		if err := MaybeSeedDemoData(ctx, store); err != nil {
			t.Fatalf("failed MaybeSeedDemoData: %v", err)
		}
		events, _ := store.ListEvents(ctx)
		if len(events) != 3 {
			t.Errorf("expected 3 events, got %d", len(events))
		}
	})

	t.Run("KALYX_DEV_MODE=true seeds when empty", func(t *testing.T) {
		t.Setenv("KALYX_DEV_MODE", "true")
		t.Setenv("KALYX_SEED_DEMO", "false")
		store := &MemoryStorage{
			events:    make(map[string]core.EventDefinition),
			versions:  make(map[string][]core.EventVersion),
			proposals: make(map[string]core.ChangeProposal),
			tokens:    make(map[string]core.PATToken),
		}

		if err := MaybeSeedDemoData(ctx, store); err != nil {
			t.Fatalf("failed MaybeSeedDemoData: %v", err)
		}
		events, _ := store.ListEvents(ctx)
		if len(events) != 3 {
			t.Errorf("expected 3 events, got %d", len(events))
		}
	})

	t.Run("Disabled env does not seed", func(t *testing.T) {
		t.Setenv("KALYX_DEV_MODE", "false")
		t.Setenv("KALYX_SEED_DEMO", "false")
		store := &MemoryStorage{
			events:    make(map[string]core.EventDefinition),
			versions:  make(map[string][]core.EventVersion),
			proposals: make(map[string]core.ChangeProposal),
			tokens:    make(map[string]core.PATToken),
		}

		if err := MaybeSeedDemoData(ctx, store); err != nil {
			t.Fatalf("failed MaybeSeedDemoData: %v", err)
		}
		events, _ := store.ListEvents(ctx)
		if len(events) != 0 {
			t.Errorf("expected 0 events, got %d", len(events))
		}
	})
}
