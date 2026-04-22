package main

import (
	"log"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/config"
	nhttp "github.com/Noryxlab/NoryxLab-CE/backend/internal/http"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/http/handlers"
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

	h := handlers.New(
		projectStore,
		buildStore,
		podStore,
		accessStore,
		runtime,
		handlers.Options{
			RegistryPullSecret: cfg.RegistryPullSecret,
			RegistryPushSecret: cfg.RegistryPushSecret,
		},
	)

	srv := nhttp.NewServer(cfg, h)

	log.Printf("noryx-api listening on %s", cfg.ListenAddr)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
