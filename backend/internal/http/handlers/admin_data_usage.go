package handlers

import (
	"encoding/csv"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/job"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/workspace"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

type dataUsageReport struct {
	GeneratedAt time.Time          `json:"generatedAt"`
	Summary     dataUsageSummary   `json:"summary"`
	Nodes       []dataUsageNode    `json:"nodes"`
	Edges       []dataUsageEdge    `json:"edges"`
	Rows        []dataUsageCSVLine `json:"rows,omitempty"`
}

type dataUsageSummary struct {
	Datasets      int `json:"datasets"`
	HDSDatasets   int `json:"hdsDatasets"`
	Projects      int `json:"projects"`
	Users         int `json:"users"`
	Organizations int `json:"organizations"`
	Workloads     int `json:"workloads"`
	Edges         int `json:"edges"`
}

type dataUsageNode struct {
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Label    string `json:"label"`
	SubLabel string `json:"subLabel,omitempty"`
	Class    string `json:"class,omitempty"`
}

type dataUsageEdge struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Relation  string `json:"relation"`
	Role      string `json:"role,omitempty"`
	ProjectID string `json:"projectId,omitempty"`
}

type dataUsageCSVLine struct {
	DatasetID             string
	DatasetName           string
	DatasetClassification string
	DatasetOwnerType      string
	DatasetOwnerID        string
	Relation              string
	SubjectType           string
	SubjectID             string
	SubjectName           string
	Role                  string
	ProjectID             string
	ProjectName           string
	WorkloadKind          string
	WorkloadID            string
	WorkloadName          string
	WorkloadStatus        string
}

func (h Handlers) GetAdminDataUsage(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "data usage"); !ok {
		return
	}
	report, err := h.buildDataUsageReport(false)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to build data usage report: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h Handlers) ExportAdminDataUsageCSV(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.requireAdminModule(w, r, "data usage export"); !ok {
		return
	}
	report, err := h.buildDataUsageReport(true)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to export data usage report: " + err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="noryx-data-usage.csv"`)
	writer := csv.NewWriter(w)
	_ = writer.Write([]string{"dataset_id", "dataset_name", "classification", "dataset_owner_type", "dataset_owner_id", "relation", "subject_type", "subject_id", "subject_name", "role", "project_id", "project_name", "workload_kind", "workload_id", "workload_name", "workload_status"})
	for _, row := range report.Rows {
		_ = writer.Write([]string{row.DatasetID, row.DatasetName, row.DatasetClassification, row.DatasetOwnerType, row.DatasetOwnerID, row.Relation, row.SubjectType, row.SubjectID, row.SubjectName, row.Role, row.ProjectID, row.ProjectName, row.WorkloadKind, row.WorkloadID, row.WorkloadName, row.WorkloadStatus})
	}
	writer.Flush()
}

