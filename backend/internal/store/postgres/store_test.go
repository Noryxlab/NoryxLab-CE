package postgres

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/hardware"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
)

// The store that actually runs in production had no tests at all, while the
// packages around it were well covered. Everything here needs a real Postgres:
// the behaviour worth checking is what the *database* enforces - a partial
// unique index, an ON CONFLICT clause, a column added by a migration - and
// none of that exists in a fake.
//
// Set NORYX_TEST_POSTGRES_DSN to run them. Without it they skip loudly rather
// than passing quietly, because a test suite that reports success while
// checking nothing is worse than one that is absent.
func testStore(t *testing.T) *Store {
	t.Helper()

	dsn := os.Getenv("NORYX_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("NORYX_TEST_POSTGRES_DSN is not set: no database to test against")
	}
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("opening the test database: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		t.Fatalf("reaching the test database: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	store := &Store{db: db}
	if err := store.migrate(ctx); err != nil {
		t.Fatalf("migrating the test database: %v", err)
	}
	return store
}

// Migrations are re-run on every start, so running them twice must be a no-op
// rather than an error. A platform that only survives its first boot is not a
// platform.
func TestMigrationsAreIdempotent(t *testing.T) {
	store := testStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := store.migrate(ctx); err != nil {
		t.Fatalf("running the migrations a second time: %v", err)
	}
}

func TestAProjectSurvivesAWriteAndReadBack(t *testing.T) {
	store := testStore(t)
	item := project.NewOwned("test-owner", "Store test project", "written by a test")
	item.WorkspaceStorageSize = "50Gi"
	if err := store.Create(item); err != nil {
		t.Fatalf("creating the project: %v", err)
	}
	t.Cleanup(func() { _ = store.DeleteProject(item.ID) })

	found := readProject(t, store, item.ID)
	if found.Name != item.Name || found.OwnerID != "test-owner" {
		t.Fatalf("the project came back different: %+v", found)
	}
	// The column added on 2026-09-05. A migration that adds a column the store
	// never reads back is indistinguishable from one that did nothing.
	if found.WorkspaceStorageSize != "50Gi" {
		t.Fatalf("workspace storage size = %q, want 50Gi", found.WorkspaceStorageSize)
	}

	if err := store.UpdateProjectWorkspaceStorageSize(item.ID, ""); err != nil {
		t.Fatalf("clearing the storage size: %v", err)
	}
	if cleared := readProject(t, store, item.ID); cleared.WorkspaceStorageSize != "" {
		t.Fatalf("cleared storage size = %q, want empty (follow the platform default)", cleared.WorkspaceStorageSize)
	}
}

// One default tier, enforced by the database rather than by the handler above
// it. Two defaults would make the preselected machine size depend on row order.
func TestOnlyOneHardwareTierCanBeTheDefault(t *testing.T) {
	store := testStore(t)
	first := hardware.Tier{ID: "test-a", Name: "Test A", CPURequest: "100m", CPULimit: "1", MemoryRequest: "64Mi", MemoryLimit: "1Gi", EphemeralStorageRequest: "64Mi", EphemeralStorageLimit: "1Gi", Default: true, Position: 90}
	second := first
	second.ID = "test-b"
	second.Name = "Test B"
	second.Position = 91

	if err := store.UpsertHardwareTier(first); err != nil {
		t.Fatalf("saving the first tier: %v", err)
	}
	t.Cleanup(func() { _ = store.DeleteHardwareTier("test-a") })
	if err := store.UpsertHardwareTier(second); err != nil {
		t.Fatalf("saving the second tier: %v", err)
	}
	t.Cleanup(func() { _ = store.DeleteHardwareTier("test-b") })

	tiers, err := store.ListHardwareTiers()
	if err != nil {
		t.Fatalf("listing tiers: %v", err)
	}
	defaults := []string{}
	for _, tier := range tiers {
		if tier.Default {
			defaults = append(defaults, tier.ID)
		}
	}
	if len(defaults) != 1 || defaults[0] != "test-b" {
		t.Fatalf("expected test-b alone as default, got %v", defaults)
	}
}

// Saving the same tier twice updates it. Without the ON CONFLICT clause the
// second save fails on the primary key, and an administrator editing a tier
// gets an error that says nothing.
func TestSavingATierTwiceUpdatesIt(t *testing.T) {
	store := testStore(t)
	tier := hardware.Tier{ID: "test-upsert", Name: "Before", CPURequest: "100m", CPULimit: "1", MemoryRequest: "64Mi", MemoryLimit: "1Gi", EphemeralStorageRequest: "64Mi", EphemeralStorageLimit: "1Gi", Position: 92}
	if err := store.UpsertHardwareTier(tier); err != nil {
		t.Fatalf("first save: %v", err)
	}
	t.Cleanup(func() { _ = store.DeleteHardwareTier("test-upsert") })

	tier.Name = "After"
	tier.MemoryLimit = "8Gi"
	if err := store.UpsertHardwareTier(tier); err != nil {
		t.Fatalf("second save: %v", err)
	}

	tiers, err := store.ListHardwareTiers()
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range tiers {
		if candidate.ID == "test-upsert" {
			if candidate.Name != "After" || candidate.MemoryLimit != "8Gi" {
				t.Fatalf("the tier was not updated: %+v", candidate)
			}
			return
		}
	}
	t.Fatal("the tier disappeared after being saved twice")
}

func readProject(t *testing.T, store *Store, id string) project.Project {
	t.Helper()
	projects, err := store.List()
	if err != nil {
		t.Fatalf("listing projects: %v", err)
	}
	for _, candidate := range projects {
		if candidate.ID == id {
			return candidate
		}
	}
	t.Fatalf("project %s was written and cannot be read back", id)
	return project.Project{}
}
