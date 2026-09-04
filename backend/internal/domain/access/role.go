package access

type Role string

const (
	RoleViewer Role = "viewer"
	RoleEditor Role = "editor"
	RoleAdmin  Role = "admin"
)

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
