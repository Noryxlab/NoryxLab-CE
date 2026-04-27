package handlers

import (
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/edition"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
	"github.com/minio/minio-go/v7"
)

type Handlers struct {
	projectStore                  store.ProjectStore
	buildStore                    store.BuildStore
	podStore                      store.PodStore
	workspaceStore                store.WorkspaceStore
	sessionStore                  store.SessionStore
	accessStore                   store.AccessStore
	secretStore                   store.SecretStore
	datasetStore                  store.DatasetStore
	repositoryStore               store.RepositoryStore
	projectResourceStore          store.ProjectResourceStore
	runtime                       runtime.Runner
	authVerifier                  auth.Verifier
	keycloak                      *keycloak.Client
	minioClient                   *minio.Client
	minioRegion                   string
	secretsMasterKey              string
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
	editionHooks                  edition.Hooks
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
	SecretsMasterKey              string
	MinIOClient                   *minio.Client
	MinIORegion                   string
	EditionHooks                  *edition.Hooks
}

func New(
	projectStore store.ProjectStore,
	buildStore store.BuildStore,
	podStore store.PodStore,
	workspaceStore store.WorkspaceStore,
	sessionStore store.SessionStore,
	accessStore store.AccessStore,
	secretStore store.SecretStore,
	datasetStore store.DatasetStore,
	repositoryStore store.RepositoryStore,
	projectResourceStore store.ProjectResourceStore,
	runtime runtime.Runner,
	authVerifier auth.Verifier,
	keycloakClient *keycloak.Client,
	options Options,
) Handlers {
	hooks := edition.DefaultHooks()
	if options.EditionHooks != nil {
		if options.EditionHooks.RBAC != nil {
			hooks.RBAC = options.EditionHooks.RBAC
		}
		if options.EditionHooks.Feature != nil {
			hooks.Feature = options.EditionHooks.Feature
		}
		if options.EditionHooks.Audit != nil {
			hooks.Audit = options.EditionHooks.Audit
		}
	}

	return Handlers{
		projectStore:                  projectStore,
		buildStore:                    buildStore,
		podStore:                      podStore,
		workspaceStore:                workspaceStore,
		sessionStore:                  sessionStore,
		accessStore:                   accessStore,
		secretStore:                   secretStore,
		datasetStore:                  datasetStore,
		repositoryStore:               repositoryStore,
		projectResourceStore:          projectResourceStore,
		runtime:                       runtime,
		authVerifier:                  authVerifier,
		keycloak:                      keycloakClient,
		minioClient:                   options.MinIOClient,
		minioRegion:                   options.MinIORegion,
		secretsMasterKey:              options.SecretsMasterKey,
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
		editionHooks:                  hooks,
	}
}
