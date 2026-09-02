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
	// Label and Description are English fallbacks. The interface translates by
	// key where it has a catalogue entry, so a backend string never decides
	// which language a user reads.
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
	KeyAlertFormat          = "alert.format"
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
			Label:       "Maximum workspace lifetime",
			Description: "A workspace is stopped past this age. \"0\" disables the sweep entirely. This is an age limit, not idle detection.",
			Fallback:    "48h",
		},
		{
			Key:         KeyAlertWebhookURL,
			EnvVar:      "NORYX_ALERT_WEBHOOK_URL",
			Kind:        KindURL,
			Label:       "Alert webhook",
			Description: "HTTP endpoint receiving alerts. Empty: alerts remain visible in the interface only.",
			Fallback:    "",
		},
		{
			Key:         KeyAlertInstanceName,
			EnvVar:      "NORYX_ALERT_INSTANCE_NAME",
			Kind:        KindString,
			Label:       "Instance name",
			Description: "Identifies this platform in alerts, to tell several installations apart.",
			Fallback:    "",
		},
		{
			Key:    KeyAlertFormat,
			EnvVar: "NORYX_ALERT_FORMAT",
			Kind:   KindEnum,
			Label:  "Alert format",
			Description: "JSON carries the whole alert and suits Slack, Teams and any " +
				"integration that reads it. Plain text suits a receiver that shows the body " +
				"as-is, such as a self-hosted ntfy.",
			Values:   []string{"json", "text"},
			Fallback: "json",
		},
		{
			Key:         KeyDefaultTheme,
			EnvVar:      "NORYX_DEFAULT_THEME",
			Kind:        KindEnum,
			Label:       "Default theme",
			Description: "Theme applied to users who have expressed no preference.",
			Values:      []string{"", "light", "dark"},
			Fallback:    "",
		},
		{
			Key:         KeyBackendVersion,
			Kind:        KindString,
			Label:       "Backend version",
			Description: "Stamped into the binary at build time. It changes with the code, not with a setting.",
			ReadOnly:    true,
		},
		{
			Key:         KeyEdition,
			Kind:        KindString,
			Label:       "Edition",
			Description: "Determined by the deployment configuration.",
			ReadOnly:    true,
		},
		{
			Key:         KeyNamespace,
			Kind:        KindString,
			Label:       "Kubernetes namespace",
			Description: "Namespace hosting the control plane.",
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
			return fmt.Errorf("invalid duration: expected for example 48h, 90m or 0")
		}
		if parsed < 0 {
			return fmt.Errorf("duration cannot be negative")
		}
	case KindURL:
		parsed, err := url.Parse(value)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("invalid address: expected an absolute URL, for example https://hooks.example.com/…")
		}
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return fmt.Errorf("only http and https are accepted")
		}
	case KindEnum:
		for _, candidate := range d.Values {
			if candidate == value {
				return nil
			}
		}
		return fmt.Errorf("invalid value: expected %s", strings.Join(nonEmpty(d.Values), ", "))
	case KindString:
		if len(value) > 200 {
			return fmt.Errorf("value too long: 200 characters at most")
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
