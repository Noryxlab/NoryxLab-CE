package main

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/config"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/hardware"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/edition"
	nhttp "github.com/Noryxlab/NoryxLab-CE/backend/internal/http"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/http/handlers"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime/k8s"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/settings"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/postgres"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	cfg := config.Load()

	var projectStore store.ProjectStore = memory.NewProjectStore()
	var healthEventStore store.HealthEventStore = memory.NewHealthEventStore()
	var apiTokenStore store.APITokenStore = memory.NewAPITokenStore()
	var hardwareTierStore store.HardwareTierStore = memory.NewHardwareTierStore()
	var quotaStore store.QuotaStore = memory.NewQuotaStore()
	var usageStore store.UsageStore = memory.NewUsageStore()
	var appStore store.AppStore = memory.NewAppStore()
	var buildStore store.BuildStore = memory.NewBuildStore()
	var jobStore store.JobStore = memory.NewJobStore()
	var podStore store.PodStore = memory.NewPodStore()
	var workspaceStore store.WorkspaceStore = memory.NewWorkspaceStore()
	var sessionStore store.SessionStore = memory.NewSessionStore()
	var auditStore store.AuditStore = memory.NewAuditStore()
	var datasetSizeStore store.DatasetSizeStore = memory.NewDatasetSizeStore()
	var egressRuleStore store.EgressRuleStore = memory.NewEgressRuleStore()
	var accessStore store.AccessStore = memory.NewAccessStore()
	var secretStore store.SecretStore = memory.NewSecretStore()
	var projectVariableStore store.ProjectVariableStore = memory.NewProjectVariableStore()
	var datasetStore store.DatasetStore = memory.NewDatasetStore()
	var datasourceStore store.DatasourceStore = memory.NewDatasourceStore()
	var ontologyStore store.OntologyStore = memory.NewOntologyObjectStore()
	var cohortStore store.CohortStore = memory.NewCohortStore()
	var repositoryStore store.RepositoryStore = memory.NewRepositoryStore()
	var projectResourceStore store.ProjectResourceStore = memory.NewProjectResourceStore()
	var projectOntologyStore store.ProjectOntologyStore = memory.NewProjectOntologyStore()
	var userPreferenceStore store.UserPreferenceStore = memory.NewUserPreferenceStore()
	var rbacPolicyStore store.RBACPolicyStore = memory.NewRBACPolicyStore()
	var settingsStore settings.Store = memory.NewSettingsStore()
	var backupRunStore store.BackupRunStore = memory.NewBackupRunStore()
	// Agents exist only where they can be remembered. Without a database the
	// platform keeps none: an agent that forgets its instructions on restart is
	// worse than an absent feature, because somebody relied on it.
	var agentStore store.AgentStore
	var agentTeamStore store.AgentTeamStore
	var storageEndpointStore store.StorageEndpointStore = memory.NewStorageEndpointStore()

	if strings.EqualFold(cfg.StoreBackend, "postgres") {
		pg, err := openPostgresWaitingForIt(postgres.Config{
			Host:     cfg.DatabaseHost,
			Port:     cfg.DatabasePort,
			DBName:   cfg.DatabaseName,
			User:     cfg.DatabaseUser,
			Password: cfg.DatabasePassword,
			SSLMode:  cfg.DatabaseSSLMode,
		})
		if err != nil {
			log.Fatalf("postgres store backend required but init failed: %v", err)
		} else {
			defer func() {
				_ = pg.Close()
			}()
			projectStore = &postgres.ProjectStore{Store: pg}
			healthEventStore = &postgres.HealthEventStore{Store: pg}
			apiTokenStore = &postgres.APITokenStore{Store: pg}
			hardwareTierStore = &postgres.HardwareTierStore{Store: pg}
			quotaStore = &postgres.QuotaStore{Store: pg}
			usageStore = &postgres.UsageStore{Store: pg}
			// An installation upgrading into editable tiers keeps the four
			// sizes it already ran, under the same ids: workspaces in flight
			// refer to them by id.
			if existing, err := hardwareTierStore.List(); err == nil && len(existing) == 0 {
				for _, tier := range hardware.Defaults() {
					if err := hardwareTierStore.Upsert(tier); err != nil {
						log.Printf("seeding hardware tier %s failed: %v", tier.ID, err)
					}
				}
			}
			appStore = &postgres.AppStore{Store: pg}
			buildStore = &postgres.BuildStore{Store: pg}
			jobStore = &postgres.JobStore{Store: pg}
			podStore = &postgres.PodStore{Store: pg}
			workspaceStore = &postgres.WorkspaceStore{Store: pg}
			sessionStore = &postgres.SessionStore{Store: pg}
			auditStore = &postgres.AuditStore{Store: pg}
			datasetSizeStore = &postgres.DatasetSizeStore{Store: pg}
			egressRuleStore = &postgres.EgressRuleStore{Store: pg}
			accessStore = &postgres.AccessStore{Store: pg}
			secretStore = &postgres.SecretStore{Store: pg}
			projectVariableStore = &postgres.ProjectVariableStore{Store: pg}
			datasetStore = &postgres.DatasetStore{Store: pg}
			datasourceStore = &postgres.DatasourceStore{Store: pg}
			ontologyStore = &postgres.OntologyStore{Store: pg}
			cohortStore = &postgres.CohortStore{Store: pg}
			repositoryStore = &postgres.RepositoryStore{Store: pg}
			projectResourceStore = &postgres.ProjectResourceStore{Store: pg}
			projectOntologyStore = &postgres.ProjectOntologyStore{Store: pg}
			userPreferenceStore = &postgres.UserPreferenceStore{Store: pg}
			rbacPolicyStore = &postgres.RBACPolicyStore{Store: pg}
			settingsStore = &postgres.SettingsStore{Store: pg}
			backupRunStore = &postgres.BackupRunStore{Store: pg}
			agentStore = &postgres.AgentStore{Store: pg}
			agentTeamStore = &postgres.AgentTeamStore{Store: pg}
			storageEndpointStore = &postgres.StorageEndpointStore{Store: pg}
			log.Printf("postgres store backend enabled")
		}
	}

	var minioClient *minio.Client
	if strings.TrimSpace(cfg.MinIOEndpoint) != "" && strings.TrimSpace(cfg.MinIOAccessKey) != "" && strings.TrimSpace(cfg.MinIOSecretKey) != "" {
		client, err := minio.New(cfg.MinIOEndpoint, &minio.Options{
			Creds:  credentials.NewStaticV4(cfg.MinIOAccessKey, cfg.MinIOSecretKey, ""),
			Secure: cfg.MinIOUseSSL,
		})
		if err != nil {
			log.Printf("warning: minio client disabled: %v", err)
		} else {
			minioClient = client
		}
	}

	var runtime noryxruntime.Runner
	if cfg.EnableK8sRuntime {
		k8sRuntime, err := k8s.NewFromInCluster(cfg.KubernetesNamespace, cfg.WorkloadNamespace)
		if err != nil {
			log.Printf("warning: kubernetes runtime disabled: %v", err)
		} else {
			runtime = k8sRuntime
		}
	}

	var verifier auth.Verifier
	if strings.EqualFold(cfg.AuthMode, "oidc") {
		oidcVerifier, err := auth.NewOIDCVerifier(cfg.OIDCIssuerURL, cfg.OIDCJWKSURL, cfg.OIDCAudience)
		if err != nil {
			log.Printf("warning: oidc verifier disabled: %v", err)
		} else {
			verifier = oidcVerifier
		}
	}

	var keycloakClient *keycloak.Client
	kc, err := keycloak.New(keycloak.Config{
		BaseURL:       cfg.KeycloakBaseURL,
		Realm:         cfg.KeycloakRealm,
		AdminRealm:    cfg.KeycloakAdminRealm,
		AdminUsername: cfg.KeycloakAdminUser,
		AdminPassword: cfg.KeycloakAdminPass,
	})
	if err != nil {
		log.Printf("warning: keycloak admin client disabled: %v", err)
	} else {
		keycloakClient = kc
	}

	settingsResolver := settings.NewResolver(settingsStore)
	// Facts, not settings: determined by the build and the deployment, shown in
	// the administration screen so they are visible in one place, and refused
	// for writing.
	settingsResolver.SetFact(settings.KeyBackendVersion, cfg.BackendVersion)
	settingsResolver.SetFact(settings.KeyEdition, cfg.Edition)
	settingsResolver.SetFact(settings.KeyNamespace, cfg.KubernetesNamespace)

	// Declared once: the same gate answers what the edition sells and what the
	// permission engine is allowed to decide.
	features := edition.FeatureGateFromCSV(cfg.EnabledFeatures)

	h := handlers.New(
		projectStore,
		appStore,
		buildStore,
		jobStore,
		podStore,
		workspaceStore,
		sessionStore,
		auditStore,
		egressRuleStore,
		accessStore,
		secretStore,
		projectVariableStore,
		datasetStore,
		datasourceStore,
		ontologyStore,
		cohortStore,
		repositoryStore,
		projectResourceStore,
		projectOntologyStore,
		userPreferenceStore,
		rbacPolicyStore,
		backupRunStore,
		storageEndpointStore,
		runtime,
		verifier,
		keycloakClient,
		handlers.Options{
			AgentStore:                       agentStore,
			AgentTeamStore:                   agentTeamStore,
			RegistryPullSecret:               cfg.RegistryPullSecret,
			RegistryPushSecret:               cfg.RegistryPushSecret,
			BootstrapAdminUser:               cfg.BootstrapAdminUser,
			BootstrapAdminEmail:              cfg.BootstrapAdminEmail,
			OrganizationRequired:             cfg.OrganizationRequired,
			ProductName:                      cfg.ProductName,
			HealthEventStore:                 healthEventStore,
			APITokenStore:                    apiTokenStore,
			HardwareTierStore:                hardwareTierStore,
			QuotaStore:                       quotaStore,
			UsageStore:                       usageStore,
			OIDCAudience:                     cfg.OIDCAudience,
			OIDCFrontendClientID:             cfg.OIDCFrontendClientID,
			PublicURL:                        cfg.PublicURL,
			AuthMode:                         cfg.AuthMode,
			ServiceToken:                     cfg.ServiceToken,
			WorkspaceJupyterImage:            cfg.WorkspaceJupyterImage,
			WorkspaceVSCodeImage:             cfg.WorkspaceVSCodeImage,
			WorkspaceRStudioImage:            cfg.WorkspaceRStudioImage,
			WorkspaceNamespace:               cfg.WorkloadNamespace,
			WorkspaceCPU:                     cfg.WorkspaceCPU,
			WorkspaceCPURequest:              cfg.WorkspaceCPURequest,
			WorkspaceMemory:                  cfg.WorkspaceMemory,
			WorkspaceEphemeralStorageRequest: cfg.WorkspaceEphemeralStorageRequest,
			WorkspaceEphemeralStorageLimit:   cfg.WorkspaceEphemeralStorageLimit,
			WorkspacePVCEnabled:              cfg.WorkspacePVCEnabled,
			WorkspacePVCClass:                cfg.WorkspacePVCClass,
			WorkspacePVCSize:                 cfg.WorkspacePVCSize,
			WorkspacePVCAccessMode:           cfg.WorkspacePVCAccessMode,
			WorkspacePVCMountPath:            cfg.WorkspacePVCMountPath,
			WorkspaceProfilePVCEnabled:       cfg.WorkspaceProfilePVCEnabled,
			WorkspaceProfilePVCClass:         cfg.WorkspaceProfilePVCClass,
			WorkspaceProfilePVCSize:          cfg.WorkspaceProfilePVCSize,
			WorkspaceProfilePVCAccessMode:    cfg.WorkspaceProfilePVCAccessMode,
			WorkspaceProfilePVCMountPath:     cfg.WorkspaceProfilePVCMountPath,
			ProjectFilesImage:                cfg.ProjectFilesImage,
			BackendVersion:                   cfg.BackendVersion,
			Edition:                          cfg.Edition,
			DefaultTheme:                     cfg.DefaultTheme,
			AlertWebhookURL:                  cfg.AlertWebhookURL,
			AlertInstanceName:                cfg.AlertInstanceName,
			WorkspaceMaxLifetime:             cfg.WorkspaceMaxLifetime,
			PasswordLinkLifetime:             cfg.PasswordLinkLifetime,
			DatasetSizeStore:                 datasetSizeStore,
			Settings:                         settingsResolver,
			EditionHooks: &edition.Hooks{
				Feature: features,
				// Nil on Community, where the handlers' own rule is the answer.
				RBAC: editionRBACProvider(rbacPolicyStore, features.Enabled),
			},
			SecretsMasterKey:             cfg.SecretsMasterKey,
			MinIOClient:                  minioClient,
			MinIOEndpoint:                cfg.MinIOEndpoint,
			MinIOAccessKey:               cfg.MinIOAccessKey,
			MinIOSecretKey:               cfg.MinIOSecretKey,
			MinIOUseSSL:                  cfg.MinIOUseSSL,
			MinIORegion:                  cfg.MinIORegion,
			HarborURL:                    cfg.HarborURL,
			HarborUsername:               cfg.HarborUsername,
			HarborPassword:               cfg.HarborPassword,
			HarborInsecureSkipVerify:     cfg.HarborInsecureSkipVerify,
			HarborCAFile:                 cfg.HarborCAFile,
			AssistantURL:                 cfg.AssistantURL,
			AssistantInternalToken:       cfg.AssistantInternalToken,
			AssistantDeveloperSigningKey: cfg.AssistantDeveloperSigningKey,
			AssistantPublicURL:           cfg.AssistantPublicURL,
			LLMaaSBaseURL:                cfg.LLMaaSBaseURL,
			LLMaaSAPIKey:                 cfg.LLMaaSAPIKey,
			AssistantWorkspaceURL:        cfg.AssistantWorkspaceURL,
		},
	)

	// Background sweeps start before the listener so a long-running instance
	// reclaims capacity even if no request is ever served.
	reaperCtx, stopReaper := context.WithCancel(context.Background())
	defer stopReaper()
	h.StartWorkspaceReaper(reaperCtx)
	h.StartJobWatcher(reaperCtx)
	h.StartHealthWatcher(reaperCtx)
	h.StartUsageSampler(reaperCtx)
	h.StartDatasetSizeSampler(reaperCtx)

	srv := nhttp.NewServer(cfg, h)

	log.Printf("noryx-api listening on %s", cfg.ListenAddr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}

