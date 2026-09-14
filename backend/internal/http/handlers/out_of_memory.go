package handlers

import noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"

// What a workload killed for memory says about itself.
//
// Kubernetes reports this as "OOMKilled", which is accurate and tells a
// researcher nothing: it names a kernel mechanism, not what they should do.
// Worse, the workload simply disappears - a workspace that vanished mid-session
// and an application that stopped answering look identical to a crash, a reaper,
// or somebody else pressing a button.
//
// So the platform says the two things the person actually needs: why it closed,
// and the one action that changes the outcome. Raising the tier is the only
// remedy a user has; retrying the same workload in the same tier reproduces the
// same kill, and letting them discover that themselves is what turns a clear
// failure into a support ticket.
//
// One sentence, one place. The three families of workload - workspaces, jobs
// and applications - surface it through different fields, and a message written
// three times drifts three ways.
const outOfMemoryNotice = "Fermé pour cause de manque de mémoire (OOM kill), veuillez augmenter le tier de ressources pour relancer la charge."

// outOfMemoryStatus reports whether this status is an out-of-memory kill, in
// which case the notice replaces whatever Kubernetes had to say.
func outOfMemoryStatus(status noryxruntime.PodStatus) (string, bool) {
	if !status.OutOfMemory {
		return "", false
	}
	return outOfMemoryNotice, true
}
