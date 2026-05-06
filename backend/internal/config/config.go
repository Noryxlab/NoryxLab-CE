package config

import "os"

type Config struct {
	BackendVersion                   string
	FrontendVersion                  string
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
	WorkspaceJupyterImage            string
	WorkspaceVSCodeImage             string
	WorkspaceCPU                     string
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
	SecretsMasterKey                 string
	MinIOEndpoint                    string
	MinIOAccessKey                   string
	MinIOSecretKey                   string
	MinIOUseSSL                      bool
	MinIORegion                      string
}

func Load() Config {
	backendVersion := os.Getenv("NORYX_BACKEND_VERSION")
	if backendVersion == "" {
		backendVersion = "0.5.67"
	}
	frontendVersion := os.Getenv("NORYX_FRONTEND_VERSION")
	if frontendVersion == "" {
		frontendVersion = "unknown"
	}

	listenAddr := os.Getenv("NORYX_LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
	}
	storeBackend := os.Getenv("NORYX_STORE_BACKEND")
	if storeBackend == "" {
		storeBackend = "postgres"
	}

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
		workspaceJupyterImage = "harbor.lan/noryx-environments/noryx-python:0.2.2"
	}
	workspaceVSCodeImage := os.Getenv("NORYX_WORKSPACE_VSCODE_IMAGE")
	if workspaceVSCodeImage == "" {
		workspaceVSCodeImage = "harbor.lan/noryx-environments/noryx-python:0.2.2"
	}
	workspaceCPU := os.Getenv("NORYX_WORKSPACE_CPU")
	if workspaceCPU == "" {
		workspaceCPU = "500m"
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

	return Config{
		BackendVersion:                   backendVersion,
		FrontendVersion:                  frontendVersion,
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
		WorkspaceJupyterImage:            workspaceJupyterImage,
		WorkspaceVSCodeImage:             workspaceVSCodeImage,
		WorkspaceCPU:                     workspaceCPU,
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
		WorkspaceProfilePVCAccessMode:    workspaceProfilePVCAccessMode,
		WorkspaceProfilePVCMountPath:     workspaceProfilePVCMountPath,
		SecretsMasterKey:                 os.Getenv("NORYX_SECRETS_MASTER_KEY"),
		MinIOEndpoint:                    minioEndpoint,
		MinIOAccessKey:                   minioAccessKey,
		MinIOSecretKey:                   os.Getenv("NORYX_MINIO_SECRET_KEY"),
		MinIOUseSSL:                      os.Getenv("NORYX_MINIO_USE_SSL") == "true",
		MinIORegion:                      minioRegion,
	}
}
