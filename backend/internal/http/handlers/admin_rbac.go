package handlers

import (
	"encoding/csv"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

type rbacMatrixReport struct {
	GeneratedAt time.Time         `json:"generatedAt"`
	Summary     rbacMatrixSummary `json:"summary"`
	Subjects    []rbacSubject     `json:"subjects"`
	Resources   []rbacResource    `json:"resources"`
	Cells       []rbacCell        `json:"cells"`
}

type rbacMatrixSummary struct {
	Users         int `json:"users"`
	Organizations int `json:"organizations"`
	Projects      int `json:"projects"`
	Datasets      int `json:"datasets"`
	Ontologies    int `json:"ontologies"`
	Datasources   int `json:"datasources"`
	Grants        int `json:"grants"`
	Inherited     int `json:"inherited"`
}

type rbacSubject struct {
	Type             string `json:"type"`
	ID               string `json:"id"`
	Name             string `json:"name"`
	Email            string `json:"email,omitempty"`
	OrganizationID   string `json:"organizationId,omitempty"`
	OrganizationName string `json:"organizationName,omitempty"`
}

type rbacResource struct {
	Type           string `json:"type"`
	ID             string `json:"id"`
	Name           string `json:"name"`
	OwnerType      string `json:"ownerType"`
	OwnerID        string `json:"ownerId"`
	OwnerName      string `json:"ownerName"`
	Classification string `json:"classification,omitempty"`
	Source         string `json:"source,omitempty"`
}

type rbacCell struct {
	SubjectType  string `json:"subjectType"`
	SubjectID    string `json:"subjectId"`
	SubjectName  string `json:"subjectName"`
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	ResourceName string `json:"resourceName"`
	Role         string `json:"role"`
	Source       string `json:"source"`
	Inherited    bool   `json:"inherited"`
}

func (h Handlers) GetAdminRBACMatrix(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "rbac matrix"); !ok {
		return
	}
	report, err := h.buildRBACMatrixReport()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to build RBAC matrix: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h Handlers) ExportAdminRBACMatrixCSV(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "rbac matrix export"); !ok {
		return
	}
	report, err := h.buildRBACMatrixReport()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to export RBAC matrix: " + err.Error()})
		return
	}
	resourceByKey := map[string]rbacResource{}
	for _, resource := range report.Resources {
		resourceByKey[resource.Type+":"+resource.ID] = resource
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="noryx-rbac-matrix.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"subject_type", "subject_id", "subject_name", "resource_type", "resource_id", "resource_name", "role", "source", "inherited", "owner_type", "owner_id", "owner_name", "classification"})
	for _, cell := range report.Cells {
		resource := resourceByKey[cell.ResourceType+":"+cell.ResourceID]
		_ = writer.Write([]string{cell.SubjectType, cell.SubjectID, cell.SubjectName, cell.ResourceType, cell.ResourceID, cell.ResourceName, cell.Role, cell.Source, boolCSV(cell.Inherited), resource.OwnerType, resource.OwnerID, resource.OwnerName, resource.Classification})
	}
	writer.Flush()
}

