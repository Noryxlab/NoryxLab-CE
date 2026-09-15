// Package agent holds the standing instructions a user has left with the
// platform, and what came of them.
//
// An agent is written in the user's own words - "surveille mes workspaces et
// dis-moi quand l'un ne démarre pas" - and not as a rule, a filter or a
// threshold. That is the whole point: the people who know what is worth
// watching are not the people who enjoy writing conditions, and every
// monitoring product that asked them to has ended up unused.
//
// Two things follow from that, and they are why this type looks the way it
// does. The mission is free text and stays free text: nothing here parses it,
// because the moment a field constrains what may be said, the writer starts
// writing for the field. And what an agent may *do* is not in the mission -
// text cannot be a permission, since the person writing it is also the person
// the permission protects. Actions are a separate, closed list.
package agent

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Schedules an agent can keep. Deliberately three, in the vocabulary of
// somebody describing a colleague's hours rather than a cron expression.
const (
	ScheduleManual = "manual"
	ScheduleHourly = "hourly"
	ScheduleDaily  = "daily"
)

// ActionRestartApp is the one thing an agent may change, for now.
//
// Chosen because it is reversible and already routine: an app that has fallen
// over is restarted, which is what a person would do, and the worst case of
// doing it wrongly is an app that restarts when it did not need to. Anything
// that destroys state, spends money, or reaches outside the platform is not on
// this list and does not get here by being asked for nicely in a mission.
const ActionRestartApp = "restart_app"

type Agent struct {
	ID          string `json:"id"`
	OwnerUserID string `json:"ownerUserId"`
	// ProjectID is the project this agent works in, and it is required.
	//
	// It scopes what the agent sees, what it may act on, and - once agents
	// read data - which data that is. It travels in the credential signed for
	// each run, so the scope is a fact rather than something the model chooses
	// by leaving an argument out.
	//
	// It was optional and, worse, decorative: the field claimed to scope and
	// the tools filtered on an argument the model supplied, so an agent placed
	// in a project saw everything its owner saw. Making it required is what
	// lets every later rule - data access, cost attribution, the perimeter -
	// be a property of the project instead of a second permission system built
	// for agents alone.
	//
	// A person who wants something across all their projects has the platform
	// assistant, which is that surface and is deliberately unscoped.
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	// Mission is what the user wrote. It reaches the model as instructions and
	// is never interpreted by the platform.
	Mission  string `json:"mission"`
	Schedule string `json:"schedule"`
	// TeamID is the group this agent works in. Empty means it works alone,
	// which is what every agent did before teams existed and stays the
	// default.
	TeamID string `json:"teamId,omitempty"`
	// Role is what it is for inside that team, and the ceiling on what it may
	// hold. See team.go: the ceiling is applied where actions are stored, not
	// argued about when they are used.
	Role Role `json:"role"`
	// Actions the agent may take, from the closed list above. Empty - the
	// default - means it only looks and reports.
	Actions    []string   `json:"actions"`
	Enabled    bool       `json:"enabled"`
	CreatedAt  time.Time  `json:"createdAt"`
	UpdatedAt  time.Time  `json:"updatedAt"`
	LastRunAt  *time.Time `json:"lastRunAt,omitempty"`
	LastReport string     `json:"lastReport,omitempty"`
	// LastQuiet records whether the last run found nothing. An agent that is
	// working and has nothing to say looks exactly like a broken one unless
	// the difference is stored.
	LastQuiet bool `json:"lastQuiet"`
}

// Run is one working session, kept because an agent nobody can audit is an
// agent nobody should be given an action.
type Run struct {
	ID      string `json:"id"`
	AgentID string `json:"agentId"`
	// Question is what a person asked, when this run came from somebody
	// talking to the agent rather than from its schedule.
	//
	// Kept in the same timeline as the scheduled runs, deliberately. An
	// agent's journal is the record of what it did and why; a conversation
	// held somewhere else would be a second history, and the first question
	// anybody asks about an action is what prompted it.
	Question string `json:"question,omitempty"`
	// Report is what the agent has to say, in the language of the mission.
	Report string `json:"report"`
	// Quiet marks a run that found nothing worth reporting. Kept and shown,
	// rather than hidden: a row of quiet hours is how a reader knows the agent
	// was there.
	Quiet bool `json:"quiet"`
	// Actions taken, named and in order. A report claiming an action that is
	// not in this list is a model writing fiction, and the interface shows this
	// list rather than trusting the prose.
	Actions    []string   `json:"actions"`
	Error      string     `json:"error,omitempty"`
	StartedAt  time.Time  `json:"startedAt"`
	FinishedAt *time.Time `json:"finishedAt,omitempty"`
}

