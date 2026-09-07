package handlers

import (
	"strings"
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/repository"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/secret"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/store/memory"
)

// A token expires on a date somebody knew when they created it and nobody
// wrote down. The morning it passes, every workspace that attaches the
// repository stops cloning, silently and for everyone at once.
func tokenHealthFixture(t *testing.T, expiry *time.Time, wired bool) Handlers {
	t.Helper()
	secrets := memory.NewSecretStore()
	item := secret.New("stef", "github-token", "pat", "encrypted")
	item.ExpiresAt = expiry
	if err := secrets.Upsert(item); err != nil {
		t.Fatal(err)
	}
	repositories := memory.NewRepositoryStore()
	if wired {
		repo := repository.New("stef", "test-git-share", "https://github.com/x/y.git", "main", "github-token", "persat", "", "")
		if err := repositories.Create(repo); err != nil {
			t.Fatal(err)
		}
	}
	return Handlers{secretStore: secrets, repositoryStore: repositories}
}

func at(days int) *time.Time {
	when := time.Now().UTC().Add(time.Duration(days) * 24 * time.Hour)
	return &when
}

func TestATokenAboutToExpireIsReported(t *testing.T) {
	cases := []struct {
		days     int
		severity healthSeverity
	}{
		{days: -1, severity: healthCritical},
		{days: 3, severity: healthCritical},
		{days: 20, severity: healthWarning},
	}
	for _, testCase := range cases {
		alerts := tokenHealthFixture(t, at(testCase.days), true).tokenExpiryAlerts()
		if len(alerts) != 1 {
			t.Fatalf("%d days: expected one alert, got %d", testCase.days, len(alerts))
		}
		if alerts[0].Severity != testCase.severity {
			t.Errorf("%d days: severity %s, expected %s", testCase.days, alerts[0].Severity, testCase.severity)
		}
		// The repository is named: an operator has to know what breaks, not
		// only that something will.
		if !strings.Contains(alerts[0].Detail, "test-git-share") {
			t.Errorf("%d days: the alert does not name the repository: %s", testCase.days, alerts[0].Detail)
		}
	}
}

func TestATokenWithTimeLeftIsQuiet(t *testing.T) {
	if alerts := tokenHealthFixture(t, at(90), true).tokenExpiryAlerts(); len(alerts) != 0 {
		t.Errorf("a token with three months left should say nothing, got %d alerts", len(alerts))
	}
}

// A personal secret nobody wired to a repository is that person's business.
func TestATokenNothingDependsOnIsNotThePlatformsBusiness(t *testing.T) {
	if alerts := tokenHealthFixture(t, at(1), false).tokenExpiryAlerts(); len(alerts) != 0 {
		t.Errorf("an unused secret should not raise a platform alert, got %d", len(alerts))
	}
}

func TestATokenWithNoExpiryIsQuiet(t *testing.T) {
	if alerts := tokenHealthFixture(t, nil, true).tokenExpiryAlerts(); len(alerts) != 0 {
		t.Errorf("a token with no expiry has nothing to warn about, got %d", len(alerts))
	}
}
