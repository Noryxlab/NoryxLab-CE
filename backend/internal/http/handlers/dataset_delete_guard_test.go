package handlers

import (
	"testing"

	"github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/dataset"
)

// Retirer un dataset ne detruit jamais de la donnee regulee.
//
// La branche de suppression d'objets etait gardee par le seul provider, et
// dataset.New lui donne "minio" par defaut quand l'appelant n'en nomme aucun.
// Un bucket externe enregistre par un script, ou par un formulaire au champ
// laisse vide, se lisait donc comme local : supprimer l'enregistrement
// supprimait les fichiers des patients, et rien dans ce chemin ne regardait la
// classification.
//
// Mesure le 2026-10-12 avant de retirer l'acces HDS du bac a sable : les trois
// datasets HDS portaient "s3" et "clever-cloud", donc la branche ne partait
// pas. C'est de la chance, pas une conception.
func TestUnDatasetHdsNePerdJamaisSesObjetsAvecSonEnregistrement(t *testing.T) {
	for _, cas := range []struct {
		nom     string
		item    dataset.Dataset
		attendu bool
	}{
		{
			// Le cas du defaut: aucun provider nomme, donc "minio".
			nom:     "hds avec le provider par defaut",
			item:    dataset.New("stef", "HDS-For", "", "hds-for", "", "", "hds", "https://cellar-c2.services.clever-cloud.com", ""),
			attendu: false,
		},
		{
			nom:     "hds declare minio explicitement",
			item:    dataset.New("stef", "HDS local", "", "hds-local", "", "minio", "hds", "", ""),
			attendu: false,
		},
		{
			nom:     "hds sur s3 externe",
			item:    dataset.New("stef", "HDS-Essilor", "", "hds-essilor.wp2", "", "s3", "hds", "https://cellar-c2.services.clever-cloud.com", ""),
			attendu: false,
		},
		{
			// Le seul cas vrai: un bucket que la plateforme a cree elle-meme.
			nom:     "bucket non-hds de la plateforme",
			item:    dataset.New("stef", "verif-dataset", "", "noryx-ds-9e89bcf3", "", "", "non-hds", "", ""),
			attendu: true,
		},
		{
			nom:     "non-hds sur s3 externe",
			item:    dataset.New("stef", "imt", "", "nonhds-imt", "", "s3", "non-hds", "https://cellar-c2.services.clever-cloud.com", ""),
			attendu: false,
		},
		{
			nom:     "non-hds sans bucket",
			item:    dataset.New("stef", "vide", "", "", "", "minio", "non-hds", "", ""),
			attendu: false,
		},
	} {
		t.Run(cas.nom, func(t *testing.T) {
			if got := datasetObjectsFollowTheRegistration(cas.item); got != cas.attendu {
				t.Fatalf("%s: provider=%q classification=%q bucket=%q -> %v, attendu %v",
					cas.nom, cas.item.Provider, cas.item.Classification, cas.item.Bucket, got, cas.attendu)
			}
		})
	}
}
