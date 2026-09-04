package main

import (
	"context"
	"log"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/config"
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
	var appStore store.AppStore = memory.NewAppStore()
	var buildStore store.BuildStore = memory.NewBuildStore()
	var jobStore store.JobStore = memory.NewJobStore()
	var podStore store.PodStore = memory.NewPodStore()
	var workspaceStore store.WorkspaceStore = memory.NewWorkspaceStore()
	var sessionStore store.SessionStore = memory.NewSessionStore()
	var auditStore store.AuditStore = memory.NewAuditStore()
	var egressRuleStore store.EgressRuleStore = memory.NewEgressRuleStore()
	var accessStore store.AccessStore = memory.NewAccessStore()
	var secretStore store.SecretStore = memory.NewSecretStore()
	var datasetStore store.DatasetStore = memory.NewDatasetStore()
	var datasourceStore store.DatasourceStore = memory.NewDatasourceStore()
	var ontologyStore store.OntologyStore = memory.NewOntologyObjectStore()
	var repositoryStore store.RepositoryStore = memory.NewRepositoryStore()
	var projectResourceStore store.ProjectResourceStore = memory.NewProjectResourceStore()
	var projectOntologyStore store.ProjectOntologyStore = memory.NewProjectOntologyStore()
	var userPreferenceStore store.UserPreferenceStore = memory.NewUserPreferenceStore()
	var rbacPolicyStore store.RBACPolicyStore = memory.NewRBACPolicyStore()
	var settingsStore settings.Store = memory.NewSettingsStore()
	var backupRunStore store.BackupRunStore = memory.NewBackupRunStore()
	var storageEndpointStore store.StorageEndpointStore = memory.NewStorageEndpointStore()

	if strings.EqualFold(cfg.StoreBackend, "postgres") {
		pg, err := postgres.New(postgres.Config{
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
			appStore = &postgres.AppStore{Store: pg}
			buildStore = &postgres.BuildStore{Store: pg}
			jobStore = &postgres.JobStore{Store: pg}
			podStore = &postgres.PodStore{Store: pg}
			workspaceStore = &postgres.WorkspaceStore{Store: pg}
			sessionStore = &postgres.SessionStore{Store: pg}
			auditStore = &postgres.AuditStore{Store: pg}
			egressRuleStore = &postgres.EgressRuleStore{Store: pg}
			accessStore = &postgres.AccessStore{Store: pg}
			secretStore = &postgres.SecretStore{Store: pg}
			datasetStore = &postgres.DatasetStore{Store: pg}
			datasourceStore = &postgres.DatasourceStore{Store: pg}
			ontologyStore = &postgres.OntologyStore{Store: pg}
			repositoryStore = &postgres.RepositoryStore{Store: pg}
			projectResourceStore = &postgres.ProjectResourceStore{Store: pg}
			projectOntologyStore = &postgres.ProjectOntologyStore{Store: pg}
			userPreferenceStore = &postgres.UserPreferenceStore{Store: pg}
			rbacPolicyStore = &postgres.RBACPolicyStore{Store: pg}
			settingsStore = &postgres.SettingsStore{Store: pg}
			backupRunStore = &postgres.BackupRunStore{Store: pg}
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
		datasetStore,
		datasourceStore,
		ontologyStore,
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
			RegistryPullSecret:               cfg.RegistryPullSecret,
			RegistryPushSecret:               cfg.RegistryPushSecret,
			BootstrapAdminUser:               cfg.BootstrapAdminUser,
			BootstrapAdminEmail:              cfg.BootstrapAdminEmail,
			OrganizationRequired:             cfg.OrganizationRequired,
			HealthEventStore:                 healthEventStore,
			APITokenStore:                    apiTokenStore,
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
			Settings:                         settingsResolver,
			EditionHooks: &edition.Hooks{
				Feature: edition.FeatureGateFromCSV(cfg.EnabledFeatures),
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
			AssistantURL:                 cfg.AssistantURL,
			AssistantInternalToken:       cfg.AssistantInternalToken,
			AssistantDeveloperSigningKey: cfg.AssistantDeveloperSigningKey,
			AssistantPublicURL:           cfg.AssistantPublicURL,
		},
	)

	// Background sweeps start before the listener so a long-running instance
	// reclaims capacity even if no request is ever served.
	reaperCtx, stopReaper := context.WithCancel(context.Background())
	defer stopReaper()
	h.StartWorkspaceReaper(reaperCtx)
	h.StartJobWatcher(reaperCtx)
	h.StartHealthWatcher(reaperCtx)

	srv := nhttp.NewServer(cfg, h)

	log.Printf("noryx-api listening on %s", cfg.ListenAddr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
