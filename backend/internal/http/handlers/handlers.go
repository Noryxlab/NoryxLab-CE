package handlers

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/store"

type Handlers struct {
    projectStore store.ProjectStore
}

func New(projectStore store.ProjectStore) Handlers {
    return Handlers{projectStore: projectStore}
}
