package build

import (
	"time"

	"github.com/google/uuid"
)

type Build struct {
	ID                string `json:"id"`
	ProjectID         string `json:"projectId"`
	GitRepository     string `json:"gitRepository"`
	GitRef            string `json:"gitRef"`
	DockerfilePath    string `json:"dockerfilePath"`
	DockerfileContent string `json:"dockerfileContent,omitempty"`
	ContextPath       string `json:"contextPath"`
	DestinationImage  string `json:"destinationImage"`
	JobName           string `json:"jobName"`
	// Name is what a person called this environment. Without it the only
	// identity left is the image reference, and a screen has no choice but to
	// show "1cf6b279-114-test-stef:1788808618" to somebody who typed
	// "test-stef".
	Name string `json:"name,omitempty"`
	// CommitSHA and ImageDigest are what makes a build an account of itself
	// rather than a line in a list.
	//
	// GitRef holds a branch, and a branch is a moving target: "built from
	// main" names something different every week, so a record carrying only
	// the ref cannot answer "what source produced this image" and cannot be
	// replayed. CommitSHA is resolved when the build is submitted - the commit
	// the ref pointed at, at that moment.
	//
	// ImageDigest does the same on the other side. A tag can be overwritten in
	// the registry; a digest cannot. Together they close the loop from source
	// to artifact, which is the pair an audit asks for and the pair this
	// platform could not produce before 2026-09-18.
	//
	// Both are empty when they could not be established - a private repository
	// the platform cannot read anonymously, a registry that will not answer -
	// and empty means unknown. Neither is ever guessed.
	CommitSHA   string    `json:"commitSha,omitempty"`
	ImageDigest string    `json:"imageDigest,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

func New(projectID, gitRepo, gitRef, dockerfilePath, contextPath, destinationImage, jobName string) Build {
	return Build{
		ID:               uuid.NewString(),
		ProjectID:        projectID,
		GitRepository:    gitRepo,
		GitRef:           gitRef,
		DockerfilePath:   dockerfilePath,
		ContextPath:      contextPath,
		DestinationImage: destinationImage,
		JobName:          jobName,
		Status:           "submitted",
		CreatedAt:        time.Now().UTC(),
	}
}
