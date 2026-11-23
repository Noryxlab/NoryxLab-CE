package k8s

import "testing"

func TestLEmpreinteEstExtraiteDesFormesQueLesRuntimesPublient(t *testing.T) {
	for _, essai := range []struct {
		imageID string
		attendu string
	}{
		{
			"harbor.lan/noryx-environments/noryx-runtime@sha256:1eb79da6b80710e648e1fe03d3ec5786768ef6d09094a822e9fef4eee75c57ff",
			"sha256:1eb79da6b80710e648e1fe03d3ec5786768ef6d09094a822e9fef4eee75c57ff",
		},
		{
			"docker-pullable://harbor.lan/noryx-environments/noryx-vscode@sha256:abc123",
			"sha256:abc123",
		},
		// Une empreinte seule, sans depot : certaines versions la publient
		// ainsi.
		{"sha256:abc123", "sha256:abc123"},
		// Un algorithme autre que sha256, prevu par la specification OCI.
		{"harbor.lan/x/y@sha512:def456", "sha512:def456"},
	} {
		if got := digestFromImageID(essai.imageID); got != essai.attendu {
			t.Errorf("digestFromImageID(%q) = %q, want %q", essai.imageID, got, essai.attendu)
		}
	}
}

// Rien plutot qu'une provenance fausse.
//
// Un identifiant local - une image chargee a la main dans le noeud - ne
// designe rien dans un registre. L'enregistrer comme empreinte donnerait une
// ligne qui a l'air d'une provenance et dont aucun rejeu ne pourrait repartir.
func TestUnIdentifiantLocalNeDonneAucuneEmpreinte(t *testing.T) {
	for _, imageID := range []string{"", "   ", "noryx-runtime:0.1.0", "abcdef123456"} {
		if got := digestFromImageID(imageID); got != "" {
			t.Errorf("digestFromImageID(%q) = %q, want empty", imageID, got)
		}
	}
}
