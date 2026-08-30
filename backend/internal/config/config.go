package config

import (
	"log"
	"os"
	"strings"
	"time"
)

type Config struct {
	BackendVersion                   string
	Edition                          string
	EnabledFeatures                  string
	DefaultTheme                     string
	ListenAddr                       string
	StoreBackend                     string
	DatabaseHost                     string
	DatabasePort                     string
	DatabaseName                     string
	DatabaseUser                     string
	DatabasePassword                 string
	DatabaseSSLMode                  string
	KubernetesNamespace              string
	WorkloadNamespace                string
	EnableK8sRuntime                 bool
	RegistryPullSecret               string
	RegistryPushSecret               string
	AuthMode                         string
	OIDCIssuerURL                    string
	OIDCJWKSURL                      string
	OIDCAudience                     string
	BootstrapAdminUser               string
	BootstrapAdminEmail              string
	KeycloakBaseURL                  string
	KeycloakRealm                    string
	KeycloakAdminRealm               string
	KeycloakAdminUser                string
	KeycloakAdminPass                string
	OrganizationRequired             bool
	WorkspaceJupyterImage            string
	WorkspaceVSCodeImage             string
	WorkspaceRStudioImage            string
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
	// WorkspaceMaxLifetime stops workspaces once they reach this age. Zero
	// disables the sweep, which is the default: reclaiming a user's workspace
	// is a policy decision an operator has to make deliberately.
	WorkspaceMaxLifetime time.Duration
	// AlertWebhookURL receives operator alerts. Empty disables delivery.
	AlertWebhookURL string
	// AlertInstanceName names this platform in an alert, so an operator
	// watching several installations can tell them apart.
	AlertInstanceName             string
	WorkspaceProfilePVCAccessMode string
	WorkspaceProfilePVCMountPath  string
	ProjectFilesImage             string
	SecretsMasterKey              string
	MinIOEndpoint                 string
	MinIOAccessKey                string
	MinIOSecretKey                string
	MinIOUseSSL                   bool
	MinIORegion                   string
	HarborURL                     string
	HarborUsername                string
	HarborPassword                string
	HarborInsecureSkipVerify      bool
	AssistantURL                  string
	AssistantInternalToken        string
	AssistantDeveloperSigningKey  string
	AssistantPublicURL            string
}

