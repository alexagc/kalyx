package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alexagc/kalyx/internal/core"
)

func TestMemoryStorage_EventOperations(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStorage()

	// 1. Initial seeded event check
	events, err := store.ListEvents(ctx)
	if err != nil {
		t.Fatalf("unexpected error listing events: %v", err)
	}
	if len(events) == 0 {
		t.Fatalf("expected seeded event, got 0")
	}

	// 2. Create new event
	newEvent := core.EventDefinition{
		ID:          "ev_payments_processed",
		Domain:      "Payments",
		Name:        "PaymentProcessed",
		Type:        "com.company.payments.processed.v1",
		Description: "Emitted when a customer payment succeeds.",
		OwnerTeam:   "Billing Core",
		CreatedAt:   time.Now().UTC(),
		UpdatedAt:   time.Now().UTC(),
	}

	err = store.CreateEvent(ctx, newEvent)
	if err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	// 3. Duplicate event should return ErrConflict
	err = store.CreateEvent(ctx, newEvent)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on duplicate event, got %v", err)
	}

	// 4. Get created event
	fetched, err := store.GetEvent(ctx, newEvent.ID)
	if err != nil {
		t.Fatalf("failed to get created event: %v", err)
	}
	if fetched.Name != newEvent.Name {
		t.Errorf("expected name %s, got %s", newEvent.Name, fetched.Name)
	}

	// 5. Get non-existent event should return ErrEventNotFound
	_, err = store.GetEvent(ctx, "non_existent_event_id")
	if !errors.Is(err, ErrEventNotFound) {
		t.Fatalf("expected ErrEventNotFound, got %v", err)
	}
}

func TestMemoryStorage_VersionOperations(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStorage()

	eventID := "ev_orders_completed"

	// 1. Get initial seeded version
	v1, err := store.GetEventVersion(ctx, eventID, "1.0.0")
	if err != nil {
		t.Fatalf("failed to get seeded version 1.0.0: %v", err)
	}
	if v1.Status != core.StatusApproved {
		t.Errorf("expected StatusApproved, got %s", v1.Status)
	}

	// 2. Create new version 1.1.0
	v11 := core.EventVersion{
		ID:         "ver_1_1_0",
		EventID:    eventID,
		SemVer:     "1.1.0",
		Status:     core.StatusDraft,
		JSONSchema: `{"type": "object"}`,
		CreatedAt:  time.Now().UTC(),
		CreatedBy:  "bob@company.com",
	}

	err = store.CreateEventVersion(ctx, v11)
	if err != nil {
		t.Fatalf("failed to create version 1.1.0: %v", err)
	}

	// 3. Duplicate version should return ErrConflict
	err = store.CreateEventVersion(ctx, v11)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on duplicate version, got %v", err)
	}

	// 4. Update status
	err = store.UpdateEventVersionStatus(ctx, eventID, "1.1.0", core.StatusApproved)
	if err != nil {
		t.Fatalf("failed to update version status: %v", err)
	}

	updated, err := store.GetEventVersion(ctx, eventID, "1.1.0")
	if err != nil {
		t.Fatalf("failed to get updated version: %v", err)
	}
	if updated.Status != core.StatusApproved {
		t.Errorf("expected StatusApproved, got %s", updated.Status)
	}

	// 5. List versions
	versions, err := store.ListVersions(ctx, eventID)
	if err != nil {
		t.Fatalf("failed to list versions: %v", err)
	}
	if len(versions) < 2 {
		t.Errorf("expected at least 2 versions, got %d", len(versions))
	}
}

