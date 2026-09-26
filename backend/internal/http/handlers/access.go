package handlers

import (
	"crypto/subtle"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/access"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/project"
)

const (
	userHeader      = "X-Noryx-User"
	serviceHeader   = "X-Noryx-Service-Token"
	authHeader      = "Authorization"
	globalAdminRole = "noryx-admin"
	sessionCookie   = "noryx_session"

	projectActionRead         = "project.read"
	projectActionLaunch       = "project.launch"
	projectActionRunBuild     = "project.build"
	projectActionManageMember = "project.manage_members"

	// Attaching a resource to a project is not launching a workload, and the
	// platform used to decide both with the same rule: everything below was
	// actionLaunch, so "may run a notebook" and "may mount a cohort" were one
	// permission. That is why five of the matrix's seven columns could be
	// filled in and read back by nothing - there was no question to answer
	// with them. Each of these is the question that column was for.
	projectActionAttachDataset     = "dataset.attach"
	projectActionAttachOntology    = "ontology.attach"
	projectActionAttachDatasource  = "datasource.attach"
	projectActionManageEnvironment = "environment.manage"
)

func (h Handlers) requireIdentity(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	token := strings.TrimSpace(r.Header.Get(authHeader))
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimSpace(token)

	// A personal token is checked before the identity provider: it looks like
	// a bearer token, and asking Keycloak to verify something it never issued
	// wastes a round trip to learn nothing.
	if token != "" {
		if identity, ok := h.identityFromAPIToken(token); ok {
			if !h.requireOrganizationMembership(w, identity) {
				return auth.Identity{}, false
			}
			return identity, true
		}
	}

	if token != "" && h.authVerifier != nil {
		identity, err := h.authVerifier.VerifyBearerToken(token)
		if err != nil {
			// Six different failures reach the caller as one opaque message -
			// a missing key id, an unreachable JWKS, a rotated key, a wrong
			// issuer, an expired token, a missing audience. The caller learns
			// nothing on purpose, but the operator has to be able to tell them
			// apart: without this line, diagnosing "invalid bearer token" is
			// guesswork against a platform that knows the answer and will not
			// say it.
			//
			// The token itself is never logged. Its length and the first
			// characters of its key id are enough to correlate, and are not a
			// credential.
			log.Printf("bearer token refused on %s: %v", r.URL.Path, err)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid bearer token"})
			return auth.Identity{}, false
		}
		if !h.requireOrganizationMembership(w, identity) {
			return auth.Identity{}, false
		}
		return identity, true
	}

	// A platform component with no human behind it - the scheduled backup
	// trigger, the validation suite - presents a shared secret instead.
	//
	// It is deliberately exempt from the organization requirement: an
	// organization is a tenancy fact about a person, and a service belongs to
	// no tenant. Requiring one would mean inventing a Keycloak account for
	// every internal component, which is how "platform-validator" ended up as
	// a phantom user whose backups were refused for three nights.
	//
	// The token carries platform administrator rights, because triggering a
	// backup needs them. Treat it as an administrator credential: it lives in
	// a Secret, never in a manifest.
	if identity, ok := h.serviceIdentity(r); ok {
		return identity, true
	}

	// The user header is a development convenience and nothing else. It used
	// to be honoured unconditionally, so any caller that could reach this
	// service could act as any user simply by naming them - no token, no
	// session. The bearer check above was added in front of it and this was
	// never closed behind.
	if !strings.EqualFold(strings.TrimSpace(h.authMode), "header") {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
		return auth.Identity{}, false
	}

	userID := strings.TrimSpace(r.Header.Get(userHeader))
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
		return auth.Identity{}, false
	}
	identity := auth.Identity{
		Username: userID,
		Roles:    map[string]struct{}{},
	}
	if !h.requireOrganizationMembership(w, identity) {
		return auth.Identity{}, false
	}
	return identity, true
}

