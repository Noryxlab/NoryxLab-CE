package store

import "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/pod"

type PodStore interface {
	List() ([]pod.Launch, error)
	Create(p pod.Launch) error
}
