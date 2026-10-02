package handlers

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/auth"
)

// Authentication regression tests.
//
// The header identity was honoured unconditionally: any caller that could
// reach this service could act as any user by naming them, with no token and
// no session. On the deployed platform, `X-Noryx-User: stef` returned that
// user's real projects from an unauthenticated pod. The bearer check was added
// in front of it and this path was never closed behind.

func request(header, value string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	if header != "" {
		r.Header.Set(header, value)
	}
	return r
}

func TestUserHeaderIsRefusedOutsideHeaderAuthMode(t *testing.T) {
	handlers := Handlers{authMode: "oidc"}
	recorder := httptest.NewRecorder()

	if _, ok := handlers.requireIdentity(recorder, request(userHeader, "stef")); ok {
		t.Fatal("a bare user header authenticated in oidc mode")
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

func TestUserHeaderStillWorksInHeaderAuthMode(t *testing.T) {
	// The development mode has to keep working, or every local setup breaks.
	handlers := Handlers{authMode: "header"}
	identity, ok := handlers.requireIdentity(httptest.NewRecorder(), request(userHeader, "stef"))
	if !ok {
		t.Fatal("header mode refused a named user")
	}
	if identity.UserID() != "stef" {
		t.Fatalf("identity = %q, want stef", identity.UserID())
	}
}

func TestServiceTokenAuthenticatesAPlatformComponent(t *testing.T) {
	handlers := Handlers{authMode: "oidc", serviceToken: "s3cr3t"}
	req := request(serviceHeader, "s3cr3t")
	req.Header.Set(userHeader, "platform-validator")

	identity, ok := handlers.requireIdentity(httptest.NewRecorder(), req)
	if !ok {
		t.Fatal("a valid service token was refused")
	}
	// The component still says which one it is - a backup run has to record
	// that - but it says it beside the identity, not as the identity. Written
	// into the username, the name made every downstream user lookup resolve
	// whoever was named, while the identity kept the administrator role.
	if identity.DeclaredBy != "platform-validator" {
		t.Fatalf("declared caller = %q, want platform-validator", identity.DeclaredBy)
	}
	if identity.UserID() != auth.ServiceUsername {
		t.Fatalf("identity = %q, want %q", identity.UserID(), auth.ServiceUsername)
	}
	if !identity.HasRole(globalAdminRole) {
		t.Fatal("a platform component cannot trigger a backup without admin rights")
	}
}

func TestAWrongOrAbsentServiceTokenAuthenticatesNothing(t *testing.T) {
	handlers := Handlers{authMode: "oidc", serviceToken: "s3cr3t"}
	for name, req := range map[string]*http.Request{
		"wrong token":  request(serviceHeader, "not-it"),
		"empty header": request(serviceHeader, ""),
		"no header":    request("", ""),
	} {
		recorder := httptest.NewRecorder()
		if _, ok := handlers.requireIdentity(recorder, req); ok {
			t.Fatalf("%s authenticated", name)
		}
		if recorder.Code != http.StatusUnauthorized {
			t.Fatalf("%s: status = %d, want 401", name, recorder.Code)
		}
	}
}

// A deployment that forgets the secret must refuse its own services, never
// accept an empty header from anyone.
func TestAnUnconfiguredServiceTokenMatchesNothing(t *testing.T) {
	handlers := Handlers{authMode: "oidc", serviceToken: ""}
	recorder := httptest.NewRecorder()
	if _, ok := handlers.requireIdentity(recorder, request(serviceHeader, "")); ok {
		t.Fatal("an empty configured token accepted an empty presented token")
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", recorder.Code)
	}
}

// Both identity paths must behave the same way, or a platform component is
// accepted on some routes and refused on others for reasons invisible from the
// caller's side.
func TestBothIdentityPathsAgreeOnTheServiceToken(t *testing.T) {
	handlers := Handlers{authMode: "oidc", serviceToken: "s3cr3t"}
	req := request(serviceHeader, "s3cr3t")
	req.Header.Set(userHeader, "platform-validator")

	direct, okDirect := handlers.requireIdentity(httptest.NewRecorder(), req)
	session, okSession := handlers.requireIdentityFromSessionOrBearer(httptest.NewRecorder(), req)

	if !okDirect || !okSession {
		t.Fatalf("paths disagree: requireIdentity=%v, sessionOrBearer=%v", okDirect, okSession)
	}
	if direct.UserID() != session.UserID() {
		t.Fatalf("identities differ: %q vs %q", direct.UserID(), session.UserID())
	}
}

// And neither may accept a bare user header outside header auth mode.
func TestNeitherIdentityPathTrustsTheUserHeader(t *testing.T) {
	handlers := Handlers{authMode: "oidc"}
	for name, resolve := range map[string]func(http.ResponseWriter, *http.Request) (auth.Identity, bool){
		"requireIdentity":                    handlers.requireIdentity,
		"requireIdentityFromSessionOrBearer": handlers.requireIdentityFromSessionOrBearer,
	} {
		if _, ok := resolve(httptest.NewRecorder(), request(userHeader, "stef")); ok {
			t.Fatalf("%s authenticated a bare user header", name)
		}
	}
}

// Le schema de JupyterLab ne doit pas manger la session du navigateur.
//
// JupyterLab envoie `Authorization: token <le jeton de serveur qu'on lui a
// donne>` sur chacun de ses appels d'API - noyaux, sessions, terminaux,
// reglages. Tant que la condition etait "l'en-tete est non vide", cet en-tete
// partait au verificateur de JWT, qui le refusait en `invalid jwt format` et
// rendait 401 - alors que le cookie de session du meme appel, parfaitement
// valide, n'etait jamais lu. Resultat : un notebook qui s'ouvre et ne demarre
// aucun noyau. VS Code n'etait pas touche : il s'authentifie par le cookie et
// n'envoie pas d'en-tete Authorization.
//
// Le test porte sur la distinction de schema, pas sur le mot "jupyter" : tout
// schema qui n'est pas le notre doit laisser passer a la session.
func TestUnSchemaEtrangerNEmpechePasLaSession(t *testing.T) {
	for _, entete := range []string{
		"token abcdef0123456789", // JupyterLab
		"Token abcdef0123456789", // la casse ne doit rien changer
		"Basic dXNlcjpwYXNz",     // et tout autre schema
	} {
		// La propriete exacte : un schema etranger ne doit pas atteindre le
		// verificateur de jetons. Tester le message de refus ne dirait rien,
		// parce qu'un deploiement sans magasin de sessions refuse de toute
		// facon - ce qu'on veut savoir, c'est quelle branche a ete prise.
		espion := &verificateurEspion{}
		handlers := Handlers{authMode: "oidc", authVerifier: espion}
		if _, ok := handlers.requireIdentityFromSessionOrBearer(
			httptest.NewRecorder(), request(authHeader, entete)); ok {
			t.Fatalf("%q a authentifie alors qu'il n'y a pas de session", entete)
		}
		if espion.appels > 0 {
			t.Fatalf("%q est parti au verificateur de jetons (%d appel(s)) ; "+
				"le cookie de session de la meme requete n'aurait jamais ete lu",
				entete, espion.appels)
		}
	}
}

// verificateurEspion compte ce qu'on lui soumet, et refuse tout.
type verificateurEspion struct{ appels int }

func (v *verificateurEspion) VerifyBearerToken(string) (auth.Identity, error) {
	v.appels++
	return auth.Identity{}, errors.New("invalid jwt format")
}

// Et notre propre schema continue d'aller au verificateur, sinon la correction
// aurait desactive l'authentification par jeton.
func TestNotreSchemaVaToujoursAuVerificateur(t *testing.T) {
	espion := &verificateurEspion{}
	handlers := Handlers{authMode: "oidc", authVerifier: espion}
	if _, ok := handlers.requireIdentityFromSessionOrBearer(
		httptest.NewRecorder(), request(authHeader, "Bearer pas-un-jwt")); ok {
		t.Fatal("un jeton invalide a ete accepte")
	}
	if espion.appels != 1 {
		t.Fatalf("%d appel(s) au verificateur, attendu 1 : la correction aurait "+
			"desactive l'authentification par jeton", espion.appels)
	}
}
