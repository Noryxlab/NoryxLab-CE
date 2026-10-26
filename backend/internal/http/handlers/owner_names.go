package handlers

import (
	"strings"

	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	datasourcedomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/datasource"
	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
	organizationdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/organization"
)

// What a screen shows for an owner, in one notation everywhere.
//
// A user is stored by their username, which reads perfectly well. An
// organization is stored by its identifier, which reads as nothing at all -
// "0f2c8a1e-..." owning a dataset tells a person only that something owns it.
// Projects resolved those names; datasets and ontologies did not, so the same
// organization appeared under two different notations depending on the screen.
//
// The directory is asked once for a whole list rather than once per row, and
// silently: a directory that cannot be reached is a reason to show the
// identifier, never a reason to fail the page.
type ownedResource interface {
	ownerKind() string
	ownerIdentifier() string
	setOwnerName(string)
}

func (h Handlers) nameOwners(items []ownedResource) {
	needsOrganization := false
	teamIDs := map[string]struct{}{}
	for _, item := range items {
		switch {
		case strings.EqualFold(item.ownerKind(), ownerOrganization):
			needsOrganization = true
		case strings.EqualFold(item.ownerKind(), ownerTeam):
			if identifier := strings.TrimSpace(item.ownerIdentifier()); identifier != "" {
				teamIDs[identifier] = struct{}{}
			}
		}
	}
	names := map[string]string{}
	if needsOrganization && h.keycloak != nil {
		if organizations, err := h.keycloak.ListOrganizations(); err == nil {
			for _, organization := range organizations {
				names[organization.ID] = organization.Name
			}
		}
	}
	if len(teamIDs) > 0 && h.teamStore != nil {
		for identifier := range teamIDs {
			if item, found, err := h.teamStore.GetByID(identifier); err == nil && found {
				names[identifier] = item.Name
			}
		}
	}
	for _, item := range items {
		identifier := item.ownerIdentifier()
		switch {
		case strings.EqualFold(item.ownerKind(), ownerOrganization), strings.EqualFold(item.ownerKind(), ownerTeam):
			if name, ok := names[identifier]; ok && strings.TrimSpace(name) != "" {
				item.setOwnerName(name)
				continue
			}
		}
		item.setOwnerName(identifier)
	}
}

type datasetOwner struct{ item *datasetdomain.Dataset }

func (o datasetOwner) ownerKind() string        { return o.item.OwnerType }
func (o datasetOwner) ownerIdentifier() string  { return o.item.OwnerID }
func (o datasetOwner) setOwnerName(name string) { o.item.OwnerName = name }

func (h Handlers) nameDatasetOwners(items []datasetdomain.Dataset) {
	owners := make([]ownedResource, 0, len(items))
	for index := range items {
		owners = append(owners, datasetOwner{item: &items[index]})
	}
	h.nameOwners(owners)
}

type ontologyOwner struct{ item *ontologydomain.Ontology }

func (o ontologyOwner) ownerKind() string        { return o.item.OwnerType }
func (o ontologyOwner) ownerIdentifier() string  { return o.item.OwnerID }
func (o ontologyOwner) setOwnerName(name string) { o.item.OwnerName = name }

func (h Handlers) nameOntologyOwners(items []ontologydomain.Ontology) {
	owners := make([]ownedResource, 0, len(items))
	for index := range items {
		owners = append(owners, ontologyOwner{item: &items[index]})
	}
	h.nameOwners(owners)
}

// nameAccessSubjects fills in who each identifier belongs to.
//
// Same resolution as an owner's name, on the other side of the same screen:
// the owner row was readable and every permission row below it was a UUID.
func (h Handlers) nameAccessSubjects(items []datasetdomain.Access) {
	// Organizations are listed once, and indexed by both of the things a
	// permission row can hold.
	//
	// A grant records the identifier the resolver returned, but an owner may
	// carry the alias somebody typed - the two paths disagreed, so an
	// organization showed as "compta" on one row and "Compta" on the next.
	// Indexing by id and by alias answers for both without asking which wrote
	// the row.
	organizations := map[string]string{}
	teams := map[string]string{}
	besoinOrganisations, besoinEquipes := false, false
	for _, item := range items {
		switch strings.ToLower(strings.TrimSpace(item.SubjectType)) {
		case "organization":
			besoinOrganisations = true
		case "team":
			besoinEquipes = true
		}
	}
	if besoinOrganisations && h.keycloak != nil {
		if listed, err := h.keycloak.ListOrganizations(); err == nil {
			for _, organization := range listed {
				if strings.TrimSpace(organization.Name) == "" {
					continue
				}
				organizations[organization.ID] = organization.Name
				if alias := strings.TrimSpace(organization.Alias); alias != "" {
					organizations[alias] = organization.Name
				}
			}
		}
	}
	if besoinEquipes && h.teamStore != nil {
		// Une equipe etait toujours un UUID a l'ecran, alors que c'est le
		// sujet qu'on accorde le plus souvent. Chacune est demandee par son
		// identifiant plutot que listee par organisation : la resolution
		// repond alors meme quand l'annuaire est injoignable, et ne coute que
		// les lignes affichees.
		for _, item := range items {
			if !strings.EqualFold(item.SubjectType, "team") {
				continue
			}
			id := strings.TrimSpace(item.SubjectID)
			if id == "" || teams[id] != "" {
				continue
			}
			if found, exists, err := h.teamStore.GetByID(id); err == nil && exists && strings.TrimSpace(found.Name) != "" {
				teams[id] = found.Name
			}
		}
	}
	for index := range items {
		subject := strings.TrimSpace(items[index].SubjectID)
		switch strings.ToLower(strings.TrimSpace(items[index].SubjectType)) {
		case "organization":
			if name, ok := organizations[subject]; ok {
				items[index].SubjectName = name
				continue
			}
		case "team":
			if name, ok := teams[subject]; ok {
				items[index].SubjectName = name
				continue
			}
		}
		// A name that cannot be resolved falls back to the identifier: an
		// unreadable row is better than a missing one.
		items[index].SubjectName = subject
	}
}

