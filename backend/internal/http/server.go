package http

import (
    "net/http"

    "github.com/Noryxlab/NoryxLab-CE/backend/internal/config"
    "github.com/Noryxlab/NoryxLab-CE/backend/internal/http/handlers"
)

func NewServer(cfg config.Config, h handlers.Handlers) *http.Server {
    mux := http.NewServeMux()

    mux.HandleFunc("GET /healthz", h.GetHealth)
    mux.HandleFunc("GET /api/v1/projects", h.ListProjects)
    mux.HandleFunc("POST /api/v1/projects", h.CreateProject)

    return &http.Server{
        Addr:    cfg.ListenAddr,
        Handler: mux,
    }
}