// serviceIdentity resolves a platform component's shared-secret credential.
//
// The comparison is constant-time, and an absent configuration matches
// nothing: a deployment that forgets NORYX_SERVICE_TOKEN refuses its own
// services rather than accepting an empty header from anyone.
func (h Handlers) serviceIdentity(r *http.Request) (auth.Identity, bool) {
	configured := strings.TrimSpace(h.serviceToken)
	presented := strings.TrimSpace(r.Header.Get(serviceHeader))
	if configured == "" || presented == "" {
		return auth.Identity{}, false
	}
	if subtle.ConstantTimeCompare([]byte(configured), []byte(presented)) != 1 {
		return auth.Identity{}, false
	}

	// The component may name itself, so a backup run records which one asked
	// rather than attributing every automated action to the same opaque
	// identity. It cannot name a *person*, and now it cannot become one
	// either: the name reaches the audit trail and stops there.
	//
	// It used to be written into Username. Everything downstream that resolves
	// a user then treated the service as that person - their organizations,
	// their project roles, their name in the audit - while the identity still
	// carried the global administrator role. A component could therefore read
	// every dataset on the platform and have it recorded as the work of a
	// named researcher who had done nothing.
	return auth.Identity{
		Username:   auth.ServiceUsername,
		DeclaredBy: strings.TrimSpace(r.Header.Get(userHeader)),
		Roles:      map[string]struct{}{globalAdminRole: {}},
	}, true
}

func (h Handlers) requireUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return "", false
	}
	userID := identity.UserID()
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authenticated identity"})
		return "", false
	}
	return userID, true
}

func (h Handlers) requireUserIDFromSessionOrBearer(w http.ResponseWriter, r *http.Request) (string, bool) {
	identity, ok := h.requireIdentityFromSessionOrBearer(w, r)
	if !ok {
		return "", false
	}
	userID := identity.UserID()
	if userID == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authenticated identity"})
		return "", false
	}
	return userID, true
}

func (h Handlers) userIDFromSessionOrBearerNoWrite(r *http.Request) (string, bool) {
	identity, ok := h.identityFromSessionOrBearerNoWrite(r)
	if !ok {
		return "", false
	}
	userID := strings.TrimSpace(identity.UserID())
	if userID == "" {
		return "", false
	}
	return userID, true
}

func (h Handlers) identityFromSessionOrBearerNoWrite(r *http.Request) (auth.Identity, bool) {
	token := strings.TrimSpace(r.Header.Get(authHeader))
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimSpace(token)
	if token != "" {
		// A platform token, before asking the identity provider about something
		// it never issued.
		//
		// This path is what makes a deployed application callable by another
		// system. Until now the proxy accepted only a browser session or an
		// OIDC bearer, so an endpoint could be deployed and then reached by
		// nobody but a person with a browser - which is not what an endpoint is
		// for.
		if identity, ok := h.identityFromAPIToken(token); ok {
			return identity, true
		}
		if h.authVerifier == nil {
			return auth.Identity{}, false
		}
		identity, err := h.authVerifier.VerifyBearerToken(token)
		if err != nil {
			return auth.Identity{}, false
		}
		if h.organizationRequired && !h.identityHasOrganizationNoWrite(identity) {
			return auth.Identity{}, false
		}
		return identity, true
	}

	if h.sessionStore == nil {
		return auth.Identity{}, false
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return auth.Identity{}, false
	}
	item, ok, err := h.sessionStore.Get(strings.TrimSpace(cookie.Value))
	if err != nil || !ok {
		return auth.Identity{}, false
	}
	if time.Now().UTC().After(item.ExpiresAt) {
		_ = h.sessionStore.Delete(item.Token)
		return auth.Identity{}, false
	}
	userID := strings.TrimSpace(item.Identity)
	if userID == "" {
		return auth.Identity{}, false
	}
	identity := auth.Identity{Username: userID, Roles: map[string]struct{}{}}
	if h.organizationRequired && !h.identityHasOrganizationNoWrite(identity) {
		return auth.Identity{}, false
	}
	return identity, true
}

func (h Handlers) identityHasOrganizationNoWrite(identity auth.Identity) bool {
	if h.keycloak == nil {
		return false
	}
	identifier := strings.TrimSpace(identity.Subject)
	if identifier == "" {
		identifier = strings.TrimSpace(identity.UserID())
	}
	if identifier == "" {
		return false
	}
	hasOrganization, err := h.keycloak.HasOrganization(identifier)
	return err == nil && hasOrganization
}

