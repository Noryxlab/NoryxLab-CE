// Package flow is a graph of jobs, and what running one means.
//
// It is deliberately not the workflow package next door: that one is what an
// agent executes (ADR-046), a sequence of reasoning and approval steps with
// no graph. This one is compute - "mesh this, then simulate it, and render
// both while you are at it" - and the shape is a graph because that work is
// one.
//
// Three properties, and each is a decision.
//
// A step is a job, not a second kind of execution. It goes down the same
// launch path as any other job, so it gets the same mounts, the same egress
// profile, the same rights resolution and the same provenance record - image
// digest, datasets, extracts, repositories at their commit. A pipeline whose
// steps ran through a path of their own would diverge from ordinary runs, and
// the day that showed would be the day a result could not be explained.
//
// The state lives in the database and the scheduler is a watcher, not a
// goroutine holding the graph in memory. The workflow package learned this
// from LangGraph and it is just as true here: a flow that only existed in a
// process would die with the next rollout, halfway through, with pods still
// running and nothing to reconcile them against.
//
// And a failure stops what depends on it, not everything. Branches that do
// not need the failed step keep going. Failing the whole graph the instant
// one step fails would throw away work that was fine - and on a mesh that
// took forty minutes, that is the difference between a usable tool and one
// people stop using.
package flow

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Step statuses. A step is in exactly one of these.
const (
	StepPending   = "pending"
	StepRunning   = "running"
	StepSucceeded = "succeeded"
	StepFailed    = "failed"
	// StepSkipped is a step that will never run because something it depends
	// on failed. It is not a failure of its own, and saying so matters: a
	// person looking at twelve red steps must be able to see which one broke.
	StepSkipped = "skipped"
	// StepReused is a step a resume did not re-run because it had already
	// succeeded. Recorded as its own status rather than as "succeeded", so a
	// run never claims to have executed something it did not.
	StepReused = "reused"
)

// Run statuses.
const (
	RunRunning   = "running"
	RunSucceeded = "succeeded"
	RunFailed    = "failed"
	RunCancelled = "cancelled"
)

// Step is one node: a job specification plus what it waits for.
type Step struct {
	// Key identifies the step inside its flow and is what DependsOn names. It
	// is stable across runs and resumes, which is what lets a resume say "this
	// one already succeeded".
	Key          string   `json:"key"`
	Name         string   `json:"name,omitempty"`
	Image        string   `json:"image"`
	Command      []string `json:"command,omitempty"`
	Args         []string `json:"args,omitempty"`
	HardwareTier string   `json:"hardwareTier,omitempty"`
	DependsOn    []string `json:"dependsOn,omitempty"`
}

// Flow is the graph itself.
type Flow struct {
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	OwnerUserID string `json:"ownerUserId,omitempty"`
	Steps       []Step `json:"steps"`
	// MaxParallel bounds how many steps run at once. A graph that fans out to
	// fifty steps must not ask the cluster for fifty pods because its author
	// drew fifty boxes.
	MaxParallel int       `json:"maxParallel"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// DefaultMaxParallel is what a flow gets when its author did not say.
const DefaultMaxParallel = 3

// Run is one execution of a flow.
type Run struct {
	ID         string     `json:"id"`
	FlowID     string     `json:"flowId"`
	ProjectID  string     `json:"projectId"`
	Status     string     `json:"status"`
	StartedBy  string     `json:"startedBy"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	// ResumeOf names the run this one continues, empty for a first run.
	ResumeOf string `json:"resumeOf,omitempty"`
}

