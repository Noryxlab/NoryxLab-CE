package handlers

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/health"
)

// Access tokens expire, and a repository stops cloning the morning they do.
//
// GitHub caps a fine-grained token at a year and GitLab caps its project
// tokens at a year, so every repository wired to one has a date on which it
// breaks - a date somebody knew when they created it and nobody wrote down.
// The failure then arrives in a workspace's startup log, silently, for
// everyone at once.
//
// The warning windows are wide on purpose: renewing a token means asking a
// person to go to a provider, create a credential and paste it back, which is
// not a five-minute job on the morning it breaks.
const (
	tokenWarningWindow  = 30 * 24 * time.Hour
	tokenCriticalWindow = 7 * 24 * time.Hour
)

func (h Handlers) tokenExpiryAlerts() []healthAlert {
	if h.secretStore == nil || h.repositoryStore == nil {
		return nil
	}
	repositories, err := h.repositoryStore.ListAll()
	if err != nil {
		return nil
	}
	secrets, err := h.secretStore.ListAll()
	if err != nil {
		return nil
	}

	// Only the tokens something depends on. A personal secret nobody wired to
	// a repository is that person's business; a token three repositories clone
	// with is the platform's.
	usedBy := map[string][]string{}
	for _, repo := range repositories {
		name := strings.TrimSpace(repo.AuthSecretName)
		if name == "" {
			continue
		}
		key := strings.TrimSpace(repo.OwnerUserID) + "|" + name
		usedBy[key] = append(usedBy[key], repo.Name)
	}

	now := time.Now().UTC()
	alerts := []healthAlert{}
	for _, item := range secrets {
		key := strings.TrimSpace(item.UserID) + "|" + strings.TrimSpace(item.Name)
		repos, used := usedBy[key]
		if !used || item.ExpiresAt == nil {
			continue
		}
		remaining := item.ExpiresAt.Sub(now)
		if remaining > tokenWarningWindow {
			continue
		}
		sort.Strings(repos)
		subject := fmt.Sprintf("%s (%s)", item.Name, strings.Join(repos, ", "))
		switch {
		case remaining <= 0:
			alerts = append(alerts, healthAlert{
				Scope:    health.ScopePlatform,
				Severity: healthCritical,
				Source:   "tokens",
				Summary:  "an access token a repository clones with has expired",
				Detail: subject + " expired " + item.ExpiresAt.Format(time.DateOnly) +
					"; those repositories no longer clone, in every workspace that attaches them",
				Action: "repositories",
			})
		case remaining <= tokenCriticalWindow:
			alerts = append(alerts, healthAlert{
				Scope:    health.ScopePlatform,
				Severity: healthCritical,
				Source:   "tokens",
				Summary:  "an access token expires within a week",
				Detail: subject + " expires " + item.ExpiresAt.Format(time.DateOnly) +
					"; renewing it means asking its owner to create a new one at the provider",
				Action: "repositories",
			})
		default:
			alerts = append(alerts, healthAlert{
				Scope:    health.ScopePlatform,
				Severity: healthWarning,
				Source:   "tokens",
				Summary:  "an access token expires within a month",
				Detail:   subject + " expires " + item.ExpiresAt.Format(time.DateOnly),
				Action:   "repositories",
			})
		}
	}
	return alerts
}
