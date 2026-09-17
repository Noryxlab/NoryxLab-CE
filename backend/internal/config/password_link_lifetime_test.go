package config

import (
	"os"
	"testing"
	"time"
)

// The window an emailed password link stays open.
//
// It was twelve hours, hard-coded, reasoned as "long enough for somebody who
// reads their mail the next morning". That holds on a Tuesday and fails for
// the case the button is mostly used for: an account created on a Friday
// afternoon and opened on Monday, sixty hours later.
func TestPasswordLinkLifetimeSurvivesAWeekend(t *testing.T) {
	os.Unsetenv("NORYX_PASSWORD_LINK_LIFETIME")
	cfg := Load()
	// Friday 17:00 to Monday 09:00 is sixty-four hours. A default that does not
	// clear it turns every Friday invitation into a second one on Monday.
	const fridayEveningToMondayMorning = 64 * time.Hour
	if cfg.PasswordLinkLifetime < fridayEveningToMondayMorning {
		t.Errorf("default is %s, which expires over a weekend", cfg.PasswordLinkLifetime)
	}
}

func TestPasswordLinkLifetimeIsConfigurable(t *testing.T) {
	// An installation that considers the window too generous has to be able to
	// close it without editing the binary - the reset case is a legitimate
	// reason to want an hour.
	t.Setenv("NORYX_PASSWORD_LINK_LIFETIME", "1h")
	if got := Load().PasswordLinkLifetime; got != time.Hour {
		t.Errorf("got %s, want 1h", got)
	}
}

func TestAnUnusableLifetimeIsRefused(t *testing.T) {
	// Zero and negative are not settings, they are mistakes: a link valid for
	// no time is a button that always fails, and it would fail in the
	// recipient's mailbox rather than here. Nonsense keeps the default and says
	// so in the log.
	for _, raw := range []string{"0", "-1h", "soon", ""} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("NORYX_PASSWORD_LINK_LIFETIME", raw)
			if got := Load().PasswordLinkLifetime; got <= 0 {
				t.Errorf("%q produced %s, which is not a usable window", raw, got)
			}
		})
	}
}