func TestMemoryStorage_ProposalOperations(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStorage()

	prop := core.ChangeProposal{
		ID:             "prop_001",
		EventID:        "ev_orders_completed",
		TargetSemVer:   "1.2.0",
		ProposedSchema: `{"type": "object", "properties": {"note": {"type": "string"}}}`,
		ProposedBy:     "alice@company.com",
		Status:         core.StatusInReview,
		IsBreaking:     false,
		CreatedAt:      time.Now().UTC(),
	}

	// 1. Create proposal
	err := store.CreateProposal(ctx, prop)
	if err != nil {
		t.Fatalf("failed to create proposal: %v", err)
	}

	// 2. Conflict on duplicate
	err = store.CreateProposal(ctx, prop)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on duplicate proposal, got %v", err)
	}

	// 3. Get proposal
	fetched, err := store.GetProposal(ctx, prop.ID)
	if err != nil {
		t.Fatalf("failed to get proposal: %v", err)
	}
	if fetched.TargetSemVer != prop.TargetSemVer {
		t.Errorf("expected %s, got %s", prop.TargetSemVer, fetched.TargetSemVer)
	}

	// 4. List by status
	reviewStatus := core.StatusInReview
	props, err := store.ListProposals(ctx, &reviewStatus)
	if err != nil {
		t.Fatalf("failed to list proposals: %v", err)
	}
	if len(props) != 1 {
		t.Errorf("expected 1 proposal in review, got %d", len(props))
	}

	// 5. Update proposal status
	err = store.UpdateProposalStatus(ctx, prop.ID, core.StatusApproved, "lead@company.com", "LGTM")
	if err != nil {
		t.Fatalf("failed to update proposal: %v", err)
	}

	approvedProp, err := store.GetProposal(ctx, prop.ID)
	if err != nil {
		t.Fatalf("failed to get updated proposal: %v", err)
	}
	if approvedProp.Status != core.StatusApproved {
		t.Errorf("expected StatusApproved, got %s", approvedProp.Status)
	}
	if approvedProp.ReviewNotes != "LGTM" {
		t.Errorf("expected LGTM, got %s", approvedProp.ReviewNotes)
	}
}

func TestMemoryStorage_PATOperations(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStorage()

	token := core.PATToken{
		ID:         "pat_001",
		Name:       "CI GitHub Actions",
		TokenHash:  "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		OwnerEmail: "ci@company.com",
		Team:       "Checkout Core",
		CreatedAt:  time.Now().UTC(),
	}

	// 1. Save PAT
	err := store.SavePAT(ctx, token)
	if err != nil {
		t.Fatalf("failed to save PAT: %v", err)
	}

	// 2. Conflict on duplicate ID or Hash
	err = store.SavePAT(ctx, token)
	if !errors.Is(err, ErrConflict) {
		t.Fatalf("expected ErrConflict on duplicate token, got %v", err)
	}

	// 3. Get PAT by hash
	fetched, err := store.GetPATByHash(ctx, token.TokenHash)
	if err != nil {
		t.Fatalf("failed to get PAT by hash: %v", err)
	}
	if fetched.Name != token.Name {
		t.Errorf("expected name %s, got %s", token.Name, fetched.Name)
	}

	// 4. Touch PAT
	err = store.TouchPAT(ctx, token.ID)
	if err != nil {
		t.Fatalf("failed to touch PAT: %v", err)
	}
	touched, _ := store.GetPATByHash(ctx, token.TokenHash)
	if touched.LastUsedAt == nil {
		t.Errorf("expected LastUsedAt to be set after TouchPAT")
	}

	// 5. List PATs and ListPATTokens
	pats, err := store.ListPATs(ctx, "ci@company.com")
	if err != nil {
		t.Fatalf("failed to list PATs: %v", err)
	}
	if len(pats) != 1 {
		t.Errorf("expected 1 PAT, got %d", len(pats))
	}

	patTokens, err := store.ListPATTokens(ctx, "ci@company.com")
	if err != nil {
		t.Fatalf("failed to list PAT tokens: %v", err)
	}
	if len(patTokens) != 1 || patTokens[0].ID != token.ID {
		t.Errorf("expected 1 PAT token with id %s, got %v", token.ID, patTokens)
	}

	// 6. Delete PAT Token (via DeletePATToken)
	err = store.DeletePATToken(ctx, token.ID)
	if err != nil {
		t.Fatalf("failed to delete PAT token: %v", err)
	}
	_, err = store.GetPATByHash(ctx, token.TokenHash)
	if !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound after delete, got %v", err)
	}

	// Deleting again should return ErrTokenNotFound
	err = store.DeletePATToken(ctx, token.ID)
	if !errors.Is(err, ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound on deleting non-existent token, got %v", err)
	}
}

func TestMemoryStorage_ConcurrentStress(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStorage()

	var wg sync.WaitGroup
	numWorkers := 30
	iterations := 100

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		workerID := i
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				// Alternating reads and writes
				evID := fmt.Sprintf("ev_worker_%d_%d", workerID, j)
				ev := core.EventDefinition{
					ID:        evID,
					Domain:    "Stress",
					Name:      "Event",
					Type:      "com.test.event",
					CreatedAt: time.Now().UTC(),
				}
				_ = store.CreateEvent(ctx, ev)
				_, _ = store.GetEvent(ctx, evID)
				_, _ = store.ListEvents(ctx)
			}
		}()
	}

	wg.Wait()
}
