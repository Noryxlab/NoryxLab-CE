package handlers

import (
	"testing"
	"time"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/attestation"
)

// Un document sans date n'est pas un document valide pour toujours : c'est un
// document dont personne n'a dit quand il expire. Les deux se lisent
// differemment dans un rapport, et c'est tout l'interet de la date.
func TestLeStatutDitCeQuOnSaitEtPasCeQuOnEspere(t *testing.T) {
	maintenant := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	hier := maintenant.Add(-24 * time.Hour)
	demain := maintenant.Add(24 * time.Hour)

	cas := []struct {
		nom        string
		validUntil *time.Time
		attendu    string
		expire     bool
	}{
		{"sans date", nil, "undated", false},
		{"encore valable", &demain, "valid", false},
		{"perime", &hier, "expired", true},
	}
	for _, item := range cas {
		document := attestation.Attestation{ValidUntil: item.validUntil}
		if got := document.Status(maintenant); got != item.attendu {
			t.Errorf("%s : statut %q, attendu %q", item.nom, got, item.attendu)
		}
		if got := document.Expired(maintenant); got != item.expire {
			t.Errorf("%s : expire=%v, attendu %v", item.nom, got, item.expire)
		}
	}
}

// Une date de fin vaut jusqu'au bout du jour nomme : un certificat « valable
// jusqu'au 31 mars » est valable le 31 mars, et le lire comme minuit le
// perimerait un jour trop tot.
func TestUneDateDeFinCouvreLeJourEntier(t *testing.T) {
	moment, err := parseOptionalDate("2027-03-31")
	if err != nil || moment == nil {
		t.Fatalf("date refusee : %v", err)
	}
	document := attestation.Attestation{ValidUntil: moment}
	midi := time.Date(2027, 3, 31, 12, 0, 0, 0, time.UTC)
	if document.Expired(midi) {
		t.Error("perime a midi le jour meme de sa fin")
	}
	lendemain := time.Date(2027, 4, 1, 0, 30, 0, 0, time.UTC)
	if !document.Expired(lendemain) {
		t.Error("toujours valable le lendemain")
	}
}

func TestUneDateAbsenteEstAcceptee(t *testing.T) {
	moment, err := parseOptionalDate("   ")
	if err != nil || moment != nil {
		t.Fatalf("une date absente doit etre acceptee comme absente : %v %v", moment, err)
	}
	if _, err := parseOptionalDate("31/03/2027"); err == nil {
		t.Error("un format libre a ete accepte ; la date serait fausse sans que personne le voie")
	}
}

// Le nom du fichier revient dans un en-tete HTTP et dans un telechargement :
// il ne doit porter ni chemin, ni guillemet, ni retour a la ligne.
func TestLeNomDuFichierNeSortPasDeSonEnTete(t *testing.T) {
	cas := map[string]string{
		`../../etc/passwd`:         "passwd",
		`C:\Users\a\formation.pdf`: "formation.pdf",
		"attestation\".pdf":        "attestation.pdf",
		"atte\r\nstation.pdf":      "attestation.pdf",
		"   ":                      "attestation.pdf",
		"formation-hds-2026.pdf":   "formation-hds-2026.pdf",
	}
	for brut, attendu := range cas {
		if got := sanitizeAttestationName(brut); got != attendu {
			t.Errorf("%q -> %q, attendu %q", brut, got, attendu)
		}
	}
}
