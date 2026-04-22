package handlers

import (
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/iam/keycloak"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store"
)

type Handlers struct {
	projectStore        store.ProjectStore
	buildStore          store.BuildStore
	podStore            store.PodStore
	accessStore         store.AccessStore
	runtime             runtime.Runner
	authVerifier        auth.Verifier
	keycloak            *keycloak.Client
	registryPullSecret  string
	registryPushSecret  string
	bootstrapAdminUser  string
	bootstrapAdminEmail string
}

type Options struct {
	RegistryPullSecret  string
	RegistryPushSecret  string
	BootstrapAdminUser  string
	BootstrapAdminEmail string
}

func New(
	projectStore store.ProjectStore,
	buildStore store.BuildStore,
	podStore store.PodStore,
	accessStore store.AccessStore,
	runtime runtime.Runner,
	authVerifier auth.Verifier,
	keycloakClient *keycloak.Client,
	options Options,
) Handlers {
	return Handlers{
		projectStore:        projectStore,
		buildStore:          buildStore,
		podStore:            podStore,
		accessStore:         accessStore,
		runtime:             runtime,
		authVerifier:        authVerifier,
		keycloak:            keycloakClient,
		registryPullSecret:  options.RegistryPullSecret,
		registryPushSecret:  options.RegistryPushSecret,
		bootstrapAdminUser:  options.BootstrapAdminUser,
		bootstrapAdminEmail: options.BootstrapAdminEmail,
	}
}
