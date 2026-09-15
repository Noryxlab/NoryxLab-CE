package agent

import "testing"

// written returns the mandate for exactly the ask being made. Passed even to
// the calls expected to fail, so each refusal proves the stronger thing: that
// a written mandate does not rescue an ask the capability rules refuse.
func written(lead, member Agent, action string) []Mandate {
	return []Mandate{NewMandate(lead.TeamID, lead.ID, member.ID, action, "stef")}
}

func member(owner, team string, role Role, actions []string) Agent {
	item := New(owner, "", "member", "watch things", ScheduleHourly, actions)
	return item.WithTeam(team, role)
}

// The rule the whole design rests on: a lead asking a member to act never
// exceeds either of them.
func TestDelegationNeedsBothSidesToHoldTheAction(t *testing.T) {
	lead := member("stef", "ops", RoleLead, []string{ActionRestartApp})
	holder := member("stef", "ops", RoleOperator, []string{ActionRestartApp})
	watcher := member("stef", "ops", RoleObserver, nil)

	if decision := Delegate(lead, holder, ActionRestartApp, written(lead, holder, ActionRestartApp)); !decision.Allowed {
		t.Errorf("a lead and a member that both hold the action were refused: %s", decision.Refusal)
	}

	// The member does not hold it: the owner decided that, and a lead does not
	// get to decide otherwise.
	if decision := Delegate(lead, watcher, ActionRestartApp, written(lead, watcher, ActionRestartApp)); decision.Allowed {
		t.Error("a member with no grant was made to act")
	} else if decision.Refusal != RefusalMemberLacks {
		t.Errorf("refused for the wrong reason: %s", decision.Refusal)
	}

	// And the direction that matters most: an agent that cannot restart an app
	// itself must not obtain one by asking somebody who can. Without this,
	// delegation is a way to launder authority.
	quietLead := member("stef", "ops", RoleLead, nil)
	if decision := Delegate(quietLead, holder, ActionRestartApp, written(quietLead, holder, ActionRestartApp)); decision.Allowed {
		t.Error("a lead obtained by delegation an action it was never granted")
	} else if decision.Refusal != RefusalLeadLacks {
		t.Errorf("refused for the wrong reason: %s", decision.Refusal)
	}
}

func TestOnlyALeadDelegates(t *testing.T) {
	operator := member("stef", "ops", RoleOperator, []string{ActionRestartApp})
	other := member("stef", "ops", RoleOperator, []string{ActionRestartApp})
	if decision := Delegate(operator, other, ActionRestartApp, written(operator, other, ActionRestartApp)); decision.Allowed {
		t.Error("an operator delegated to a peer")
	} else if decision.Refusal != RefusalNotALead {
		t.Errorf("refused for the wrong reason: %s", decision.Refusal)
	}
}

// Two agents are only colleagues if the same person stands behind both. This
// is the transitive form of "an agent never exceeds its owner".
func TestDelegationDoesNotCrossOwnersOrTeams(t *testing.T) {
	lead := member("stef", "ops", RoleLead, []string{ActionRestartApp})
	stranger := member("cedric", "ops", RoleOperator, []string{ActionRestartApp})
	elsewhere := member("stef", "research", RoleOperator, []string{ActionRestartApp})

	if decision := Delegate(lead, stranger, ActionRestartApp, written(lead, stranger, ActionRestartApp)); decision.Allowed {
		t.Error("a lead reached an agent belonging to somebody else")
	} else if decision.Refusal != RefusalDifferentOwners {
		t.Errorf("refused for the wrong reason: %s", decision.Refusal)
	}
	if decision := Delegate(lead, elsewhere, ActionRestartApp, written(lead, elsewhere, ActionRestartApp)); decision.Allowed {
		t.Error("a lead reached across into another team")
	} else if decision.Refusal != RefusalDifferentTeams {
		t.Errorf("refused for the wrong reason: %s", decision.Refusal)
	}

	// A lead with no team of its own must not match a member with no team.
	loose := New("stef", "", "loose", "watch", ScheduleHourly, []string{ActionRestartApp})
	loose.Role = RoleLead
	orphan := New("stef", "", "orphan", "watch", ScheduleHourly, []string{ActionRestartApp})
	if decision := Delegate(loose, orphan, ActionRestartApp, written(loose, orphan, ActionRestartApp)); decision.Allowed {
		t.Error("two agents with no team were treated as colleagues")
	}
}

// The role is a ceiling, and a ceiling that can be argued with by granting an
// action anyway is not a ceiling.
func TestAnObserverCannotBeGrantedAnAction(t *testing.T) {
	watcher := member("stef", "ops", RoleObserver, []string{ActionRestartApp})
	if len(watcher.Actions) != 0 {
		t.Errorf("an observer kept %v", watcher.Actions)
	}
	if watcher.May(ActionRestartApp) {
		t.Error("an observer may act")
	}

	// Promotion is what widens it, and it is explicit.
	promoted := watcher.WithTeam("ops", RoleOperator)
	if promoted.May(ActionRestartApp) {
		t.Error("promoting restored a grant the ceiling had already removed")
	}
}

