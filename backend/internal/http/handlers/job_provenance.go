package handlers

import (
	"strings"

	jobdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/job"
)

// What a run read and which code it ran, written down when it starts.
//
// The platform already recorded the image digest and the datasets, with a
// comment saying why: asked in two years which data produced a figure, "what
// happened to be attached to that project today" is not an answer. Two halves
// of that promise were missing.
//
// The first was a defect rather than a gap: the extracts were put on the
// record and the INSERT did not carry them, so the provenance was exact in
// memory and gone at the first reload. The second is the code - a repository
// is cloned at a ref, and a ref moves. Keeping only "main" answers the
// question with today's code.
//
// Resolving the commit is best effort, always. A forge that does not reply,
// a credential that has expired, a network that is closed - none of them is a
// reason to refuse to run somebody's job. An empty commit is recorded as
// empty, which reads as "unknown" rather than as a promise.
//
// The resolver is the one build_provenance.go already uses for environment
// builds: git's smart HTTP ref discovery, no git binary and no clone. That
// matters here beyond not repeating code - the backend ships distroless, so a
// resolver shelling out to `git` would have failed every time, silently, and
// recorded "unknown" for every run.

// jobRepositoriesFrom turns what the resolver mounted into what the record
// keeps, resolving each ref to the commit it points at now.
func jobRepositoriesFrom(repos []workspaceAttachedRepo, resolve func(url, ref string) string) []jobdomain.Repository {
	out := make([]jobdomain.Repository, 0, len(repos))
	for _, item := range repos {
		ref := strings.TrimSpace(item.DefaultRef)
		entry := jobdomain.Repository{
			Name: strings.TrimSpace(item.Name),
			URL:  strings.TrimSpace(item.URL),
			Ref:  ref,
		}
		if resolve != nil && entry.URL != "" {
			entry.Commit = resolve(entry.URL, ref)
		}
		out = append(out, entry)
	}
	return out
}
