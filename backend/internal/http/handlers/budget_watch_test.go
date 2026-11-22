package handlers

import (
	"testing"
	"time"
)

// Le mois est la periode, en UTC. Un budget qui se reinitialiserait a une
// heure differente selon le lecteur serait impossible a rapprocher d'une
// facture.
func TestLaPeriodeEstLeMoisCalendaireEnUTC(t *testing.T) {
	got := monthStart(time.Date(2026, 11, 22, 23, 41, 0, 0, time.FixedZone("CET", 3600)))
	want := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("monthStart = %s, attendu %s", got, want)
	}
	if got.Location() != time.UTC {
		t.Fatalf("periode hors UTC : %s", got.Location())
	}
	// Un instant tot dans le mois suivant bascule bien de periode.
	suivant := monthStart(time.Date(2026, 12, 1, 0, 0, 1, 0, time.UTC))
	if suivant.Month() != time.December {
		t.Fatalf("le 1er decembre retombe en %s", suivant.Month())
	}
}
