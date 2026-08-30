// Package settings holds the platform settings an administrator can change
// without a redeployment.
//
// Configuration had accumulated in three places that drift: the Kubernetes
// manifest, the container environment, and whatever an operator had last set
// on the live deployment. The symptom was NORYX_BACKEND_VERSION reading
// 0.5.188-dev in the manifest while 0.5.196 was running, so the platform
// misreported its own version until someone noticed.
//
// The rule here is one line: an environment variable is the bootstrap value, a
// stored setting overrides it. A setting that has never been set falls through
// to the environment, so an installation that configures nothing behaves
// exactly as before.
package settings

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

// Kind constrains what a value may hold, so the API can reject a bad value
// before it reaches the component that would choke on it.
type Kind string

const (
	KindDuration Kind = "duration"
	KindString   Kind = "string"
	KindURL      Kind = "url"
	KindEnum     Kind = "enum"
)

// Definition declares one setting: its identity, how it is validated, and
// where its bootstrap value comes from.
type Definition struct {
	Key string `json:"key"`
	// EnvVar supplies the bootstrap value when nothing is stored.
	EnvVar string `json:"envVar"`
	Kind   Kind   `json:"kind"`
	// Label and Description are shown in the administration interface.
	Label       string `json:"label"`
	Description string `json:"description"`
	// Values enumerates the accepted options for KindEnum.
	Values []string `json:"values,omitempty"`
	// Fallback applies when neither a stored value nor the environment
	// supplies one.
	Fallback string `json:"fallback"`
	// Secret marks a value that must never be returned to a client.
	Secret bool `json:"secret"`
	// ReadOnly marks a fact rather than a setting: something an administrator
	// should be able to see in one place but cannot change, because it is
	// determined elsewhere. The version is the archetype - it changes when the
	// code changes, not when someone edits a field.
	ReadOnly bool `json:"readOnly"`
}

// Keys of the settings this platform exposes. Declared rather than free-form:
// an arbitrary key/value store becomes undocumented configuration nobody can
// audit.
const (
	KeyWorkspaceMaxLifetime = "workspace.max_lifetime"
	KeyAlertWebhookURL      = "alert.webhook_url"
	KeyAlertInstanceName    = "alert.instance_name"
	KeyDefaultTheme         = "ui.default_theme"

	// Facts, exposed for visibility and refused for writing.
	KeyBackendVersion = "platform.backend_version"
	KeyEdition        = "platform.edition"
	KeyNamespace      = "platform.namespace"
)

// Definitions is the complete registry.
func Definitions() []Definition {
	return []Definition{
		{
			Key:         KeyWorkspaceMaxLifetime,
			EnvVar:      "NORYX_WORKSPACE_MAX_LIFETIME",
			Kind:        KindDuration,
			Label:       "Durée de vie maximale des workspaces",
			Description: "Un workspace est arrêté au-delà de cette durée. « 0 » désactive complètement l'arrêt automatique. Il s'agit d'un âge, pas d'une détection d'inactivité.",
			Fallback:    "48h",
		},
		{
			Key:         KeyAlertWebhookURL,
			EnvVar:      "NORYX_ALERT_WEBHOOK_URL",
			Kind:        KindURL,
			Label:       "Webhook d'alerte",
			Description: "Adresse HTTP recevant les alertes. Vide : les alertes restent visibles dans l'interface uniquement.",
			Fallback:    "",
		},
		{
			Key:         KeyAlertInstanceName,
			EnvVar:      "NORYX_ALERT_INSTANCE_NAME",
			Kind:        KindString,
			Label:       "Nom de l'instance",
			Description: "Identifie cette plateforme dans les alertes, pour distinguer plusieurs installations.",
			Fallback:    "",
		},
		{
			Key:         KeyDefaultTheme,
			EnvVar:      "NORYX_DEFAULT_THEME",
			Kind:        KindEnum,
			Label:       "Thème par défaut",
			Description: "Thème appliqué aux utilisateurs n'ayant pas exprimé de préférence.",
			Values:      []string{"", "light", "dark"},
			Fallback:    "",
		},
		{
			Key:         KeyBackendVersion,
			Kind:        KindString,
			Label:       "Version du backend",
			Description: "Estampillée dans le binaire à la compilation. Elle change avec le code, pas avec un réglage.",
			ReadOnly:    true,
		},
		{
			Key:         KeyEdition,
			Kind:        KindString,
			Label:       "Édition",
			Description: "Déterminée par la configuration de déploiement.",
			ReadOnly:    true,
		},
		{
			Key:         KeyNamespace,
			Kind:        KindString,
			Label:       "Namespace Kubernetes",
			Description: "Namespace hébergeant le plan de contrôle.",
			ReadOnly:    true,
		},
	}
}

// Lookup returns the definition for a key.
func Lookup(key string) (Definition, bool) {
	for _, definition := range Definitions() {
		if definition.Key == key {
			return definition, true
		}
	}
	return Definition{}, false
}

// Validate reports whether raw is acceptable for this definition, returning a
// message an administrator can act on rather than a type error.
func (d Definition) Validate(raw string) error {
	value := strings.TrimSpace(raw)
	if value == "" {
		// Empty always means "fall back", never an invalid value.
		return nil
	}
	switch d.Kind {
	case KindDuration:
		parsed, err := time.ParseDuration(value)
		if err != nil {
			return fmt.Errorf("durée invalide : attendu par exemple 48h, 90m ou 0")
		}
		if parsed < 0 {
			return fmt.Errorf("la durée ne peut pas être négative")
		}
	case KindURL:
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("adresse invalide : attendu une URL absolue, par exemple https://hooks.example.com/…")
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fmt.Errorf("seuls http et https sont acceptés")
		}
	case KindEnum:
		for _, candidate := range d.Values {
			if candidate == value {
				return nil
			}
		}
		return fmt.Errorf("valeur invalide : attendu %s", strings.Join(nonEmpty(d.Values), ", "))
	case KindString:
		if len(value) > 200 {
			return fmt.Errorf("valeur trop longue : 200 caractères au maximum")
		}
	}
	return nil
}

func nonEmpty(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
