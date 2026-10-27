package handlers

import (
	"path"
	"regexp"
	"strings"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/workspacekind"
)

// A Dockerfile written in the environment screen names its base the way a
// person does - "noryx-jupyter" - and not the way a registry does. kaniko
// resolves a bare name on Docker Hub, which has no such image, so every
// environment started from the blank template failed with "authentication
// required" against index.docker.io: a message about a registry nobody asked
// for. The platform knows which image it runs for each of its environments,
// and that reference is the only one a build on this installation can mean.

// systemBaseAliases are the names a person may write for a platform
// environment, each resolved to the image this installation is configured
// to run. "noryx-python" is the old name of the Jupyter image and still the
// one the blank template carried.
func (h Handlers) systemBaseAliases() map[string]string {
	aliases := map[string]string{}
	add := func(image string, names ...string) {
		image = strings.TrimSpace(image)
		if image == "" {
			return
		}
		for _, name := range names {
			aliases[name] = image
		}
		if base := path.Base(imageRepository(image)); base != "" && base != "." {
			aliases[base] = image
		}
	}
	// Only the platform's own names. "python" is a Docker Hub image somebody
	// may genuinely mean, and a build is not the place to guess otherwise.
	add(h.workspaceJupyterImage, "noryx-python", "noryx-jupyter")
	add(h.workspaceVSCodeImage, "noryx-vscode")
	add(h.workspaceRStudioImage, "noryx-rstudio")
	for _, kind := range workspacekind.All() {
		if kind.DefaultImage != nil {
			add(kind.DefaultImage(), "noryx-"+kind.ID)
		}
	}
	return aliases
}

var fromLine = regexp.MustCompile(`(?i)^(\s*FROM\s+)((?:--[\w=/.-]+\s+)*)([^\s]+)(.*)$`)

// qualifyDockerfileBase rewrites every FROM that names a platform
// environment without its registry into the reference the platform runs.
// A reference that already names a registry, or a name the platform does
// not know, is left exactly as written: the person may mean Docker Hub.
func (h Handlers) qualifyDockerfileBase(content string) string {
	aliases := h.systemBaseAliases()
	if len(aliases) == 0 {
		return content
	}
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		match := fromLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		reference := match[3]
		if strings.Contains(reference, "/") || strings.HasPrefix(reference, "$") {
			continue
		}
		name, tag, hasTag := strings.Cut(reference, ":")
		configured, known := aliases[strings.ToLower(name)]
		if !known {
			continue
		}
		resolved := configured
		if hasTag && tag != "" && tag != "latest" {
			// A tag somebody chose on purpose is kept, on the repository the
			// platform knows.
			resolved = imageRepository(configured) + ":" + tag
		}
		lines[i] = match[1] + match[2] + resolved + match[4]
	}
	return strings.Join(lines, "\n")
}

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// buildFailureReason reads what the build said last. The pod is deleted
// with its job within the hour, and a reason that is not copied out of it
// then is a reason nobody will ever read.
func (h Handlers) buildFailureReason(jobName string) string {
	reader, ok := h.runtime.(noryxruntime.JobLogReader)
	if !ok || strings.TrimSpace(jobName) == "" {
		return ""
	}
	logs, err := reader.GetJobLogs(jobName, 60)
	if err != nil {
		return ""
	}
	return failureReasonFrom(logs.Logs)
}

// failureReasonFrom picks the last line that names an error, stripped of
// the colour codes a build tool writes for a terminal, and bounded so a
// stack trace does not become a column.
func failureReasonFrom(logs string) string {
	lines := strings.Split(ansiEscape.ReplaceAllString(logs, ""), "\n")
	reason := ""
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if reason == "" {
			reason = line
		}
		lower := strings.ToLower(line)
		if strings.Contains(lower, "error") || strings.Contains(lower, "failed") || strings.Contains(lower, "denied") {
			reason = line
			break
		}
	}
	if len(reason) > 500 {
		reason = reason[:500] + "…"
	}
	return reason
}