func (h Handlers) buildRBACMatrixReport() (rbacMatrixReport, error) {
	projects, err := h.projectStore.List()
	if err != nil {
		return rbacMatrixReport{}, err
	}
	datasets, err := h.datasetStore.ListAll()
	if err != nil {
		return rbacMatrixReport{}, err
	}
	datasets = h.filterDatasetsForEdition(datasets)
	ontologies, err := h.ontologyStore.ListAll()
	if err != nil {
		return rbacMatrixReport{}, err
	}
	datasources, err := h.datasourceStore.ListAll()
	if err != nil {
		return rbacMatrixReport{}, err
	}
	projectRoles, err := h.accessStore.ListProjectRoles()
	if err != nil {
		return rbacMatrixReport{}, err
	}

	users, organizations, orgMembers := h.dataUsageIdentityMaps()
	projectByID := map[string]project.Project{}
	rolesByProject := map[string][]store.ProjectRole{}
	for _, item := range projects {
		projectByID[item.ID] = item
	}
	for _, role := range projectRoles {
		rolesByProject[role.ProjectID] = append(rolesByProject[role.ProjectID], role)
	}

	subjects := map[string]rbacSubject{}
	resources := map[string]rbacResource{}
	cells := map[string]rbacCell{}
	addSubject := func(subjectType, subjectID string) rbacSubject {
		subjectType = normalizedSubjectType(subjectType)
		subjectID = strings.TrimSpace(subjectID)
		if subjectID == "" {
			return rbacSubject{}
		}
		user := keycloak.User{}
		if subjectType == "user" {
			user = users[subjectID]
			subjectID = firstNonEmpty(user.ID, subjectID)
		}
		key := subjectType + ":" + subjectID
		if existing, ok := subjects[key]; ok {
			return existing
		}
		item := rbacSubject{Type: subjectType, ID: subjectID, Name: subjectID}
		if subjectType == "organization" {
			organization := organizations[subjectID]
			item.Name = firstNonEmpty(organization.Name, organization.Alias, subjectID)
		} else {
			item.Name = firstNonEmpty(user.Username, user.Email, subjectID)
			item.Email = user.Email
			if orgID, orgName := userOrganization(subjectID, orgMembers, organizations); orgID != "" {
				item.OrganizationID = orgID
				item.OrganizationName = orgName
			}
		}
		subjects[key] = item
		return item
	}
	addResource := func(resource rbacResource) {
		resource.Type = strings.ToLower(strings.TrimSpace(resource.Type))
		resource.ID = strings.TrimSpace(resource.ID)
		if resource.Type == "" || resource.ID == "" {
			return
		}
		resource.Name = firstNonEmpty(resource.Name, resource.ID)
		resource.OwnerType = normalizedSubjectType(resource.OwnerType)
		resource.OwnerID = strings.TrimSpace(resource.OwnerID)
		resource.OwnerName = subjectName(resource.OwnerType, resource.OwnerID, users, organizations)
		resources[resource.Type+":"+resource.ID] = resource
	}
	addCell := func(subjectType, subjectID, resourceType, resourceID, role, source string, inherited bool) {
		subject := addSubject(subjectType, subjectID)
		if subject.ID == "" || strings.TrimSpace(resourceID) == "" || strings.TrimSpace(role) == "" {
			return
		}
		resource := resources[resourceType+":"+resourceID]
		key := strings.Join([]string{subject.Type, subject.ID, resourceType, resourceID, role, source, boolCSV(inherited)}, "|")
		cells[key] = rbacCell{SubjectType: subject.Type, SubjectID: subject.ID, SubjectName: subject.Name, ResourceType: resourceType, ResourceID: resourceID, ResourceName: firstNonEmpty(resource.Name, resourceID), Role: role, Source: source, Inherited: inherited}
	}
	addSubjectWithOrganizationExpansion := func(subjectType, subjectID, resourceType, resourceID, role, source string) {
		subjectType = normalizedSubjectType(subjectType)
		subjectID = strings.TrimSpace(subjectID)
		addCell(subjectType, subjectID, resourceType, resourceID, role, source, false)
		if subjectType != "organization" {
			return
		}
		for _, member := range orgMembers[subjectID] {
			addCell("user", member.ID, resourceType, resourceID, role, "organization:"+firstNonEmpty(organizations[subjectID].Name, organizations[subjectID].Alias, subjectID), true)
		}
	}

	for _, user := range users {
		if user.ID != "" {
			addSubject("user", user.ID)
		}
	}
	for _, organization := range organizations {
		if organization.ID != "" {
			addSubject("organization", organization.ID)
		}
	}

	for _, item := range projects {
		ownerType := firstNonEmpty(item.OwnerType, "user")
		addResource(rbacResource{Type: "project", ID: item.ID, Name: item.Name, OwnerType: ownerType, OwnerID: item.OwnerID})
		addSubjectWithOrganizationExpansion(ownerType, item.OwnerID, "project", item.ID, "owner", "owner")
		for _, role := range rolesByProject[item.ID] {
			addCell("user", role.UserID, "project", item.ID, string(role.Role), "project-role", false)
		}
	}

	for _, item := range datasets {
		ownerType := firstNonEmpty(item.OwnerType, "user")
		ownerID := firstNonEmpty(item.OwnerID, item.OwnerUserID)
		addResource(rbacResource{Type: "dataset", ID: item.ID, Name: item.Name, OwnerType: ownerType, OwnerID: ownerID, Classification: item.Classification, Source: item.Provider})
		addSubjectWithOrganizationExpansion(ownerType, ownerID, "dataset", item.ID, "owner", "owner")
		accessItems, err := h.datasetStore.ListAccess(item.ID)
		if err != nil {
			return rbacMatrixReport{}, err
		}
		for _, accessItem := range accessItems {
			addSubjectWithOrganizationExpansion(accessItem.SubjectType, accessItem.SubjectID, "dataset", item.ID, accessItem.Role, "direct")
		}
	}

	for _, item := range ontologies {
		ownerType := firstNonEmpty(item.OwnerType, "user")
		ownerID := firstNonEmpty(item.OwnerID, item.OwnerUserID)
		addResource(rbacResource{Type: "ontology", ID: item.ID, Name: item.Name, OwnerType: ownerType, OwnerID: ownerID, Classification: item.SourceType, Source: item.InferenceProfile})
		addSubjectWithOrganizationExpansion(ownerType, ownerID, "ontology", item.ID, "owner", "owner")
		accessItems, err := h.ontologyStore.ListAccess(item.ID)
		if err != nil {
			return rbacMatrixReport{}, err
		}
		for _, accessItem := range accessItems {
			addSubjectWithOrganizationExpansion(accessItem.SubjectType, accessItem.SubjectID, "ontology", item.ID, accessItem.Role, "direct")
		}
	}

	for _, item := range datasources {
		addResource(rbacResource{Type: "datasource", ID: item.ID, Name: item.Name, OwnerType: "user", OwnerID: item.OwnerUserID, Classification: item.Type, Source: item.Source})
		addCell("user", item.OwnerUserID, "datasource", item.ID, "owner", "owner", false)
	}

	for _, p := range projects {
		projectSubjects := effectiveProjectSubjects(p, rolesByProject[p.ID], orgMembers)
		attachProjectCells := func(resourceType string, ids []string) {
			for _, resourceID := range ids {
				if _, ok := resources[resourceType+":"+resourceID]; !ok {
					continue
				}
				for _, subject := range projectSubjects {
					addCell(subject.Type, subject.ID, resourceType, resourceID, "project-"+subject.Role, "project:"+firstNonEmpty(p.Name, p.ID), subject.Inherited)
				}
			}
		}
		if ids, err := h.projectResourceStore.ListProjectDatasetIDs(p.ID); err == nil {
			attachProjectCells("dataset", ids)
		}
		if ids, err := h.projectResourceStore.ListProjectOntologyIDs(p.ID); err == nil {
			attachProjectCells("ontology", ids)
		}
		if ids, err := h.projectResourceStore.ListProjectDatasourceIDs(p.ID); err == nil {
			attachProjectCells("datasource", ids)
		}
	}

	subjectList := make([]rbacSubject, 0, len(subjects))
	for _, subject := range subjects {
		subjectList = append(subjectList, subject)
	}
	sort.Slice(subjectList, func(i, j int) bool {
		return subjectList[i].Type+subjectList[i].Name < subjectList[j].Type+subjectList[j].Name
	})
	resourceList := make([]rbacResource, 0, len(resources))
	for _, resource := range resources {
		resourceList = append(resourceList, resource)
	}
	sort.Slice(resourceList, func(i, j int) bool {
		return resourceList[i].Type+resourceList[i].Name < resourceList[j].Type+resourceList[j].Name
	})
	cellList := make([]rbacCell, 0, len(cells))
	inherited := 0
	for _, cell := range cells {
		if cell.Inherited {
			inherited++
		}
		cellList = append(cellList, cell)
	}
	sort.Slice(cellList, func(i, j int) bool {
		return cellList[i].SubjectName+cellList[i].ResourceType+cellList[i].ResourceName+cellList[i].Role < cellList[j].SubjectName+cellList[j].ResourceType+cellList[j].ResourceName+cellList[j].Role
	})
	return rbacMatrixReport{
		GeneratedAt: time.Now().UTC(),
		Summary: rbacMatrixSummary{
			Users:         countRBACSubjects(subjectList, "user"),
			Organizations: countRBACSubjects(subjectList, "organization"),
			Projects:      len(projects),
			Datasets:      len(datasets),
			Ontologies:    len(ontologies),
			Datasources:   len(datasources),
			Grants:        len(cellList),
			Inherited:     inherited,
		},
		Subjects:  subjectList,
		Resources: resourceList,
		Cells:     cellList,
	}, nil
}