func Load() Config {
	backendVersion := os.Getenv("NORYX_BACKEND_VERSION")
	if backendVersion == "" {
		backendVersion = "0.5.161"
	}
	edition := os.Getenv("NORYX_EDITION")
	if edition == "" {
		edition = "community"
	}
	defaultTheme := os.Getenv("NORYX_UI_DEFAULT_THEME")
	if defaultTheme == "" {
		defaultTheme = "noryx"
	}
	listenAddr := os.Getenv("NORYX_LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
	}
	storeBackend := os.Getenv("NORYX_STORE_BACKEND")
	if storeBackend == "" {
		storeBackend = "postgres"
	}

	// Deliberately not called an idle timeout: without an activity signal from
	// the workspace this is a maximum age, and naming it otherwise would
	// promise behaviour the platform cannot deliver (ADR-034).
	// 48h by default: long enough that a workspace left open over a weekend
	// survives, short enough that one forgotten before a holiday does not run
	// for a fortnight. Administrators override it per installation.
	workspaceMaxLifetime := 48 * time.Hour
	if raw := strings.TrimSpace(os.Getenv("NORYX_WORKSPACE_MAX_LIFETIME")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		switch {
		case err != nil:
			log.Printf("config: NORYX_WORKSPACE_MAX_LIFETIME=%q is not a duration, keeping the %s default", raw, workspaceMaxLifetime)
		case parsed < 0:
			log.Printf("config: NORYX_WORKSPACE_MAX_LIFETIME=%q is negative, keeping the %s default", raw, workspaceMaxLifetime)
		default:
			// An explicit zero disables the sweep entirely.
			workspaceMaxLifetime = parsed
		}
	}

	alertWebhookURL := strings.TrimSpace(os.Getenv("NORYX_ALERT_WEBHOOK_URL"))
	alertInstanceName := strings.TrimSpace(os.Getenv("NORYX_ALERT_INSTANCE_NAME"))

	namespace := os.Getenv("NORYX_KUBE_NAMESPACE")
	if namespace == "" {
		namespace = "noryx-ce"
	}
	workloadNamespace := os.Getenv("NORYX_WORKLOAD_NAMESPACE")
	if workloadNamespace == "" {
		workloadNamespace = namespace
	}

	pullSecret := os.Getenv("NORYX_REGISTRY_PULL_SECRET")
	if pullSecret == "" {
		pullSecret = "harbor-regcred"
	}

	pushSecret := os.Getenv("NORYX_REGISTRY_PUSH_SECRET")
	if pushSecret == "" {
		pushSecret = pullSecret
	}

	enableRuntime := os.Getenv("NORYX_ENABLE_K8S_RUNTIME") == "true"
	authMode := os.Getenv("NORYX_AUTH_MODE")
	if authMode == "" {
		authMode = "oidc"
	}

	oidcIssuer := os.Getenv("NORYX_OIDC_ISSUER_URL")
	if oidcIssuer == "" {
		oidcIssuer = "http://keycloak:8080/auth/realms/noryx"
	}

	keycloakBaseURL := os.Getenv("NORYX_KEYCLOAK_BASE_URL")
	if keycloakBaseURL == "" {
		keycloakBaseURL = "http://keycloak:8080/auth"
	}

	keycloakRealm := os.Getenv("NORYX_KEYCLOAK_REALM")
	if keycloakRealm == "" {
		keycloakRealm = "noryx"
	}

	keycloakAdminRealm := os.Getenv("NORYX_KEYCLOAK_ADMIN_REALM")
	if keycloakAdminRealm == "" {
		keycloakAdminRealm = "master"
	}

	keycloakAdminUser := os.Getenv("NORYX_KEYCLOAK_ADMIN_USER")
	if keycloakAdminUser == "" {
		keycloakAdminUser = "admin"
	}

	workspaceJupyterImage := os.Getenv("NORYX_WORKSPACE_JUPYTER_IMAGE")
	if workspaceJupyterImage == "" {
		workspaceJupyterImage = "harbor.example.local/noryx-environments/noryx-jupyter:0.1.0"
	}
	workspaceVSCodeImage := os.Getenv("NORYX_WORKSPACE_VSCODE_IMAGE")
	if workspaceVSCodeImage == "" {
		workspaceVSCodeImage = "harbor.example.local/noryx-environments/noryx-vscode:0.1.2"
	}
	workspaceRStudioImage := os.Getenv("NORYX_WORKSPACE_RSTUDIO_IMAGE")
	if workspaceRStudioImage == "" {
		workspaceRStudioImage = "harbor.example.local/noryx-environments/noryx-rstudio:0.1.0"
	}
	workspaceCPU := os.Getenv("NORYX_WORKSPACE_CPU")
	if workspaceCPU == "" {
		workspaceCPU = "500m"
	}
	workspaceCPURequest := os.Getenv("NORYX_WORKSPACE_CPU_REQUEST")
	if workspaceCPURequest == "" {
		workspaceCPURequest = "250m"
	}
	workspaceMemory := os.Getenv("NORYX_WORKSPACE_MEMORY")
	if workspaceMemory == "" {
		workspaceMemory = "512Mi"
	}
	workspaceEphemeralStorageRequest := os.Getenv("NORYX_WORKSPACE_EPHEMERAL_STORAGE_REQUEST")
	if workspaceEphemeralStorageRequest == "" {
		workspaceEphemeralStorageRequest = "1Gi"
	}
	workspaceEphemeralStorageLimit := os.Getenv("NORYX_WORKSPACE_EPHEMERAL_STORAGE_LIMIT")
	if workspaceEphemeralStorageLimit == "" {
		workspaceEphemeralStorageLimit = "4Gi"
	}
	workspacePVCEnabled := os.Getenv("NORYX_WORKSPACE_PVC_ENABLED")
	if workspacePVCEnabled == "" {
		workspacePVCEnabled = "true"
	}
	workspacePVCClass := os.Getenv("NORYX_WORKSPACE_PVC_STORAGE_CLASS")
	if workspacePVCClass == "" {
		workspacePVCClass = "longhorn"
	}
	workspacePVCSize := os.Getenv("NORYX_WORKSPACE_PVC_SIZE")
	if workspacePVCSize == "" {
		workspacePVCSize = "10Gi"
	}
	workspacePVCAccessMode := os.Getenv("NORYX_WORKSPACE_PVC_ACCESS_MODE")
	if workspacePVCAccessMode == "" {
		workspacePVCAccessMode = "ReadWriteMany"
	}
	workspacePVCMountPath := os.Getenv("NORYX_WORKSPACE_PVC_MOUNT_PATH")
	if workspacePVCMountPath == "" {
		workspacePVCMountPath = "/mnt"
	}
	workspaceProfilePVCEnabled := os.Getenv("NORYX_WORKSPACE_PROFILE_PVC_ENABLED")
	if workspaceProfilePVCEnabled == "" {
		workspaceProfilePVCEnabled = "true"
	}
	workspaceProfilePVCClass := os.Getenv("NORYX_WORKSPACE_PROFILE_PVC_STORAGE_CLASS")
	if workspaceProfilePVCClass == "" {
		workspaceProfilePVCClass = "longhorn"
	}
	workspaceProfilePVCSize := os.Getenv("NORYX_WORKSPACE_PROFILE_PVC_SIZE")
	if workspaceProfilePVCSize == "" {
		workspaceProfilePVCSize = "5Gi"
	}
	workspaceProfilePVCAccessMode := os.Getenv("NORYX_WORKSPACE_PROFILE_PVC_ACCESS_MODE")
	if workspaceProfilePVCAccessMode == "" {
		workspaceProfilePVCAccessMode = "ReadWriteMany"
	}
	workspaceProfilePVCMountPath := os.Getenv("NORYX_WORKSPACE_PROFILE_PVC_MOUNT_PATH")
	if workspaceProfilePVCMountPath == "" {
		workspaceProfilePVCMountPath = "/home/noryx/.noryx-profile"
	}
	projectFilesImage := os.Getenv("NORYX_PROJECT_FILES_IMAGE")
	if projectFilesImage == "" {
		projectFilesImage = "harbor.example.local/noryx-ce/noryx-backend:0.5.161-dev"
	}
	minioEndpoint := os.Getenv("NORYX_MINIO_ENDPOINT")
	if minioEndpoint == "" {
		minioEndpoint = "minio:9000"
	}
	minioAccessKey := os.Getenv("NORYX_MINIO_ACCESS_KEY")
	if minioAccessKey == "" {
		minioAccessKey = "noryx"
	}
	minioRegion := os.Getenv("NORYX_MINIO_REGION")
	if minioRegion == "" {
		minioRegion = "us-east-1"
	}
	harborURL := os.Getenv("NORYX_HARBOR_URL")
	if harborURL == "" {
		harborURL = "https://harbor.example.local"
	}

	return Config{
		BackendVersion:                   backendVersion,
		Edition:                          edition,
		EnabledFeatures:                  os.Getenv("NORYX_ENABLED_FEATURES"),
		DefaultTheme:                     defaultTheme,
		ListenAddr:                       listenAddr,
		StoreBackend:                     storeBackend,
		DatabaseHost:                     os.Getenv("NORYX_DATABASE_HOST"),
		DatabasePort:                     os.Getenv("NORYX_DATABASE_PORT"),
		DatabaseName:                     os.Getenv("NORYX_DATABASE_NAME"),
		DatabaseUser:                     os.Getenv("NORYX_DATABASE_USER"),
		DatabasePassword:                 os.Getenv("NORYX_DATABASE_PASSWORD"),
		DatabaseSSLMode:                  os.Getenv("NORYX_DATABASE_SSLMODE"),
		KubernetesNamespace:              namespace,
		WorkloadNamespace:                workloadNamespace,
		EnableK8sRuntime:                 enableRuntime,
		RegistryPullSecret:               pullSecret,
		RegistryPushSecret:               pushSecret,
		AuthMode:                         authMode,
		OIDCIssuerURL:                    oidcIssuer,
		OIDCJWKSURL:                      os.Getenv("NORYX_OIDC_JWKS_URL"),
		OIDCAudience:                     os.Getenv("NORYX_OIDC_AUDIENCE"),
		BootstrapAdminUser:               os.Getenv("NORYX_BOOTSTRAP_ADMIN_USER"),
		BootstrapAdminEmail:              os.Getenv("NORYX_BOOTSTRAP_ADMIN_EMAIL"),
		KeycloakBaseURL:                  keycloakBaseURL,
		KeycloakRealm:                    keycloakRealm,
		KeycloakAdminRealm:               keycloakAdminRealm,
		KeycloakAdminUser:                keycloakAdminUser,
		KeycloakAdminPass:                os.Getenv("NORYX_KEYCLOAK_ADMIN_PASSWORD"),
		OrganizationRequired:             os.Getenv("NORYX_ORGANIZATION_REQUIRED") == "true",
		WorkspaceJupyterImage:            workspaceJupyterImage,
		WorkspaceVSCodeImage:             workspaceVSCodeImage,
		WorkspaceRStudioImage:            workspaceRStudioImage,
		WorkspaceCPU:                     workspaceCPU,
		WorkspaceCPURequest:              workspaceCPURequest,
		WorkspaceMemory:                  workspaceMemory,
		WorkspaceEphemeralStorageRequest: workspaceEphemeralStorageRequest,
		WorkspaceEphemeralStorageLimit:   workspaceEphemeralStorageLimit,
		WorkspacePVCEnabled:              workspacePVCEnabled == "true",
		WorkspacePVCClass:                workspacePVCClass,
		WorkspacePVCSize:                 workspacePVCSize,
		WorkspacePVCAccessMode:           workspacePVCAccessMode,
		WorkspacePVCMountPath:            workspacePVCMountPath,
		WorkspaceProfilePVCEnabled:       workspaceProfilePVCEnabled == "true",
		WorkspaceProfilePVCClass:         workspaceProfilePVCClass,
		WorkspaceProfilePVCSize:          workspaceProfilePVCSize,
		WorkspaceMaxLifetime:             workspaceMaxLifetime,
		AlertWebhookURL:                  alertWebhookURL,
		AlertInstanceName:                alertInstanceName,
		WorkspaceProfilePVCAccessMode:    workspaceProfilePVCAccessMode,
		WorkspaceProfilePVCMountPath:     workspaceProfilePVCMountPath,
		ProjectFilesImage:                projectFilesImage,
		SecretsMasterKey:                 os.Getenv("NORYX_SECRETS_MASTER_KEY"),
		MinIOEndpoint:                    minioEndpoint,
		MinIOAccessKey:                   minioAccessKey,
		MinIOSecretKey:                   os.Getenv("NORYX_MINIO_SECRET_KEY"),
		MinIOUseSSL:                      os.Getenv("NORYX_MINIO_USE_SSL") == "true",
		MinIORegion:                      minioRegion,
		HarborURL:                        harborURL,
		HarborUsername:                   os.Getenv("NORYX_HARBOR_USERNAME"),
		HarborPassword:                   os.Getenv("NORYX_HARBOR_PASSWORD"),
		HarborInsecureSkipVerify:         os.Getenv("NORYX_HARBOR_INSECURE_SKIP_VERIFY") == "true",
		AssistantURL:                     os.Getenv("NORYX_ASSISTANT_URL"),
		AssistantInternalToken:           os.Getenv("NORYX_ASSISTANT_INTERNAL_TOKEN"),
		AssistantDeveloperSigningKey:     os.Getenv("NORYX_ASSISTANT_DEVELOPER_SIGNING_KEY"),
		AssistantPublicURL:               os.Getenv("NORYX_ASSISTANT_PUBLIC_URL"),
	}
}
