package http

import (
	"net/http"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/config"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/http/handlers"
)

func NewServer(cfg config.Config, h handlers.Handlers) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", GetHome)

	// An unknown /api path must not fall into the home page. `GET /` is the
	// catch-all, so before this a missing API route answered 200 with HTML:
	// alive in a browser, undebuggable from a client, and precisely what hid
	// the Enterprise routes going unregistered for a whole release. More
	// specific patterns still win, so every real route is unaffected.
	//
	// Scoped to GET because a bare "/api/" would be ambiguous against "GET /"
	// and the mux refuses to register it. An unknown POST therefore gets 405
	// rather than 404 - still an API-shaped answer, never the application.
	mux.HandleFunc("GET /api/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"no such endpoint"}` + "\n"))
	})
	mux.HandleFunc("GET /healthz", h.GetHealth)
	mux.HandleFunc("GET /api/v1/version", h.GetVersion)
	mux.HandleFunc("GET /api/v1/platform/overview", h.GetPlatformOverview)
	mux.HandleFunc("GET /api/v1/hardware-tiers", h.GetHardwareTiers)
	mux.HandleFunc("GET /api/v1/admin/smtp", h.GetSMTPSettings)
	mux.HandleFunc("PUT /api/v1/admin/smtp", h.UpdateSMTPSettings)
	mux.HandleFunc("POST /api/v1/admin/smtp/tests", h.TestSMTPSettings)
	mux.HandleFunc("GET /api/v1/admin/hardware-tiers", h.ListAdminHardwareTiers)
	mux.HandleFunc("POST /api/v1/admin/hardware-tiers", h.SaveHardwareTier)
	mux.HandleFunc("PUT /api/v1/admin/hardware-tiers/{tierID}", h.SaveHardwareTier)
	mux.HandleFunc("DELETE /api/v1/admin/hardware-tiers/{tierID}", h.DeleteHardwareTier)
	mux.HandleFunc("GET /api/v1/user/preferences", h.GetUserPreferences)
	mux.HandleFunc("GET /api/v1/organizations", h.ListAvailableOrganizations)
	mux.HandleFunc("PUT /api/v1/user/preferences", h.UpdateUserPreferences)
	mux.HandleFunc("GET /api/v1/search", h.Search)
	mux.HandleFunc("GET /api/v1/projects", h.ListProjects)
	mux.HandleFunc("POST /api/v1/projects", h.CreateProject)
	mux.HandleFunc("PUT /api/v1/projects/{projectID}", h.UpdateProjectMetadata)
	mux.HandleFunc("DELETE /api/v1/projects/{projectID}", h.DeleteProject)
	mux.HandleFunc("PUT /api/v1/projects/{projectID}/ownership", h.UpdateProjectOwner)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/files", h.ProxyProjectFiles)
	mux.HandleFunc("POST /api/v1/projects/{projectID}/folders", h.ProxyProjectFiles)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/files/{path...}", h.ProxyProjectFiles)
	mux.HandleFunc("PUT /api/v1/projects/{projectID}/files/{path...}", h.ProxyProjectFiles)
	mux.HandleFunc("DELETE /api/v1/projects/{projectID}/files/{path...}", h.ProxyProjectFiles)
	mux.HandleFunc("PUT /api/v1/projects/{projectID}/members/{userID}/role", h.SetProjectMemberRole)
	mux.HandleFunc("POST /api/v1/projects/{projectID}/invitations", h.InviteProjectMember)
	mux.HandleFunc("GET /api/v1/builds", h.ListBuilds)
	mux.HandleFunc("POST /api/v1/builds", h.CreateBuild)
	mux.HandleFunc("DELETE /api/v1/builds/{buildID}", h.DeleteBuild)
	mux.HandleFunc("GET /api/v1/apps", h.ListApps)
	mux.HandleFunc("POST /api/v1/apps", h.CreateApp)
	mux.HandleFunc("GET /api/v1/apps/{appID}/logs", h.GetAppLogs)
	mux.HandleFunc("GET /api/v1/apps/{appID}/usage", h.GetAppUsage)
	mux.HandleFunc("POST /api/v1/apps/{appID}/publish", h.PublishApp)
	mux.HandleFunc("GET /api/v1/apps/{appID}/revisions", h.ListAppRevisions)
	mux.HandleFunc("POST /api/v1/apps/{appID}/revisions/{revisionID}/rollback", h.RollbackAppRevision)
	mux.HandleFunc("POST /api/v1/apps/{appID}/restart", h.RestartApp)
	mux.HandleFunc("POST /api/v1/apps/{appID}/stop", h.StopApp)
	mux.HandleFunc("DELETE /api/v1/apps/{appID}", h.DeleteApp)
	mux.HandleFunc("GET /api/v1/production/apps", h.ListProductionApps)
	mux.HandleFunc("GET /api/v1/dashboards", h.ListDashboards)
	mux.HandleFunc("POST /api/v1/dashboards", h.CreateDashboard)
	mux.HandleFunc("DELETE /api/v1/dashboards/{dashboardID}", h.DeleteDashboard)
	mux.HandleFunc("GET /api/v1/builds/{buildID}/dockerfile", h.GetBuildDockerfile)
	mux.HandleFunc("GET /api/v1/environments", h.ListEnvironments)
	mux.HandleFunc("DELETE /api/v1/environments/{environmentID}", h.DeleteEnvironment)
	mux.HandleFunc("GET /api/v1/pods", h.ListPods)
	mux.HandleFunc("POST /api/v1/pods", h.LaunchPod)
	mux.HandleFunc("GET /api/v1/workspaces", h.ListWorkspaces)
	mux.HandleFunc("POST /api/v1/workspaces", h.CreateWorkspace)
	mux.HandleFunc("DELETE /api/v1/workspaces/{workspaceID}", h.DeleteWorkspace)
	mux.HandleFunc("GET /api/v1/jobs", h.ListJobs)
	mux.HandleFunc("POST /api/v1/jobs", h.CreateJob)
	mux.HandleFunc("GET /api/v1/jobs/{jobID}/logs", h.GetJobLogs)
	mux.HandleFunc("DELETE /api/v1/jobs/{jobID}", h.DeleteJob)
	mux.HandleFunc("GET /api/v1/cronjobs", h.ListCronJobs)
	mux.HandleFunc("POST /api/v1/cronjobs", h.CreateCronJob)
	mux.HandleFunc("DELETE /api/v1/cronjobs/{cronJobID}", h.DeleteCronJob)
	mux.HandleFunc("GET /api/v1/secrets", h.ListSecrets)
	mux.HandleFunc("POST /api/v1/secrets", h.UpsertSecret)
	mux.HandleFunc("GET /api/v1/secrets/{name}", h.GetSecret)
	mux.HandleFunc("DELETE /api/v1/secrets/{name}", h.DeleteSecret)
	mux.HandleFunc("GET /api/v1/datasets", h.ListDatasets)
	mux.HandleFunc("POST /api/v1/datasets", h.CreateDataset)
	mux.HandleFunc("PUT /api/v1/datasets/{datasetID}", h.UpdateDatasetMetadata)
	mux.HandleFunc("DELETE /api/v1/datasets/{datasetID}", h.DeleteDataset)
	mux.HandleFunc("GET /api/v1/datasets/{datasetID}/objects", h.ListDatasetObjects)
	mux.HandleFunc("POST /api/v1/datasets/{datasetID}/folders", h.CreateDatasetFolder)
	mux.HandleFunc("GET /api/v1/datasets/{datasetID}/objects/{path...}", h.GetDatasetObject)
	mux.HandleFunc("PUT /api/v1/datasets/{datasetID}/objects/{path...}", h.PutDatasetObject)
	mux.HandleFunc("DELETE /api/v1/datasets/{datasetID}/objects/{path...}", h.DeleteDatasetObject)
	mux.HandleFunc("POST /api/v1/datasets/{datasetID}/download-url", h.CreateDatasetObjectDownloadURL)
	mux.HandleFunc("POST /api/v1/datasets/{datasetID}/download", h.DownloadDatasetObjects)
	mux.HandleFunc("GET /api/v1/datasets/{datasetID}/access", h.ListDatasetAccess)
	mux.HandleFunc("PUT /api/v1/datasets/{datasetID}/ownership", h.UpdateDatasetOwner)
	mux.HandleFunc("PUT /api/v1/datasets/{datasetID}/access/{subjectType}/{subjectID}", h.SetDatasetAccess)
	mux.HandleFunc("DELETE /api/v1/datasets/{datasetID}/access/{subjectType}/{subjectID}", h.DeleteDatasetAccess)
	mux.HandleFunc("PUT /api/v1/datasets/{datasetID}/access/{userID}", h.SetDatasetAccess)
	mux.HandleFunc("DELETE /api/v1/datasets/{datasetID}/access/{userID}", h.DeleteDatasetAccess)
	mux.HandleFunc("GET /api/v1/datasources", h.ListDatasources)
	mux.HandleFunc("GET /api/v1/datasource-definitions", h.ListDatasourceDefinitions)
	mux.HandleFunc("POST /api/v1/dataservices", h.CreateDataService)
	mux.HandleFunc("POST /api/v1/datasources", h.CreateDatasource)
	mux.HandleFunc("DELETE /api/v1/datasources/{datasourceID}", h.DeleteDatasource)
	mux.HandleFunc("POST /api/v1/datasources/{datasourceID}/validate", h.ValidateDatasource)
	mux.HandleFunc("GET /api/v1/datasources/{datasourceID}/logs", h.GetDataServiceLogs)
	mux.HandleFunc("POST /api/v1/datasources/{datasourceID}/restart", h.RestartDataService)
	mux.HandleFunc("GET /api/v1/repositories", h.ListRepositories)
	mux.HandleFunc("POST /api/v1/repositories", h.CreateRepository)
	mux.HandleFunc("PUT /api/v1/repositories/{repositoryID}", h.UpdateRepository)
	mux.HandleFunc("POST /api/v1/repositories/{repositoryID}/validate", h.ValidateRepository)
	mux.HandleFunc("DELETE /api/v1/repositories/{repositoryID}", h.DeleteRepository)
	mux.HandleFunc("GET /api/v1/ontologies", h.ListOntologies)
	mux.HandleFunc("POST /api/v1/ontologies/{ontologyID}/query", h.QueryOntology)
	mux.HandleFunc("PUT /api/v1/ontologies/{ontologyID}", h.UpdateOntologyMetadata)
	mux.HandleFunc("DELETE /api/v1/ontologies/{ontologyID}", h.DeleteOntology)
	mux.HandleFunc("GET /api/v1/ontologies/{ontologyID}/access", h.ListOntologyAccess)
	mux.HandleFunc("PUT /api/v1/ontologies/{ontologyID}/access/{subjectType}/{subjectID}", h.SetOntologyAccess)
	mux.HandleFunc("DELETE /api/v1/ontologies/{ontologyID}/access/{subjectType}/{subjectID}", h.DeleteOntologyAccess)
	mux.HandleFunc("PUT /api/v1/ontologies/{ontologyID}/ownership", h.UpdateOntologyOwner)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/datasets", h.ListProjectDatasets)
	mux.HandleFunc("PUT /api/v1/projects/{projectID}/datasets/{datasetID}", h.AttachProjectDataset)
	mux.HandleFunc("DELETE /api/v1/projects/{projectID}/datasets/{datasetID}", h.DetachProjectDataset)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/ontology", h.GetProjectOntology)
	mux.HandleFunc("POST /api/v1/projects/{projectID}/ontology/scans", h.ScanProjectOntology)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/ontologies", h.ListProjectOntologies)
	mux.HandleFunc("PUT /api/v1/projects/{projectID}/ontologies/{ontologyID}", h.AttachProjectOntology)
	mux.HandleFunc("DELETE /api/v1/projects/{projectID}/ontologies/{ontologyID}", h.DetachProjectOntology)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/datasources", h.ListProjectDatasources)
	mux.HandleFunc("PUT /api/v1/projects/{projectID}/datasources/{datasourceID}", h.AttachProjectDatasource)
	mux.HandleFunc("DELETE /api/v1/projects/{projectID}/datasources/{datasourceID}", h.DetachProjectDatasource)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/repositories", h.ListProjectRepositories)
	mux.HandleFunc("PUT /api/v1/projects/{projectID}/repositories/{repositoryID}", h.AttachProjectRepository)
	mux.HandleFunc("DELETE /api/v1/projects/{projectID}/repositories/{repositoryID}", h.DetachProjectRepository)
	mux.HandleFunc("GET /api/v1/auth/login", h.DirectLogin)
	mux.HandleFunc("POST /api/v1/auth/session", h.CreateWebSession)
	mux.HandleFunc("DELETE /api/v1/auth/session", h.DeleteWebSession)
	// Workspace reverse-proxy must support all HTTP methods used by Jupyter.
	for _, method := range []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
		mux.HandleFunc(method+" /workspaces/{workspaceID}", h.ProxyWorkspace)
		mux.HandleFunc(method+" /workspaces/{workspaceID}/{path...}", h.ProxyWorkspace)
		mux.HandleFunc(method+" /apps/{slug}", h.ProxyApp)
		mux.HandleFunc(method+" /apps/{slug}/{path...}", h.ProxyApp)
		mux.HandleFunc(method+" /dashboards/{slug}", h.ProxyApp)
		mux.HandleFunc(method+" /dashboards/{slug}/{path...}", h.ProxyApp)
	}
	mux.HandleFunc("GET /api/v1/admin/users", h.ListUsers)
	mux.HandleFunc("POST /api/v1/admin/users", h.CreateUserAccount)
	mux.HandleFunc("POST /api/v1/admin/users/{userID}/password", h.ResetUserPassword)
	mux.HandleFunc("POST /api/v1/admin/users/{userID}/password-reset-email", h.SendUserPasswordResetEmail)
	mux.HandleFunc("GET /api/v1/admin/modules", h.GetModulesStatus)
	mux.HandleFunc("GET /api/v1/admin/executions", h.ListAdminExecutions)
	mux.HandleFunc("DELETE /api/v1/admin/executions/{kind}/{executionID}", h.StopAdminExecution)
	mux.HandleFunc("GET /api/v1/admin/overview", h.GetAdminOverview)
	mux.HandleFunc("GET /api/v1/admin/health", h.GetPlatformHealth)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/organization-roles", h.ListProjectOrganizationRoles)
	mux.HandleFunc("PUT /api/v1/projects/{projectID}/organization-roles/{organizationID}", h.SetProjectOrganizationRole)
	mux.HandleFunc("DELETE /api/v1/projects/{projectID}/organization-roles/{organizationID}", h.DeleteProjectOrganizationRole)
	mux.HandleFunc("GET /api/v1/user/api-tokens", h.ListAPITokens)
	mux.HandleFunc("POST /api/v1/user/api-tokens", h.CreateAPIToken)
	mux.HandleFunc("DELETE /api/v1/user/api-tokens/{tokenID}", h.DeleteAPIToken)
	mux.HandleFunc("GET /api/v1/admin/software-inventory", h.GetSoftwareInventory)
	mux.HandleFunc("GET /api/v1/admin/software-inventory.csv", h.ExportSoftwareInventoryCSV)
	mux.HandleFunc("GET /api/v1/admin/health/history", h.GetPlatformHealthHistory)
	mux.HandleFunc("GET /api/v1/admin/settings", h.ListPlatformSettings)
	mux.HandleFunc("PUT /api/v1/admin/settings/{key}", h.UpdatePlatformSetting)
	mux.HandleFunc("GET /api/v1/admin/inventory", h.GetAdminInventory)
	mux.HandleFunc("GET /api/v1/admin/data-usage", h.GetAdminDataUsage)
	mux.HandleFunc("GET /api/v1/admin/data-usage.csv", h.ExportAdminDataUsageCSV)
	mux.HandleFunc("GET /api/v1/admin/rbac-matrix", h.GetAdminRBACMatrix)
	mux.HandleFunc("GET /api/v1/admin/rbac-matrix.csv", h.ExportAdminRBACMatrixCSV)
	mux.HandleFunc("GET /api/v1/admin/rbac-policy", h.GetAdminRBACPolicy)
	mux.HandleFunc("PUT /api/v1/admin/rbac-policy", h.UpdateAdminRBACPolicy)
	mux.HandleFunc("GET /api/v1/admin/storage-endpoints", h.ListAdminStorageEndpoints)
	mux.HandleFunc("POST /api/v1/admin/storage-endpoints", h.CreateAdminStorageEndpoint)
	mux.HandleFunc("PUT /api/v1/admin/storage-endpoints/{endpointID}", h.UpdateAdminStorageEndpoint)
	mux.HandleFunc("DELETE /api/v1/admin/storage-endpoints/{endpointID}", h.DeleteAdminStorageEndpoint)
	mux.HandleFunc("POST /api/v1/admin/storage-endpoints/{endpointID}/test", h.TestAdminStorageEndpoint)
	mux.HandleFunc("GET /api/v1/admin/organizations", h.ListOrganizations)
	mux.HandleFunc("POST /api/v1/admin/organizations", h.CreateOrganization)
	mux.HandleFunc("DELETE /api/v1/admin/organizations/{organizationID}", h.DeleteOrganization)
	mux.HandleFunc("GET /api/v1/admin/organizations/{organizationID}/members", h.ListOrganizationMembers)
	mux.HandleFunc("PUT /api/v1/admin/organizations/{organizationID}/members/{userID}", h.AddOrganizationMember)
	mux.HandleFunc("DELETE /api/v1/admin/organizations/{organizationID}/members/{userID}", h.RemoveOrganizationMember)

	// The Enterprise surfaces, if this binary has them. In a Community build
	// this call registers nothing, because the implementation it resolves to is
	// the stub: the routes are absent rather than forbidden.
	//
	// The call site is Community and the implementation is Enterprise, which is
	// the seam working as intended. Forgetting this line once already made every
	// Enterprise route fall through to the SPA and return the application's HTML
	// with a 200 - a shape that looks alive from a browser and is dead to every
	// client.
	registerEnterpriseRoutes(mux, h)

	mux.HandleFunc("GET /swagger", GetSwaggerUI)
	mux.HandleFunc("GET /swagger/", GetSwaggerUI)
	mux.HandleFunc("GET /swagger/openapi.yaml", GetOpenAPI)

	return &http.Server{
		Addr: cfg.ListenAddr,
		// Scope enforcement sits inside the audit middleware so a refusal is
		// recorded like any other outcome: "this token tried and was stopped"
		// is exactly what an audit is for.
		Handler: h.AuditMutations(h.EnforceTokenScopes(mux)),
	}
}
