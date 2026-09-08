package handlers

import (
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	datasetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	ontologydomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/ontology"
)

// What a platform administrator can see.
//
// The permission checks already let an administrator *open* anything - reading
// a dataset by its id has always been allowed - but the listings only ever
// returned what the caller held a grant on. So transferring a dataset to an
// organization you do not belong to made it vanish from your own screen while
// remaining yours to administer: you could still reach it, if you knew its id,
// and nothing on the platform would offer it to you again.
//
// An administrator now sees everything, and every row that is there only
// because of that role is marked. Seeing across every project and every
// organization is real power over regulated data, and a screen that shows it
// without saying so invites people to forget they are using it.

func (h Handlers) datasetsVisibleTo(identity auth.Identity) ([]datasetdomain.Dataset, error) {
	own, err := h.datasetStore.ListBySubjects(h.datasetSubjects(identity))
	if err != nil {
		return nil, err
	}
	if !h.isGlobalAdmin(identity) {
		return own, nil
	}
	all, err := h.datasetStore.ListAll()
	if err != nil {
		// The administrator keeps the view a member would get rather than an
		// error page: a broken wide listing must not cost them the narrow one.
		return own, nil
	}
	granted := map[string]bool{}
	for _, item := range own {
		granted[item.ID] = true
	}
	for index := range all {
		all[index].AdminVisible = !granted[all[index].ID]
	}
	return all, nil
}

func (h Handlers) ontologiesVisibleTo(identity auth.Identity) ([]ontologydomain.Ontology, error) {
	own, err := h.ontologyStore.ListBySubjects(h.ontologySubjects(identity))
	if err != nil {
		return nil, err
	}
	if !h.isGlobalAdmin(identity) {
		return own, nil
	}
	all, err := h.ontologyStore.ListAll()
	if err != nil {
		return own, nil
	}
	granted := map[string]string{}
	for _, item := range own {
		granted[item.ID] = item.AccessRole
	}
	for index := range all {
		role, held := granted[all[index].ID]
		all[index].AdminVisible = !held
		if held {
			all[index].AccessRole = role
		}
	}
	return all, nil
}

// projectIsAdminVisible reports whether a project is on an administrator's
// screen only through the role, so the caller can mark it.
func (h Handlers) projectIsAdminVisible(userID, projectID string) bool {
	return !h.hasProjectMembership(strings.TrimSpace(userID), strings.TrimSpace(projectID))
}
