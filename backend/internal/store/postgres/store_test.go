package postgres

import (
	"context"
	"database/sql"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/hardware"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
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

// A migration that runs on an empty database.
//
// The statements are executed in order, so an ALTER placed before the CREATE
// it alters works on every existing platform and fails on every new one -
// which is what happened: "relation \"jobs\" does not exist", and only a fresh
// database ever showed it. This is that database.
func TestMigrationsRunOnAnEmptyDatabase(t *testing.T) {
	dsn := os.Getenv("NORYX_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("NORYX_TEST_POSTGRES_DSN is not set: no database to test against")
	}
	// A schema of its own, dropped afterwards: the point is to start from
	// nothing, which the shared test database is not.
	admin, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()

	schema := "migration_check"
	if _, err := admin.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE"); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec("CREATE SCHEMA " + schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = admin.Exec("DROP SCHEMA IF EXISTS " + schema + " CASCADE") })

	fresh, err := sql.Open("postgres", dsn+"&search_path="+schema)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := (&Store{db: fresh}).migrate(ctx); err != nil {
		t.Fatalf("the migrations must run on an empty database: %v", err)
	}
}

// A workspace written and read back.
//
// The column count and the scan destinations are two lists that must agree,
// and nothing in Go checks that they do: adding image_digest to the query and
// forgetting one Scan site produced "expected 18 destination arguments in
// Scan, not 17" - which surfaced as a *degraded backup*, because the backup is
// the only thing that reads every workspace at once.
func TestAWorkspaceSurvivesAWriteAndReadBack(t *testing.T) {
	store := testStore(t)
	item := workspace.New("jupyter", "test-project", "round trip", "harbor/x:1", "pod", "svc", "1", "4Gi", "/w/1", "token")
	item.ImageDigest = "sha256:1234"
	if err := store.CreateWorkspace(item); err != nil {
		t.Fatalf("creating the workspace: %v", err)
	}
	t.Cleanup(func() { _ = store.DeleteWorkspace(item.ID) })

	// Both readers: the one that fetches a single workspace and the one the
	// backup uses, which is the one that broke.
	single, found, err := store.GetWorkspaceByID(item.ID)
	if err != nil || !found {
		t.Fatalf("reading it back: found=%v err=%v", found, err)
	}
	if single.ImageDigest != "sha256:1234" {
		t.Errorf("the digest must survive, got %q", single.ImageDigest)
	}

	all, err := store.ListWorkspaces()
	if err != nil {
		t.Fatalf("listing workspaces: %v", err)
	}
	for _, candidate := range all {
		if candidate.ID == item.ID && candidate.ImageDigest != "sha256:1234" {
			t.Errorf("the listing lost the digest: %q", candidate.ImageDigest)
		}
	}
}

// Every column a query selects must have somewhere to land.
//
// Adding `hardware_tier` to the app queries without adding its scan destination
// made GetAppBySlug fail for every call, which took the whole application proxy
// down: a deployed app answered 500, and nothing was logged because the handler
// turned the scan error into "failed to read app". The compiler cannot catch
// this - Scan takes a variadic list of `any` - so a test counts instead.
func TestAppQueriesScanEveryColumnTheySelect(t *testing.T) {
	source, err := os.ReadFile("store.go")
	if err != nil {
		t.Fatalf("read store.go: %v", err)
	}
	queries := regexp.MustCompile("SELECT ([^`]*?) FROM apps").FindAllStringSubmatch(string(source), -1)
	if len(queries) == 0 {
		t.Fatal("no app query found: this test has stopped testing anything")
	}
	for _, query := range queries {
		columns := strings.Count(query[1], ",") + 1
		if columns < 22 {
			t.Fatalf("an app query selects %d columns; the record has 22 fields to fill:\n%s",
				columns, query[0])
		}
	}
}

// A column added to the model has to reach the schema and both queries.
//
// The component field was added to the token struct and nowhere else: it was
// written by the insert, never read back, and every component token came back
// as a nameless personal one - so the credential authenticated and then failed
// every authorisation, for a reason nothing in the code pointed at. The field
// existed, the behaviour did not.
func TestTokenQueriesCarryTheComponent(t *testing.T) {
	schema, err := os.ReadFile("store.go")
	if err != nil {
		t.Fatalf("read store.go: %v", err)
	}
	if !strings.Contains(string(schema), "api_tokens ADD COLUMN IF NOT EXISTS component") {
		t.Fatal("api_tokens has no component column: a field the schema does not know is written and never read")
	}

	source, err := os.ReadFile("api_token_store.go")
	if err != nil {
		t.Fatalf("read api_token_store.go: %v", err)
	}
	text := string(source)

	selects := regexp.MustCompile("(?s)SELECT ([^`]*?)\\s+FROM api_tokens").FindAllStringSubmatch(text, -1)
	if len(selects) == 0 {
		t.Fatal("no token query found: this test has stopped testing anything")
	}
	for _, query := range selects {
		if !strings.Contains(query[1], "component") {
			t.Fatalf("a token query does not select the component:\n%s", strings.TrimSpace(query[1]))
		}
	}
	if !strings.Contains(text, "&token.Component") {
		t.Fatal("the component is selected and never scanned: it comes back empty on every read")
	}
	if !strings.Contains(text, "token.Component)") {
		t.Fatal("the component is never written: it is empty from the moment it is stored")
	}
}
