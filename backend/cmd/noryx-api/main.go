package main

import (
    "log"

    "github.com/Noryxlab/NoryxLab-CE/backend/internal/config"
    nhttp "github.com/Noryxlab/NoryxLab-CE/backend/internal/http"
    "github.com/Noryxlab/NoryxLab-CE/backend/internal/http/handlers"
    "github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

func main() {
    cfg := config.Load()

    projectStore := memory.NewProjectStore()
    h := handlers.New(projectStore)

    srv := nhttp.NewServer(cfg, h)

    log.Printf("noryx-api listening on %s", cfg.ListenAddr)
    if err := srv.ListenAndServe(); err != nil {
        log.Fatal(err)
    }
}
