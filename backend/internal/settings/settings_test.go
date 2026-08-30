package settings

import (
	"testing"
	"time"
)

type fakeStore struct{ values map[string]string }

func (f *fakeStore) Get(key string) (string, bool, error) { v, ok := f.values[key]; return v, ok, nil }
func (f *fakeStore) Set(key, value, _ string) error       { f.values[key] = value; return nil }
func (f *fakeStore) List() (map[string]string, error)     { return f.values, nil }

// Precedence is the whole contract: stored beats environment beats fallback.
func TestPrecedence(t *testing.T) {
	store := &fakeStore{values: map[string]string{}}
	r := NewResolver(store)

	if got := r.String(KeyWorkspaceMaxLifetime); got != "48h" {
		t.Fatalf("with nothing set, expected the declared fallback 48h, got %q", got)
	}

	t.Setenv("NORYX_WORKSPACE_MAX_LIFETIME", "12h")
	r = NewResolver(store)
	if got := r.String(KeyWorkspaceMaxLifetime); got != "12h" {
		t.Fatalf("environment must beat the fallback, got %q", got)
	}

	if err := r.Set(KeyWorkspaceMaxLifetime, "6h", "tester"); err != nil {
		t.Fatalf("set failed: %v", err)
	}
	if got := r.String(KeyWorkspaceMaxLifetime); got != "6h" {
		t.Fatalf("a stored value must beat the environment, got %q", got)
	}
}

// A typo must not silently disable a feature: an unusable duration falls back
// to the declared default rather than to zero, which would mean "never reap".
func TestUnusableDurationFallsBackNotOff(t *testing.T) {
	store := &fakeStore{values: map[string]string{KeyWorkspaceMaxLifetime: "quarante-huit heures"}}
	r := NewResolver(store)
	if got := r.Duration(KeyWorkspaceMaxLifetime); got != 48*time.Hour {
		t.Fatalf("expected the 48h fallback, got %s", got)
	}
}

// An explicit zero is a deliberate choice and must be honoured.
func TestExplicitZeroDisables(t *testing.T) {
	store := &fakeStore{values: map[string]string{KeyWorkspaceMaxLifetime: "0"}}
	if got := NewResolver(store).Duration(KeyWorkspaceMaxLifetime); got != 0 {
		t.Fatalf("expected 0, got %s", got)
	}
}

func TestValidationRejectsBadValues(t *testing.T) {
	cases := []struct {
		key     string
		value   string
		wantErr bool
	}{
		{KeyWorkspaceMaxLifetime, "48h", false},
		{KeyWorkspaceMaxLifetime, "", false},
		{KeyWorkspaceMaxLifetime, "-1h", true},
		{KeyWorkspaceMaxLifetime, "bientot", true},
		{KeyAlertWebhookURL, "https://hooks.example.com/x", false},
		{KeyAlertWebhookURL, "pas-une-url", true},
		{KeyAlertWebhookURL, "ftp://example.com", true},
		{KeyDefaultTheme, "dark", false},
		{KeyDefaultTheme, "chartreuse", true},
	}
	for _, tc := range cases {
		definition, ok := Lookup(tc.key)
		if !ok {
			t.Fatalf("unknown key %q", tc.key)
		}
		err := definition.Validate(tc.value)
		if tc.wantErr && err == nil {
			t.Fatalf("%s=%q should have been rejected", tc.key, tc.value)
		}
		if !tc.wantErr && err != nil {
			t.Fatalf("%s=%q should have been accepted: %v", tc.key, tc.value, err)
		}
	}
}

func TestUnknownKeyIsRejected(t *testing.T) {
	r := NewResolver(&fakeStore{values: map[string]string{}})
	if err := r.Set("noryx.enable_magic", "true", "tester"); err == nil {
		t.Fatal("an undeclared key must be rejected rather than stored")
	}
}

// A fact must be visible and unwritable: the version is determined by the
// build, and an administrator able to edit it recreates exactly the drift this
// design removes.
func TestReadOnlyFactsAreVisibleButRefused(t *testing.T) {
	r := NewResolver(&fakeStore{values: map[string]string{}})
	r.SetFact(KeyBackendVersion, "0.5.200-ee.1")

	if got := r.String(KeyBackendVersion); got != "0.5.200-ee.1" {
		t.Fatalf("a fact must be readable, got %q", got)
	}
	if err := r.Set(KeyBackendVersion, "1.0.0-marketing", "tester"); err == nil {
		t.Fatal("a read-only entry must refuse a write")
	}
	if got := r.String(KeyBackendVersion); got != "0.5.200-ee.1" {
		t.Fatalf("the refused write must not have taken effect, got %q", got)
	}

	for _, entry := range r.Effective() {
		if entry.Key != KeyBackendVersion {
			continue
		}
		if entry.Overridable {
			t.Fatal("a fact must not be presented as overridable")
		}
		if entry.Source != "build" {
			t.Fatalf("expected source build, got %q", entry.Source)
		}
	}
}
