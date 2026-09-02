package handlers

import "testing"

// The platform default theme was inert three ways at once, and each layer
// looked correct on its own: the settings registry declared
// NORYX_DEFAULT_THEME while the config read NORYX_UI_DEFAULT_THEME, the stored
// value was never consulted, and normalizeTheme rejected exactly the two
// values the setting offers and the interface acts on. An operator could pick
// a default, see it saved, and watch nothing happen.
func TestNormalizeThemeAcceptsWhatTheInterfaceActsOn(t *testing.T) {
	for input, want := range map[string]string{
		"light":   "light",
		"dark":    "dark",
		"  DARK ": "dark",
		"Light":   "light",
		// Empty means "no platform default, follow the viewer".
		"":     "",
		"auto": "",
		// Legacy brand names. Branding moved to config.js with the frontend
		// rewrite, so these no longer name a theme and mean "no default".
		"noryx":   "",
		"premyom": "",
		"default": "",
	} {
		if got := normalizeTheme(input); got != want {
			t.Errorf("normalizeTheme(%q) = %q, want %q", input, got, want)
		}
	}
}
