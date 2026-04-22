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
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

func main() {
	cfg := config.Load()

	projectStore := memory.NewProjectStore()
	buildStore := memory.NewBuildStore()
	podStore := memory.NewPodStore()
	accessStore := memory.NewAccessStore()

	var runtime noryxruntime.Runner
	if cfg.EnableK8sRuntime {
		k8sRuntime, err := k8s.NewFromInCluster(cfg.KubernetesNamespace)
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
		buildStore,
		podStore,
		accessStore,
		runtime,
		verifier,
		keycloakClient,
		handlers.Options{
			RegistryPullSecret:  cfg.RegistryPullSecret,
			RegistryPushSecret:  cfg.RegistryPushSecret,
			BootstrapAdminUser:  cfg.BootstrapAdminUser,
			BootstrapAdminEmail: cfg.BootstrapAdminEmail,
		},
	)

	srv := nhttp.NewServer(cfg, h)

	log.Printf("noryx-api listening on %s", cfg.ListenAddr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
