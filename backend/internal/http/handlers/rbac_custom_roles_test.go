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
