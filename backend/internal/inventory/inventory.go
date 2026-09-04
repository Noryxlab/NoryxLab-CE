// Package inventory holds what software the platform ships, and under which
// licences.
//
// A customer's compliance officer asks the question and the honest answer is
// generated from the resolved dependency graph rather than remembered. It is
// embedded rather than computed at runtime: the answer must describe the build
// that is running, not whatever a container happens to be able to reach.
//
// scripts/ops/generate-software-inventory.py produces the file, and CI fails
// when it no longer matches the dependencies, so the document cannot quietly
// drift away from the product it describes.
package inventory

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed software-inventory.json
var raw []byte

type Item struct {
	Name      string `json:"name"`
	Version   string `json:"version"`
	Licence   string `json:"licence"`
	Component string `json:"component"`
	// Origin distinguishes a licence read from the dependency itself
	// ("detected") from one stated here ("declared"), and marks the ones that
	// could not be resolved. A compliance reader is entitled to know which is
	// which rather than being handed a uniform-looking list.
	Origin   string `json:"origin"`
	Role     string `json:"role,omitempty"`
	Upstream string `json:"upstream,omitempty"`
}

type Document struct {
	GeneratedAt string         `json:"generatedAt"`
	Note        string         `json:"note"`
	Counts      map[string]int `json:"counts"`
	Items       []Item         `json:"items"`
}

// Raw returns the embedded document as served.
func Raw() []byte { return raw }

// Parse decodes it, for callers that need the items rather than the bytes.
func Parse() (Document, error) {
	var document Document
	err := json.Unmarshal(raw, &document)
	return document, err
}

// UnknownLicences counts the components whose licence could not be read. It is
// reported rather than hidden: a gap gets investigated, while a guess in a
// compliance document gets believed.
func (d Document) UnknownLicences() int {
	count := 0
	for _, item := range d.Items {
		if strings.EqualFold(item.Licence, "unknown") {
			count++
		}
	}
	return count
}
