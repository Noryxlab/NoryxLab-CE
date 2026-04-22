package runtime

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type PodSpec struct {
	PodName    string
	Image      string
	Command    []string
	Args       []string
	Env        []EnvVar
	Labels     map[string]string
	PullSecret string
}

type BuildSpec struct {
	JobName            string
	ContextGitURL      string
	GitRef             string
	DockerfilePath     string
	ContextPath        string
	DestinationImage   string
	PullSecret         string
	RegistrySecretName string
	Labels             map[string]string
}

type Runner interface {
	CreatePod(spec PodSpec) error
	CreateBuild(spec BuildSpec) error
}