func New(ownerUserID, projectID, name, mission, schedule string, actions []string) Agent {
	now := time.Now().UTC()
	return Agent{
		ID:          uuid.NewString(),
		OwnerUserID: strings.TrimSpace(ownerUserID),
		ProjectID:   strings.TrimSpace(projectID),
		Name:        strings.TrimSpace(name),
		Mission:     strings.TrimSpace(mission),
		Schedule:    NormaliseSchedule(schedule),
		Role:        roleFor(actions),
		Actions:     NormaliseActions(actions),
		Enabled:     true,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

func NewRun(agentID string) Run {
	return Run{ID: uuid.NewString(), AgentID: agentID, StartedAt: time.Now().UTC(), Actions: []string{}}
}

// NewAskedRun is a run a person started by asking something.
func NewAskedRun(agentID, question string) Run {
	run := NewRun(agentID)
	run.Question = strings.TrimSpace(question)
	return run
}

// NormaliseSchedule falls back to manual, which is the schedule that cannot
// surprise anybody.
func NormaliseSchedule(schedule string) string {
	switch strings.ToLower(strings.TrimSpace(schedule)) {
	case ScheduleHourly:
		return ScheduleHourly
	case ScheduleDaily:
		return ScheduleDaily
	default:
		return ScheduleManual
	}
}

// ErrProjectRequired is returned where an agent arrives without one.
var ErrProjectRequired = errors.New("an agent works in a project, and none was given")

// Validate reports whether this agent can be stored.
func (a Agent) Validate() error {
	if strings.TrimSpace(a.ProjectID) == "" {
		return ErrProjectRequired
	}
	return nil
}

// NormaliseActions drops anything not on the closed list, in silence rather
// than with an error - the caller is checked separately, and this is the last
// gate before an agent is stored with a power nobody granted it.
func NormaliseActions(actions []string) []string {
	kept := make([]string, 0, len(actions))
	seen := map[string]bool{}
	for _, action := range actions {
		action = strings.ToLower(strings.TrimSpace(action))
		if action != ActionRestartApp || seen[action] {
			continue
		}
		seen[action] = true
		kept = append(kept, action)
	}
	return kept
}

// ValidAction reports whether a name is one the platform implements.
func ValidAction(action string) bool {
	return strings.ToLower(strings.TrimSpace(action)) == ActionRestartApp
}

// roleFor derives the base role from what was granted.
//
// Holding an action is what being an operator means, so the role is read off
// the grant rather than asked for a second time. A lead is never derived: that
// one is a decision somebody makes about a team, and deriving it would promote
// an agent nobody promoted.
func roleFor(actions []string) Role {
	if len(NormaliseActions(actions)) > 0 {
		return RoleOperator
	}
	return RoleObserver
}

// WithTeam places this agent in a team under a role, applying the role's
// ceiling to what it holds.
func (a Agent) WithTeam(teamID string, role Role) Agent {
	a.TeamID = strings.TrimSpace(teamID)
	a.Role = NormaliseRole(string(role))
	a.Actions = NormaliseForRole(a.Role, a.Actions)
	a.UpdatedAt = time.Now().UTC()
	return a
}

// EffectiveRole is the role to reason with, including for agents stored before
// teams existed.
//
// Those rows carry no role at all. Rather than migrate them - a migration that
// guesses is worse than a derivation that is stated - the role is derived the
// same way New derives it. The consequence is deliberate: such an agent can be
// a member of a team, and can never be its lead, because nobody made it one.
func (a Agent) EffectiveRole() Role {
	if a.Role != "" {
		return NormaliseRole(string(a.Role))
	}
	return roleFor(a.Actions)
}

// May reports whether this agent was granted an action.
func (a Agent) May(action string) bool {
	for _, granted := range a.Actions {
		if granted == action {
			return true
		}
	}
	return false
}

// DueAt reports whether the agent owes a run at this moment.
func (a Agent) DueAt(now time.Time) bool {
	if !a.Enabled {
		return false
	}
	var every time.Duration
	switch a.Schedule {
	case ScheduleHourly:
		every = time.Hour
	case ScheduleDaily:
		every = 24 * time.Hour
	default:
		// Manual agents are never due; somebody presses the button.
		return false
	}
	if a.LastRunAt == nil {
		return true
	}
	return now.Sub(*a.LastRunAt) >= every
}