type projectSubjectGrant struct {
	Type      string
	ID        string
	Role      string
	Inherited bool
}

func effectiveProjectSubjects(item project.Project, roles []store.ProjectRole, orgMembers map[string][]keycloak.User) []projectSubjectGrant {
	out := []projectSubjectGrant{}
	ownerType := normalizedSubjectType(firstNonEmpty(item.OwnerType, "user"))
	ownerID := strings.TrimSpace(item.OwnerID)
	if ownerID != "" {
		out = append(out, projectSubjectGrant{Type: ownerType, ID: ownerID, Role: "owner"})
		if ownerType == "organization" {
			for _, member := range orgMembers[ownerID] {
				out = append(out, projectSubjectGrant{Type: "user", ID: member.ID, Role: "owner", Inherited: true})
			}
		}
	}
	for _, role := range roles {
		out = append(out, projectSubjectGrant{Type: "user", ID: role.UserID, Role: string(role.Role)})
	}
	return out
}

func normalizedSubjectType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "organization" || value == "org" {
		return "organization"
	}
	return "user"
}

func userOrganization(userID string, orgMembers map[string][]keycloak.User, organizations map[string]keycloak.Organization) (string, string) {
	for orgID, members := range orgMembers {
		for _, member := range members {
			if member.ID == userID || member.Username == userID {
				organization := organizations[orgID]
				return orgID, firstNonEmpty(organization.Name, organization.Alias, orgID)
			}
		}
	}
	return "", ""
}

func countRBACSubjects(subjects []rbacSubject, subjectType string) int {
	count := 0
	for _, subject := range subjects {
		if subject.Type == subjectType {
			count++
		}
	}
	return count
}

func boolCSV(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
