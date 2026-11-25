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
	// OwnerUserID is who launched it, and therefore whose rights filled it.
	//
	// A job is mounted like a workspace: the datasets inside are the ones its
	// launcher may read, resolved when it starts. Nothing recorded who that
	// was, so its logs - which routinely print a few rows to check a script -
	// were readable by anybody holding the project's launch role, including
	// people with no right at all over that data.
	//
	// Empty on a row written before this field existed: that falls back to
	// the project role, which is what the platform did for all of them.
	OwnerUserID string `json:"ownerUserId,omitempty"`
	Status      string `json:"status"`
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
	Datasets []Dataset `json:"datasets"`
	// Extracts names the frozen selections this run read, for the same reason
	// the datasets are recorded: a project's attachments change and a result
	// does not. "Which data produced this figure" is answerable only if the
	// answer was written down when the run started.
	Extracts []string `json:"extracts,omitempty"`
	// Repositories are the repositories this run cloned, and at which commit.
	//
	// The ref is what somebody asked for; the commit is what ran. "main" moves,
	// so a record that keeps only the ref answers "which code produced this
	// figure" with the code of today rather than the code of that day - which
	// is not an answer. The commit is resolved when the job starts and never
	// blocks the launch: a forge that does not reply leaves it empty, and an
	// empty commit says "unknown" rather than pretending.
	Repositories []Repository `json:"repositories,omitempty"`
	// RerunOf names the run this one replays, empty for a first run.
	//
	// Kept so a result can be traced to the one it was meant to reproduce, and
	// so a chain of replays reads as a chain rather than as unrelated jobs
	// with similar names.
	RerunOf string `json:"rerunOf,omitempty"`
	// Divergences are the ways this replay could not be identical: an image
	// that no longer resolves to the same digest, a dataset no longer
	// attached or no longer readable by whoever pressed the button, a commit
	// that has moved.
	//
	// Written down rather than refused. Refusing would make the button
	// useless the first time anything changed; hiding it would be worse - a
	// replay that silently reads different data and produces a different
	// figure is exactly the failure this record exists to prevent.
	Divergences     []string   `json:"divergences,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	CompletedAt     *time.Time `json:"completedAt,omitempty"`
	Result          string     `json:"-"`
	ResultAvailable bool       `json:"resultAvailable"`
}

// Repository is one checkout, as it was at the moment the job started.
type Repository struct {
	Name string `json:"name"`
	URL  string `json:"url"`
	Ref  string `json:"ref,omitempty"`
	// Commit is the hash the ref pointed at when this run started. Empty when
	// it could not be resolved, which is a fact worth keeping rather than a
	// reason to refuse the run.
	Commit string `json:"commit,omitempty"`
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
	// ReadOnlyReason says why, when the person could have written to it.
	//
	// A mount can be read-only because the person holds Read, or because the
	// co-mount rule demoted it (ADR-055): the run also mounts regulated data.
	// The second case is the one that surprises - the person owns the
	// dataset and is refused - and until 2026-11-25 nothing on the job screen
	// said it. Cédric found out from a failed job. Empty when the reason is
	// simply the person's own right.
	ReadOnlyReason string `json:"readOnlyReason,omitempty"`
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
