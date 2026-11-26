package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func headersFrom(t *testing.T, request *http.Request) http.Header {
	t.Helper()
	recorder := httptest.NewRecorder()
	Handlers{}.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(recorder, request)
	return recorder.Result().Header
}

func TestLesEnTetesDeBaseNeDependentDAucunProxy(t *testing.T) {
	got := headersFrom(t, httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil))
	for name, want := range map[string]string{
		"X-Content-Type-Options": "nosniff",
		"X-Frame-Options":        "DENY",
		"Referrer-Policy":        "strict-origin-when-cross-origin",
	} {
		if got.Get(name) != want {
			t.Errorf("%s = %q, attendu %q", name, got.Get(name), want)
		}
	}
}

// HSTS sur la preuve, pas sur l'espoir : la plateforme voit du HTTP en clair
// parce que le bord termine le TLS, et l'en-tete transmis est le seul temoin.
func TestHSTSSuitLeProtocoleVuParLeNavigateur(t *testing.T) {
	plain := httptest.NewRequest(http.MethodGet, "/", nil)
	if value := headersFrom(t, plain).Get("Strict-Transport-Security"); value != "" {
		t.Errorf("HSTS pose sur une requete en clair: %q", value)
	}

	terminated := httptest.NewRequest(http.MethodGet, "/", nil)
	terminated.Header.Set("X-Forwarded-Proto", "https")
	if value := headersFrom(t, terminated).Get("Strict-Transport-Security"); value == "" {
		t.Error("HSTS absent alors que le bord a termine du TLS")
	}

	// Une chaine de proxys ajoute a droite ; celui qui a parle au navigateur
	// est le premier, et c'est lui qui decide.
	chained := httptest.NewRequest(http.MethodGet, "/", nil)
	chained.Header.Set("X-Forwarded-Proto", "https, http")
	if value := headersFrom(t, chained).Get("Strict-Transport-Security"); value == "" {
		t.Error("une chaine de proxys a fait perdre HSTS")
	}
}

// Verrou de regression : si quelqu'un deplace ce middleware vers l'interieur
// de la chaine, les refus cesseraient de porter les en-tetes sans que rien ne
// casse - exactement la panne silencieuse qu'il remplace.
func TestLesEnTetesSontLaMemeSurUnRefus(t *testing.T) {
	recorder := httptest.NewRecorder()
	Handlers{}.SecurityHeaders(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "forbidden", http.StatusForbidden)
	})).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api/v1/admin/budgets", nil))

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("code = %d", recorder.Code)
	}
	if recorder.Result().Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Error("un refus est sorti sans ses en-tetes")
	}
}
