package handlers

import "testing"

// A prefix is a folder inside the dataset, never a way out of it.
func TestDatasetPrefixStaysInsideTheDataset(t *testing.T) {
	for raw, want := range map[string]string{
		"":                    "",
		"/":                   "",
		"subject-001":         "subject-001/",
		"/subject-001/visit/": "subject-001/visit/",
		"../../etc":           "",
		"a/../../b":           "",
		".":                   "",
	} {
		if got := sanitizeDatasetPrefix(raw); got != want {
			t.Fatalf("sanitizeDatasetPrefix(%q) = %q, want %q", raw, got, want)
		}
	}
}
