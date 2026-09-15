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
	HardwareTier string `json:"hardwareTier,omitempty"`
	// Datasets are what this job had mounted when it ran.
	//
	// Recorded on the job rather than left to be reconstructed from the
	// project, because a project's attachments change and a result does not.
	// Asked in two years which data produced a figure, a platform that can
	// only answer "whatever was attached to that project today" has not
	// answered - and for work that ends in a medical device, that question is
	// asked by someone who does not take an approximation.
	//
	// The name and the bucket are kept beside the identifier deliberately: a
	// dataset that has since been renamed, moved or deleted still has to mean
	// something to whoever reads this.
	Datasets        []Dataset  `json:"datasets"`
	CreatedAt       time.Time  `json:"createdAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	Result          string     `json:"-"`
	ResultAvailable bool       `json:"resultAvailable"`
}

// Dataset is one mount, as it was at the moment the job started.
type Dataset struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Bucket and Prefix say what was actually mounted, which is narrower than
	// the dataset when a prefix scopes it.
	Bucket string `json:"bucket,omitempty"`
	Prefix string `json:"prefix,omitempty"`
	// ReadOnly records whether the job could have changed what it read. A
	// result produced beside a writable mount is a different kind of evidence
	// from one produced beside a read-only one.
	ReadOnly bool `json:"readOnly"`
}

func New(projectID, name, image, jobName string, command, args []string) Job {
	id := uuid.NewString()
	return Job{
		ID:        id,
		ProjectID: projectID,
		Datasets:  []Dataset{},
		Name:      name,
		Image:     image,
		Command:   command,
		Args:      args,
		JobName:   jobName,
		Status:    "submitted",
		CreatedAt: time.Now().UTC(),
	}
}
