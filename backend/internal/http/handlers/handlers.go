package handlers

import (
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

type Handlers struct {
	projectStore       store.ProjectStore
	buildStore         store.BuildStore
	podStore           store.PodStore
	accessStore        store.AccessStore
	runtime            runtime.Runner
	registryPullSecret string
	registryPushSecret string
}

type Options struct {
	RegistryPullSecret string
	RegistryPushSecret string
}

func New(
	projectStore store.ProjectStore,
	buildStore store.BuildStore,
	podStore store.PodStore,
	accessStore store.AccessStore,
	runtime runtime.Runner,
	options Options,
) Handlers {
	return Handlers{
		projectStore:       projectStore,
		buildStore:         buildStore,
		podStore:           podStore,
		accessStore:        accessStore,
		runtime:            runtime,
		registryPullSecret: options.RegistryPullSecret,
		registryPushSecret: options.RegistryPushSecret,
	}
}
