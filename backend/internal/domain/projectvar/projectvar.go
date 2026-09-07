// Package projectvar holds a project's environment variables: the settings a
// piece of work needs, as opposed to the credentials a person carries.
//
// A secret belongs to somebody. A tracking server's address, a bucket name, a
// model registry's URL belong to the work, and everybody doing that work needs
// the same value. Keeping them as personal secrets meant a colleague could not
// run what you ran until they had recreated your secrets by hand, under the
// same names, from a list nobody had written down.
package projectvar

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type Variable struct {
	ProjectID string `json:"projectId"`
	Name      string `json:"name"`
	// ValueEncrypted never leaves the store. Values are encrypted at rest -
	// people put connection strings in these whatever the screen says - and
	// decrypted for the members allowed to read them.
	ValueEncrypted string    `json:"-"`
	Description    string    `json:"description,omitempty"`
	UpdatedBy      string    `json:"updatedBy,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
	UpdatedAt      time.Time `json:"updatedAt"`
}

// Name rules are the shell's, because that is where these end up: a name the
// shell cannot export is a variable nothing can read.
var namePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Reserved prefixes belong to the platform. A project that could set
// NORYX_SECRET_… or PATH would be rewriting the environment its own workload
// depends on, and the failure would look like a platform bug.
var reservedPrefixes = []string{"NORYX_", "KUBERNETES_"}

var reservedNames = map[string]bool{
	"PATH": true, "HOME": true, "USER": true, "SHELL": true,
	"HOSTNAME": true, "PWD": true, "TERM": true, "LANG": true,
}

const (
	MaxNameLength  = 64
	MaxValueLength = 8192
)

// ValidateName says whether a name may be used, and why not when it may not.
func ValidateName(name string) error {
	name = strings.TrimSpace(name)
	switch {
	case name == "":
		return fmt.Errorf("a name is required")
	case len(name) > MaxNameLength:
		return fmt.Errorf("a name is at most %d characters", MaxNameLength)
	case !namePattern.MatchString(name):
		return fmt.Errorf("a name holds letters, digits and underscores, and does not start with a digit")
	case reservedNames[strings.ToUpper(name)]:
		return fmt.Errorf("%s belongs to the environment the workload runs in", strings.ToUpper(name))
	}
	for _, prefix := range reservedPrefixes {
		if strings.HasPrefix(strings.ToUpper(name), prefix) {
			return fmt.Errorf("names starting with %s are the platform's", prefix)
		}
	}
	return nil
}

func New(projectID, name, valueEncrypted, description, updatedBy string) Variable {
	now := time.Now().UTC()
	return Variable{
		ProjectID:      strings.TrimSpace(projectID),
		Name:           strings.TrimSpace(name),
		ValueEncrypted: valueEncrypted,
		Description:    strings.TrimSpace(description),
		UpdatedBy:      strings.TrimSpace(updatedBy),
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}
