package auth

import (
	"context"
	"strings"
)

type Identity struct {
	Subject  string
	Username string
	Email    string
	Roles    map[string]struct{}
	// Scopes is set when the caller authenticated with a restricted API token.
	// Empty means unrestricted: a browser session, an OIDC bearer, or a token
	// created before scopes existed.
	Scopes []string
	// TokenProjectID is set when the caller authenticated with a token that
	// belongs to a project rather than to a person. It is what keeps an
	// invoking credential inside the project that issued it.
	TokenProjectID string
	// DeclaredBy is the name a platform service put on its own request, and it
	// is never an identity: it reaches the audit trail and nothing else.
	//
	// It used to be written into Username, so a component holding the shared
	// service secret acted with every right the service has while the audit
	// recorded a named person as the author. Over-permission is a defect;
	// recording it under somebody else's name is a different and worse one.
	DeclaredBy string

	// Component is a platform component's own name, and CredentialID is the
	// identifier of the credential that carried the request. Both are read
	// from the stored token rather than from a header, which is the whole
	// difference between them and DeclaredBy: nobody can claim them.
	//
	// They exist because the audit could not tell two components apart. Every
	// component holds the global administrator role, every one of them
	// authenticated as "platform-service", and a trail that records an
	// administrative deletion without saying which credential performed it
	// cannot answer the first question anybody asks of it.
	Component    string
	CredentialID string
}

// ComponentActorPrefix marks an audit actor as a platform component.
//
// It is kept out of the namespace of usernames on purpose: an account could
// be called "platform-validator", and an action must never be readable as
// that person's work. The prefix is only ever applied to a name the
// credential itself carried.
const ComponentActorPrefix = "service:"

// AuditActor names who acted, for the record kept of it.
func (i Identity) AuditActor() string {
	if component := strings.TrimSpace(i.Component); component != "" {
		return ComponentActorPrefix + component
	}
	return i.UserID()
}

// IsComponent reports whether a platform component is behind this request,
// whether its name was verified from its own credential or merely declared.
func (i Identity) IsComponent() bool {
	return strings.TrimSpace(i.Component) != "" || strings.TrimSpace(i.DeclaredBy) != ""
}

// IsService reports whether this identity is a platform component rather than
// a person, which is what decides whether an action may be attributed to
// anyone.
func (i Identity) IsService() bool { return i.DeclaredBy != "" || i.Username == ServiceUsername }

// ServiceUsername is the only name a shared-secret caller ever carries.
const ServiceUsername = "platform-service"

func (i Identity) UserID() string {
	if i.Username != "" {
		return i.Username
	}
	if i.Email != "" {
		return i.Email
	}
	return i.Subject
}

func (i Identity) HasRole(role string) bool {
	_, ok := i.Roles[role]
	return ok
}

func (i Identity) MatchesUsername(username string) bool {
	return strings.EqualFold(strings.TrimSpace(i.Username), strings.TrimSpace(username))
}

func (i Identity) MatchesEmail(email string) bool {
	return strings.EqualFold(strings.TrimSpace(i.Email), strings.TrimSpace(email))
}

type Verifier interface {
	VerifyBearerToken(token string) (Identity, error)
}

// The identity a platform component resolved itself, carried in the request
// context.
//
// An HTTP client cannot set a context value, which is the whole property: a
// request that carries one was built inside this process by code that had
// already decided who it acts for - an agent calling the API as its owner -
// and the handlers take it as resolved rather than asking for a credential a
// second time.
type identityKey struct{}

func WithIdentity(ctx context.Context, identity Identity) context.Context {
	return context.WithValue(ctx, identityKey{}, identity)
}

func IdentityFrom(ctx context.Context) (Identity, bool) {
	identity, ok := ctx.Value(identityKey{}).(Identity)
	return identity, ok && strings.TrimSpace(identity.UserID()) != ""
}

// The credential that carried an inbound request, resolved once at the edge
// and read by the audit at the other end of the request.
//
// It is not an identity and it does not decide anything: every authorisation
// check downstream resolves the caller itself, exactly as before. This
// records which key turned, so an event can name the component or the token
// behind an action rather than the role they all share.
//
// It is resolved by a middleware rather than by each handler because the
// handlers that most need naming - a deletion, a detach - are the ones that
// answer 204 and have nothing else to say.
type credentialKey struct{}

// Credential is what authenticated a request, as opposed to who acted.
type Credential struct {
	// Actor is the audit actor this credential stands for: a person's
	// username, or a component under ComponentActorPrefix.
	Actor string
	// Component is set when a platform component's own token was presented,
	// and it is verified: it comes from the token record.
	Component string
	// TokenID identifies the credential itself, which is what lets an owner
	// tell which of their tokens acted.
	TokenID string
	// Kind says what sort of credential it was, in one word.
	Kind string
}

func WithCredential(ctx context.Context, credential Credential) context.Context {
	return context.WithValue(ctx, credentialKey{}, credential)
}

func CredentialFrom(ctx context.Context) (Credential, bool) {
	credential, ok := ctx.Value(credentialKey{}).(Credential)
	return credential, ok
}
