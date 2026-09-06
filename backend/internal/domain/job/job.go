package job

import (
	"time"

	"github.com/google/uuid"
)

type Job struct {
	ID        string `json:"id"`
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	Image     string `json:"image"`
	// ImageDigest is what actually ran, so a result can be traced to the code
	// that produced it rather than to a tag that has since moved.
	ImageDigest string   `json:"imageDigest,omitempty"`
	Command     []string `json:"command"`
	Args        []string `json:"args"`
	JobName     string   `json:"jobName"`
	Status      string   `json:"status"`
	// HardwareTier is the machine size this job was launched with. Kept on the
	// record so a project's usage can be measured without asking Kubernetes,
	// and so what it consumed is still known after the pod is gone.
	HardwareTier    string     `json:"hardwareTier,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	Result          string     `json:"-"`
	ResultAvailable bool       `json:"resultAvailable"`
}

func New(projectID, name, image, jobName string, command, args []string) Job {
	id := uuid.NewString()
	return Job{
		ID:        id,
		ProjectID: projectID,
		Name:      name,
		Image:     image,
		Command:   command,
		Args:      args,
		JobName:   jobName,
		Status:    "submitted",
		CreatedAt: time.Now().UTC(),
	}
}
