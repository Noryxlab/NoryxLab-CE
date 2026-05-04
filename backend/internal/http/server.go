package http

import (
	"net/http"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/config"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/http/handlers"
)

func NewServer(cfg config.Config, h handlers.Handlers) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", GetHome)
	mux.HandleFunc("GET /healthz", h.GetHealth)
	mux.HandleFunc("GET /api/v1/projects", h.ListProjects)
	mux.HandleFunc("POST /api/v1/projects", h.CreateProject)
	mux.HandleFunc("DELETE /api/v1/projects/{projectID}", h.DeleteProject)
	mux.HandleFunc("PUT /api/v1/projects/{projectID}/members/{userID}/role", h.SetProjectMemberRole)
	mux.HandleFunc("GET /api/v1/builds", h.ListBuilds)
	mux.HandleFunc("POST /api/v1/builds", h.CreateBuild)
	mux.HandleFunc("GET /api/v1/apps", h.ListApps)
	mux.HandleFunc("POST /api/v1/apps", h.CreateApp)
	mux.HandleFunc("DELETE /api/v1/apps/{appID}", h.DeleteApp)
	mux.HandleFunc("GET /api/v1/builds/{buildID}/dockerfile", h.GetBuildDockerfile)
	mux.HandleFunc("GET /api/v1/environments", h.ListEnvironments)
	mux.HandleFunc("GET /api/v1/pods", h.ListPods)
	mux.HandleFunc("POST /api/v1/pods", h.LaunchPod)
	mux.HandleFunc("GET /api/v1/workspaces", h.ListWorkspaces)
	mux.HandleFunc("POST /api/v1/workspaces", h.CreateWorkspace)
	mux.HandleFunc("DELETE /api/v1/workspaces/{workspaceID}", h.DeleteWorkspace)
	mux.HandleFunc("GET /api/v1/jobs", h.ListJobs)
	mux.HandleFunc("POST /api/v1/jobs", h.CreateJob)
	mux.HandleFunc("GET /api/v1/jobs/{jobID}/logs", h.GetJobLogs)
	mux.HandleFunc("DELETE /api/v1/jobs/{jobID}", h.DeleteJob)
	mux.HandleFunc("GET /api/v1/secrets", h.ListSecrets)
	mux.HandleFunc("POST /api/v1/secrets", h.UpsertSecret)
	mux.HandleFunc("GET /api/v1/secrets/{name}", h.GetSecret)
	mux.HandleFunc("DELETE /api/v1/secrets/{name}", h.DeleteSecret)
	mux.HandleFunc("GET /api/v1/datasets", h.ListDatasets)
	mux.HandleFunc("POST /api/v1/datasets", h.CreateDataset)
	mux.HandleFunc("DELETE /api/v1/datasets/{datasetID}", h.DeleteDataset)
	mux.HandleFunc("PUT /api/v1/datasets/{datasetID}/objects/{path...}", h.PutDatasetObject)
	mux.HandleFunc("GET /api/v1/datasources", h.ListDatasources)
	mux.HandleFunc("POST /api/v1/datasources", h.CreateDatasource)
	mux.HandleFunc("DELETE /api/v1/datasources/{datasourceID}", h.DeleteDatasource)
	mux.HandleFunc("POST /api/v1/datasources/{datasourceID}/validate", h.ValidateDatasource)
	mux.HandleFunc("GET /api/v1/repositories", h.ListRepositories)
	mux.HandleFunc("POST /api/v1/repositories", h.CreateRepository)
	mux.HandleFunc("POST /api/v1/repositories/{repositoryID}/validate", h.ValidateRepository)
	mux.HandleFunc("DELETE /api/v1/repositories/{repositoryID}", h.DeleteRepository)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/datasets", h.ListProjectDatasets)
	mux.HandleFunc("PUT /api/v1/projects/{projectID}/datasets/{datasetID}", h.AttachProjectDataset)
	mux.HandleFunc("DELETE /api/v1/projects/{projectID}/datasets/{datasetID}", h.DetachProjectDataset)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/datasources", h.ListProjectDatasources)
	mux.HandleFunc("PUT /api/v1/projects/{projectID}/datasources/{datasourceID}", h.AttachProjectDatasource)
	mux.HandleFunc("DELETE /api/v1/projects/{projectID}/datasources/{datasourceID}", h.DetachProjectDatasource)
	mux.HandleFunc("GET /api/v1/projects/{projectID}/repositories", h.ListProjectRepositories)
	mux.HandleFunc("PUT /api/v1/projects/{projectID}/repositories/{repositoryID}", h.AttachProjectRepository)
	mux.HandleFunc("DELETE /api/v1/projects/{projectID}/repositories/{repositoryID}", h.DetachProjectRepository)
	mux.HandleFunc("POST /api/v1/auth/session", h.CreateWebSession)
	mux.HandleFunc("DELETE /api/v1/auth/session", h.DeleteWebSession)
	// Workspace reverse-proxy must support all HTTP methods used by Jupyter.
	for _, method := range []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
		mux.HandleFunc(method+" /workspaces/{workspaceID}", h.ProxyWorkspace)
		mux.HandleFunc(method+" /workspaces/{workspaceID}/{path...}", h.ProxyWorkspace)
		mux.HandleFunc(method+" /apps/{slug}", h.ProxyApp)
		mux.HandleFunc(method+" /apps/{slug}/{path...}", h.ProxyApp)
	}
	mux.HandleFunc("GET /api/v1/admin/users", h.ListUsers)
	mux.HandleFunc("GET /api/v1/admin/modules", h.GetModulesStatus)

	mux.HandleFunc("GET /swagger", GetSwaggerUI)
	mux.HandleFunc("GET /swagger/", GetSwaggerUI)
	mux.HandleFunc("GET /swagger/openapi.yaml", GetOpenAPI)

	return &http.Server{
		Addr:    cfg.ListenAddr,
		Handler: mux,
	}
}
