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
	mux.HandleFunc("PUT /api/v1/projects/{projectID}/members/{userID}/role", h.SetProjectMemberRole)
	mux.HandleFunc("GET /api/v1/builds", h.ListBuilds)
	mux.HandleFunc("POST /api/v1/builds", h.CreateBuild)
	mux.HandleFunc("GET /api/v1/pods", h.ListPods)
	mux.HandleFunc("POST /api/v1/pods", h.LaunchPod)
	mux.HandleFunc("GET /api/v1/workspaces", h.ListWorkspaces)
	mux.HandleFunc("POST /api/v1/workspaces", h.CreateWorkspace)
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
