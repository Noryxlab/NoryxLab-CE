package access

type Role string

const (
	RoleViewer Role = "viewer"
	RoleEditor Role = "editor"
	RoleAdmin  Role = "admin"
)

// Builtins are the roles the platform itself understands.
//
// An installation may describe roles of its own in the Enterprise matrix, and
// those are held as their own key - "data-steward", not "editor". Every rule
// written in Go still has to answer for such a role, and it answers through
// the built-in the custom role declares as its base: what the platform can
// demonstrate on its own is the base, and the matrix refines it where it has
// something to say. A role whose base is unknown grants nothing, which is the
// only safe reading of a permission nobody can resolve.
func Builtins() []Role { return []Role{RoleViewer, RoleEditor, RoleAdmin} }

// IsBuiltin reports whether the platform decides this role by itself.
func (r Role) IsBuiltin() bool {
	switch r {
	case RoleViewer, RoleEditor, RoleAdmin:
		return true
	}
	return false
}

// Rank orders the roles, so a user who holds one directly and another through
// an organization gets the stronger of the two. Zero means no role at all.
func (r Role) Rank() int {
	switch r {
	case RoleAdmin:
		return 3
	case RoleEditor:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}

// Strongest returns whichever role grants more. Grants add up rather than
// override: removing someone from an organization must not silently take away
// access they were given personally, and vice versa.
func Strongest(roles ...Role) Role {
	best := Role("")
	for _, role := range roles {
		if role.Rank() > best.Rank() {
			best = role
		}
	}
	return best
}

func (r Role) CanLaunchPod() bool {
	return r == RoleEditor || r == RoleAdmin
}

func (r Role) CanRunBuild() bool {
	return r == RoleEditor || r == RoleAdmin
}
