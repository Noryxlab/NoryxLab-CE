// Package workspacekind is the list of things a workspace can be, and the few
// details that differ between them.
//
// Three switches used to carry that knowledge, in three files: what kinds are
// allowed, how a browser reaches one, and how the proxy rewrites its paths. A
// fourth - how to start it - sat in the bootstrap builder. Adding a kind meant
// finding all four, and an edition that wanted its own could not add one at
// all without editing Community sources.
//
// So the differences are declared in one place, and a deployment registers
// what it runs. Community registers the three it ships. An edition with a
// specialised tool registers it from its own build, and Community never learns
// that tool exists - which is the point: a medical imaging viewer belongs to
// the customer who needs it, not to the open product.
package workspacekind

import (
	"sort"
	"strings"
	"sync"
)

// Start is what a kind needs to know to write its launch command.
//
// Deliberately small. A kind that needs more than this is asking the platform
// to special-case it, which is the thing this package exists to stop.
type Start struct {
	WorkspaceID string
	// AccessToken is the credential on the hop between the platform and the
	// workspace, for a tool that wants one. Most do not.
	AccessToken string
	// ProfileDir survives the pod; ProjectDir is the shared project volume.
	ProfileDir string
	ProjectDir string
}

// Kind is one thing a workspace can be.
type Kind struct {
	// ID is what a caller asks for and what is stored on the record.
	ID string
	// StripProxyPrefix is true when the application builds browser-facing URLs
	// under the public prefix itself and expects the proxy to remove it before
	// forwarding. RStudio does this; Jupyter does the opposite and wants the
	// prefix kept. Getting it backwards produces a blank page and no error,
	// which is why it is a declared property and not a guess.
	StripProxyPrefix bool
	// AccessURL is where the browser goes, relative to the platform root.
	AccessURL func(workspaceID string) string
	// DefaultImage names what to run when the caller asked for no image. A
	// function rather than a string, because the answer is deployment
	// configuration - one registry on one site, another elsewhere - and a kind
	// that hard-coded it would only ever run where its author works.
	//
	// Returning "" means this build has the kind but no image configured for
	// it, which the caller must refuse rather than paper over: falling back to
	// some other kind's image starts the wrong application and blames the user.
	DefaultImage func() string
	// DefaultTier is the resource tier to give this kind when the caller asked
	// for none, by tier id.
	//
	// The platform default suits an editor. It does not suit every tool: a
	// medical imaging viewer rendering a volume in software needs several times
	// the memory of a text editor, and a user who never chose a tier should not
	// have to learn that from a workload killed mid-session.
	//
	// Advisory, not binding. Tier ids are deployment configuration and a site
	// may not have the one named here, in which case the platform default
	// applies - a kind that refused to start because it could not have its
	// preferred size would be worse than one that starts smaller.
	DefaultTier func() string
	// StartLines are the shell lines that launch it, the last of which is
	// expected to exec.
	StartLines func(start Start) []string
}

var (
	mu    sync.RWMutex
	kinds = map[string]Kind{}
)

// Register adds a kind. Called from init functions, so a build either contains
// a kind or does not - there is no runtime flag that half-enables one.
func Register(kind Kind) {
	id := normalise(kind.ID)
	if id == "" || kind.StartLines == nil {
		return
	}
	kind.ID = id
	mu.Lock()
	defer mu.Unlock()
	kinds[id] = kind
}

// Lookup returns a registered kind.
func Lookup(id string) (Kind, bool) {
	mu.RLock()
	defer mu.RUnlock()
	kind, ok := kinds[normalise(id)]
	return kind, ok
}

// Allowed reports whether this build can run that kind.
func Allowed(id string) bool {
	_, ok := Lookup(id)
	return ok
}

// IDs lists what this build can run, sorted, for an interface that offers a
// choice rather than hard-coding one.
func IDs() []string {
	mu.RLock()
	defer mu.RUnlock()
	ids := make([]string, 0, len(kinds))
	for id := range kinds {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func normalise(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}
