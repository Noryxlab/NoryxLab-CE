package handlers

import (
	"os"
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
)

// Nobody creates a principal more powerful than themselves.
//
// The rule is ADR-039's seventh decision, and today the platform satisfies it
// by construction rather than by checking: every place that creates a
// principal or hands out a role already demands the top authority over the
// thing being handed out. This file pins that, because the rule is invisible
// while it holds and expensive the day it stops.
//
// What each of them requires, read from the handlers:
//
//	accounts, organizations          the global administrator role
//	teams and their members          the global administrator, or somebody
//	                                 that organization named (ADR-048)
//	component tokens                 the global administrator role
//	project roles                    RoleAdmin on that project
//	dataset and ontology grants      ownership of that asset, or global admin
//	personal tokens                  nothing: a token is its own owner, and
//	                                 the scope can only narrow it
//
// Delegated administration now exists, so the rule stopped being free on
// 2026-10-26. What keeps it true is that a delegate creates nothing more
// powerful than themselves: a team confers nothing by itself, its members
// receive only what that team was already granted, and the perimeter is read
// from the team's own record rather than from the request - so naming an
// organization one administers cannot reach somebody else's team. Creating
// accounts and naming further administrators stay with the platform
// administrator until ADR-048's open questions are answered.

// Only a project administrator hands out project roles, so granting the
// administrator role is never a promotion of the grantor.
func TestOnlyAProjectAdministratorGrantsRoles(t *testing.T) {
	for _, role := range []access.Role{access.RoleViewer, access.RoleEditor, ""} {
		if actionManageMembers.permits(role) {
			t.Fatalf("role %q must not manage members", role)
		}
	}
	if !actionManageMembers.permits(access.RoleAdmin) {
		t.Fatal("a project administrator must manage members")
	}
}

// And the administrative modules answer to one authority only. If this test
// starts failing because a module became reachable by somebody narrower, that
// is exactly the moment rule 7 needs an explicit check at every creation.
func TestAdministrativeCreationIsGlobalAdminOnly(t *testing.T) {
	sources := map[string]string{
		"organizations":    readHandler(t, "organizations.go"),
		"teams":            readHandler(t, "teams.go"),
		"user accounts":    readHandler(t, "admin_user_accounts.go"),
		"component tokens": readHandler(t, "component_tokens.go"),
	}
	// Une portee compte comme une autorite : elle est plus etroite que le
	// role global, jamais plus large, et elle se lit sur l'objet et non sur
	// la requete.
	gardes := []string{"requireAdminModule", "isGlobalAdmin", "requireOrganizationScope", "requireTeamScope"}
	for what, body := range sources {
		garde := false
		for _, nom := range gardes {
			if strings.Contains(body, nom) {
				garde = true
				break
			}
		}
		if !garde {
			t.Fatalf("%s no longer gates creation on any authority; rule 7 now needs an explicit check", what)
		}
	}
}

func readHandler(t *testing.T, name string) string {
	t.Helper()
	body, err := os.ReadFile("./" + name)
	if err != nil {
		t.Fatalf("cannot read %s: %v", name, err)
	}
	return string(body)
}
