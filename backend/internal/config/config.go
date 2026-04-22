package config

import "os"

type Config struct {
	ListenAddr          string
	KubernetesNamespace string
	EnableK8sRuntime    bool
	RegistryPullSecret  string
	RegistryPushSecret  string
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

	pullSecret := os.Getenv("NORYX_REGISTRY_PULL_SECRET")
	if pullSecret == "" {
		pullSecret = "harbor-regcred"
	}

	pushSecret := os.Getenv("NORYX_REGISTRY_PUSH_SECRET")
	if pushSecret == "" {
		pushSecret = pullSecret
	}

	enableRuntime := os.Getenv("NORYX_ENABLE_K8S_RUNTIME") == "true"

	return Config{
		ListenAddr:          listenAddr,
		KubernetesNamespace: namespace,
		EnableK8sRuntime:    enableRuntime,
		RegistryPullSecret:  pullSecret,
		RegistryPushSecret:  pushSecret,
	}
}
