package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/alexagc/kalyx/internal/core"
	"github.com/alexagc/kalyx/internal/storage"
	"github.com/alexagc/kalyx/internal/storage/sqlite"
)

func createTempSQLite(t *testing.T) (*sqlite.Storage, string) {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "kalyx-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	dbPath := filepath.Join(tempDir, "test.db")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	store, err := sqlite.New(ctx, dbPath)
	if err != nil {
		t.Fatalf("failed to initialize sqlite storage: %v", err)
	}

	return store, tempDir
}

func TestSQLite_AutoMigrationAndLifecycle(t *testing.T) {
	store, tempDir := createTempSQLite(t)
	defer os.RemoveAll(tempDir)
	defer store.Close()

	ctx := context.Background()

	// 1. Verify Ping
	if err := store.Ping(ctx); err != nil {
		t.Fatalf("ping failed: %v", err)
	}

	// 2. Verify empty list on fresh database
	events, err := store.ListEvents(ctx)
	if err != nil {
		t.Fatalf("failed to list events on fresh db: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events on fresh db, got %d", len(events))
	}
}

func TestSQLite_EventAndVersionCRUD(t *testing.T) {
	store, tempDir := createTempSQLite(t)
	defer os.RemoveAll(tempDir)
	defer store.Close()

	ctx := context.Background()

	// 1. Create event
	ev := core.EventDefinition{
		ID:          "ev_orders_placed",
		Domain:      "Orders",
		Name:        "OrderPlaced",
		Type:        "com.company.orders.placed.v1",
		Description: "Emitted when a customer places an order.",
		OwnerTeam:   "Checkout Core",
	}

	if err := store.CreateEvent(ctx, ev); err != nil {
		t.Fatalf("failed to create event: %v", err)
	}

	// 2. Duplicate event check
	if err := store.CreateEvent(ctx, ev); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}

	// 3. Get event
	fetched, err := store.GetEvent(ctx, ev.ID)
	if err != nil {
		t.Fatalf("failed to get event: %v", err)
	}
	if fetched.Name != ev.Name {
		t.Errorf("expected %s, got %s", ev.Name, fetched.Name)
	}

	// 4. Create version
	v1 := core.EventVersion{
		ID:         "ver_orders_placed_1_0_0",
		EventID:    ev.ID,
		SemVer:     "1.0.0",
		Status:     core.StatusDraft,
		JSONSchema: `{"type": "object", "properties": {"order_id": {"type": "string"}}}`,
		CreatedBy:  "alice@company.com",
		DataLakeMetadata: core.DataLakeMetadata{
			TargetTable: "lake_orders_placed",
			PartitionBy: []string{"country"},
			PIIFields:   []string{"customer_email"},
		},
	}

	if err := store.CreateEventVersion(ctx, v1); err != nil {
		t.Fatalf("failed to create event version: %v", err)
	}

	// 5. Duplicate version check
	if err := store.CreateEventVersion(ctx, v1); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("expected ErrConflict on duplicate version, got %v", err)
	}

	// 6. Get version
	fetchedV, err := store.GetEventVersion(ctx, ev.ID, "1.0.0")
	if err != nil {
		t.Fatalf("failed to get event version: %v", err)
	}
	if fetchedV.DataLakeMetadata.TargetTable != "lake_orders_placed" {
		t.Errorf("expected TargetTable lake_orders_placed, got %s", fetchedV.DataLakeMetadata.TargetTable)
	}
	if len(fetchedV.DataLakeMetadata.PartitionBy) != 1 || fetchedV.DataLakeMetadata.PartitionBy[0] != "country" {
		t.Errorf("expected partition by [country], got %v", fetchedV.DataLakeMetadata.PartitionBy)
	}

	// 7. Update version status
	if err := store.UpdateEventVersionStatus(ctx, ev.ID, "1.0.0", core.StatusApproved); err != nil {
		t.Fatalf("failed to update status: %v", err)
	}
	approvedV, _ := store.GetEventVersion(ctx, ev.ID, "1.0.0")
	if approvedV.Status != core.StatusApproved {
		t.Errorf("expected StatusApproved, got %s", approvedV.Status)
	}
}

