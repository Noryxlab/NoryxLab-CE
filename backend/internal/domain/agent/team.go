package agent

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Agents working together, and what that does to their authority.
//
// One agent with a standing assignment is a colleague. Several of them on the
// same subject need what any group of colleagues needs: a purpose they share,
// a name for what each one is for, and somebody who can ask the others to do
// something. That last part is the whole difficulty, and it is not solved by
// letting them talk to each other.
//
// The platforms that deploy agents today mostly stop at the conversation: two
// agents can exchange messages and neither can establish what the other is
// allowed to do. The message arrives and the receiving side either does as it
// is told - in which case the sender has just borrowed authority it was never
// granted - or refuses on a rule nobody wrote down. A protocol moves the
// problem; it does not answer it.
//
// So the rule here is the one that already governs a single agent, made
// transitive. An agent never exceeds its owner. A lead asking a member to act
// never exceeds *either* of them: not the member, because the owner decided
// what that member may do, and not the lead, because an agent that cannot
// restart an app on its own must not be able to obtain one by delegating. The
// intersection is the answer, and it is small enough to state to a regulator
// in one sentence.
//
// Note what is deliberately *not* constrained: a member's own scheduled runs.
// Those were granted by the owner directly and a lead has no say in them. A
// team is a structure for asking, not a chain that re-authorises work nobody
// delegated.

// Role is what an agent is for inside its team, and what that lets it do.
//
// Three, in the vocabulary of describing a colleague rather than a policy
// object. The role is a ceiling on actions and not a label beside them: a
// ceiling that can be argued with by granting an action anyway is not a
// ceiling, so NormaliseForRole is applied where actions are stored.
type Role string

const (
	// RoleObserver looks and reports. No action, whatever was requested.
	// This is the default, because an agent nobody thought about should be
	// the harmless kind.
	RoleObserver Role = "observer"
	// RoleOperator may take the actions it was granted, on its own schedule.
	RoleOperator Role = "operator"
	// RoleLead may do what an operator may, and may also ask the other
	// members of its team to act - never beyond what both of them hold.
	RoleLead Role = "lead"
)

// NormaliseRole keeps an unknown role out rather than failing on it, and
// defaults to the harmless one.
func NormaliseRole(role string) Role {
	switch Role(strings.ToLower(strings.TrimSpace(role))) {
	case RoleOperator:
		return RoleOperator
	case RoleLead:
		return RoleLead
	default:
		return RoleObserver
	}
}

// MayHoldActions reports whether this role is allowed any action at all.
func (r Role) MayHoldActions() bool {
	return r == RoleOperator || r == RoleLead
}

// MayDelegate reports whether this role may ask another member to act.
func (r Role) MayDelegate() bool { return r == RoleLead }

// NormaliseForRole is the ceiling, applied where actions are stored.
//
// An observer with a granted action is not a contradiction to resolve at call
// time - it is a record that will eventually be read by something that forgets
// to check the role. The grant does not survive being written down.
func NormaliseForRole(role Role, actions []string) []string {
	if !role.MayHoldActions() {
		return []string{}
	}
	return NormaliseActions(actions)
}

// Team is a group of agents on one subject.
type Team struct {
	ID          string `json:"id"`
	OwnerUserID string `json:"ownerUserId"`
	// ProjectID scopes the team the way it scopes an agent. Empty means the
	// owner's whole view, which is still never more than the owner has.
	ProjectID string `json:"projectId,omitempty"`
	Name      string `json:"name"`
	// Purpose is what the team is for, in the owner's words. Like a mission,
	// nothing parses it.
	Purpose   string    `json:"purpose"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// NewTeam creates a team owned by one person.
func NewTeam(ownerUserID, projectID, name, purpose string) Team {
	now := time.Now().UTC()
	return Team{
		ID:          uuid.NewString(),
		OwnerUserID: strings.TrimSpace(ownerUserID),
		ProjectID:   strings.TrimSpace(projectID),
		Name:        strings.TrimSpace(name),
		Purpose:     strings.TrimSpace(purpose),
		CreatedAt:   now,
		UpdatedAt:   now,
	}
}

// Delegation is one lead asking one member to take one action.
//
// Returned rather than performed, so the caller records what was decided and
// why. A refusal that leaves no trace is indistinguishable from a request
// nobody made, and the difference is what an audit asks about.
type Delegation struct {
	LeadID   string `json:"leadId"`
	MemberID string `json:"memberId"`
	Action   string `json:"action"`
	Allowed  bool   `json:"allowed"`
	// Refusal names which side lacked the right, in the platform's words
	// rather than the model's. Empty when allowed.
	Refusal string `json:"refusal,omitempty"`
}

// Reasons a delegation is refused. Named, because "forbidden" tells an owner
// nothing about which grant to widen if they decide to.
const (
	RefusalNotALead        = "the asking agent is not the lead of a team"
	RefusalDifferentTeams  = "the two agents are not on the same team"
	RefusalDifferentOwners = "the two agents do not answer to the same person"
	RefusalUnknownAction   = "the platform does not implement that action"
	RefusalLeadLacks       = "the lead was not granted that action"
	RefusalMemberLacks     = "the member was not granted that action"
	RefusalMemberDisabled  = "the member is switched off"
)

// Delegate decides whether lead may have member take action.
//
// Every condition is checked here rather than split between this and the
// caller: a permission spread across two places is a permission that will
// eventually be enforced in one of them.
func Delegate(lead, member Agent, action string) Delegation {
	decision := Delegation{LeadID: lead.ID, MemberID: member.ID, Action: action}

	refuse := func(reason string) Delegation {
		decision.Refusal = reason
		return decision
	}

	if !lead.EffectiveRole().MayDelegate() {
		return refuse(RefusalNotALead)
	}
	// Same owner first: two agents on a team they should not share is a worse
	// fault than a missing grant, and saying so in that order keeps the
	// message honest about what actually went wrong.
	if lead.OwnerUserID == "" || lead.OwnerUserID != member.OwnerUserID {
		return refuse(RefusalDifferentOwners)
	}
	if lead.TeamID == "" || lead.TeamID != member.TeamID {
		return refuse(RefusalDifferentTeams)
	}
	if !ValidAction(action) {
		return refuse(RefusalUnknownAction)
	}
	if !member.Enabled {
		return refuse(RefusalMemberDisabled)
	}
	// Both sides, which is the whole rule. The member's grant is the owner's
	// decision about the member; the lead's is what stops an agent obtaining
	// by delegation what it could not do itself.
	if !lead.May(action) {
		return refuse(RefusalLeadLacks)
	}
	if !member.May(action) {
		return refuse(RefusalMemberLacks)
	}

	decision.Allowed = true
	return decision
}
