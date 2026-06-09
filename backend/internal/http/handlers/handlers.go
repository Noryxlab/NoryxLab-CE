package handlers

import (
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/edition"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
	"github.com/minio/minio-go/v7"
)

type Handlers struct {
	projectStore                     store.ProjectStore
	appStore                         store.AppStore
	buildStore                       store.BuildStore
	jobStore                         store.JobStore
	podStore                         store.PodStore
	workspaceStore                   store.WorkspaceStore
	sessionStore                     store.SessionStore
	auditStore                       store.AuditStore
	accessStore                      store.AccessStore
	secretStore                      store.SecretStore
	datasetStore                     store.DatasetStore
	datasourceStore                  store.DatasourceStore
	repositoryStore                  store.RepositoryStore
	projectResourceStore             store.ProjectResourceStore
	userPreferenceStore              store.UserPreferenceStore
	runtime                          runtime.Runner
	authVerifier                     auth.Verifier
	keycloak                         *keycloak.Client
	minioClient                      *minio.Client
	minioEndpoint                    string
	minioAccessKey                   string
	minioSecretKey                   string
	minioUseSSL                      bool
	minioRegion                      string
	secretsMasterKey                 string
	registryPullSecret               string
	registryPushSecret               string
	bootstrapAdminUser               string
	bootstrapAdminEmail              string
	organizationRequired             bool
	workspaceJupyterImage            string
	workspaceVSCodeImage             string
	workspaceRStudioImage            string
	workspaceNamespace               string
	workspaceCPU                     string
	workspaceCPURequest              string
	workspaceMemory                  string
	workspaceEphemeralStorageRequest string
	workspaceEphemeralStorageLimit   string
	workspacePVCEnabled              bool
	workspacePVCClass                string
	workspacePVCSize                 string
	workspacePVCAccessMode           string
	workspacePVCMountPath            string
	workspaceProfilePVCEnabled       bool
	workspaceProfilePVCClass         string
	workspaceProfilePVCSize          string
	workspaceProfilePVCAccessMode    string
	workspaceProfilePVCMountPath     string
	hardwareTiers                    []hardwareTier
	backendVersion                   string
	edition                          string
	defaultTheme                     string
	editionHooks                     edition.Hooks
	harborURL                        string
	harborUsername                   string
	harborPassword                   string
	harborInsecureSkipVerify         bool
}

type Options struct {
	RegistryPullSecret               string
	RegistryPushSecret               string
	BootstrapAdminUser               string
	BootstrapAdminEmail              string
	OrganizationRequired             bool
	WorkspaceJupyterImage            string
	WorkspaceVSCodeImage             string
	WorkspaceRStudioImage            string
	WorkspaceNamespace               string
	WorkspaceCPU                     string
	WorkspaceCPURequest              string
	WorkspaceMemory                  string
	WorkspaceEphemeralStorageRequest string
	WorkspaceEphemeralStorageLimit   string
	WorkspacePVCEnabled              bool
	WorkspacePVCClass                string
	WorkspacePVCSize                 string
	WorkspacePVCAccessMode           string
	WorkspacePVCMountPath            string
	WorkspaceProfilePVCEnabled       bool
	WorkspaceProfilePVCClass         string
	WorkspaceProfilePVCSize          string
	WorkspaceProfilePVCAccessMode    string
	WorkspaceProfilePVCMountPath     string
	BackendVersion                   string
	Edition                          string
	DefaultTheme                     string
	SecretsMasterKey                 string
	MinIOClient                      *minio.Client
	MinIOEndpoint                    string
	MinIOAccessKey                   string
	MinIOSecretKey                   string
	MinIOUseSSL                      bool
	MinIORegion                      string
	EditionHooks                     *edition.Hooks
	HarborURL                        string
	HarborUsername                   string
	HarborPassword                   string
	HarborInsecureSkipVerify         bool
}

