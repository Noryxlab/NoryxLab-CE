package config

import "os"

type Config struct {
	ListenAddr                    string
	KubernetesNamespace           string
	WorkloadNamespace             string
	EnableK8sRuntime              bool
	RegistryPullSecret            string
	RegistryPushSecret            string
	AuthMode                      string
	OIDCIssuerURL                 string
	OIDCJWKSURL                   string
	OIDCAudience                  string
	BootstrapAdminUser            string
	BootstrapAdminEmail           string
	KeycloakBaseURL               string
	KeycloakRealm                 string
	KeycloakAdminRealm            string
	KeycloakAdminUser             string
	KeycloakAdminPass             string
	WorkspaceJupyterImage         string
	WorkspaceVSCodeImage          string
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

func Load() Config {
	listenAddr := os.Getenv("NORYX_LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8080"
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

	return Config{
		ListenAddr:                    listenAddr,
		KubernetesNamespace:           namespace,
		WorkloadNamespace:             workloadNamespace,
		EnableK8sRuntime:              enableRuntime,
		RegistryPullSecret:            pullSecret,
		RegistryPushSecret:            pushSecret,
		AuthMode:                      authMode,
		OIDCIssuerURL:                 oidcIssuer,
		OIDCJWKSURL:                   os.Getenv("NORYX_OIDC_JWKS_URL"),
		OIDCAudience:                  os.Getenv("NORYX_OIDC_AUDIENCE"),
		BootstrapAdminUser:            os.Getenv("NORYX_BOOTSTRAP_ADMIN_USER"),
		BootstrapAdminEmail:           os.Getenv("NORYX_BOOTSTRAP_ADMIN_EMAIL"),
		KeycloakBaseURL:               keycloakBaseURL,
		KeycloakRealm:                 keycloakRealm,
		KeycloakAdminRealm:            keycloakAdminRealm,
		KeycloakAdminUser:             keycloakAdminUser,
		KeycloakAdminPass:             os.Getenv("NORYX_KEYCLOAK_ADMIN_PASSWORD"),
		WorkspaceJupyterImage:         workspaceJupyterImage,
		WorkspaceVSCodeImage:          workspaceVSCodeImage,
		WorkspaceCPU:                  workspaceCPU,
		WorkspaceMemory:               workspaceMemory,
		WorkspacePVCEnabled:           workspacePVCEnabled == "true",
		WorkspacePVCClass:             workspacePVCClass,
		WorkspacePVCSize:              workspacePVCSize,
		WorkspacePVCAccessMode:        workspacePVCAccessMode,
		WorkspacePVCMountPath:         workspacePVCMountPath,
		WorkspaceProfilePVCEnabled:    workspaceProfilePVCEnabled == "true",
		WorkspaceProfilePVCClass:      workspaceProfilePVCClass,
		WorkspaceProfilePVCSize:       workspaceProfilePVCSize,
		WorkspaceProfilePVCAccessMode: workspaceProfilePVCAccessMode,
		WorkspaceProfilePVCMountPath:  workspaceProfilePVCMountPath,
	}
}