func TestAnAgentStoredBeforeTeamsIsAMemberAndNeverALead(t *testing.T) {
	// The rows that exist today carry no role at all.
	old := New("stef", "", "charlotte", "watch the workspaces", ScheduleHourly, []string{ActionRestartApp})
	old.Role = ""

	if role := old.EffectiveRole(); role != RoleOperator {
		t.Errorf("an agent holding an action derives %q, want operator", role)
	}
	if old.EffectiveRole().MayDelegate() {
		t.Error("an agent nobody promoted became a lead by migration")
	}

	quiet := New("stef", "", "watcher", "just look", ScheduleHourly, nil)
	quiet.Role = ""
	if role := quiet.EffectiveRole(); role != RoleObserver {
		t.Errorf("an agent holding nothing derives %q, want observer", role)
	}
}

func TestADisabledMemberIsNotAskedToAct(t *testing.T) {
	lead := member("stef", "ops", RoleLead, []string{ActionRestartApp})
	off := member("stef", "ops", RoleOperator, []string{ActionRestartApp})
	off.Enabled = false
	if decision := Delegate(lead, off, ActionRestartApp, written(lead, off, ActionRestartApp)); decision.Allowed {
		t.Error("a switched-off agent was given work")
	} else if decision.Refusal != RefusalMemberDisabled {
		t.Errorf("refused for the wrong reason: %s", decision.Refusal)
	}
}

// A refusal that leaves no trace is indistinguishable from a request nobody
// made, and the difference is what an audit asks about.
func TestEveryDecisionNamesItsTwoAgents(t *testing.T) {
	lead := member("stef", "ops", RoleLead, nil)
	other := member("stef", "ops", RoleOperator, []string{ActionRestartApp})
	decision := Delegate(lead, other, ActionRestartApp, written(lead, other, ActionRestartApp))
	if decision.LeadID != lead.ID || decision.MemberID != other.ID || decision.Action != ActionRestartApp {
		t.Error("a refusal did not record who asked whom for what")
	}
	if decision.Refusal == "" {
		t.Error("a refusal carried no reason")
	}
}

// The organisation is written by a person, in advance, and nothing happens
// without it.
func TestAnAskWithNoWrittenMandateIsRefused(t *testing.T) {
	lead := member("stef", "ops", RoleLead, []string{ActionRestartApp})
	holder := member("stef", "ops", RoleOperator, []string{ActionRestartApp})

	// Everything else is in order: same owner, same team, both hold the
	// action. Only the organisation is missing.
	decision := Delegate(lead, holder, ActionRestartApp, nil)
	if decision.Allowed {
		t.Fatal("a lead asked a member for something nobody wrote down")
	}
	if decision.Refusal != RefusalNoMandate {
		t.Errorf("refused for the wrong reason: %s", decision.Refusal)
	}

	// And the message is only reached when writing one would actually work,
	// which is why the check comes last.
	if allowed := Delegate(lead, holder, ActionRestartApp, written(lead, holder, ActionRestartApp)); !allowed.Allowed {
		t.Errorf("the written mandate did not take effect: %s", allowed.Refusal)
	}
}

// A mandate authorises; it never widens. Writing one for an agent whose owner
// never granted the action must change nothing, or the organisation becomes a
// second permission system competing with the first.
func TestAMandateCannotGrantWhatTheOwnerDidNot(t *testing.T) {
	lead := member("stef", "ops", RoleLead, []string{ActionRestartApp})
	watcher := member("stef", "ops", RoleObserver, nil)

	decision := Delegate(lead, watcher, ActionRestartApp, written(lead, watcher, ActionRestartApp))
	if decision.Allowed {
		t.Fatal("a written mandate granted an action the owner never gave")
	}
	if decision.Refusal != RefusalMemberLacks {
		t.Errorf("refused for the wrong reason: %s", decision.Refusal)
	}
}

// A mandate is specific. One written for a different pair, or a different
// action, is not this one.
func TestAMandateDoesNotCoverItsNeighbours(t *testing.T) {
	lead := member("stef", "ops", RoleLead, []string{ActionRestartApp})
	first := member("stef", "ops", RoleOperator, []string{ActionRestartApp})
	second := member("stef", "ops", RoleOperator, []string{ActionRestartApp})

	// Written for the first member, invoked for the second.
	elsewhere := written(lead, first, ActionRestartApp)
	if Delegate(lead, second, ActionRestartApp, elsewhere).Allowed {
		t.Error("a mandate for one member covered another")
	}

	// Written for an action that is not the one being asked.
	other := []Mandate{NewMandate(lead.TeamID, lead.ID, second.ID, "something_else", "stef")}
	if Delegate(lead, second, ActionRestartApp, other).Allowed {
		t.Error("a mandate for one action covered another")
	}
}

// A run has to record which line of the organisation it acted under, or the
// document and the history cannot be reconciled a month later.
func TestAnAllowedDelegationNamesTheMandateItRestedOn(t *testing.T) {
	lead := member("stef", "ops", RoleLead, []string{ActionRestartApp})
	holder := member("stef", "ops", RoleOperator, []string{ActionRestartApp})
	mandates := written(lead, holder, ActionRestartApp)

	decision := Delegate(lead, holder, ActionRestartApp, mandates)
	if !decision.Allowed {
		t.Fatalf("refused: %s", decision.Refusal)
	}
	if decision.MandateID != mandates[0].ID {
		t.Errorf("the decision names mandate %q, not the one it used", decision.MandateID)
	}
	// And who wrote it is kept, because "who allowed this" is the first
	// question asked about any delegation.
	if mandates[0].GrantedByUserID == "" {
		t.Error("a mandate does not record who wrote it")
	}
}
