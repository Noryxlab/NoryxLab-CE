package handlers

import (
	"strings"

	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	datasourcedomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/datasource"
	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
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
	organizations := map[string]string{}
	for _, item := range items {
		if strings.EqualFold(item.SubjectType, "organization") {
			if h.keycloak != nil {
				if listed, err := h.keycloak.ListOrganizations(); err == nil {
					for _, organization := range listed {
						organizations[organization.ID] = organization.Name
					}
				}
			}
			break
		}
	}
	for index := range items {
		subject := strings.TrimSpace(items[index].SubjectID)
		if strings.EqualFold(items[index].SubjectType, "organization") {
			if name, ok := organizations[subject]; ok && strings.TrimSpace(name) != "" {
				items[index].SubjectName = name
				continue
			}
		}
		// A name that cannot be resolved falls back to the identifier: an
		// unreadable row is better than a missing one.
		items[index].SubjectName = subject
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
