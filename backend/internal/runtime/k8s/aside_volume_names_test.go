package k8s

import (
	"testing"

	noryxruntime "github.com/Noryxlab/NoryxLab-CE/backend/internal/runtime"
)

// Un pod qui porte un conteneur a cote ET un conteneur avant ne doit pas
// declarer deux volumes du meme nom.
//
// Le nom etait numerote par l'indice de la boucle, qui repart a zero au second
// appel : deux volumes secret-side-0 dans un pod, et l'API refuse la spec
// entiere. Le workload ne demarrait jamais, avec un message nommant un volume
// que personne n'avait ecrit.
func TestDeuxConteneursAnnexesNeCollisionnentPas(t *testing.T) {
	volumes := []map[string]any{}
	noms := map[string]string{}

	cote := &noryxruntime.SidecarSpec{
		Name:  "a-cote",
		Image: "busybox",
		Secrets: []noryxruntime.SecretMount{
			{SecretName: "s1", MountPath: "/run/un"},
			{SecretName: "s2", MountPath: "/run/deux"},
		},
	}
	avant := &noryxruntime.SidecarSpec{
		Name:  "avant",
		Image: "busybox",
		Secrets: []noryxruntime.SecretMount{
			{SecretName: "s3", MountPath: "/run/trois"},
		},
	}

	if c := asideContainer(cote, "cote", &volumes, noms); c == nil {
		t.Fatal("le conteneur a cote n'a pas ete construit")
	}
	if c := asideContainer(avant, "avant", &volumes, noms); c == nil {
		t.Fatal("le conteneur avant n'a pas ete construit")
	}

	vus := map[string]int{}
	for _, v := range volumes {
		nom, _ := v["name"].(string)
		vus[nom]++
	}
	for nom, compte := range vus {
		if compte > 1 {
			t.Fatalf("le volume %q est declare %d fois ; l'API refuserait le pod", nom, compte)
		}
	}
	if len(volumes) != 3 {
		t.Fatalf("%d volumes pour 3 secrets", len(volumes))
	}
}