func (h Handlers) buildDataUsageReport(includeRows bool) (dataUsageReport, error) {
	projects, err := h.projectStore.List()
	if err != nil {
		return dataUsageReport{}, err
	}
	datasets, err := h.datasetStore.ListAll()
	if err != nil {
		return dataUsageReport{}, err
	}
	datasets = h.filterDatasetsForEdition(datasets)
	projectRoles, err := h.accessStore.ListProjectRoles()
	if err != nil {
		return dataUsageReport{}, err
	}
	workspaces, err := h.workspaceStore.List()
	if err != nil {
		return dataUsageReport{}, err
	}
	jobs, err := h.jobStore.List()
	if err != nil {
		return dataUsageReport{}, err
	}
	apps, err := h.appStore.List()
	if err != nil {
		return dataUsageReport{}, err
	}

	users, organizations, orgMembers := h.dataUsageIdentityMaps()
	projectByID := map[string]project.Project{}
	for _, item := range projects {
		projectByID[item.ID] = item
	}
	rolesByProject := map[string][]store.ProjectRole{}
	for _, role := range projectRoles {
		rolesByProject[role.ProjectID] = append(rolesByProject[role.ProjectID], role)
	}
	workloadsByProject := buildWorkloadUsage(workspaces, jobs, apps)

	nodes := map[string]dataUsageNode{}
	edges := map[string]dataUsageEdge{}
	rows := []dataUsageCSVLine{}
	addNode := func(node dataUsageNode) {
		if strings.TrimSpace(node.ID) == "" {
			return
		}
		if _, ok := nodes[node.ID]; !ok {
			nodes[node.ID] = node
		}
	}
	addEdge := func(edge dataUsageEdge) {
		if edge.From == "" || edge.To == "" {
			return
		}
		key := edge.From + "|" + edge.To + "|" + edge.Relation + "|" + edge.Role + "|" + edge.ProjectID
		if _, ok := edges[key]; !ok {
			edges[key] = edge
		}
	}
	addSubject := func(subjectType, subjectID string) string {
		subjectType = strings.ToLower(strings.TrimSpace(subjectType))
		subjectID = strings.TrimSpace(subjectID)
		if subjectType == "organization" {
			organization := organizations[subjectID]
			label := subjectID
			if organization.ID != "" {
				label = firstNonEmpty(organization.Name, organization.Alias, organization.ID)
			}
			id := "organization:" + subjectID
			addNode(dataUsageNode{ID: id, Kind: "organization", Label: label, SubLabel: subjectID})
			for _, member := range orgMembers[subjectID] {
				uid := "user:" + member.ID
				addNode(dataUsageNode{ID: uid, Kind: "user", Label: firstNonEmpty(member.Username, member.Email, member.ID), SubLabel: member.Email})
				addEdge(dataUsageEdge{From: uid, To: id, Relation: "member_of"})
			}
			return id
		}
		user := users[subjectID]
		label := subjectID
		if user.ID != "" {
			label = firstNonEmpty(user.Username, user.Email, user.ID)
		}
		id := "user:" + subjectID
		addNode(dataUsageNode{ID: id, Kind: "user", Label: label, SubLabel: user.Email})
		return id
	}

	for _, d := range datasets {
		datasetID := "dataset:" + d.ID
		addNode(dataUsageNode{ID: datasetID, Kind: "dataset", Label: firstNonEmpty(d.Name, d.ID), SubLabel: d.Bucket, Class: d.Classification})
		ownerNode := addSubject(firstNonEmpty(d.OwnerType, "user"), firstNonEmpty(d.OwnerID, d.OwnerUserID))
		addEdge(dataUsageEdge{From: ownerNode, To: datasetID, Relation: "owns", Role: "owner"})
		rows = append(rows, dataUsageCSVLine{DatasetID: d.ID, DatasetName: d.Name, DatasetClassification: d.Classification, DatasetOwnerType: d.OwnerType, DatasetOwnerID: firstNonEmpty(d.OwnerID, d.OwnerUserID), Relation: "owns", SubjectType: d.OwnerType, SubjectID: firstNonEmpty(d.OwnerID, d.OwnerUserID), SubjectName: subjectName(d.OwnerType, firstNonEmpty(d.OwnerID, d.OwnerUserID), users, organizations), Role: "owner"})

		accessItems, err := h.datasetStore.ListAccess(d.ID)
		if err != nil {
			return dataUsageReport{}, err
		}
		for _, accessItem := range accessItems {
			subjectNode := addSubject(accessItem.SubjectType, accessItem.SubjectID)
			addEdge(dataUsageEdge{From: subjectNode, To: datasetID, Relation: "dataset_permission", Role: accessItem.Role})
			rows = append(rows, datasetUsageLine(d, "dataset_permission", accessItem.SubjectType, accessItem.SubjectID, subjectName(accessItem.SubjectType, accessItem.SubjectID, users, organizations), accessItem.Role, project.Project{}, "", "", "", ""))
		}

		for _, projectID := range projectsUsingDataset(h.projectResourceStore, projects, d.ID) {
			p := projectByID[projectID]
			projectNode := "project:" + projectID
			addNode(dataUsageNode{ID: projectNode, Kind: "project", Label: firstNonEmpty(p.Name, projectID), SubLabel: projectID})
			addEdge(dataUsageEdge{From: projectNode, To: datasetID, Relation: "attached_to_project", ProjectID: projectID})
			rows = append(rows, datasetUsageLine(d, "attached_to_project", "project", projectID, firstNonEmpty(p.Name, projectID), "", p, "", "", "", ""))

			projectOwnerNode := addSubject(firstNonEmpty(p.OwnerType, "user"), p.OwnerID)
			addEdge(dataUsageEdge{From: projectOwnerNode, To: projectNode, Relation: "project_owner", Role: "owner", ProjectID: projectID})
			rows = append(rows, datasetUsageLine(d, "project_owner", firstNonEmpty(p.OwnerType, "user"), p.OwnerID, subjectName(p.OwnerType, p.OwnerID, users, organizations), "owner", p, "", "", "", ""))
			for _, role := range rolesByProject[projectID] {
				userNode := addSubject("user", role.UserID)
				addEdge(dataUsageEdge{From: userNode, To: projectNode, Relation: "project_member", Role: string(role.Role), ProjectID: projectID})
				rows = append(rows, datasetUsageLine(d, "project_member", "user", role.UserID, subjectName("user", role.UserID, users, organizations), string(role.Role), p, "", "", "", ""))
			}
			for _, workload := range workloadsByProject[projectID] {
				workloadNode := "workload:" + workload.ID
				addNode(dataUsageNode{ID: workloadNode, Kind: "workload", Label: firstNonEmpty(workload.Name, workload.ID), SubLabel: workload.Kind + " · " + workload.Status})
				addEdge(dataUsageEdge{From: workloadNode, To: projectNode, Relation: "runs_in_project", ProjectID: projectID})
				rows = append(rows, datasetUsageLine(d, "workload_project_access", "workload", workload.ID, workload.Name, "", p, workload.Kind, workload.ID, workload.Name, workload.Status))
			}
		}
	}

	nodeList := make([]dataUsageNode, 0, len(nodes))
	for _, node := range nodes {
		nodeList = append(nodeList, node)
	}
	sort.Slice(nodeList, func(i, j int) bool { return nodeList[i].Kind+nodeList[i].Label < nodeList[j].Kind+nodeList[j].Label })
	edgeList := make([]dataUsageEdge, 0, len(edges))
	for _, edge := range edges {
		edgeList = append(edgeList, edge)
	}
	sort.Slice(edgeList, func(i, j int) bool {
		return edgeList[i].Relation+edgeList[i].From+edgeList[i].To < edgeList[j].Relation+edgeList[j].From+edgeList[j].To
	})
	hds := 0
	for _, d := range datasets {
		if strings.EqualFold(d.Classification, "hds") {
			hds++
		}
	}
	report := dataUsageReport{
		GeneratedAt: time.Now().UTC(),
		Summary: dataUsageSummary{
			Datasets:      len(datasets),
			HDSDatasets:   hds,
			Projects:      len(projects),
			Users:         countNodes(nodeList, "user"),
			Organizations: countNodes(nodeList, "organization"),
			Workloads:     countNodes(nodeList, "workload"),
			Edges:         len(edgeList),
		},
		Nodes: nodeList,
		Edges: edgeList,
	}
	if includeRows {
		report.Rows = rows
	}
	return report, nil
}

