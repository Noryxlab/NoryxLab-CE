package main

import (
	"log"
	"strings"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/config"
	nhttp "github.com/Noryxlab/NoryxLab-CE/backend/internal/http"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/http/handlers"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime/k8s"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/postgres"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {
	cfg := config.Load()

	var projectStore store.ProjectStore = memory.NewProjectStore()
	var appStore store.AppStore = memory.NewAppStore()
	var buildStore store.BuildStore = memory.NewBuildStore()
	var jobStore store.JobStore = memory.NewJobStore()
	var podStore store.PodStore = memory.NewPodStore()
	var workspaceStore store.WorkspaceStore = memory.NewWorkspaceStore()
	var sessionStore store.SessionStore = memory.NewSessionStore()
	var accessStore store.AccessStore = memory.NewAccessStore()
	var secretStore store.SecretStore = memory.NewSecretStore()
	var datasetStore store.DatasetStore = memory.NewDatasetStore()
	var datasourceStore store.DatasourceStore = memory.NewDatasourceStore()
	var repositoryStore store.RepositoryStore = memory.NewRepositoryStore()
	var projectResourceStore store.ProjectResourceStore = memory.NewProjectResourceStore()

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
			log.Printf("warning: postgres store init failed, fallback to memory: %v", err)
		} else {
			defer func() {
				_ = pg.Close()
			}()
			projectStore = &postgres.ProjectStore{Store: pg}
			appStore = &postgres.AppStore{Store: pg}
			buildStore = &postgres.BuildStore{Store: pg}
			jobStore = &postgres.JobStore{Store: pg}
			podStore = &postgres.PodStore{Store: pg}
			workspaceStore = &postgres.WorkspaceStore{Store: pg}
			sessionStore = &postgres.SessionStore{Store: pg}
			accessStore = &postgres.AccessStore{Store: pg}
			secretStore = &postgres.SecretStore{Store: pg}
			datasetStore = &postgres.DatasetStore{Store: pg}
			datasourceStore = &postgres.DatasourceStore{Store: pg}
			repositoryStore = &postgres.RepositoryStore{Store: pg}
			projectResourceStore = &postgres.ProjectResourceStore{Store: pg}
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

	h := handlers.New(
		projectStore,
		appStore,
		buildStore,
		jobStore,
		podStore,
		workspaceStore,
		sessionStore,
		accessStore,
		secretStore,
		datasetStore,
		datasourceStore,
		repositoryStore,
		projectResourceStore,
		runtime,
		verifier,
		keycloakClient,
		handlers.Options{
			RegistryPullSecret:               cfg.RegistryPullSecret,
			RegistryPushSecret:               cfg.RegistryPushSecret,
			BootstrapAdminUser:               cfg.BootstrapAdminUser,
			BootstrapAdminEmail:              cfg.BootstrapAdminEmail,
			WorkspaceJupyterImage:            cfg.WorkspaceJupyterImage,
			WorkspaceVSCodeImage:             cfg.WorkspaceVSCodeImage,
			WorkspaceNamespace:               cfg.WorkloadNamespace,
			WorkspaceCPU:                     cfg.WorkspaceCPU,
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
			BackendVersion:                   cfg.BackendVersion,
			FrontendVersion:                  cfg.FrontendVersion,
			SecretsMasterKey:                 cfg.SecretsMasterKey,
			MinIOClient:                      minioClient,
			MinIOEndpoint:                    cfg.MinIOEndpoint,
			MinIOAccessKey:                   cfg.MinIOAccessKey,
			MinIOSecretKey:                   cfg.MinIOSecretKey,
			MinIOUseSSL:                      cfg.MinIOUseSSL,
			MinIORegion:                      cfg.MinIORegion,
		},
	)

	srv := nhttp.NewServer(cfg, h)

	log.Printf("noryx-api listening on %s", cfg.ListenAddr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
