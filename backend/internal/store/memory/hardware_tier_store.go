package memory

import (
	"sort"
	"strings"
	"sync"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/hardware"
)

type HardwareTierStore struct {
	mu    sync.RWMutex
	items []hardware.Tier
}

func NewHardwareTierStore() *HardwareTierStore {
	return &HardwareTierStore{items: hardware.Defaults()}
}

func (s *HardwareTierStore) List() ([]hardware.Tier, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]hardware.Tier, len(s.items))
	copy(out, s.items)
	sort.SliceStable(out, func(i, j int) bool { return out[i].Position < out[j].Position })
	return out, nil
}

func (s *HardwareTierStore) Upsert(tier hardware.Tier) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// One default, always. Two would make the preselected tier depend on list
	// order, which is not a decision anybody made.
	if tier.Default {
		for i := range s.items {
			if !strings.EqualFold(s.items[i].ID, tier.ID) {
				s.items[i].Default = false
			}
		}
	}
	for i := range s.items {
		if strings.EqualFold(s.items[i].ID, tier.ID) {
			s.items[i] = tier
			return nil
		}
	}
	s.items = append(s.items, tier)
	return nil
}

func (s *HardwareTierStore) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	filtered := make([]hardware.Tier, 0, len(s.items))
	for _, item := range s.items {
		if !strings.EqualFold(item.ID, id) {
			filtered = append(filtered, item)
		}
	}
	s.items = filtered
	return nil
}
