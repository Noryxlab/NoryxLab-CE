package handlers

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/edition"
)

// Roles an installation defined for itself.
//
// The matrix could describe them for months and nobody could hold one:
// a membership carried "viewer", "editor" or "admin" and nothing else, so a
// customer could write a role down and then find no way to give it to a
// person. A role that cannot be held is a role the screen asserts and the
// platform cannot demonstrate, which is the thing ADR-034 forbids.
//
// What makes a custom role safe to hold is that it always answers as a
// built-in. Every rule written in Go reads the base - the matrix refines the
// answer where it has something to say, and Community, where the matrix
// decides nothing at all, still gives a coherent verdict rather than refusing
// a member it cannot resolve. The base is also the ceiling of what the
// platform will grant on its own: a role based on viewer can be described as
// doing anything, and until the matrix is enforcing, it views.

// rbacRoleBaseCache keeps the resolved bases for a few seconds.
//
// A permission check happens on nearly every request and must not cost a round
// trip to the database. The document changes when an administrator saves it,
// so a stale answer lasting seconds is the price, and it is a much smaller
// problem than one lasting until a restart.
type rbacRoleBaseCache struct {
	mu       sync.RWMutex
	bases    map[string]access.Role
	loadedAt time.Time
}

const rbacRoleBaseCacheFor = 10 * time.Second

func (h Handlers) rbacRoleBases() map[string]access.Role {
	cache := h.roleBaseCache
	if cache == nil {
		return nil
	}
	cache.mu.RLock()
	if time.Since(cache.loadedAt) < rbacRoleBaseCacheFor {
		bases := cache.bases
		cache.mu.RUnlock()
		return bases
	}
	cache.mu.RUnlock()

	cache.mu.Lock()
	defer cache.mu.Unlock()
	if time.Since(cache.loadedAt) < rbacRoleBaseCacheFor {
		return cache.bases
	}
	cache.loadedAt = time.Now()
	rows, err := h.loadRBACPolicyRows()
	if err != nil {
		// A document that cannot be read resolves nothing, and the built-in
		// roles keep working: refusing everything would lock an installation
		// out of its own platform over a storage hiccup.
		cache.bases = nil
		return nil
	}
	bases := make(map[string]access.Role, len(rows))
	for _, row := range rows {
		key := rbacRoleKey(firstNonEmpty(row.Key, row.Role))
		if key == "" {
			continue
		}
		base, err := rbacRowBase(row)
		if err != nil {
			continue
		}
		bases[key] = base
	}
	cache.bases = bases
	return bases
}

// baseRole is the built-in a held role answers as.
//
// A built-in answers as itself. A custom role answers as the base its row
// declares. A role no document describes answers as nothing - it grants no
// access rather than being guessed at, because the alternative is inventing a
// permission for a value nobody can explain.
func (h Handlers) baseRole(role access.Role) access.Role {
	if role.IsBuiltin() {
		return role
	}
	key := rbacRoleKey(strings.TrimSpace(string(role)))
	if key == "" {
		return ""
	}
	return h.rbacRoleBases()[key]
}

// customRoleExists reports whether the stored matrix describes this role.
//
// Assignment asks before saving: a membership naming a role nobody wrote down
// would be a permission that cannot be explained to the person holding it.
func (h Handlers) customRoleExists(role string) bool {
	key := rbacRoleKey(strings.TrimSpace(role))
	if key == "" {
		return false
	}
	_, found := h.rbacRoleBases()[key]
	return found
}

// assignableRoleError refuses a role that cannot be granted here, and says why.
//
// Three refusals, in the order a person meets them: the role does not exist,
// the edition cannot enforce it, or the installation has not turned the matrix
// on. Each is a different thing to do next, so each says something different.
func (h Handlers) assignableRoleError(role string) string {
	candidate := access.Role(strings.TrimSpace(strings.ToLower(role)))
	if candidate.IsBuiltin() {
		return ""
	}
	if !h.customRoleExists(string(candidate)) {
		return "unknown role " + role + "; define it in the role matrix first"
	}
	if !h.featureEnabled(edition.FeatureCustomRBACMatrix) {
		return "role " + role + " is defined in the matrix, but this edition decides permissions from the built-in roles only"
	}
	return ""
}

// assignableRole is one role a project may grant, as a member screen needs it.
type assignableRole struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// BasedOn is what the platform grants on its own. A screen shows it on a
	// custom role because that is what the person will actually get until the
	// matrix has something more to say.
	BasedOn string `json:"basedOn"`
	Builtin bool   `json:"builtin"`
}

// ListAssignableRoles answers what may be given to somebody here.
//
// The three built-in roles always, and the installation's own roles where this
// edition can enforce them. A role the platform would not honour is left out
// rather than listed and refused on save: an option that cannot be chosen is a
// promise the screen should not make.
func (h Handlers) ListAssignableRoles(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireIdentity(w, r); !ok {
		return
	}
	out := []assignableRole{}
	for _, builtin := range access.Builtins() {
		out = append(out, assignableRole{
			Key: string(builtin), Name: string(builtin),
			BasedOn: string(builtin), Builtin: true,
		})
	}
	if h.featureEnabled(edition.FeatureCustomRBACMatrix) {
		rows, err := h.loadRBACPolicyRows()
		if err == nil {
			for _, row := range rows {
				key := rbacRoleKey(firstNonEmpty(row.Key, row.Role))
				if key == "" || access.Role(key).IsBuiltin() {
					continue
				}
				// A shipped row describes how the platform resolves access -
				// owner, reader, writer - and is not something to hand to a
				// person on a project. Only what the installation added is.
				if _, shipped := rbacShippedRowBases[key]; shipped {
					continue
				}
				base, err := rbacRowBase(row)
				if err != nil {
					continue
				}
				out = append(out, assignableRole{
					Key: key, Name: row.Role, Description: row.Description,
					BasedOn: string(base),
				})
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"roles": out})
}
