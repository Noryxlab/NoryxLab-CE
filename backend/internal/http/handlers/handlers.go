package handlers

import (
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

type Handlers struct {
	projectStore          store.ProjectStore
	buildStore            store.BuildStore
	podStore              store.PodStore
	workspaceStore        store.WorkspaceStore
	accessStore           store.AccessStore
	runtime               runtime.Runner
	authVerifier          auth.Verifier
	keycloak              *keycloak.Client
	registryPullSecret    string
	registryPushSecret    string
	bootstrapAdminUser    string
	bootstrapAdminEmail   string
	workspaceJupyterImage string
	workspaceCPU          string
	workspaceMemory       string
}

type Options struct {
	RegistryPullSecret    string
	RegistryPushSecret    string
	BootstrapAdminUser    string
	BootstrapAdminEmail   string
	WorkspaceJupyterImage string
	WorkspaceCPU          string
	WorkspaceMemory       string
}

func New(
	projectStore store.ProjectStore,
	buildStore store.BuildStore,
	podStore store.PodStore,
	workspaceStore store.WorkspaceStore,
	accessStore store.AccessStore,
	runtime runtime.Runner,
	authVerifier auth.Verifier,
	keycloakClient *keycloak.Client,
	options Options,
) Handlers {
	return Handlers{
		projectStore:          projectStore,
		buildStore:            buildStore,
		podStore:              podStore,
		workspaceStore:        workspaceStore,
		accessStore:           accessStore,
		runtime:               runtime,
		authVerifier:          authVerifier,
		keycloak:              keycloakClient,
		registryPullSecret:    options.RegistryPullSecret,
		registryPushSecret:    options.RegistryPushSecret,
		bootstrapAdminUser:    options.BootstrapAdminUser,
		bootstrapAdminEmail:   options.BootstrapAdminEmail,
		workspaceJupyterImage: options.WorkspaceJupyterImage,
		workspaceCPU:          options.WorkspaceCPU,
		workspaceMemory:       options.WorkspaceMemory,
	}
}