func (h Handlers) requireIdentityFromSessionOrBearer(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	// Checked first, and on both identity paths, so a platform component is
	// not accepted on some routes and refused on others for reasons nobody
	// can see from the caller's side.
	if identity, ok := h.serviceIdentity(r); ok {
		return identity, true
	}

	token := strings.TrimSpace(r.Header.Get(authHeader))
	token = strings.TrimPrefix(token, "Bearer ")
	token = strings.TrimSpace(token)
	if token != "" {
		return h.requireIdentity(w, r)
	}

	if h.sessionStore == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing bearer token"})
		return auth.Identity{}, false
	}

	cookie, err := r.Cookie(sessionCookie)
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "missing authenticated session"})
		return auth.Identity{}, false
	}

	item, ok, err := h.sessionStore.Get(strings.TrimSpace(cookie.Value))
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to read authenticated session"})
		return auth.Identity{}, false
	}
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid authenticated session"})
		return auth.Identity{}, false
	}
	if time.Now().UTC().After(item.ExpiresAt) {
		_ = h.sessionStore.Delete(item.Token)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "expired authenticated session"})
		return auth.Identity{}, false
	}

	identity := auth.Identity{
		Username: item.Identity,
		Roles:    map[string]struct{}{},
	}
	if !h.requireOrganizationMembership(w, identity) {
		return auth.Identity{}, false
	}
	return identity, true
}

func (h Handlers) requireOrganizationMembership(w http.ResponseWriter, identity auth.Identity) bool {
	if !h.organizationRequired {
		return true
	}
	// A platform component belongs to no organization, and never will.
	//
	// The gate exists so a person cannot reach an Enterprise installation
	// without belonging somewhere. Applied to the backup runner it refused
	// every request it made, which is why components had to keep using a
	// shared secret that skipped this path entirely - the gate was pushing
	// exactly the credential it made necessary.
	if identity.IsService() {
		return true
	}
	if h.keycloak == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "organization membership verification is unavailable"})
		return false
	}
	identifier := strings.TrimSpace(identity.Subject)
	if identifier == "" {
		identifier = strings.TrimSpace(identity.UserID())
	}
	hasOrganization, err := h.keycloak.HasOrganization(identifier)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "failed to verify organization membership"})
		return false
	}
	if !hasOrganization {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "organization membership required",
			"code":  "organization_required",
		})
		return false
	}
	return true
}

// projectAction is what a caller is trying to do, and the rule that decides it.
//
// Both together, deliberately. The caller used to pass a *predicate* and a
// human label, and the action was then inferred by calling the predicate with
// two roles and searching the label for the substring "build". An authorization
// decision therefore depended on the wording of an error message: renaming
// "app restart" to "app rebuild" would have silently re-classified it, and
// adding a fourth action would have made the guess worse. Nothing failed,
// which is why it survived.
type projectAction struct {
	// id is what the Enterprise role matrix reasons about.
	id string
	// permits is the Community rule, and the fallback the matrix may override.
	permits func(access.Role) bool
}

var (
	actionRead = projectAction{
		id:      projectActionRead,
		permits: func(role access.Role) bool { return role != "" },
	}
	actionLaunch = projectAction{
		id:      projectActionLaunch,
		permits: access.Role.CanLaunchPod,
	}
	actionRunBuild = projectAction{
		id:      projectActionRunBuild,
		permits: access.Role.CanRunBuild,
	}
	actionManageMembers = projectAction{
		id:      projectActionManageMember,
		permits: func(role access.Role) bool { return role == access.RoleAdmin },
	}

	// The Community rule for all four is the one they already had as
	// actionLaunch: a contributor may attach what their project works on. What
	// changes is that the matrix now sees which resource is being attached, so
	// an installation can say "this role reads cohorts and writes nothing"
	// without also saying it may not start a workspace.
	actionAttachDataset = projectAction{
		id:      projectActionAttachDataset,
		permits: access.Role.CanLaunchPod,
	}
	actionAttachOntology = projectAction{
		id:      projectActionAttachOntology,
		permits: access.Role.CanLaunchPod,
	}
	actionAttachDatasource = projectAction{
		id:      projectActionAttachDatasource,
		permits: access.Role.CanLaunchPod,
	}
	actionManageEnvironment = projectAction{
		id:      projectActionManageEnvironment,
		permits: access.Role.CanRunBuild,
	}
)