func New(
	projectStore store.ProjectStore,
	appStore store.AppStore,
	buildStore store.BuildStore,
	jobStore store.JobStore,
	podStore store.PodStore,
	workspaceStore store.WorkspaceStore,
	sessionStore store.SessionStore,
	auditStore store.AuditStore,
	accessStore store.AccessStore,
	secretStore store.SecretStore,
	datasetStore store.DatasetStore,
	datasourceStore store.DatasourceStore,
	repositoryStore store.RepositoryStore,
	projectResourceStore store.ProjectResourceStore,
	userPreferenceStore store.UserPreferenceStore,
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
		projectStore:                     projectStore,
		appStore:                         appStore,
		buildStore:                       buildStore,
		jobStore:                         jobStore,
		podStore:                         podStore,
		workspaceStore:                   workspaceStore,
		sessionStore:                     sessionStore,
		auditStore:                       auditStore,
		accessStore:                      accessStore,
		secretStore:                      secretStore,
		datasetStore:                     datasetStore,
		datasourceStore:                  datasourceStore,
		repositoryStore:                  repositoryStore,
		projectResourceStore:             projectResourceStore,
		userPreferenceStore:              userPreferenceStore,
		runtime:                          runtime,
		authVerifier:                     authVerifier,
		keycloak:                         keycloakClient,
		minioClient:                      options.MinIOClient,
		minioEndpoint:                    options.MinIOEndpoint,
		minioAccessKey:                   options.MinIOAccessKey,
		minioSecretKey:                   options.MinIOSecretKey,
		minioUseSSL:                      options.MinIOUseSSL,
		minioRegion:                      options.MinIORegion,
		secretsMasterKey:                 options.SecretsMasterKey,
		registryPullSecret:               options.RegistryPullSecret,
		registryPushSecret:               options.RegistryPushSecret,
		bootstrapAdminUser:               options.BootstrapAdminUser,
		bootstrapAdminEmail:              options.BootstrapAdminEmail,
		organizationRequired:             options.OrganizationRequired,
		workspaceJupyterImage:            options.WorkspaceJupyterImage,
		workspaceVSCodeImage:             options.WorkspaceVSCodeImage,
		workspaceRStudioImage:            options.WorkspaceRStudioImage,
		workspaceNamespace:               options.WorkspaceNamespace,
		workspaceCPU:                     options.WorkspaceCPU,
		workspaceCPURequest:              options.WorkspaceCPURequest,
		workspaceMemory:                  options.WorkspaceMemory,
		workspaceEphemeralStorageRequest: options.WorkspaceEphemeralStorageRequest,
		workspaceEphemeralStorageLimit:   options.WorkspaceEphemeralStorageLimit,
		workspacePVCEnabled:              options.WorkspacePVCEnabled,
		workspacePVCClass:                options.WorkspacePVCClass,
		workspacePVCSize:                 options.WorkspacePVCSize,
		workspacePVCAccessMode:           options.WorkspacePVCAccessMode,
		workspacePVCMountPath:            options.WorkspacePVCMountPath,
		workspaceProfilePVCEnabled:       options.WorkspaceProfilePVCEnabled,
		workspaceProfilePVCClass:         options.WorkspaceProfilePVCClass,
		workspaceProfilePVCSize:          options.WorkspaceProfilePVCSize,
		workspaceProfilePVCAccessMode:    options.WorkspaceProfilePVCAccessMode,
		workspaceProfilePVCMountPath:     options.WorkspaceProfilePVCMountPath,
		hardwareTiers:                    defaultHardwareTiers(),
		backendVersion:                   options.BackendVersion,
		edition:                          strings.TrimSpace(options.Edition),
		defaultTheme:                     strings.TrimSpace(options.DefaultTheme),
		editionHooks:                     hooks,
		harborURL:                        strings.TrimSpace(options.HarborURL),
		harborUsername:                   strings.TrimSpace(options.HarborUsername),
		harborPassword:                   options.HarborPassword,
		harborInsecureSkipVerify:         options.HarborInsecureSkipVerify,
	}
}
