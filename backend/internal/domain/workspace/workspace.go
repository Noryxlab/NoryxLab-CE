package workspace

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

type Workspace struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Image     string `json:"image"`
	// ImageDigest is what actually ran. A tag moves; this does not. Empty
	// means the registry could not be asked at launch - which is recorded as
	// unknown rather than guessed.
	ImageDigest  string `json:"imageDigest,omitempty"`
	PodName      string `json:"podName"`
	ServiceName  string `json:"serviceName"`
	PVCName      string `json:"pvcName"`
	PVCClass     string `json:"pvcClass"`
	PVCSize      string `json:"pvcSize"`
	PVCMountPath string `json:"pvcMountPath"`
	CPU          string `json:"cpu"`
	Memory       string `json:"memory"`
	// OwnerUserID is who launched it, and therefore whose rights filled it.
	//
	// A workspace is mounted for one person: the datasets inside it are the
	// ones that person may read, resolved at launch. Nothing recorded who
	// that was, so the only thing guarding the door was the project role -
	// and a colleague who could launch their own could open this one and read
	// what was mounted for somebody else. The right to enter is now the same
	// right that filled it.
	//
	// Empty on a row written before this field existed, or rebuilt from a
	// cluster listing with no record left to read it from. Empty means "ask
	// the project role", which is what the platform did for all of them.
	OwnerUserID string    `json:"ownerUserId,omitempty"`
	Status      string    `json:"status"`
	AccessURL   string    `json:"accessUrl"`
	AccessToken string    `json:"-"`
	CreatedAt   time.Time `json:"createdAt"`
	// Withheld names what the project attaches and this person was not
	// mounted: a dataset they hold no read on, or somebody else's repository.
	//
	// Not stored, and filled only by the launch that reports it. The filter
	// was a silent `continue`, so a project with three datasets started a
	// workspace with one and said nothing - the person sees an empty folder
	// with no way to learn whether the data is missing or they are. Saying it
	// at launch is the only moment anybody is looking.
	Withheld []string `json:"withheld,omitempty"`
}

func NewJupyter(projectID, name, image, podName, serviceName, cpu, memory, accessURL, accessToken string) Workspace {
	return Workspace{
		ID:          uuid.NewString(),
		ProjectID:   projectID,
		Kind:        "jupyter",
		Name:        name,
		Image:       image,
		PodName:     podName,
		ServiceName: serviceName,
		CPU:         cpu,
		Memory:      memory,
		Status:      "submitted",
		AccessURL:   accessURL,
		AccessToken: accessToken,
		CreatedAt:   time.Now().UTC(),
	}
}

func New(kind, projectID, name, image, podName, serviceName, cpu, memory, accessURL, accessToken string) Workspace {
	kind = strings.ToLower(strings.TrimSpace(kind))
	if kind == "" {
		kind = "jupyter"
	}
	return Workspace{
		ID:          uuid.NewString(),
		ProjectID:   projectID,
		Kind:        kind,
		Name:        name,
		Image:       image,
		PodName:     podName,
		ServiceName: serviceName,
		CPU:         cpu,
		Memory:      memory,
		Status:      "submitted",
		AccessURL:   accessURL,
		AccessToken: accessToken,
		CreatedAt:   time.Now().UTC(),
	}
}