// allowsProjectAction answers the same question as requireProjectRole without
// writing a refusal: a screen that shows names to everyone and values to those
// who may launch has to ask before it renders, not refuse after.
func (h Handlers) allowsProjectAction(projectID, userID string, action projectAction) bool {
	if h.isGlobalAdminUserID(userID) {
		return true
	}
	if item, found, err := h.projectByID(projectID); err == nil && found && h.projectOwnedBy(item, userID) {
		return action.permits(access.RoleAdmin)
	}
	role, ok := h.effectiveProjectRole(projectID, userID)
	// The Community rule is asked about the base: it knows three roles, and a
	// question about a fourth has no honest answer. The matrix, which may know
	// more, is asked about the role as held.
	fallback := ok && action.permits(h.baseRole(role))
	return h.canProjectAction(userID, projectID, role, action.id, fallback)
}

func (h Handlers) requireProjectRole(
	w http.ResponseWriter,
	projectID string,
	userID string,
	action projectAction,
	// label names the operation in the error the caller receives. It is prose
	// and never an input to the decision.
	label string,
) bool {
	if h.isGlobalAdminUserID(userID) {
		return true
	}
	if item, found, err := h.projectByID(projectID); err == nil && found && h.projectOwnedBy(item, userID) {
		return action.permits(access.RoleAdmin)
	}
	role, ok := h.effectiveProjectRole(projectID, userID)
	fallback := ok && action.permits(h.baseRole(role))
	if !h.canProjectAction(userID, projectID, role, action.id, fallback) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "insufficient role for " + label})
		return false
	}
	return true
}

func (h Handlers) requireProjectMember(
	w http.ResponseWriter,
	projectID string,
	userID string,
	action string,
) bool {
	if !h.hasProjectMembership(userID, projectID) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "project membership required for " + action})
		return false
	}
	return true
}

func (h Handlers) hasProjectMembership(userID, projectID string) bool {
	if h.isGlobalAdminUserID(userID) {
		return true
	}
	if item, found, err := h.projectByID(projectID); err == nil && found && h.projectOwnedBy(item, userID) {
		return true
	}
	// Every route a grant can take, not only the direct one.
	//
	// This asked the access store for the personal grant alone, so a project
	// opened to a whole organization - or to a team - stayed closed to its
	// members: the API answered 404, which reads as "no such project" rather
	// than "not for you", and nothing anywhere said a grant had been ignored.
	// Found by granting a real project to a real team on the DC and watching
	// the member still get 404; the unit tests passed throughout, because they
	// exercised effectiveProjectRole and this gate never called it.
	role, ok := h.effectiveProjectRole(strings.TrimSpace(projectID), strings.TrimSpace(userID))
	if !ok {
		return false
	}
	return h.canProjectAction(userID, projectID, role, projectActionRead, true)
}

func (h Handlers) projectOwnedBy(item project.Project, userID string) bool {
	ownerType := strings.ToLower(strings.TrimSpace(item.OwnerType))
	ownerID := strings.TrimSpace(item.OwnerID)
	if ownerType == "" {
		ownerType = "user"
	}
	if ownerType == "user" {
		return ownerID != "" && strings.EqualFold(ownerID, strings.TrimSpace(userID))
	}
	return ownerType == "organization" && h.userBelongsToOrganization(userID, ownerID)
}

func (h Handlers) userBelongsToOrganization(userID, organizationID string) bool {
	if h.keycloak == nil || strings.TrimSpace(organizationID) == "" {
		return false
	}
	organizations, err := h.keycloak.ListUserOrganizations(strings.TrimSpace(userID))
	if err != nil {
		return false
	}
	for _, organization := range organizations {
		if organization.Enabled && strings.EqualFold(organization.ID, strings.TrimSpace(organizationID)) {
			return true
		}
	}
	return false
}

