package handlers

import "testing"

func TestNormalizeAppSlugRemovesAccents(t *testing.T) {
	tests := map[string]string{
		"Météo Couilly":        "meteo-couilly",
		"Évaluation de modèle": "evaluation-de-modele",
		" déjà--propre ":       "deja-propre",
	}
	for input, expected := range tests {
		if got := normalizeAppSlug(input); got != expected {
			t.Fatalf("normalizeAppSlug(%q) = %q, want %q", input, got, expected)
		}
	}
}
