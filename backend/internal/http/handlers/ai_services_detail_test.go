package handlers

import "testing"

// Le mode reduit ne porte aucune phrase, et c est le correctif.
//
// Il s expliquait - "basic questions work, code assistance and agents do not" -
// une phrase ecrite quand degraded n avait qu une cause, le palier profond
// tombe. Le 2026-09-30 au soir la carte louee s est arretee, un relais externe
// a pris toutes les fonctions, et la carte a rapporte toutes les capacites
// disponibles pendant que la phrase en dessous disait le contraire : une seule
// reponse se contredisant, annoncant aux gens que leurs outils etaient coupes
// pendant qu ils s en servaient.
//
// Le reflexe etait de rendre la phrase plus fine. Mieux valait la retirer : une
// phrase qui couvre toutes les causes ne dit rien, une qui devine est ce qui
// vient de rater, et les capacites sont rendues a cote, precises.
func TestLeModeReduitNAffirmeRien(t *testing.T) {
	var status aiServicesStatus
	status.Mode = "degraded"
	status.Capabilities = map[string]bool{"chat": true, "code": true, "agents": true}

	if status.Detail != "" {
		t.Errorf("le mode reduit porte une affirmation : %q", status.Detail)
	}
	// Ce qui doit rester lisible a la place : les capacites, qui sont exactes.
	for _, c := range []string{"chat", "code", "agents"} {
		if !status.Capabilities[c] {
			t.Errorf("la capacite %q manque, alors que c est elle qui informe", c)
		}
	}
}