func (h Handlers) isGlobalAdminUserID(userID string) bool {
	uid := strings.TrimSpace(userID)
	if uid == "" {
		return false
	}
	adminUser := strings.TrimSpace(h.bootstrapAdminUser)
	if adminUser != "" && strings.EqualFold(uid, adminUser) {
		return true
	}
	return false
}

// effectiveProjectRole is the strongest role a user holds on a project,
// whether granted to them personally, to a team they are in, or to an
// organization they belong to.
//
// Grants add up rather than override. Removing someone from an organization
// must not silently take away access they were given personally, and a
// personal viewer role must not cap an organization's editor grant - either
// behaviour would make an administrator's action have an effect they did not
// ask for and cannot see. A team is a third source and obeys the same rule:
// leaving one takes away what the team gave and nothing else.
//
// The three are compared and not ordered. There is no sense in which a team
// grant outranks an organization grant, and building a hierarchy between the
// sources would mean an administrator could not tell what a person may do
// without first knowing where each grant came from.
func (h Handlers) effectiveProjectRole(projectID, userID string) (access.Role, bool) {
	direct, hasDirect := h.accessStore.GetRole(projectID, userID)
	viaTeam := h.teamProjectRole(projectID, userID)
	viaOrganization := h.organizationProjectRole(projectID, userID)

	best := h.strongestRole(direct, viaTeam, viaOrganization)
	return best, best != "" || hasDirect
}

// teamProjectRole is the strongest grant reaching this user through a team.
//
// An unconfigured store returns no role rather than an error, the same way the
// organization path treats an unreachable directory: a platform running
// without teams must behave exactly as it did before they existed, and a
// failure to read them must never hand out access nor take away what somebody
// holds by another route.
func (h Handlers) teamProjectRole(projectID, userID string) access.Role {
	if h.teamStore == nil {
		return ""
	}
	grants, err := h.teamStore.ListProjectRolesForUser(
		strings.TrimSpace(projectID), strings.TrimSpace(userID))
	if err != nil || len(grants) == 0 {
		return ""
	}
	best := access.Role("")
	for _, grant := range grants {
		best = h.strongestRole(best, grant.Role)
	}
	return best
}

// strongestRole compares grants that are not all built-in.
//
// access.Strongest ranks the three roles the platform defines, and a custom
// role is not one of them - it would rank zero and lose to any grant beside
// it, which would quietly cap a person the installation meant to widen. So
// the comparison is made on the bases, and the winning *grant* is returned
// whole: the matrix has to see the role that was actually given, not the
// built-in it resolves to. Equal bases keep the first, which is the direct
// grant - between two equivalent grants, the one made to the person is the one
// an administrator can find and change.
func (h Handlers) strongestRole(roles ...access.Role) access.Role {
	best := access.Role("")
	bestRank := 0
	for _, role := range roles {
		if strings.TrimSpace(string(role)) == "" {
			continue
		}
		rank := h.baseRole(role).Rank()
		if rank > bestRank {
			best, bestRank = role, rank
		}
	}
	return best
}

// organizationProjectRole is the strongest grant reaching this user through an
// organization. Failing to reach Keycloak returns no role rather than an
// error: a directory outage must not hand out access, and it must not remove
// the access a user holds personally either.
func (h Handlers) organizationProjectRole(projectID, userID string) access.Role {
	if h.accessStore == nil || h.keycloak == nil {
		return ""
	}
	grants, err := h.accessStore.ListOrganizationRoles(projectID)
	if err != nil || len(grants) == 0 {
		return ""
	}
	organizations, err := h.keycloak.ListUserOrganizations(strings.TrimSpace(userID))
	if err != nil {
		return ""
	}
	member := make(map[string]struct{}, len(organizations))
	for _, organization := range organizations {
		member[strings.TrimSpace(organization.ID)] = struct{}{}
	}

	best := access.Role("")
	for _, grant := range grants {
		if _, ok := member[strings.TrimSpace(grant.OrganizationID)]; ok {
			best = h.strongestRole(best, grant.Role)
		}
	}
	return best
}

