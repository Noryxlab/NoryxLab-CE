package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/quota"

// QuotaStore keeps what each project may run at once. A project with no row
// has no limit, which is what every project has today.
type QuotaStore interface {
	Get(projectID string) (quota.Quota, bool, error)
	List() ([]quota.Quota, error)
	Set(item quota.Quota) error
	Delete(projectID string) error
}
