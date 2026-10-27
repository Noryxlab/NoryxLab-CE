// Package platformrole holds what a role may bring into existence.
//
// A capability is true everywhere and names no object: "may this role create
// a dataset". A grant names one object and one subject: "may this person read
// dataset D". The first is a checkbox in this package; the second is an
// access list and must never become one here - expressing "sees what their
// team shared" as a capability would rebuild the inheritance that ADR-048
// removed. ADR-049 draws that line; this file is the line.
//
// Rows are roles, columns are functions, cells are yes or no. Three rows ship
// locked: the administrator, who acts on what is not theirs; the user, who
// acts on their own; and the consultant, who is the user row with every
// creation box unchecked - an auditor, a study sponsor, a student. An
// installation adds its own rows beside those and sets their boxes itself.
package platformrole

import (
	"strings"
	"time"
)

// Capability is one function the matrix governs. The string is the key
// stored and sent; it never changes once shipped, because a stored custom row
// names it.
type Capability string

const (
	CreateProject            Capability = "create-project"
	CreateDataset            Capability = "create-dataset"
	RegisterRegulatedDataset Capability = "register-regulated-dataset"
	ScanOntology             Capability = "scan-ontology"
	CreateExtract            Capability = "create-extract"
	LaunchWorkspace          Capability = "launch-workspace"
	LaunchJob                Capability = "launch-job"
	DeployApp                Capability = "deploy-app"
	BuildEnvironment         Capability = "build-environment"
	DeclareDatasource        Capability = "declare-datasource"
	DeclareRepository        Capability = "declare-repository"
	AdministerOrganization   Capability = "administer-organization"
)

// All is every capability, in the order the screen shows them: the things a
// researcher does daily first, the things an installation delegates last.
var All = []Capability{
	CreateProject,
	LaunchWorkspace,
	LaunchJob,
	DeployApp,
	BuildEnvironment,
	CreateDataset,
	ScanOntology,
	CreateExtract,
	DeclareDatasource,
	DeclareRepository,
	RegisterRegulatedDataset,
	AdministerOrganization,
}

// Known reports whether a capability string names a function the handlers
// consult. A custom row carrying an unknown one would be a box nobody reads -
// the exact defect a locked row already paid for once.
func Known(value string) bool {
	for _, capability := range All {
		if string(capability) == strings.TrimSpace(value) {
			return true
		}
	}
	return false
}

// The three shipped keys. An assignment stores one of these or a custom key.
const (
	KeyAdministrator = "administrator"
	KeyUser          = "user"
	KeyConsultant    = "consultant"
)

// Role is one row of the matrix.
type Role struct {
	Key         string `json:"key"`
	Name        string `json:"name"`
	Description string `json:"description"`
	// Capabilities is what the row may do. Absent means no.
	Capabilities []Capability `json:"capabilities"`
	// Locked rows ship with the platform and describe what it does; they are
	// shown and never edited. Custom rows are the installation's.
	Locked bool `json:"locked"`
	// Assignable is false for the administrator row only: that right is the
	// directory's realm role, never stored as an assignment. Naming one from
	// the interface writes to the directory instead.
	Assignable bool      `json:"assignable"`
	CreatedAt  time.Time `json:"createdAt,omitempty"`
	UpdatedAt  time.Time `json:"updatedAt,omitempty"`
}

// Has reports whether this role carries the capability.
func (r Role) Has(capability Capability) bool {
	for _, held := range r.Capabilities {
		if held == capability {
			return true
		}
	}
	return false
}

// Normalise trims and lowercases what a screen typed, and drops unknown
// capabilities rather than storing a box nothing reads.
func (r *Role) Normalise() {
	r.Key = strings.ToLower(strings.TrimSpace(r.Key))
	r.Name = strings.TrimSpace(r.Name)
	r.Description = strings.TrimSpace(r.Description)
	kept := make([]Capability, 0, len(r.Capabilities))
	seen := map[Capability]bool{}
	for _, capability := range r.Capabilities {
		capability = Capability(strings.TrimSpace(string(capability)))
		if !Known(string(capability)) || seen[capability] {
			continue
		}
		seen[capability] = true
		kept = append(kept, capability)
	}
	r.Capabilities = kept
}

// Shipped is the three rows the platform ships, in display order.
//
// The user row carries everything a person does on their own. Two things are
// deliberately not on it: registering regulated data, which an installation
// hands to a named data steward through a custom row, and administering an
// organization, which ADR-048 keeps with the platform administrator.
func Shipped() []Role {
	everything := append([]Capability(nil), All...)
	user := []Capability{
		CreateProject, LaunchWorkspace, LaunchJob, DeployApp, BuildEnvironment,
		CreateDataset, ScanOntology, CreateExtract, DeclareDatasource, DeclareRepository,
	}
	return []Role{
		{
			Key:          KeyAdministrator,
			Name:         "Administrateur",
			Description:  "Agit sur ce qui n'est pas à lui : les objets des autres, les équipes des autres, les réglages de la plateforme. Porté par le rôle d'administrateur de l'annuaire, jamais attribué ici.",
			Capabilities: everything,
			Locked:       true,
			Assignable:   false,
		},
		{
			Key:          KeyUser,
			Name:         "Utilisateur",
			Description:  "Agit sur le sien : crée, possède, partage ce qu'il a créé, et reçoit ce que d'autres lui partagent. Le rôle de tout compte auquel rien n'a été attribué.",
			Capabilities: user,
			Locked:       true,
			Assignable:   true,
		},
		{
			Key:          KeyConsultant,
			Name:         "Consultant",
			Description:  "Consulte ce qui lui est partagé et ouvre les applications, sans rien créer ni lancer : un auditeur, un promoteur d'étude, un stagiaire.",
			Capabilities: []Capability{},
			Locked:       true,
			Assignable:   true,
		},
	}
}

// ShippedByKey finds a shipped row, false for a custom key.
func ShippedByKey(key string) (Role, bool) {
	key = strings.ToLower(strings.TrimSpace(key))
	for _, role := range Shipped() {
		if role.Key == key {
			return role, true
		}
	}
	return Role{}, false
}

// Assignment is which row one person holds.
type Assignment struct {
	UserID     string    `json:"userId"`
	RoleKey    string    `json:"roleKey"`
	AssignedBy string    `json:"assignedBy,omitempty"`
	AssignedAt time.Time `json:"assignedAt"`
}
