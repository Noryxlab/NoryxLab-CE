package handlers

import (
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/edition"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/notify"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/settings"
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
	egressRuleStore                  store.EgressRuleStore
	accessStore                      store.AccessStore
	secretStore                      store.SecretStore
	datasetStore                     store.DatasetStore
	datasourceStore                  store.DatasourceStore
	ontologyStore                    store.OntologyStore
	repositoryStore                  store.RepositoryStore
	projectResourceStore             store.ProjectResourceStore
	projectOntologyStore             store.ProjectOntologyStore
	userPreferenceStore              store.UserPreferenceStore
	rbacPolicyStore                  store.RBACPolicyStore
	backupRunStore                   store.BackupRunStore
	notifier                         *notify.Notifier
	workspaceMaxLifetime             time.Duration
	settings                         *settings.Resolver
	storageEndpointStore             store.StorageEndpointStore
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
	healthEventStore                 store.HealthEventStore
	apiTokenStore                    store.APITokenStore
	oidcAudience                     string
	oidcFrontendClientID             string
	publicURL                        string
	authMode                         string
	serviceToken                     string
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
	projectFilesImage                string
	hardwareTiers                    []hardwareTier
	backendVersion                   string
	edition                          string
	defaultTheme                     string
	editionHooks                     edition.Hooks
	harborURL                        string
	harborUsername                   string
	harborPassword                   string
	harborInsecureSkipVerify         bool
	assistantURL                     string
	assistantInternalToken           string
	assistantDeveloperSigningKey     string
	assistantPublicURL               string
}

type Options struct {
	RegistryPullSecret   string
	RegistryPushSecret   string
	BootstrapAdminUser   string
	BootstrapAdminEmail  string
	OrganizationRequired bool
	// HealthEventStore records what the platform noticed about itself.
	HealthEventStore store.HealthEventStore
	// APITokenStore holds the credentials users present instead of a session.
	APITokenStore store.APITokenStore
	// OIDCAudience is the audience this platform requires in a token.
	OIDCAudience string
	// OIDCFrontendClientID is the client whose tokens must carry it.
	OIDCFrontendClientID string
	// PublicURL is the address users reach the platform at.
	PublicURL string
	// AuthMode gates the development header identity; see requireIdentity.
	AuthMode string
	// ServiceToken authenticates platform components that have no human
	// behind them, such as the scheduled backup trigger.
	ServiceToken                     string
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
	ProjectFilesImage                string
	BackendVersion                   string
	Edition                          string
	DefaultTheme                     string
	// AlertWebhookURL and AlertInstanceName configure operator alerting.
	// An empty URL disables it.
	AlertWebhookURL   string
	AlertInstanceName string
	// WorkspaceMaxLifetime is echoed here so the health report can tell which
	// workspaces the reaper should already have reclaimed.
	WorkspaceMaxLifetime time.Duration
	// Settings resolves administrator overrides at use time, so a change takes
	// effect without a redeployment.
	Settings                     *settings.Resolver
	SecretsMasterKey             string
	MinIOClient                  *minio.Client
	MinIOEndpoint                string
	MinIOAccessKey               string
	MinIOSecretKey               string
	MinIOUseSSL                  bool
	MinIORegion                  string
	EditionHooks                 *edition.Hooks
	HarborURL                    string
	HarborUsername               string
	HarborPassword               string
	HarborInsecureSkipVerify     bool
	AssistantURL                 string
	AssistantInternalToken       string
	AssistantDeveloperSigningKey string
	AssistantPublicURL           string
}

// newNotifier resolves the webhook from the settings store when one exists, so
// an administrator can change the destination without a redeployment, and falls
// back to the boot configuration otherwise.
func newNotifier(options Options) *notify.Notifier {
	if options.Settings == nil {
		return notify.New(options.AlertWebhookURL, options.AlertInstanceName)
	}
	return notify.NewDynamic(func() (string, string) {
		return options.Settings.String(settings.KeyAlertWebhookURL),
			options.Settings.String(settings.KeyAlertInstanceName)
	}).WithFormat(func() string {
		return options.Settings.String(settings.KeyAlertFormat)
	})
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
	egressRuleStore store.EgressRuleStore,
	accessStore store.AccessStore,
	secretStore store.SecretStore,
	datasetStore store.DatasetStore,
	datasourceStore store.DatasourceStore,
	ontologyStore store.OntologyStore,
	repositoryStore store.RepositoryStore,
	projectResourceStore store.ProjectResourceStore,
	projectOntologyStore store.ProjectOntologyStore,
	userPreferenceStore store.UserPreferenceStore,
	rbacPolicyStore store.RBACPolicyStore,
	backupRunStore store.BackupRunStore,
	storageEndpointStore store.StorageEndpointStore,
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
		egressRuleStore:                  egressRuleStore,
		accessStore:                      accessStore,
		secretStore:                      secretStore,
		datasetStore:                     datasetStore,
		datasourceStore:                  datasourceStore,
		ontologyStore:                    ontologyStore,
		repositoryStore:                  repositoryStore,
		projectResourceStore:             projectResourceStore,
		projectOntologyStore:             projectOntologyStore,
		userPreferenceStore:              userPreferenceStore,
		rbacPolicyStore:                  rbacPolicyStore,
		backupRunStore:                   backupRunStore,
		notifier:                         newNotifier(options),
		workspaceMaxLifetime:             options.WorkspaceMaxLifetime,
		settings:                         options.Settings,
		storageEndpointStore:             storageEndpointStore,
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
		healthEventStore:                 options.HealthEventStore,
		apiTokenStore:                    options.APITokenStore,
		oidcAudience:                     strings.TrimSpace(options.OIDCAudience),
		oidcFrontendClientID:             firstNonBlank(options.OIDCFrontendClientID, "noryx-frontend"),
		publicURL:                        strings.TrimSpace(options.PublicURL),
		authMode:                         options.AuthMode,
		serviceToken:                     options.ServiceToken,
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
		projectFilesImage:                strings.TrimSpace(options.ProjectFilesImage),
		hardwareTiers:                    defaultHardwareTiers(),
		backendVersion:                   options.BackendVersion,
		edition:                          strings.TrimSpace(options.Edition),
		defaultTheme:                     strings.TrimSpace(options.DefaultTheme),
		editionHooks:                     hooks,
		harborURL:                        strings.TrimSpace(options.HarborURL),
		harborUsername:                   strings.TrimSpace(options.HarborUsername),
		harborPassword:                   options.HarborPassword,
		harborInsecureSkipVerify:         options.HarborInsecureSkipVerify,
		assistantURL:                     strings.TrimRight(strings.TrimSpace(options.AssistantURL), "/"),
		assistantInternalToken:           options.AssistantInternalToken,
		assistantDeveloperSigningKey:     strings.TrimSpace(options.AssistantDeveloperSigningKey),
		assistantPublicURL:               strings.TrimRight(strings.TrimSpace(options.AssistantPublicURL), "/"),
	}
}

// firstNonBlank returns the first value that is not empty after trimming.
func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
