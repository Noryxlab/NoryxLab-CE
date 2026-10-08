package handlers

import "testing"

func TestDatasetDirectObjectKeyStaysUnderDatasetPrefix(t *testing.T) {
	for _, test := range []struct {
		name, prefix, raw, want string
		ok                      bool
	}{
		{"nested object", "incoming", "study/0001/image.dcm", "incoming/study/0001/image.dcm", true},
		{"cleaned object", "incoming/", "study/../manifest.json", "incoming/manifest.json", true},
		{"empty is refused", "incoming", "", "", false},
		{"absolute is refused", "incoming", "/other/object", "", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, ok := datasetDirectObjectKey(test.prefix, test.raw)
			if got != test.want || ok != test.ok {
				t.Fatalf("datasetDirectObjectKey(%q, %q) = (%q, %v), want (%q, %v)", test.prefix, test.raw, got, ok, test.want, test.ok)
			}
		})
	}
}
