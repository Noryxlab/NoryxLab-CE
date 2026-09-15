package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
)

// A custom role is a role the platform did not write. These tests fix what it
// is allowed to become, because the failure mode is not an error message: it
// is a person holding a role that silently grants more, or nothing at all.

func TestACustomRoleWithoutABaseGrantsTheLeast(t *testing.T) {
	// A document written before roles could declare a base must not widen
	// anybody: the weakest reading is the only safe one.
	rows, err := validateRBACPolicyRows([]rbacPolicyRow{
		{Role: "Data steward", Key: "data-steward", Dataset: "RW", Project: "RW"},
	})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if rows[0].BasedOn != string(access.RoleViewer) {
		t.Fatalf("a role with no declared base should answer as viewer, got %q", rows[0].BasedOn)
	}
}

func TestARoleCannotBeBasedOnSomethingThePlatformDoesNotKnow(t *testing.T) {
	_, err := validateRBACPolicyRows([]rbacPolicyRow{
		{Role: "Data steward", Key: "data-steward", BasedOn: "superuser"},
	})
	if err == nil {
		t.Fatal("a base the platform cannot resolve must be refused at save time, not discovered at a door")
	}
}

func TestShippedRowsKeepTheirOwnBase(t *testing.T) {
	// A shipped row describes a built-in. Letting a stored document tell the
	// platform what "project admin" is based on would be the stale-document
	// problem again, one field lower.
	rows, err := validateRBACPolicyRows([]rbacPolicyRow{
		{Role: "Project admin", Key: "project-admin", Locked: true, BasedOn: "viewer",
			Project: "Admin", Dataset: "RW attaché", Ontology: "RW attaché", Datasource: "RW attaché",
			Environment: "R", Workload: "RW", Governance: "-"},
	})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if rows[0].BasedOn != string(access.RoleAdmin) {
		t.Fatalf("a shipped row must keep the platform's base, got %q", rows[0].BasedOn)
	}
}

func TestEveryShippedRoleHasABase(t *testing.T) {
	// The mapping is written by hand, so a role added to the shipped rows and
	// forgotten here would resolve to nothing and grant nothing.
	for _, row := range defaultRBACPolicyRows() {
		if _, ok := rbacShippedRowBases[row.Key]; !ok {
			t.Fatalf("shipped role %q has no base: it would grant nothing anywhere", row.Key)
		}
	}
}

func TestBuiltinRolesStillDecideThemselves(t *testing.T) {
	// The whole change is safe only if a platform nobody customised behaves
	// exactly as before: a built-in answers as itself without reading any
	// document at all.
	h := Handlers{}
	for _, role := range access.Builtins() {
		if got := h.baseRole(role); got != role {
			t.Fatalf("built-in %q resolved to %q", role, got)
		}
	}
	if got := h.baseRole("data-steward"); got != "" {
		t.Fatalf("a role no document describes must grant nothing, got %q", got)
	}
}

func TestACustomRoleIsRefusedWhereNothingCanEnforceIt(t *testing.T) {
	// Community decides from the built-in roles. Offering a custom role there
	// would be the product asserting what it cannot demonstrate.
	h := Handlers{}
	if refusal := h.assignableRoleError("editor"); refusal != "" {
		t.Fatalf("a built-in role must always be assignable, got %q", refusal)
	}
	if refusal := h.assignableRoleError("data-steward"); refusal == "" {
		t.Fatal("a role no document describes must be refused")
	}
}

func TestACustomRoleCannotCarryGovernance(t *testing.T) {
	// Governance is the column an installation would most like to fill in and
	// the only one the platform cannot honour: a role is held inside a
	// project, and the administration screens are not inside any project. The
	// two honest options are to decide or to stop offering it, and it cannot
	// decide - so it is refused at save time rather than stored and read by
	// nobody, which is the defect the whole matrix used to have.
	_, err := validateRBACPolicyRows([]rbacPolicyRow{
		{Role: "Auditeur", Key: "auditeur", BasedOn: "viewer", Governance: "Admin"},
	})
	if err == nil {
		t.Fatal("a project role was allowed to grant platform governance")
	}
	// The platform's own rows keep the column: that is what it is for.
	if _, err := validateRBACPolicyRows(defaultRBACPolicyRows()); err != nil {
		t.Fatalf("the shipped rows must validate: %v", err)
	}
}

func TestTheShippedRowsDescribeWhatAContributorCanActuallyDo(t *testing.T) {
	// The data columns said R for a contributor while attaching a cohort has
	// always been a contributor's right - the rows were describing a stricter
	// platform than the one that ships. Harmless while nothing read them; a
	// withdrawal of access the day those columns started deciding.
	rows := map[string]rbacPolicyRow{}
	for _, row := range defaultRBACPolicyRows() {
		rows[row.Key] = row
	}
	for _, key := range []string{"editor", "project-admin"} {
		row := rows[key]
		for column, value := range map[string]string{
			"dataset":     row.Dataset,
			"ontology":    row.Ontology,
			"datasource":  row.Datasource,
			"environment": row.Environment,
		} {
			if value != "RW" && value != "RW attaché" {
				t.Errorf("%s says %q on %s, but may attach one today", key, value, column)
			}
		}
	}
	// A viewer may not, and the rows have to keep saying so.
	viewer := rows["viewer"]
	if viewer.Dataset != "R attaché" || viewer.Environment != "R" {
		t.Errorf("viewer was widened: dataset %q, environment %q", viewer.Dataset, viewer.Environment)
	}
}
