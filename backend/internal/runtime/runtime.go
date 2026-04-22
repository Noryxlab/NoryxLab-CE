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
	Ports      []int
	CPURequest string
	CPULimit   string
	MemRequest string
	MemLimit   string
	Labels     map[string]string
	PullSecret string
}

type ServiceSpec struct {
	Name     string
	Selector map[string]string
	Port     int
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
	CreateService(spec ServiceSpec) error
	CreateBuild(spec BuildSpec) error
}

type DeploymentStatus struct {
	Name              string `json:"name"`
	Replicas          int    `json:"replicas"`
	ReadyReplicas     int    `json:"readyReplicas"`
	AvailableReplicas int    `json:"availableReplicas"`
	UpdatedReplicas   int    `json:"updatedReplicas"`
}

type ServiceStatus struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type Inspector interface {
	ListDeployments() ([]DeploymentStatus, error)
	ListServices() ([]ServiceStatus, error)
}