type workloadUsage struct {
	ID     string
	Kind   string
	Name   string
	Status string
}

func buildWorkloadUsage(workspaces []workspace.Workspace, jobs []job.Job, apps []app.App) map[string][]workloadUsage {
	out := map[string][]workloadUsage{}
	for _, item := range workspaces {
		out[item.ProjectID] = append(out[item.ProjectID], workloadUsage{ID: item.ID, Kind: firstNonEmpty(item.Kind, "workspace"), Name: item.Name, Status: item.Status})
	}
	for _, item := range jobs {
		out[item.ProjectID] = append(out[item.ProjectID], workloadUsage{ID: item.ID, Kind: "job", Name: item.Name, Status: item.Status})
	}
	for _, item := range apps {
		out[item.ProjectID] = append(out[item.ProjectID], workloadUsage{ID: item.ID, Kind: firstNonEmpty(item.Kind, "app"), Name: item.Name, Status: item.Status})
	}
	return out
}

func (h Handlers) dataUsageIdentityMaps() (map[string]keycloak.User, map[string]keycloak.Organization, map[string][]keycloak.User) {
	users := map[string]keycloak.User{}
	organizations := map[string]keycloak.Organization{}
	orgMembers := map[string][]keycloak.User{}
	if h.keycloak == nil {
		return users, organizations, orgMembers
	}
	if items, err := h.keycloak.ListUsers(); err == nil {
		for _, item := range items {
			users[item.ID] = item
			if item.Username != "" {
				users[item.Username] = item
			}
		}
	}
	if items, err := h.keycloak.ListOrganizations(); err == nil {
		for _, item := range items {
			organizations[item.ID] = item
			members, err := h.keycloak.ListOrganizationMembers(item.ID)
			if err == nil {
				orgMembers[item.ID] = members
				for _, member := range members {
					users[member.ID] = member
					if member.Username != "" {
						users[member.Username] = member
					}
				}
			}
		}
	}
	return users, organizations, orgMembers
}

func datasetUsageLine(d dataset.Dataset, relation, subjectType, subjectID, subjectNameValue, role string, p project.Project, workloadKind, workloadID, workloadName, workloadStatus string) dataUsageCSVLine {
	return dataUsageCSVLine{DatasetID: d.ID, DatasetName: d.Name, DatasetClassification: d.Classification, DatasetOwnerType: d.OwnerType, DatasetOwnerID: firstNonEmpty(d.OwnerID, d.OwnerUserID), Relation: relation, SubjectType: subjectType, SubjectID: subjectID, SubjectName: subjectNameValue, Role: role, ProjectID: p.ID, ProjectName: p.Name, WorkloadKind: workloadKind, WorkloadID: workloadID, WorkloadName: workloadName, WorkloadStatus: workloadStatus}
}

func subjectName(subjectType, subjectID string, users map[string]keycloak.User, organizations map[string]keycloak.Organization) string {
	if strings.EqualFold(subjectType, "organization") {
		item := organizations[subjectID]
		return firstNonEmpty(item.Name, item.Alias, subjectID)
	}
	item := users[subjectID]
	return firstNonEmpty(item.Username, item.Email, subjectID)
}

func countNodes(nodes []dataUsageNode, kind string) int {
	count := 0
	for _, node := range nodes {
		if node.Kind == kind {
			count++
		}
	}
	return count
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func projectsUsingDataset(resourceStore store.ProjectResourceStore, projects []project.Project, datasetID string) []string {
	out := []string{}
	for _, p := range projects {
		ids, err := resourceStore.ListProjectDatasetIDs(p.ID)
		if err != nil {
			continue
		}
		for _, id := range ids {
			if strings.TrimSpace(id) == strings.TrimSpace(datasetID) {
				out = append(out, p.ID)
				break
			}
		}
	}
	return out
}
