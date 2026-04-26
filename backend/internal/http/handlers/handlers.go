package handlers

import (
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

type Handlers struct {
	projectStore                  store.ProjectStore
	buildStore                    store.BuildStore
	podStore                      store.PodStore
	workspaceStore                store.WorkspaceStore
	sessionStore                  store.SessionStore
	accessStore                   store.AccessStore
	runtime                       runtime.Runner
	authVerifier                  auth.Verifier
	keycloak                      *keycloak.Client
	registryPullSecret            string
	registryPushSecret            string
	bootstrapAdminUser            string
	bootstrapAdminEmail           string
	workspaceJupyterImage         string
	workspaceVSCodeImage          string
	workspaceNamespace            string
	workspaceCPU                  string
	workspaceMemory               string
	workspacePVCEnabled           bool
	workspacePVCClass             string
	workspacePVCSize              string
	workspacePVCAccessMode        string
	workspacePVCMountPath         string
	workspaceProfilePVCEnabled    bool
	workspaceProfilePVCClass      string
	workspaceProfilePVCSize       string
	workspaceProfilePVCAccessMode string
	workspaceProfilePVCMountPath  string
}

type Options struct {
	RegistryPullSecret            string
	RegistryPushSecret            string
	BootstrapAdminUser            string
	BootstrapAdminEmail           string
	WorkspaceJupyterImage         string
	WorkspaceVSCodeImage          string
	WorkspaceNamespace            string
	WorkspaceCPU                  string
	WorkspaceMemory               string
	WorkspacePVCEnabled           bool
	WorkspacePVCClass             string
	WorkspacePVCSize              string
	WorkspacePVCAccessMode        string
	WorkspacePVCMountPath         string
	WorkspaceProfilePVCEnabled    bool
	WorkspaceProfilePVCClass      string
	WorkspaceProfilePVCSize       string
	WorkspaceProfilePVCAccessMode string
	WorkspaceProfilePVCMountPath  string
}

func New(
	projectStore store.ProjectStore,
	buildStore store.BuildStore,
	podStore store.PodStore,
	workspaceStore store.WorkspaceStore,
	sessionStore store.SessionStore,
	accessStore store.AccessStore,
	runtime runtime.Runner,
	authVerifier auth.Verifier,
	keycloakClient *keycloak.Client,
	options Options,
) Handlers {
	return Handlers{
		projectStore:                  projectStore,
		buildStore:                    buildStore,
		podStore:                      podStore,
		workspaceStore:                workspaceStore,
		sessionStore:                  sessionStore,
		accessStore:                   accessStore,
		runtime:                       runtime,
		authVerifier:                  authVerifier,
		keycloak:                      keycloakClient,
		registryPullSecret:            options.RegistryPullSecret,
		registryPushSecret:            options.RegistryPushSecret,
		bootstrapAdminUser:            options.BootstrapAdminUser,
		bootstrapAdminEmail:           options.BootstrapAdminEmail,
		workspaceJupyterImage:         options.WorkspaceJupyterImage,
		workspaceVSCodeImage:          options.WorkspaceVSCodeImage,
		workspaceNamespace:            options.WorkspaceNamespace,
		workspaceCPU:                  options.WorkspaceCPU,
		workspaceMemory:               options.WorkspaceMemory,
		workspacePVCEnabled:           options.WorkspacePVCEnabled,
		workspacePVCClass:             options.WorkspacePVCClass,
		workspacePVCSize:              options.WorkspacePVCSize,
		workspacePVCAccessMode:        options.WorkspacePVCAccessMode,
		workspacePVCMountPath:         options.WorkspacePVCMountPath,
		workspaceProfilePVCEnabled:    options.WorkspaceProfilePVCEnabled,
		workspaceProfilePVCClass:      options.WorkspaceProfilePVCClass,
		workspaceProfilePVCSize:       options.WorkspaceProfilePVCSize,
		workspaceProfilePVCAccessMode: options.WorkspaceProfilePVCAccessMode,
		workspaceProfilePVCMountPath:  options.WorkspaceProfilePVCMountPath,
	}
}
