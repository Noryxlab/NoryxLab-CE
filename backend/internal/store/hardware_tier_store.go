package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/hardware"

type HardwareTierStore interface {
	List() ([]hardware.Tier, error)
	Upsert(tier hardware.Tier) error
	Delete(id string) error
}
