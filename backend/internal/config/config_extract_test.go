package config

import (
	"os"
	"testing"
)

// Renommer une variable d environnement sans filet ne casse rien de visible :
// la valeur disparait et le defaut prend sa place. NORYX_COHORT_CACHE_SIZE est
// posee sur EMSE ; sans ce repli le cache serait retombe a 50 Gi sans que
// personne le sache avant qu un montage manque de place.
func TestLAncienNomDeVariableEstEncoreLu(t *testing.T) {
	for _, nom := range []string{"NORYX_EXTRACT_CACHE_SIZE", "NORYX_COHORT_CACHE_SIZE"} {
		t.Setenv(nom, "")
	}
	os.Unsetenv("NORYX_EXTRACT_CACHE_SIZE")
	t.Setenv("NORYX_COHORT_CACHE_SIZE", "120Gi")
	if got := Load().ExtractCacheSize; got != "120Gi" {
		t.Errorf("taille = %q, attendu 120Gi : l ancien nom doit encore etre lu", got)
	}
}

// Et le nouveau nom gagne quand les deux sont poses, sinon une migration de
// configuration ne prendrait jamais effet.
func TestLeNouveauNomGagneSurLAncien(t *testing.T) {
	t.Setenv("NORYX_COHORT_CACHE_SIZE", "50Gi")
	t.Setenv("NORYX_EXTRACT_CACHE_SIZE", "200Gi")
	if got := Load().ExtractCacheSize; got != "200Gi" {
		t.Errorf("taille = %q, attendu 200Gi", got)
	}
}
