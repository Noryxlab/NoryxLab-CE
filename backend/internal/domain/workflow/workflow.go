// Package workflow is what runs when an agent works.
//
// ADR-046 keeps the agent as the thing a person sees - somebody to whom one
// leaves a standing instruction - and makes a workflow the thing that
// executes: an ordered sequence of steps, each one either a reasoning step
// handed to the model or a durable wait for a named person. The runtime, not
// the model, decides what comes next. That is the whole difference between a
// workflow and an agent left to its own devices, and it is what lets a risk
// function be shown the path before the run.
//
// Two things are borrowed from the frameworks that got this right and are
// worth naming so they are not reinvented worse. From LangGraph: state is
// persisted after every step, so a run survives the process and resumes from
// the first step that did not finish; and a step that waits for a person
// suspends the run instead of holding a goroutine open. From Temporal: a step
// that fails is retried a bounded number of times with the same idempotency
// key, so a crash between "the model answered" and "we wrote it down" cannot
// spend twice as a fresh conversation.
//
// What is deliberately not here yet: a step that calls a platform verb. The
// verbs come first (ADR-046, phase 1); a step kind declared before any verb
// exists would be a drawing. Nor is there a graph: steps are a sequence, which
// is the pattern the reference recommends where control matters, and edges
// arrive with the mandates that will drive them.
package workflow

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/agent"
)

// Step kinds. Two, and the list is closed.
const (
	// KindAgent is a reasoning step: an instruction, the actions it was
	// granted, and whatever the previous step produced as context. The model
	// writes an output; the platform records the actions it saw.
	KindAgent = "agent"
	// KindApproval is a durable wait for a named person. The run parks here
	// consuming nothing, and continues or stops on their word.
	KindApproval = "approval"
)

// Run statuses, in the vocabulary Mercure ADR-002 chose, so the kernel can be
// shared later without a translation table.
const (
	StatusPending         = "pending"
	StatusRunning         = "running"
	StatusWaitingApproval = "waiting_approval"
	StatusSucceeded       = "succeeded"
	StatusFailed          = "failed"
	StatusCancelled       = "cancelled"
)

// maxAttempts bounds how often a reasoning step is retried before the run is
// declared failed. Three, because the first failure is usually the assistant
// being briefly unreachable and the third is usually something a person has to
// look at.
const maxAttempts = 3

// Step is one unit of a definition. Index is its position; the sequence is
// the order.
type Step struct {
	Index int    `json:"index"`
	Kind  string `json:"kind"`
	Name  string `json:"name"`
	// Instruction is the standing text for an agent step, in the owner's own
	// words, and like an agent's mission it is never parsed by the platform.
	Instruction string `json:"instruction,omitempty"`
	// Actions an agent step may take, from the same closed list an agent
	// holds. A step cannot be granted what an agent cannot.
	Actions []string `json:"actions,omitempty"`
	// ApproverUserID is who an approval step waits for. Required for that
	// kind: a wait for nobody never ends.
	ApproverUserID string `json:"approverUserId,omitempty"`
}