// openPostgresWaitingForIt gives the database a moment to be there.
//
// The platform and its database start together, and the database is slower.
// The API exited on the first refused connection, kubelet restarted it, and it
// came up fine a second later - so every coordinated restart wrote a crash
// into the pod's history and a fatal line into its logs, for a condition that
// resolves itself. That teaches everyone reading those logs that a crash at
// startup is normal, which is exactly what one should never be taught.
//
// A minute of patience, and then the fatal error it always had: a database
// that is still absent after that is a real fault, and refusing to serve is
// the right answer to it.
func openPostgresWaitingForIt(cfg postgres.Config) (*postgres.Store, error) {
	const (
		patience = time.Minute
		between  = 2 * time.Second
	)
	deadline := time.Now().Add(patience)
	attempt := 0
	for {
		pg, err := postgres.New(cfg)
		if err == nil {
			if attempt > 0 {
				log.Printf("database reached after %d attempt(s)", attempt+1)
			}
			return pg, nil
		}
		attempt++
		if time.Now().After(deadline) {
			return nil, err
		}
		if attempt == 1 {
			// Said once. Repeating it every two seconds would bury the one
			// line that matters if the wait does end in a failure.
			log.Printf("database not ready (%v); waiting up to %s for it", err, patience)
		}
		time.Sleep(between)
	}
}