func (h Handlers) canProjectAction(userID, projectID string, role access.Role, action string, fallback bool) bool {
	if action == "" {
		action = projectActionRead
	}
	if h.editionHooks.RBAC == nil {
		return fallback
	}
	return h.editionHooks.RBAC.CanAccessProjectAction(
		auth.Identity{Username: strings.TrimSpace(userID), Roles: map[string]struct{}{}},
		strings.TrimSpace(projectID),
		role,
		action,
		fallback,
	)
}

func (h Handlers) projectExists(projectID string) (bool, error) {
	_, found, err := h.projectByID(projectID)
	return found, err
}

func (h Handlers) projectByID(projectID string) (project.Project, bool, error) {
	projects, err := h.projectStore.List()
	if err != nil {
		return project.Project{}, false, err
	}
	for _, p := range projects {
		if p.ID == projectID {
			return p, true, nil
		}
	}
	return project.Project{}, false, nil
}

func (h Handlers) requireGlobalAdmin(w http.ResponseWriter, r *http.Request) (auth.Identity, bool) {
	return h.requireAdminModule(w, r, "global")
}

func (h Handlers) requireAdminModule(w http.ResponseWriter, r *http.Request, module string) (auth.Identity, bool) {
	identity, ok := h.requireIdentity(w, r)
	if !ok {
		return auth.Identity{}, false
	}
	fallback := h.isGlobalAdmin(identity)
	if h.editionHooks.RBAC != nil && h.editionHooks.RBAC.CanAccessAdminModule(identity, module, fallback) {
		return identity, true
	}
	if fallback {
		return identity, true
	}
	writeJSON(w, http.StatusForbidden, map[string]string{"error": "global admin role required"})
	return auth.Identity{}, false
}

func (h Handlers) requireAdminModuleFromSessionOrBearer(w http.ResponseWriter, r *http.Request, module string) (auth.Identity, bool) {
	identity, ok := h.requireIdentityFromSessionOrBearer(w, r)
	if !ok {
		return auth.Identity{}, false
	}
	fallback := h.isGlobalAdmin(identity)
	if h.editionHooks.RBAC != nil && h.editionHooks.RBAC.CanAccessAdminModule(identity, module, fallback) {
		return identity, true
	}
	if fallback {
		return identity, true
	}
	writeJSON(w, http.StatusForbidden, map[string]string{"error": "global admin role required"})
	return auth.Identity{}, false
}

func (h Handlers) isGlobalAdmin(identity auth.Identity) bool {
	fallbackFn := func(id auth.Identity) bool {
		return h.defaultIsGlobalAdmin(id)
	}
	if h.editionHooks.RBAC != nil {
		return h.editionHooks.RBAC.IsGlobalAdmin(identity, fallbackFn)
	}
	return fallbackFn(identity)
}

func (h Handlers) defaultIsGlobalAdmin(identity auth.Identity) bool {
	if identity.HasRole(globalAdminRole) {
		return true
	}
	if strings.TrimSpace(h.bootstrapAdminUser) != "" && identity.MatchesUsername(h.bootstrapAdminUser) {
		return true
	}
	if strings.TrimSpace(h.bootstrapAdminEmail) != "" && identity.MatchesEmail(h.bootstrapAdminEmail) {
		return true
	}
	return false
}

func bearerTokenFromHeader(r *http.Request) (string, error) {
	authz := strings.TrimSpace(r.Header.Get(authHeader))
	if authz == "" {
		return "", errors.New("missing Authorization header")
	}
	parts := strings.SplitN(authz, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", errors.New("invalid Authorization format")
	}
	return strings.TrimSpace(parts[1]), nil
}

// isGrantableSubjectType is who a dataset or an ontology may be granted to.
//
// Teams were absent until 2026-09-26, and their absence was not a policy: the
// same line refusing them was written twice, in datasets.go and ontology.go,
// while teams already reached projects through SetProjectTeamRole. The gap
// showed when Essilor asked for a modality to be given to one of their teams
// and to one of Inria's - a grant that could only be spelled as "to this
// organization", too wide, or "to these twelve people", a list that rots.
func isGrantableSubjectType(subjectType string) bool {
	switch subjectType {
	case "user", "organization", "team":
		return true
	}
	return false
}
