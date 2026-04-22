package access

type Role string

const (
	RoleViewer Role = "viewer"
	RoleEditor Role = "editor"
	RoleAdmin  Role = "admin"
)

func (r Role) CanLaunchPod() bool {
	return r == RoleEditor || r == RoleAdmin
}

func (r Role) CanRunBuild() bool {
	return r == RoleEditor || r == RoleAdmin
}
