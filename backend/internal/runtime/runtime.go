package runtime

import (
	"encoding/json"
	"time"
)

type EnvVar struct {
	Name       string
	Value      string
	SecretName string
	SecretKey  string
}

type PodSpec struct {
	PodName                 string
	Image                   string
	Command                 []string
	Args                    []string
	Env                     []EnvVar
	Ports                   []int
	ReadinessPort           int
	CPURequest              string
	CPULimit                string
	MemRequest              string
	MemLimit                string
	EphemeralStorageRequest string
	EphemeralStorageLimit   string
	Labels                  map[string]string
	PullSecret              string
	Volumes                 []PersistentVolumeClaimMount
	Secrets                 []SecretMount
	RunAsUser               int64
	RunAsGroup              int64
	FSGroup                 int64
	RestartPolicy           string
}

type ServiceSpec struct {
	Name         string
	Selector     map[string]string
	Port         int
	OwnerPodName string
}

type BuildSpec struct {
	JobName           string
	ContextGitURL     string
	GitRef            string
	DockerfilePath    string
	DockerfileContent string
	ContextPath       string
	DestinationImage  string
	// ExtraDestinations are pushed in the same pass. Kaniko builds once and
	// pushes each: a moving `latest` beside the numbered revision costs a
	// manifest, not a copy of the layers.
	ExtraDestinations  []string
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

type CronJobSpec struct {
	JobSpec
	CronJobName string
	DisplayName string
	Schedule    string
	TimeZone    string
}

type PersistentVolumeClaimSpec struct {
	Name             string
	StorageClassName string
	Size             string
	AccessModes      []string
	Labels           map[string]string
}

// StorageCapacity is how much room the cluster still has to place a volume.
//
// The figure Kubernetes does not give you. A PersistentVolumeClaim is accepted
// the moment it is written and fails silently later, at attach time, with a
// message on a pod nobody is reading - so the only way to warn an
// administrator before a person cannot work is to ask the storage layer what
// it has left.
//
// Scheduling capacity is not free disk. A provisioner schedules on what
// volumes *claim*, so a cluster with most of its disk empty can be unable to
// place one more volume; both numbers are carried because they answer
// different questions, and reporting one as the other is how an operator is
// told there is plenty of room on the morning nothing starts.
type StorageCapacity struct {
	// Available says whether the platform could measure anything at all. False
	// leaves every figure below meaningless, and the interface says so rather
	// than drawing a full gauge from zeroes.
	Available bool   `json:"available"`
	Source    string `json:"source,omitempty"`
	// Detail explains an unavailable reading: no supported storage layer, or
	// no permission to ask it.
	Detail string        `json:"detail,omitempty"`
	Nodes  []StorageNode `json:"nodes,omitempty"`
}

// StorageNode is one node's contribution, in bytes.
type StorageNode struct {
	Name string `json:"name"`
	// Schedulable is what a new volume may still claim here.
	Schedulable int64 `json:"schedulable"`
	// Claimed is what existing volumes have already reserved.
	Claimed int64 `json:"claimed"`
	// Maximum is the size of the disk the storage layer manages.
	Maximum int64 `json:"maximum"`
	// Free is the disk actually unused, which is usually far larger.
	Free int64 `json:"free"`
}

// StorageCapacityReader is optional: a platform whose storage layer cannot be
// asked still runs workloads, it just cannot warn anybody in advance.
type StorageCapacityReader interface {
	StorageCapacity() (StorageCapacity, error)
}

type PersistentVolumeClaimMount struct {
	ClaimName string
	MountPath string
	ReadOnly  bool
}

type S3VolumeSpec struct {
	Name         string
	Bucket       string
	Prefix       string
	Endpoint     string
	Region       string
	AccessKey    string
	SecretKey    string
	MountOptions string
	Labels       map[string]string
}

type SecretSpec struct {
	Name   string
	Data   map[string]string
	Labels map[string]string
}

type SecretMount struct {
	SecretName string
	MountPath  string
	ReadOnly   bool
}

type Runner interface {
	CreatePersistentVolumeClaim(spec PersistentVolumeClaimSpec) error
	DeletePersistentVolumeClaim(name string) error
	EnsureS3Volume(spec S3VolumeSpec) error
	DeleteS3Volume(name string) error
	CreatePod(spec PodSpec) error
	DeletePod(name string) error
	CreateService(spec ServiceSpec) error
	DeleteService(name string) error
	CreateBuild(spec BuildSpec) error
	CreateJob(spec JobSpec) error
	DeleteJob(name string) error
	CreateCronJob(spec CronJobSpec) error
	DeleteCronJob(name string) error
	CreateSecret(spec SecretSpec) error
	DeleteSecret(name string) error
}

type ControlSecretStore interface {
	GetControlSecret(name string) (map[string]string, bool, error)
	UpsertControlSecret(spec SecretSpec) error
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

type WorkloadMetrics struct {
	Pods                 int   `json:"pods"`
	Running              int   `json:"running"`
	Pending              int   `json:"pending"`
	CPURequestMillicores int64 `json:"cpuRequestMillicores"`
	MemoryRequestBytes   int64 `json:"memoryRequestBytes"`
}

type WorkloadMetricsInspector interface {
	GetWorkloadMetrics() (WorkloadMetrics, error)
}

type WorkspaceReadiness interface {
	IsServiceReady(serviceName string) (bool, error)
}

type PodStatus struct {
	Phase        string    `json:"phase"`
	Reason       string    `json:"reason,omitempty"`
	Message      string    `json:"message,omitempty"`
	RestartCount int       `json:"restartCount"`
	StartedAt    time.Time `json:"startedAt,omitempty"`
	// OutOfMemory reports that the kernel killed this workload for exceeding
	// its memory limit, now or on its previous life.
	//
	// A separate field rather than a string to match on: the reason travels
	// under three different names depending on where it is read from, and
	// "OOMKilled" appearing in a message is the one thing every caller would
	// otherwise have to know to look for.
	OutOfMemory   bool      `json:"outOfMemory,omitempty"`
	OutOfMemoryAt time.Time `json:"outOfMemoryAt,omitempty"`
}

// PodEvent is one thing Kubernetes recorded about a pod, in its own words. The
// translation into something a person can act on happens above this layer.
type PodEvent struct {
	Reason  string    `json:"reason"`
	Message string    `json:"message"`
	Type    string    `json:"type"`
	At      time.Time `json:"at"`
	Count   int       `json:"count"`
}

// PodEventReader is optional: a runtime that cannot report events still runs
// workloads, it just cannot explain why one is not starting.
type PodEventReader interface {
	GetPodEvents(name string) ([]PodEvent, error)
}

type PodOperator interface {
	GetPodStatus(name string) (PodStatus, error)
	GetPodLogs(name string, tailLines int) (string, error)
	RestartPod(name string) error
}

type PodRevisionOperator interface {
	GetPodManifest(name string) (json.RawMessage, error)
	RestorePodManifest(name string, manifest json.RawMessage) error
}

type WorkspaceRuntimeInfo struct {
	WorkspaceID string    `json:"workspaceId"`
	ProjectID   string    `json:"projectId"`
	Kind        string    `json:"kind"`
	PodName     string    `json:"podName"`
	ServiceName string    `json:"serviceName"`
	Image       string    `json:"image"`
	AccessToken string    `json:"accessToken"`
	CreatedAt   time.Time `json:"createdAt"`
	// Phase and the limits the pod actually carries.
	//
	// The reconciler that rebuilds a lost record used to write the platform
	// defaults for size and the word "running" for status, whatever the pod
	// was doing. A workspace launched at 1 CPU and 4 GiB was then displayed as
	// 500m and 512Mi - the interface lying about a number its owner chose -
	// and one killed for memory three hours earlier was still listed as
	// running. Both were invented rather than read, so both are read now.
	Phase       string `json:"phase,omitempty"`
	CPULimit    string `json:"cpuLimit,omitempty"`
	MemoryLimit string `json:"memoryLimit,omitempty"`
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
	JobID       string     `json:"jobId"`
	ProjectID   string     `json:"projectId"`
	JobName     string     `json:"jobName"`
	Status      string     `json:"status"`
	Image       string     `json:"image"`
	CreatedAt   time.Time  `json:"createdAt"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
}

type JobDiscovery interface {
	ListJobs() ([]JobRuntimeInfo, error)
}

// JobFailureReader answers why a job ended badly, where the runtime knows
// something the job record does not.
type JobFailureReader interface {
	JobOutOfMemory(jobName string) (bool, error)
}

type CronJobRuntimeInfo struct {
	CronJobID      string     `json:"id"`
	ProjectID      string     `json:"projectId"`
	Name           string     `json:"name"`
	CronJobName    string     `json:"cronJobName"`
	Schedule       string     `json:"schedule"`
	TimeZone       string     `json:"timeZone"`
	Suspended      bool       `json:"suspended"`
	Image          string     `json:"image"`
	LastScheduleAt *time.Time `json:"lastScheduleAt,omitempty"`
	CreatedAt      time.Time  `json:"createdAt"`
}

type CronJobDiscovery interface {
	ListCronJobs() ([]CronJobRuntimeInfo, error)
}

type JobLogs struct {
	PodName string `json:"podName"`
	Logs    string `json:"logs"`
}

type JobLogReader interface {
	GetJobLogs(jobName string, tailLines int) (JobLogs, error)
}
