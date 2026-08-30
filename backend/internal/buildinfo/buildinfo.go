// Package buildinfo carries facts about the binary that are fixed when it is
// compiled.
//
// The version used to come from NORYX_BACKEND_VERSION, an environment variable
// in the deployment manifest. That made it editable by anyone who could edit
// the manifest, and independent of the image actually running: the manifest
// read 0.5.188-dev while 0.5.196 was serving traffic, so the platform
// misreported itself for months.
//
// A version is not configuration. It changes when the code changes, so it is
// stamped at link time and cannot be edited afterwards.
package buildinfo

import "strings"

// Version is set at link time:
//
//	go build -ldflags "-X .../internal/buildinfo.Version=0.5.200-ee.1"
//
// It stays "dev" for a local build, which is the honest answer there.
var Version = "dev"

// Resolve returns the compiled-in version, falling back to an explicit
// override only when the binary was not stamped. The override exists so a
// developer running `go run` can label a build; it cannot mask a stamped one.
func Resolve(override string) string {
	if Version != "" && Version != "dev" {
		return Version
	}
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return trimmed
	}
	return Version
}