func TestSQLite_ProposalLifecycle(t *testing.T) {
	store, tempDir := createTempSQLite(t)
	defer os.RemoveAll(tempDir)
	defer store.Close()

	ctx := context.Background()

	// Seed event first for foreign key
	ev := core.EventDefinition{
		ID:        "ev_inventory_adjusted",
		Domain:    "Inventory",
		Name:      "InventoryAdjusted",
		Type:      "com.company.inventory.adjusted.v1",
		OwnerTeam: "Logistics Core",
	}
	if err := store.CreateEvent(ctx, ev); err != nil {
		t.Fatalf("failed to seed event: %v", err)
	}

	prop := core.ChangeProposal{
		ID:             "prop_inv_001",
		EventID:        ev.ID,
		TargetSemVer:   "1.1.0",
		ProposedSchema: `{"type": "object"}`,
		ProposedBy:     "bob@company.com",
		Status:         core.StatusInReview,
		IsBreaking:     false,
	}

	if err := store.CreateProposal(ctx, prop); err != nil {
		t.Fatalf("failed to create proposal: %v", err)
	}

	// Duplicate check
	if err := store.CreateProposal(ctx, prop); !errors.Is(err, storage.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}

	// Update status
	if err := store.UpdateProposalStatus(ctx, prop.ID, core.StatusApproved, "lead@company.com", "Approved for release"); err != nil {
		t.Fatalf("failed to update proposal: %v", err)
	}

	fetched, err := store.GetProposal(ctx, prop.ID)
	if err != nil {
		t.Fatalf("failed to get proposal: %v", err)
	}
	if fetched.Status != core.StatusApproved {
		t.Errorf("expected StatusApproved, got %s", fetched.Status)
	}
	if fetched.ReviewNotes != "Approved for release" {
		t.Errorf("expected review notes, got %s", fetched.ReviewNotes)
	}
}

func TestSQLite_PATLifecycle(t *testing.T) {
	store, tempDir := createTempSQLite(t)
	defer os.RemoveAll(tempDir)
	defer store.Close()

	ctx := context.Background()

	token := core.PATToken{
		ID:         "pat_github_ci",
		Name:       "GitHub Actions CI Token",
		TokenHash:  "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08",
		OwnerEmail: "ci@company.com",
		Team:       "Core Devs",
	}

	if err := store.SavePAT(ctx, token); err != nil {
		t.Fatalf("failed to save PAT: %v", err)
	}

	// Get by hash
	fetched, err := store.GetPATByHash(ctx, token.TokenHash)
	if err != nil {
		t.Fatalf("failed to get PAT by hash: %v", err)
	}
	if fetched.Name != token.Name {
		t.Errorf("expected name %s, got %s", token.Name, fetched.Name)
	}

	// Touch
	if err := store.TouchPAT(ctx, token.ID); err != nil {
		t.Fatalf("failed to touch PAT: %v", err)
	}
	touched, _ := store.GetPATByHash(ctx, token.TokenHash)
	if touched.LastUsedAt == nil {
		t.Errorf("expected LastUsedAt to be non-nil after touch")
	}

	// Revoke
	if err := store.RevokePAT(ctx, token.ID); err != nil {
		t.Fatalf("failed to revoke PAT: %v", err)
	}
	if _, err := store.GetPATByHash(ctx, token.TokenHash); !errors.Is(err, storage.ErrTokenNotFound) {
		t.Fatalf("expected ErrTokenNotFound after revoke, got %v", err)
	}
}

func TestSQLite_ConcurrentStressWAL(t *testing.T) {
	store, tempDir := createTempSQLite(t)
	defer os.RemoveAll(tempDir)
	defer store.Close()

	ctx := context.Background()

	var wg sync.WaitGroup
	workers := 10
	iterations := 50

	for i := 0; i < workers; i++ {
		wg.Add(1)
		wID := i
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				evID := fmt.Sprintf("ev_stress_%d_%d", wID, j)
				ev := core.EventDefinition{
					ID:        evID,
					Domain:    "Stress",
					Name:      fmt.Sprintf("StressEvent%d", j),
					Type:      fmt.Sprintf("com.test.stress.%d.%d", wID, j),
					OwnerTeam: "QA",
				}
				_ = store.CreateEvent(ctx, ev)
				_, _ = store.GetEvent(ctx, evID)
				_, _ = store.ListEvents(ctx)
			}
		}()
	}

	wg.Wait()
}
