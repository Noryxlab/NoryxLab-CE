package runtime

type EnvVar struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type PodSpec struct {
	PodName                 string
	Image                   string
	Command                 []string
	Args                    []string
	Env                     []EnvVar
	Ports                   []int
	CPURequest              string
	CPULimit                string
	MemRequest              string
	MemLimit                string
	EphemeralStorageRequest string
	EphemeralStorageLimit   string
	Labels                  map[string]string
	PullSecret              string
	Volumes                 []PersistentVolumeClaimMount
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

type JobSpec struct {
	JobName                 string
	Image                   string
	Command                 []string
	Args                    []string
	Env                     []EnvVar
	CPURequest              string
	CPULimit                string
	MemRequest              string
	MemLimit                string
	EphemeralStorageRequest string
	EphemeralStorageLimit   string
	PullSecret              string
	Volumes                 []PersistentVolumeClaimMount
	Labels                  map[string]string
}

type PersistentVolumeClaimSpec struct {
	Name             string
	StorageClassName string
	Size             string
	AccessModes      []string
	Labels           map[string]string
}

type PersistentVolumeClaimMount struct {
	ClaimName string
	MountPath string
	ReadOnly  bool
}

type Runner interface {
	CreatePersistentVolumeClaim(spec PersistentVolumeClaimSpec) error
	DeletePersistentVolumeClaim(name string) error
	CreatePod(spec PodSpec) error
	DeletePod(name string) error
	CreateService(spec ServiceSpec) error
	DeleteService(name string) error
	CreateBuild(spec BuildSpec) error
	CreateJob(spec JobSpec) error
	DeleteJob(name string) error
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

type WorkspaceReadiness interface {
	IsServiceReady(serviceName string) (bool, error)
}

type WorkspaceRuntimeInfo struct {
	WorkspaceID string `json:"workspaceId"`
	ProjectID   string `json:"projectId"`
	Kind        string `json:"kind"`
	PodName     string `json:"podName"`
	ServiceName string `json:"serviceName"`
	Image       string `json:"image"`
	AccessToken string `json:"accessToken"`
}

type WorkspaceDiscovery interface {
	ListWorkspaces() ([]WorkspaceRuntimeInfo, error)
}

type BuildRuntimeInfo struct {
	BuildID          string `json:"buildId"`
	ProjectID        string `json:"projectId"`
	JobName          string `json:"jobName"`
	Status           string `json:"status"`
	GitRepository    string `json:"gitRepository"`
	GitRef           string `json:"gitRef"`
	DockerfilePath   string `json:"dockerfilePath"`
	ContextPath      string `json:"contextPath"`
	DestinationImage string `json:"destinationImage"`
}

type BuildDiscovery interface {
	ListBuilds() ([]BuildRuntimeInfo, error)
}

type JobRuntimeInfo struct {
	JobID     string `json:"jobId"`
	ProjectID string `json:"projectId"`
	JobName   string `json:"jobName"`
	Status    string `json:"status"`
	Image     string `json:"image"`
}

type JobDiscovery interface {
	ListJobs() ([]JobRuntimeInfo, error)
}

type JobLogs struct {
	PodName string `json:"podName"`
	Logs    string `json:"logs"`
}

type JobLogReader interface {
	GetJobLogs(jobName string, tailLines int) (JobLogs, error)
}
