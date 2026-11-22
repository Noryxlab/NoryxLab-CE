package postgres

import (
	"database/sql"
	"strings"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/budget"
)

// Ce qu'un projet a le droit de consommer, et ce qu'il a consomme.
//
// La consommation n'est pas stockee : elle se calcule a partir des
// echantillons que le preleveur ecrit toutes les cinq minutes depuis
// septembre. Un total recopie quelque part serait un second chiffre a tenir a
// jour, et le jour ou les deux divergeraient personne ne saurait lequel croit.
//
// Le pas d'echantillonnage est de cinq minutes, donc douze mesures par heure :
// la somme des vCPU divisee par douze donne des vCPU-heures. Cette division
// est ici et nulle part ailleurs.
const samplesPerHour = 12.0

func (s *Store) SetProjectBudget(item budget.Budget) error {
	_, err := s.db.Exec(`
		INSERT INTO project_budgets (project_id, vcpu_hours, updated_by, updated_at)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (project_id) DO UPDATE SET
			vcpu_hours=EXCLUDED.vcpu_hours,
			updated_by=EXCLUDED.updated_by,
			updated_at=EXCLUDED.updated_at`,
		strings.TrimSpace(item.ProjectID), item.VCPUHours, item.UpdatedBy, time.Now().UTC())
	return err
}

func (s *Store) GetProjectBudget(projectID string) (budget.Budget, bool, error) {
	var item budget.Budget
	err := s.db.QueryRow(`SELECT project_id, vcpu_hours, updated_by, updated_at FROM project_budgets WHERE project_id=$1`,
		strings.TrimSpace(projectID)).Scan(&item.ProjectID, &item.VCPUHours, &item.UpdatedBy, &item.UpdatedAt)
	if err == sql.ErrNoRows {
		return budget.Budget{}, false, nil
	}
	if err != nil {
		return budget.Budget{}, false, err
	}
	return item, true, nil
}

func (s *Store) DeleteProjectBudget(projectID string) error {
	_, err := s.db.Exec(`DELETE FROM project_budgets WHERE project_id=$1`, strings.TrimSpace(projectID))
	return err
}

// ListBudgetStates joins what each project may consume with what it has
// consumed since the start of the given period.
//
// Projects with no budget are included on purpose: seeing consumption is what
// lets somebody decide what a budget should be, and a screen that only shows
// projects already under a limit cannot answer "which ones should have one".
func (s *Store) ListBudgetStates(period time.Time) ([]budget.State, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.name,
		       COALESCE(b.vcpu_hours, 0),
		       COALESCE(SUM(u.vcpu), 0) / $2
		FROM projects p
		LEFT JOIN project_budgets b ON b.project_id = p.id
		LEFT JOIN usage_samples u ON u.project_id = p.id AND u.at >= $1
		GROUP BY p.id, p.name, b.vcpu_hours
		ORDER BY p.name`, period.UTC(), samplesPerHour)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []budget.State{}
	for rows.Next() {
		var item budget.State
		if err := rows.Scan(&item.ProjectID, &item.ProjectName, &item.VCPUHours, &item.Consumed); err != nil {
			return nil, err
		}
		item.Period = period.UTC()
		if item.VCPUHours > 0 {
			item.Percent = item.Consumed / item.VCPUHours * 100
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

// MarkBudgetAnnounced records that a threshold was announced for a period, and
// answers whether this call is the one that announced it.
//
// The whole point is that it is atomic: the watcher runs every few minutes,
// and a check-then-write would send the same warning on every pass. The insert
// is the decision.
func (s *Store) MarkBudgetAnnounced(projectID string, period time.Time, threshold int) (bool, error) {
	result, err := s.db.Exec(`
		INSERT INTO project_budget_alerts (project_id, period, threshold, announced_at)
		VALUES ($1,$2,$3,$4) ON CONFLICT DO NOTHING`,
		strings.TrimSpace(projectID), period.UTC(), threshold, time.Now().UTC())
	if err != nil {
		return false, err
	}
	affected, err := result.RowsAffected()
	return affected > 0, err
}
