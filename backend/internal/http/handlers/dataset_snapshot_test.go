package handlers

import (
	"os"
	"strings"
	"testing"
)

// Un instantane se prend sur le stockage du dataset, jamais sur celui de la
// plateforme.
//
// L'erreur a existe : les deux chemins listaient par `h.minioClient`, le
// client interne, quel que soit le fournisseur du dataset. Pour un dataset sur
// un S3 externe, ca demandait un bucket du meme nom a la mauvaise machine -
// donc une erreur, ou pire un inventaire d'une donnee qui n'est pas la sienne.
// Et ce sont les datasets reglementes qui sont externes, donc c'etait le cas
// qui comptait.
//
// Le test lit la source plutot que de simuler S3 : ce qu'il faut empecher est
// la reapparition d'un appel, et un double de MinIO ne dirait pas lequel des
// deux clients a ete pris.
func TestLesInstantanesPassentParLeClientDuDataset(t *testing.T) {
	for _, name := range []string{"dataset_snapshot.go", "snapshot_scan.go", "regulated_copy_scan.go"} {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		body := string(source)
		if strings.Contains(body, "h.minioClient") {
			t.Errorf("%s uses h.minioClient: a dataset must be listed through "+
				"h.datasetS3Client(item), otherwise an external dataset is read "+
				"from the platform's own storage", name)
		}
		// datasetObjectStore resolves exactly what datasetS3Client did - the
		// dataset's own endpoint and its own credential - and hands it back
		// behind internal/objectstore. The name changed; the requirement did
		// not, and neither did the reason it exists.
		if !strings.Contains(body, "h.datasetObjectStore(item)") {
			t.Errorf("%s no longer resolves the dataset's own storage", name)
		}
	}
}
