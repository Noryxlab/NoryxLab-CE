// Package quota limits what one project can run at the same time.
//
// Until now a project launched until the cluster was full. On a platform where
// several customers share one installation - Essilor, Inria and IMT on the same
// EMSE cluster - the first incident is one team filling the nodes and every
// other team's workspaces failing to schedule, with nothing on screen to say
// why or whose fault it is.
//
// The limit is on what runs *at the same time*, not on what has been consumed
// over a month. That is the shape that protects a cluster, and it is the one
// that can be checked at the moment of launch rather than reconstructed from
// history. Cumulative consumption is a different feature - it answers "who
// should be billed", not "can this start" - and it is coming separately.
package quota

import (
	"strconv"
	"strings"
)

// Quota is what a project may hold at once. Zero means no limit on that
// dimension: a project with no quota row behaves exactly as before, which is
// the only migration that cannot break a running platform.
type Quota struct {
	ProjectID string `json:"projectId"`
	// MaxVCPU counts the CPU *limits* of everything running, because that is
	// what the scheduler has to find room for.
	MaxVCPU float64 `json:"maxVcpu"`
	// MaxMemoryGiB likewise counts memory limits.
	MaxMemoryGiB float64 `json:"maxMemoryGib"`
	// MaxWorkspaces is a count, not a size. A team that leaves twenty idle
	// notebooks running is a different problem from one that runs a big job,
	// and an administrator wants to say so separately.
	MaxWorkspaces int `json:"maxWorkspaces"`
	MaxJobs       int `json:"maxJobs"`
}

// Empty reports whether this quota constrains anything at all.
func (q Quota) Empty() bool {
	return q.MaxVCPU == 0 && q.MaxMemoryGiB == 0 && q.MaxWorkspaces == 0 && q.MaxJobs == 0
}

// Usage is what the project holds right now.
type Usage struct {
	VCPU       float64 `json:"vcpu"`
	MemoryGiB  float64 `json:"memoryGib"`
	Workspaces int     `json:"workspaces"`
	Jobs       int     `json:"jobs"`
}

// Refusal explains which limit stops a launch, in the words an operator needs:
// what was asked, what is already running, and what the ceiling is. "Quota
// exceeded" sends somebody to read the code.
type Refusal struct {
	Dimension string  `json:"dimension"`
	Requested float64 `json:"requested"`
	InUse     float64 `json:"inUse"`
	Limit     float64 `json:"limit"`
}

func (r Refusal) Error() string {
	return "project quota reached on " + r.Dimension + ": " +
		format(r.InUse) + " in use, " + format(r.Requested) + " requested, limit " + format(r.Limit)
}

// Allows reports whether adding one workload of this size stays within the
// quota, and says which dimension refuses when it does not.
func (q Quota) Allows(current Usage, addVCPU, addMemoryGiB float64, kind string) *Refusal {
	if q.MaxVCPU > 0 && current.VCPU+addVCPU > q.MaxVCPU {
		return &Refusal{Dimension: "vCPU", Requested: addVCPU, InUse: current.VCPU, Limit: q.MaxVCPU}
	}
	if q.MaxMemoryGiB > 0 && current.MemoryGiB+addMemoryGiB > q.MaxMemoryGiB {
		return &Refusal{Dimension: "memory (GiB)", Requested: addMemoryGiB, InUse: current.MemoryGiB, Limit: q.MaxMemoryGiB}
	}
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "workspace":
		if q.MaxWorkspaces > 0 && current.Workspaces+1 > q.MaxWorkspaces {
			return &Refusal{Dimension: "workspaces", Requested: 1, InUse: float64(current.Workspaces), Limit: float64(q.MaxWorkspaces)}
		}
	case "job":
		if q.MaxJobs > 0 && current.Jobs+1 > q.MaxJobs {
			return &Refusal{Dimension: "jobs", Requested: 1, InUse: float64(current.Jobs), Limit: float64(q.MaxJobs)}
		}
	}
	return nil
}

// format prints a size the way an operator writes it: 2 rather than 2.000000,
// and 1.5 rather than 1.5000. The message is read under pressure.
func format(value float64) string {
	text := strconv.FormatFloat(value, 'f', -1, 64)
	return text
}
