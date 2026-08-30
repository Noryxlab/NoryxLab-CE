package settings

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"
)

// Store persists administrator overrides. Only the resolver reads it directly.
type Store interface {
	Get(key string) (string, bool, error)
	Set(key, value, actor string) error
	List() (map[string]string, error)
}

// Resolver answers "what is the effective value of this setting", applying the
// precedence rule: stored override, then environment, then fallback.
//
// Values are cached briefly. Components such as the workspace reaper consult
// the resolver on every sweep, and a database round trip per sweep would be
// wasteful, but a change must still take effect without a restart - that is
// the whole point of the store. A few seconds of staleness is the compromise.
type Resolver struct {
	store Store

	mu        sync.RWMutex
	cache     map[string]string
	cachedAt  time.Time
	cacheFor  time.Duration
	cacheable bool
}

func NewResolver(store Store) *Resolver {
	return &Resolver{
		store:     store,
		cache:     map[string]string{},
		cacheFor:  10 * time.Second,
		cacheable: store != nil,
	}
}

// String returns the effective value, empty when nothing supplies one.
func (r *Resolver) String(key string) string {
	definition, found := Lookup(key)
	if !found {
		return ""
	}
	if stored, ok := r.stored(key); ok && strings.TrimSpace(stored) != "" {
		return strings.TrimSpace(stored)
	}
	if definition.EnvVar != "" {
		if fromEnv := strings.TrimSpace(os.Getenv(definition.EnvVar)); fromEnv != "" {
			return fromEnv
		}
	}
	return definition.Fallback
}

// Duration returns the effective value parsed as a duration. An unparseable
// value falls back to the declared default rather than disabling the feature
// silently: a typo in a setting must not quietly turn off workspace reaping.
func (r *Resolver) Duration(key string) time.Duration {
	raw := r.String(key)
	if raw == "" {
		return 0
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed < 0 {
		definition, _ := Lookup(key)
		fallback, fallbackErr := time.ParseDuration(definition.Fallback)
		if fallbackErr != nil {
			return 0
		}
		log.Printf("settings: %s=%q is not a usable duration, falling back to %s", key, raw, fallback)
		return fallback
	}
	return parsed
}

// Set validates and persists an override.
func (r *Resolver) Set(key, value, actor string) error {
	definition, found := Lookup(key)
	if !found {
		return errUnknownSetting{key: key}
	}
	if err := definition.Validate(value); err != nil {
		return err
	}
	if r.store == nil {
		return errNoStore{}
	}
	if err := r.store.Set(key, strings.TrimSpace(value), actor); err != nil {
		return err
	}
	r.invalidate()
	return nil
}

// Effective returns every setting with its current value and where it came
// from, which is what an administration screen needs in order to explain why a
// value is what it is.
type Effective struct {
	Definition
	Value string `json:"value"`
	// Source is "stored", "environment" or "default".
	Source string `json:"source"`
	// Overridable is false when the environment pins a value the store cannot
	// take precedence over. Reserved for future use; today the store always wins.
	Overridable bool `json:"overridable"`
}

func (r *Resolver) Effective() []Effective {
	out := make([]Effective, 0, len(Definitions()))
	for _, definition := range Definitions() {
		value := ""
		source := "default"
		if stored, ok := r.stored(definition.Key); ok && strings.TrimSpace(stored) != "" {
			value, source = strings.TrimSpace(stored), "stored"
		} else if definition.EnvVar != "" && strings.TrimSpace(os.Getenv(definition.EnvVar)) != "" {
			value, source = strings.TrimSpace(os.Getenv(definition.EnvVar)), "environment"
		} else {
			value = definition.Fallback
		}
		if definition.Secret && value != "" {
			value = "********"
		}
		out = append(out, Effective{
			Definition:  definition,
			Value:       value,
			Source:      source,
			Overridable: true,
		})
	}
	return out
}

func (r *Resolver) stored(key string) (string, bool) {
	if r.store == nil {
		return "", false
	}
	r.mu.RLock()
	fresh := r.cacheable && time.Since(r.cachedAt) < r.cacheFor
	if fresh {
		value, ok := r.cache[key]
		r.mu.RUnlock()
		return value, ok
	}
	r.mu.RUnlock()

	values, err := r.store.List()
	if err != nil {
		// A database blip must not change the effective configuration; fall
		// through to environment and defaults.
		return "", false
	}
	r.mu.Lock()
	r.cache = values
	r.cachedAt = time.Now()
	r.mu.Unlock()

	value, ok := values[key]
	return value, ok
}

func (r *Resolver) invalidate() {
	r.mu.Lock()
	r.cachedAt = time.Time{}
	r.mu.Unlock()
}

type errUnknownSetting struct{ key string }

func (e errUnknownSetting) Error() string {
	return "réglage inconnu : " + e.key
}

type errNoStore struct{}

func (errNoStore) Error() string {
	return "aucun magasin de réglages configuré sur cette instance"
}
