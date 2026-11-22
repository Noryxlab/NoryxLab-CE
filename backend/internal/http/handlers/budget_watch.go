package handlers

import (
	"context"
	"fmt"
	"log"
	"time"

	budgetdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/budget"
	"github.com/Noryxlab/NoryxLab-CE/backend/internal/notify"
)

// La veille des budgets.
//
// Elle ne coupe rien, et c'est le point de depart : une application est un
// service, pas un calcul qu'on arrete quand il coute - rendre le cout visible
// vaut mieux que le couper. Donc elle dit, une fois par seuil et par mois.
//
// Accrochee au preleveur plutot qu'a une boucle a elle : les deux lisent la
// meme table, et un second rythme aurait fini par decaler.
type budgetWatcher struct {
	handlers Handlers
}

// budgetCheckInterval : un budget mensuel ne se franchit pas a la minute, et
// une verification par quart d'heure suffit a dire les choses le jour meme.
const budgetCheckInterval = 15 * time.Minute

func (h Handlers) StartBudgetWatcher(ctx context.Context) {
	if h.budgetStore == nil {
		return
	}
	log.Printf("budget watcher started (every %s, thresholds %v%%)", budgetCheckInterval, budgetdomain.Thresholds)
	go func() {
		ticker := time.NewTicker(budgetCheckInterval)
		defer ticker.Stop()
		h.checkBudgets()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				h.checkBudgets()
			}
		}
	}()
}

// monthStart is the period a budget covers: a calendar month, in UTC, because
// a budget that reset at a different hour for each reader would be impossible
// to reconcile with an invoice.
func monthStart(now time.Time) time.Time {
	now = now.UTC()
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
}

func (h Handlers) checkBudgets() {
	period := monthStart(time.Now())
	states, err := h.budgetStore.ListBudgetStates(period)
	if err != nil {
		log.Printf("budget watcher: cannot read consumption: %v", err)
		return
	}
	for _, state := range states {
		threshold := budgetdomain.Reached(state.Consumed, state.VCPUHours)
		if threshold == 0 {
			continue
		}
		// L'insertion est la decision : si la ligne existait deja, c'est que
		// ce seuil a ete annonce pour ce mois, et on se tait.
		fresh, err := h.budgetStore.MarkBudgetAnnounced(state.ProjectID, period, threshold)
		if err != nil || !fresh {
			continue
		}
		h.announceBudget(state, threshold, period)
	}
}

func (h Handlers) announceBudget(state budgetdomain.State, threshold int, period time.Time) {
	severity := notify.SeverityWarning
	resume := fmt.Sprintf("Projet %q : %.0f%% du budget du mois consomme", state.ProjectName, state.Percent)
	if threshold >= 100 {
		severity = notify.SeverityCritical
		resume = fmt.Sprintf("Projet %q : budget du mois depasse (%.0f%%)", state.ProjectName, state.Percent)
	}
	h.notifier.SendAsync(notify.Alert{
		Severity: severity,
		Event:    "project.budget.threshold",
		Summary:  resume,
		Details: map[string]any{
			"projectId":   state.ProjectID,
			"projectName": state.ProjectName,
			"threshold":   threshold,
			// Les trois chiffres qui permettent de verifier l'alerte sans
			// ouvrir la plateforme : ce qui etait permis, ce qui a ete pris,
			// et sur quelle periode.
			"budgetVcpuHours":   state.VCPUHours,
			"consumedVcpuHours": state.Consumed,
			"period":            period.Format("2006-01"),
			// Et ce que l'alerte ne fait pas, dit dans l'alerte elle-meme :
			// personne ne doit croire qu'un travail a ete arrete.
			"action": "none: workloads keep running, this is a notice",
		},
	})
	log.Printf("budget: project %s reached %d%% (%.1f of %.1f vCPU-hours)",
		state.ProjectID, threshold, state.Consumed, state.VCPUHours)
}