// StepRun is one step inside one execution.
type StepRun struct {
	RunID   string `json:"runId"`
	StepKey string `json:"stepKey"`
	Status  string `json:"status"`
	// JobID is the job this step launched, or the job a reused step was
	// satisfied by - so the provenance of a reused step still points at the
	// run that actually produced it.
	JobID      string     `json:"jobId,omitempty"`
	StartedAt  *time.Time `json:"startedAt,omitempty"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
	// Reason says why a step is in its state when the state alone does not:
	// which upstream failure skipped it, which run it was reused from.
	Reason string `json:"reason,omitempty"`
}

// Validate refuses a graph that cannot be executed, naming what is wrong.
//
// At declaration rather than at run time: a cycle discovered while running
// leaves pods behind and a person waiting, and the author is no longer at the
// keyboard.
func Validate(steps []Step) error {
	if len(steps) == 0 {
		return fmt.Errorf("a flow needs at least one step")
	}
	byKey := make(map[string]Step, len(steps))
	for _, step := range steps {
		key := strings.TrimSpace(step.Key)
		if key == "" {
			return fmt.Errorf("every step needs a key")
		}
		if _, duplicate := byKey[key]; duplicate {
			return fmt.Errorf("two steps share the key %q", key)
		}
		if strings.TrimSpace(step.Image) == "" {
			return fmt.Errorf("step %q has no image", key)
		}
		byKey[key] = step
	}
	for _, step := range steps {
		for _, dependency := range step.DependsOn {
			dependency = strings.TrimSpace(dependency)
			if dependency == step.Key {
				return fmt.Errorf("step %q depends on itself", step.Key)
			}
			if _, known := byKey[dependency]; !known {
				return fmt.Errorf("step %q depends on %q, which is not in this flow", step.Key, dependency)
			}
		}
	}
	if cycle := findCycle(byKey); len(cycle) > 0 {
		return fmt.Errorf("these steps depend on each other in a circle: %s", strings.Join(cycle, " -> "))
	}
	return nil
}

// findCycle returns one cycle, named, or nil. Naming it matters: "there is a
// cycle" in a graph of thirty steps sends somebody reading all thirty.
func findCycle(byKey map[string]Step) []string {
	const (
		unvisited = 0
		visiting  = 1
		done      = 2
	)
	state := make(map[string]int, len(byKey))
	var stack []string
	var walk func(key string) []string
	walk = func(key string) []string {
		state[key] = visiting
		stack = append(stack, key)
		for _, dependency := range byKey[key].DependsOn {
			dependency = strings.TrimSpace(dependency)
			switch state[dependency] {
			case visiting:
				// Couper la pile au point ou le cycle commence, pour ne
				// montrer que le cycle et pas le chemin qui y menait.
				for index, item := range stack {
					if item == dependency {
						return append(append([]string{}, stack[index:]...), dependency)
					}
				}
				return append(append([]string{}, stack...), dependency)
			case unvisited:
				if cycle := walk(dependency); cycle != nil {
					return cycle
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[key] = done
		return nil
	}
	keys := make([]string, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if state[key] == unvisited {
			if cycle := walk(key); cycle != nil {
				return cycle
			}
		}
	}
	return nil
}

// Runnable names the steps that may start now.
//
// A step runs when it is pending and everything it depends on has succeeded -
// or was reused, which is the same thing for a dependant: the output exists.
// The result is capped by how many may run at once and sorted, so two
// readings of the same state launch the same steps in the same order.
func Runnable(steps []Step, states map[string]string, maxParallel, running int) []string {
	if maxParallel <= 0 {
		maxParallel = DefaultMaxParallel
	}
	free := maxParallel - running
	if free <= 0 {
		return nil
	}
	ready := []string{}
	for _, step := range steps {
		if states[step.Key] != StepPending {
			continue
		}
		satisfied := true
		for _, dependency := range step.DependsOn {
			switch states[strings.TrimSpace(dependency)] {
			case StepSucceeded, StepReused:
			default:
				satisfied = false
			}
		}
		if satisfied {
			ready = append(ready, step.Key)
		}
	}
	sort.Strings(ready)
	if len(ready) > free {
		ready = ready[:free]
	}
	return ready
}

// Propagate marks as skipped every pending step that can no longer run,
// saying which failure stopped it.
//
// Transitive on purpose: a step three hops downstream of a failure is just as
// stuck as its neighbour, and leaving it "pending" would make a finished run
// look like it was still going.
func Propagate(steps []Step, states map[string]string) map[string]string {
	reasons := map[string]string{}
	byKey := make(map[string]Step, len(steps))
	for _, step := range steps {
		byKey[step.Key] = step
	}
	for changed := true; changed; {
		changed = false
		for _, step := range steps {
			if states[step.Key] != StepPending {
				continue
			}
			for _, dependency := range step.DependsOn {
				dependency = strings.TrimSpace(dependency)
				switch states[dependency] {
				case StepFailed:
					states[step.Key] = StepSkipped
					reasons[step.Key] = fmt.Sprintf("%s failed", dependency)
					changed = true
				case StepSkipped:
					states[step.Key] = StepSkipped
					reasons[step.Key] = fmt.Sprintf("%s was skipped", dependency)
					changed = true
				}
				if states[step.Key] == StepSkipped {
					break
				}
			}
		}
	}
	return reasons
}

// Settled says whether a run is over, and how it ended.
//
// A run that skipped steps ends failed, not succeeded: something was meant to
// happen and did not, and a green run with skipped steps in it would be a lie
// told by a colour.
func Settled(steps []Step, states map[string]string) (bool, string) {
	failed, skipped := false, false
	for _, step := range steps {
		switch states[step.Key] {
		case StepPending, StepRunning:
			return false, RunRunning
		case StepFailed:
			failed = true
		case StepSkipped:
			skipped = true
		}
	}
	if failed || skipped {
		return true, RunFailed
	}
	return true, RunSucceeded
}
