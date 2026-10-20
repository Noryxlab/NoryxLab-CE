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

// What a workload evicted for disk says about itself.
//
// Kubernetes reports "Evicted: Pod ephemeral local storage usage exceeds the
// total limit of containers 8Gi", which names a quota and leaves the person
// to guess what filled it. What filled it is almost always a Python
// environment or a package cache built in the home directory, because only
// three paths in a workspace are persistent and the home directory is not one
// of them - so the same mistake loses the work twice: the session is killed,
// and what it had installed would have vanished at the next stop anyway.
//
// So the notice names the cause and the one place that fixes both.
const outOfDiskNotice = "La session a écrit plus que son espace temporaire autorisé et a été arrêtée. " +
	"Seuls /mnt (le projet), ~/.noryx-profile (votre espace personnel) et /datasets survivent à un arrêt : " +
	"installez les environnements et les caches à l'un de ces emplacements."

// outOfDiskStatus reports whether this status is an eviction for ephemeral
// storage, in which case the notice replaces whatever Kubernetes had to say.
func outOfDiskStatus(status noryxruntime.PodStatus) (string, bool) {
	if !status.OutOfDisk {
		return "", false
	}
	return outOfDiskNotice, true
}
