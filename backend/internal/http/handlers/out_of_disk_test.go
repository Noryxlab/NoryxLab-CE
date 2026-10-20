package handlers

import (
	"strings"
	"testing"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// Une eviction pour disque ephemere ressemble a un plantage.
//
// Kubernetes dit "Evicted: Pod ephemeral local storage usage exceeds the total
// limit of containers 8Gi" : un quota, et au lecteur de deviner ce qui l'a
// rempli. C'est presque toujours un environnement Python construit dans le
// dossier personnel, qui n'est pas persistant - donc la meme erreur coute deux
// fois : la session est arretee, et ce qui etait installe aurait disparu a
// l'arret suivant de toute facon.
func TestUneEvictionPourDisqueEstNommeeEtOrientee(t *testing.T) {
	evince := noryxruntime.PodStatus{
		Phase:     "failed",
		Reason:    "Evicted",
		Message:   "Pod ephemeral local storage usage exceeds the total limit of containers 8Gi.",
		OutOfDisk: true,
	}
	notice, evicted := outOfDiskStatus(evince)
	if !evicted {
		t.Fatal("l'eviction n'a pas ete reconnue")
	}
	for _, attendu := range []string{"/mnt", ".noryx-profile", "/datasets"} {
		if !strings.Contains(notice, attendu) {
			t.Errorf("le message ne dit pas ou installer : %q manque", attendu)
		}
	}
	if strings.Contains(notice, "8Gi") {
		t.Error("le message repete le quota au lieu de dire quoi faire")
	}

	rapport := buildStartupReport("failed", evince, nil)
	var pas *startupStep
	for i := range rapport.Steps {
		if rapport.Steps[i].Key == "environment" {
			pas = &rapport.Steps[i]
		}
	}
	if pas == nil {
		t.Fatal("pas d'etape environment dans le rapport")
	}
	if pas.Detail != "environment_out_of_disk" {
		t.Errorf("detail = %q ; l'interface doit pouvoir le traduire", pas.Detail)
	}
	if !rapport.Stuck {
		t.Error("le rapport ne dit pas que le demarrage est arrete")
	}
}

// Un manque de memoire reste un manque de memoire : les deux quotas ne se
// confondent pas, et le remede n'est pas le meme.
func TestUnManqueDeMemoireNEstPasUnManqueDeDisque(t *testing.T) {
	if _, evicted := outOfDiskStatus(noryxruntime.PodStatus{OutOfMemory: true}); evicted {
		t.Error("un OOM a ete lu comme une eviction disque")
	}
	if _, killed := outOfMemoryStatus(noryxruntime.PodStatus{OutOfDisk: true}); killed {
		t.Error("une eviction disque a ete lue comme un OOM")
	}
}