// nameDatasourceAccessSubjects, same as the three others.
func (h Handlers) nameDatasourceAccessSubjects(items []datasourcedomain.Access) {
	passerelle := make([]datasetdomain.Access, len(items))
	for index, item := range items {
		passerelle[index] = datasetdomain.Access{SubjectType: item.SubjectType, SubjectID: item.SubjectID}
	}
	h.nameAccessSubjects(passerelle)
	for index := range items {
		items[index].SubjectName = passerelle[index].SubjectName
	}
}

// nameOntologyAccessSubjects does for an ontology what the dataset version
// does: turn identifiers into names somebody can check.
func (h Handlers) nameOntologyAccessSubjects(items []ontologydomain.Access) {
	passerelle := make([]datasetdomain.Access, len(items))
	for index, item := range items {
		passerelle[index] = datasetdomain.Access{SubjectType: item.SubjectType, SubjectID: item.SubjectID}
	}
	h.nameAccessSubjects(passerelle)
	for index := range items {
		items[index].SubjectName = passerelle[index].SubjectName
	}
}

// nameExtractAccessSubjects resolves the identifiers of an extract's grants,
// for the same reason as a dataset's: a list of UUIDs is a list nobody checks.
func (h Handlers) nameExtractAccessSubjects(items []extractdomain.Access) {
	passerelle := make([]datasetdomain.Access, len(items))
	for index, item := range items {
		passerelle[index] = datasetdomain.Access{SubjectType: item.SubjectType, SubjectID: item.SubjectID}
	}
	h.nameAccessSubjects(passerelle)
	for index := range items {
		items[index].SubjectName = passerelle[index].SubjectName
	}
}

type extractOwner struct{ item *extractdomain.Extract }

func (o extractOwner) ownerKind() string        { return o.item.OwnerType }
func (o extractOwner) ownerIdentifier() string  { return o.item.OwnerID }
func (o extractOwner) setOwnerName(name string) { o.item.OwnerName = name }

func (h Handlers) nameExtractOwners(items []extractdomain.Extract) {
	owners := make([]ownedResource, 0, len(items))
	for index := range items {
		owners = append(owners, extractOwner{item: &items[index]})
	}
	h.nameOwners(owners)
}

type datasourceOwner struct{ item *datasourcedomain.Datasource }

func (o datasourceOwner) ownerKind() string        { return o.item.OwnerType }
func (o datasourceOwner) ownerIdentifier() string  { return o.item.OwnerID }
func (o datasourceOwner) setOwnerName(name string) { o.item.OwnerName = name }

func (h Handlers) nameDatasourceOwners(items []datasourcedomain.Datasource) {
	owners := make([]ownedResource, 0, len(items))
	for index := range items {
		owners = append(owners, datasourceOwner{item: &items[index]})
	}
	h.nameOwners(owners)
}

// nameOrganizationAdmins turns identifiers into the names the screen shows.
//
// This is the screen that says who may give away health data; a list of
// opaque identifiers is a list nobody can audit, and the person who most
// needs to read it cannot list teams to find out what they mean.
func (h Handlers) nameOrganizationAdmins(items []organizationdomain.Admin) {
	for index := range items {
		switch strings.ToLower(strings.TrimSpace(items[index].SubjectType)) {
		case "team":
			if h.teamStore == nil {
				continue
			}
			if team, found, err := h.teamStore.GetByID(items[index].SubjectID); err == nil && found {
				items[index].SubjectName = team.Name
			}
		default:
			items[index].SubjectName = items[index].SubjectID
		}
	}
}
