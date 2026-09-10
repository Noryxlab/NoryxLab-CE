package handlers

import (
	"strings"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/app"
	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

func TestNormalizeAppSlugRemovesAccents(t *testing.T) {
	tests := map[string]string{
		"Météo Couilly":        "meteo-couilly",
		"Évaluation de modèle": "evaluation-de-modele",
		" déjà--propre ":       "deja-propre",
	}
	for input, expected := range tests {
		if got := normalizeAppSlug(input); got != expected {
			t.Fatalf("normalizeAppSlug(%q) = %q, want %q", input, got, expected)
		}
	}
}

// An app must be able to run what the platform just installed for it.
//
// The bootstrap installed requirements into the project venv and into
// ~/.local, then ran the launch command with neither on PATH - so a project
// whose requirements.txt held streamlit died with "streamlit: not found",
// after installing streamlit. The workspace bootstrap had always exported it;
// the app one never did, which is why the same file worked in one and not the
// other.
func TestAppBootstrapRunsWhatItInstalled(t *testing.T) {
	script := appBootstrapScript(8501, []string{"streamlit", "run", "app.py", "--server.port", "8501"}, nil)
	for _, expected := range []string{
		"export PATH=/mnt/.venv/bin:$PATH",
		"export PATH=$HOME/.local/bin:$PATH",
	} {
		if !strings.Contains(script, expected) {
			t.Fatalf("app bootstrap missing %q:\n%s", expected, script)
		}
	}
	// The launch command becomes the container's process, so a stop signal
	// reaches the server rather than the shell that started it.
	if !strings.Contains(script, "exec 'streamlit' 'run' 'app.py'") {
		t.Fatalf("the launch command is not exec'd:\n%s", script)
	}
	// And it runs from the project, where the file it names actually is.
	if !strings.Contains(script, "cd /mnt") {
		t.Fatalf("the app does not start in the project directory:\n%s", script)
	}
}

// An application holds a pod for as long as it is published, and held it for
// free: the hardware tier it was launched with was never stored, so it counted
// for nothing in its project's consumption or against its quota. Workspaces and
// jobs were counted all along - an app was the one workload that was invisible.
func TestProjectUsageCountsRunningApps(t *testing.T) {
	apps := memory.NewAppStore()
	record := app.NewWithKind("app", "project-1", "meteo", "meteo", "image", nil, nil, 8501, "pod", "svc", "/apps/meteo")
	record.Status = "running"
	record.HardwareTier = "small"
	if err := apps.Create(record); err != nil {
		t.Fatalf("create: %v", err)
	}

	h := Handlers{
		appStore:          apps,
		workspaceStore:    memory.NewWorkspaceStore(),
		hardwareTierStore: memory.NewHardwareTierStore(),
	}
	usage, err := h.projectUsage("project-1")
	if err != nil {
		t.Fatalf("usage: %v", err)
	}
	if usage.Apps != 1 {
		t.Fatalf("project usage counts %d apps, want 1", usage.Apps)
	}
}

// An application whose node Kubernetes cannot reach is not a running one.
//
// The phase switch handled failed, succeeded, pending and running, and let
// "unknown" fall through - so the stored value survived and applications whose
// node had been gone for weeks were still reported as running. A call to them
// answered 502 from a screen that said everything was fine, which is the worst
// combination: the platform asserting health it has no evidence for.
// L'interface est encastree plutot que reimplementee : le test ne s'interesse
// qu'a la phase du pod, et toute autre methode appelee par erreur produira une
// panique explicite au lieu d'un zero silencieux.
type stubPodStatus struct {
	noryxruntime.Runner
	phase string
}

func (s stubPodStatus) GetPodStatus(string) (noryxruntime.PodStatus, error) {
	return noryxruntime.PodStatus{Phase: s.phase}, nil
}
func (s stubPodStatus) GetPodLogs(string, int) (string, error) { return "", nil }
func (s stubPodStatus) RestartPod(string) error                { return nil }

func TestUnreachableApplicationIsNotReportedAsRunning(t *testing.T) {
	for phase, want := range map[string]string{
		"unknown":   "unchecked",
		"failed":    "failed",
		"succeeded": "stopped",
		"pending":   "launching",
	} {
		h := Handlers{runtime: stubPodStatus{phase: phase}}
		item := h.enrichAppRuntimeStatus(app.App{PodName: "app-1", Status: "running"})
		if item.Status != want {
			t.Fatalf("pod phase %q reported as %q, want %q", phase, item.Status, want)
		}
	}
}

// A launch word carrying spaces stays one word.
//
// The bootstrap used to join command and args into a single line and hand it
// to the shell, which split it again. An application declared the way the API
// documents it - {"command":["/bin/sh","-lc"],"args":["FOO=1 run.sh"]} - became
// `exec /bin/sh -lc FOO=1 run.sh`; sh took "FOO=1" as its whole command
// string, ran an assignment, and exited 0. Kubernetes then reported a
// container that had "succeeded", and the application never started.
func TestAppBootstrapKeepsLaunchWordsIntact(t *testing.T) {
	script := appBootstrapScript(9000, []string{"/bin/sh", "-lc", "FOO=1 exec /repos/app/run.sh"}, nil)

	if !strings.Contains(script, "exec '/bin/sh' '-lc' 'FOO=1 exec /repos/app/run.sh'") {
		t.Fatalf("the multi-word argument was split by the shell:\n%s", script)
	}
	// A quote inside a word must not end the quoting and turn the rest into
	// shell code.
	hostile := appBootstrapScript(9000, []string{"sh", "-c", "echo 'a'; rm -rf /"}, nil)
	if strings.Contains(hostile, "exec 'sh' '-c' 'echo 'a'; rm -rf /'") {
		t.Fatalf("a quote inside an argument escaped its quoting:\n%s", hostile)
	}
}

// An application with no launch command falls through to the other entrypoints.
func TestAppBootstrapWithoutACommandFallsThrough(t *testing.T) {
	script := appBootstrapScript(9000, nil, nil)
	if !strings.Contains(script, "if [ false ]; then") {
		t.Fatalf("an app with no command must not take the command branch:\n%s", script)
	}
	if !strings.Contains(script, "/mnt/app.sh") {
		t.Fatalf("the /mnt/app.sh fallback is gone:\n%s", script)
	}
}
