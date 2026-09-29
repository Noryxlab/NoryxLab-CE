package handlers

import (
	"net/http"
	"strings"
)

// What a project still holds, before anybody is allowed to delete it.
//
// Deleting a project used to remove its workspaces and stop there. Apps and
// jobs were left where they were, and an app is deliberately never reaped:
// nothing stops one, so an app whose project has gone keeps running against a
// project identifier that resolves to nothing. It costs a card, serves an
// endpoint, and appears on no screen, because every screen finds it through
// the project it no longer has.
//
// Neither cluster carries such an orphan today, checked before this was
// written. That is luck rather than design - nobody has yet deleted a project
// that had one - and it is the kind of luck that ends quietly.
//
// So the inventory is taken first and the refusal lists it. A person deleting
// a project is deciding to destroy what is inside it, and they cannot decide
// that about things nobody showed them.

type projectWorkloadItem struct {
	Kind   string `json:"kind"`
	ID     string `json:"id"`
	Name   string `json:"name"`
	Status string `json:"status,omitempty"`
}

type projectInventory struct {
	Items []projectWorkloadItem `json:"items"`
	// Running is how many are consuming something right now. It is the figure
	// that belongs in a warning: ten stopped workspaces and one running app
	// are not the same situation.
	Running int `json:"running"`
}

func (h Handlers) projectWorkloads(projectID string) projectInventory {
	inventory := projectInventory{Items: []projectWorkloadItem{}}
	add := func(kind, id, name, status string) {
		inventory.Items = append(inventory.Items, projectWorkloadItem{
			Kind: kind, ID: id, Name: firstNonEmpty(name, id), Status: status,
		})
		if isRunningStatus(status) {
			inventory.Running++
		}
	}

	if h.workspaceStore != nil {
		if items, err := h.workspaceStore.List(); err == nil {
			for _, item := range items {
				if item.ProjectID == projectID {
					add("workspace", item.ID, item.Name, item.Status)
				}
			}
		}
	}
	if h.appStore != nil {
		if items, err := h.appStore.List(); err == nil {
			for _, item := range items {
				if item.ProjectID == projectID {
					add("app", item.ID, item.Name, item.Status)
				}
			}
		}
	}
	if h.jobStore != nil {
		if items, err := h.jobStore.List(); err == nil {
			for _, item := range items {
				if item.ProjectID == projectID {
					add("job", item.ID, item.Name, item.Status)
				}
			}
		}
	}
	return inventory
}

// isRunningStatus is deliberately generous about the words.
//
// Each kind names its states its own way, and a warning that undercounts is
// worse than one that overcounts: being told three things are running when one
// has just stopped costs a second look, while being told none are running when
// one is costs the thing.
func isRunningStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "stopped", "deleted", "failed", "succeeded", "completed", "terminated":
		return false
	}
	return true
}

// GetProjectWorkloads answers what deleting this project would destroy.
//
// Read by the confirmation screen before it asks anything, so the list and the
// refusal below agree about what is there.
func (h Handlers) GetProjectWorkloads(w http.ResponseWriter, r *http.Request) {
	callerID, ok := h.requireUserID(w, r)
	if !ok {
		return
	}
	projectID := strings.TrimSpace(r.PathValue("projectID"))
	if !h.requireProjectMember(w, projectID, callerID, "project workloads") {
		return
	}
	writeJSON(w, http.StatusOK, h.projectWorkloads(projectID))
}

// deleteProjectApps removes what an app leaves in the cluster, then its record.
//
// The same three objects the app screen removes, and in the same order: a
// service, a pod, and the secret holding whatever the app was given. Failures
// are tolerated individually because the goal is an empty project rather than
// a perfect sequence - an object already gone must not stop the one after it -
// but the record only goes once the cluster side has been attempted.
func (h Handlers) deleteProjectApps(projectID string) error {
	if h.appStore == nil {
		return nil
	}
	items, err := h.appStore.List()
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ProjectID != projectID {
			continue
		}
		if h.runtime != nil {
			_ = h.runtime.DeleteService(item.ServiceName)
			_ = h.runtime.DeletePod(item.PodName)
			_ = h.runtime.DeleteSecret(item.PodName + "-user-secrets")
		}
		if err := h.appStore.Delete(item.ID); err != nil {
			return err
		}
	}
	return nil
}

// deleteProjectJobs stops the cluster job, then forgets it.
func (h Handlers) deleteProjectJobs(projectID string) error {
	if h.jobStore == nil {
		return nil
	}
	items, err := h.jobStore.List()
	if err != nil {
		return err
	}
	for _, item := range items {
		if item.ProjectID != projectID {
			continue
		}
		if h.runtime != nil {
			if err := h.runtime.DeleteJob(item.JobName); err != nil && !isNotFoundError(err) {
				return err
			}
		}
		if err := h.jobStore.Delete(item.ID); err != nil {
			return err
		}
	}
	return nil
}