// Definition is a workflow as somebody wrote it.
type Definition struct {
	ID          string `json:"id"`
	OwnerUserID string `json:"ownerUserId"`
	// ProjectID scopes every step, for the same reason it is required on an
	// agent: it travels in the credential signed for each reasoning step, so
	// the scope is a fact rather than something the model chooses.
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	// Schedule reuses the agent's three values. Triggers in words, and
	// platform events, are the next phase; a workflow that keeps a colleague's
	// hours is enough to prove the kernel.
	Schedule  string     `json:"schedule"`
	Steps     []Step     `json:"steps"`
	Enabled   bool       `json:"enabled"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	LastRunAt *time.Time `json:"lastRunAt,omitempty"`
}

// StepRun is what happened at one step of one run.
type StepRun struct {
	Index  int    `json:"index"`
	Status string `json:"status"`
	// Attempts counts how many times this step was started. The idempotency
	// key is built from it, so a retry is a new attempt and not a replay.
	Attempts int `json:"attempts"`
	// Output is what the step produced: a report for an agent step, the
	// decision for an approval. It is the context handed to the next step.
	Output string `json:"output,omitempty"`
	// Actions the platform saw this step take. Never read back from Output.
	Actions          []string   `json:"actions"`
	Error            string     `json:"error,omitempty"`
	ApprovedByUserID string     `json:"approvedByUserId,omitempty"`
	StartedAt        *time.Time `json:"startedAt,omitempty"`
	FinishedAt       *time.Time `json:"finishedAt,omitempty"`
}

// Run is one execution of a definition, persisted after every step so that it
// can be reconstructed with no memory of the process that started it.
type Run struct {
	ID         string     `json:"id"`
	WorkflowID string     `json:"workflowId"`
	Status     string     `json:"status"`
	Steps      []StepRun  `json:"steps"`
	Error      string     `json:"error,omitempty"`
	StartedAt  time.Time  `json:"startedAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

var (
	ErrProjectRequired = errors.New("a workflow works in a project, and none was given")
	ErrNoSteps         = errors.New("a workflow needs at least one step")
	ErrUnknownKind     = errors.New("a step must be an agent step or an approval")
	ErrNoInstruction   = errors.New("an agent step needs an instruction")
	ErrNoApprover      = errors.New("an approval step waits for a named person, and none was given")
	ErrNotWaiting      = errors.New("this run is not waiting for an approval")
	ErrWrongApprover   = errors.New("this approval waits for somebody else")
	ErrRunFinished     = errors.New("this run has finished")
)

// New writes a definition down, normalising what can be normalised. The
// steps are re-indexed by position, because the position is the truth and an
// index somebody typed is not.
func New(ownerUserID, projectID, name, schedule string, steps []Step) Definition {
	now := time.Now().UTC()
	return Definition{
		ID:          uuid.NewString(),
		OwnerUserID: strings.TrimSpace(ownerUserID),
		ProjectID:   strings.TrimSpace(projectID),
		Name:        strings.TrimSpace(name),
		Schedule:    agent.NormaliseSchedule(schedule),
		Steps:       NormaliseSteps(steps),
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// NormaliseSteps re-indexes steps by position and re-filters what each may
// hold. Applied on the way in and on the way out of the store, for the same
// reason an agent's actions are: a row written by a version that knew a grant
// this one has withdrawn must not grant it.
func NormaliseSteps(steps []Step) []Step {
	normalised := make([]Step, 0, len(steps))
	for index, step := range steps {
		normalised = append(normalised, Step{
			Index:          index,
			Kind:           strings.ToLower(strings.TrimSpace(step.Kind)),
			Name:           strings.TrimSpace(step.Name),
			Instruction:    strings.TrimSpace(step.Instruction),
			Actions:        agent.NormaliseActions(step.Actions),
			ApproverUserID: strings.TrimSpace(step.ApproverUserID),
		})
	}
	return normalised
}

// Validate reports whether this definition can be stored.
func (d Definition) Validate() error {
	if d.ProjectID == "" {
		return ErrProjectRequired
	}
	if len(d.Steps) == 0 {
		return ErrNoSteps
	}
	for _, step := range d.Steps {
		switch step.Kind {
		case KindAgent:
			if step.Instruction == "" {
				return fmt.Errorf("step %d: %w", step.Index, ErrNoInstruction)
			}
		case KindApproval:
			if step.ApproverUserID == "" {
				return fmt.Errorf("step %d: %w", step.Index, ErrNoApprover)
			}
		default:
			return fmt.Errorf("step %d: %w", step.Index, ErrUnknownKind)
		}
	}
	return nil
}

// DueAt reports whether this definition owes a run, with the same calendar an
// agent keeps.
func (d Definition) DueAt(now time.Time) bool {
	if !d.Enabled {
		return false
	}
	var every time.Duration
	switch d.Schedule {
	case agent.ScheduleHourly:
		every = time.Hour
	case agent.ScheduleDaily:
		every = 24 * time.Hour
	default:
		return false
	}
	if d.LastRunAt == nil {
		return true
	}
	return now.Sub(*d.LastRunAt) >= every
}

// NewRun starts one, with every step pending. Nothing has happened yet, and
// the row says so.
func NewRun(definition Definition) Run {
	now := time.Now().UTC()
	steps := make([]StepRun, 0, len(definition.Steps))
	for _, step := range definition.Steps {
		steps = append(steps, StepRun{Index: step.Index, Status: StatusPending, Actions: []string{}})
	}
	return Run{
		ID:         uuid.NewString(),
		WorkflowID: definition.ID,
		Status:     StatusPending,
		Steps:      steps,
		StartedAt:  now,
		UpdatedAt:  now,
	}
}

// IsTerminal reports whether anything more can happen to this run.
func (r Run) IsTerminal() bool {
	switch r.Status {
	case StatusSucceeded, StatusFailed, StatusCancelled:
		return true
	}
	return false
}

// Current is the first step that has not finished, which is where a resumed
// run picks up. This is the LangGraph checkpoint idea in one line: the
// position is derived from what was persisted, never remembered.
func (r Run) Current() (int, bool) {
	for index, step := range r.Steps {
		switch step.Status {
		case StatusSucceeded:
			continue
		default:
			return index, true
		}
	}
	return -1, false
}

// Context is what the next step is told about the one before it: the last
// finished output, which is how one agent's report becomes another's brief.
func (r Run) Context() string {
	for index := len(r.Steps) - 1; index >= 0; index-- {
		if r.Steps[index].Status == StatusSucceeded && strings.TrimSpace(r.Steps[index].Output) != "" {
			return r.Steps[index].Output
		}
	}
	return ""
}

// IdempotencyKey names one attempt of one step of one run. Handed to the
// assistant as the conversation identifier, so that a step retried after a
// crash continues a conversation instead of starting a second one that spends
// the same tokens again.
func (r Run) IdempotencyKey(stepIndex int) string {
	return fmt.Sprintf("workflow-%s-step-%d-attempt-%d", r.ID, stepIndex, r.Steps[stepIndex].Attempts)
}

// Start marks a step as begun, counting the attempt. Returns the run so the
// caller persists it before doing anything with side effects - the order that
// makes a crash recoverable.
func (r Run) Start(stepIndex int) (Run, error) {
	if r.IsTerminal() {
		return r, ErrRunFinished
	}
	now := time.Now().UTC()
	step := r.Steps[stepIndex]
	step.Status = StatusRunning
	step.Attempts++
	step.StartedAt = &now
	step.Error = ""
	r.Steps[stepIndex] = step
	r.Status = StatusRunning
	r.UpdatedAt = now
	return r, nil
}

// Succeed records a finished step and moves the run on, or finishes it when
// that was the last one.
func (r Run) Succeed(stepIndex int, output string, actions []string) Run {
	now := time.Now().UTC()
	step := r.Steps[stepIndex]
	step.Status = StatusSucceeded
	step.Output = strings.TrimSpace(output)
	if actions == nil {
		actions = []string{}
	}
	step.Actions = actions
	step.FinishedAt = &now
	r.Steps[stepIndex] = step
	r.UpdatedAt = now
	if _, more := r.Current(); !more {
		r.Status = StatusSucceeded
		r.FinishedAt = &now
	} else {
		r.Status = StatusRunning
	}
	return r
}

// Fail records a failed attempt. Below the attempt limit the step goes back
// to pending and will be tried again; at the limit the run fails and says
// which step, which is the sentence a person needs.
func (r Run) Fail(stepIndex int, cause error) Run {
	now := time.Now().UTC()
	step := r.Steps[stepIndex]
	step.Error = cause.Error()
	r.UpdatedAt = now
	if step.Attempts < maxAttempts {
		step.Status = StatusPending
		r.Steps[stepIndex] = step
		r.Status = StatusRunning
		return r
	}
	step.Status = StatusFailed
	step.FinishedAt = &now
	r.Steps[stepIndex] = step
	r.Status = StatusFailed
	r.Error = fmt.Sprintf("step %d failed after %d attempts: %s", stepIndex, step.Attempts, cause.Error())
	r.FinishedAt = &now
	return r
}

// Wait parks the run at an approval step. No goroutine, no timer: the row
// says it is waiting, and a person's decision is what moves it.
func (r Run) Wait(stepIndex int) Run {
	now := time.Now().UTC()
	step := r.Steps[stepIndex]
	step.Status = StatusWaitingApproval
	if step.StartedAt == nil {
		step.StartedAt = &now
	}
	r.Steps[stepIndex] = step
	r.Status = StatusWaitingApproval
	r.UpdatedAt = now
	return r
}

// Approve lets the run continue past the step it waits at. Only the person
// the step names may do it: an approval is a decision somebody is accountable
// for, and that somebody was chosen when the workflow was written.
func (r Run) Approve(definition Definition, byUserID string) (Run, error) {
	index, step, err := r.waitingStep(definition)
	if err != nil {
		return r, err
	}
	if strings.TrimSpace(byUserID) != step.ApproverUserID {
		return r, ErrWrongApprover
	}
	r.Steps[index].ApprovedByUserID = step.ApproverUserID
	return r.Succeed(index, "approved by "+step.ApproverUserID, nil), nil
}

// Reject stops the run at the step it waits at. Recorded as cancelled rather
// than failed: nothing broke, somebody said no, and the two must not look
// alike in a list.
func (r Run) Reject(definition Definition, byUserID string) (Run, error) {
	index, step, err := r.waitingStep(definition)
	if err != nil {
		return r, err
	}
	if strings.TrimSpace(byUserID) != step.ApproverUserID {
		return r, ErrWrongApprover
	}
	now := time.Now().UTC()
	r.Steps[index].Status = StatusCancelled
	r.Steps[index].ApprovedByUserID = step.ApproverUserID
	r.Steps[index].Output = "rejected by " + step.ApproverUserID
	r.Steps[index].FinishedAt = &now
	r.Status = StatusCancelled
	r.UpdatedAt = now
	r.FinishedAt = &now
	return r, nil
}

func (r Run) waitingStep(definition Definition) (int, Step, error) {
	if r.Status != StatusWaitingApproval {
		return -1, Step{}, ErrNotWaiting
	}
	index, more := r.Current()
	if !more || index >= len(definition.Steps) || definition.Steps[index].Kind != KindApproval {
		return -1, Step{}, ErrNotWaiting
	}
	return index, definition.Steps[index], nil
}
