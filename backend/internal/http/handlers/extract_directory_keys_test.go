package handlers

import (
	"strings"
	"testing"

	extractdomain "github.com/Noryxlab/NoryxLab-CE/backend/internal/domain/extract"
	"github.com/minio/minio-go/v7"
)

// Une cle de dossier ne se monte pas : elle cacherait l'etude.
//
// S3 n'a pas de dossiers, mais un bucket rempli par un outil qui croit le
// contraire porte des cles de zero octet pour eux. En faire un lien pointe
// droit dans le dataset en lecture seule, et le mkdir dont chaque fichier
// dessous a besoin echoue alors contre ce lien : les fichiers sont sautes.
//
// Sur EMSE le 2026-10-01, un extrait de 4 019 fichiers de SELENA-01 en a monte
// 186. Onze dossiers avaient masque tout ce qui etait dessous, et l'arbre
// montrait la disposition brute du bucket a l'interieur de chaque modalite.
func TestUneCleDeDossierNeSeMontePas(t *testing.T) {
	membres := []extractdomain.Member{
		{Path: "SELENA/S1/20260218/ANTERION"},
		{Path: "SELENA/S1/20260218/ANTERION/DICOM"},
		{Path: "SELENA/S1/20260218/ANTERION/DICOM/MC27/f1.dcm"},
		{Path: "SELENA/S1/20260218/ANTERION/DICOM/MC27/f2.dcm"},
		{Path: "SELENA/S1/20260218/ANTERION/mesures.csv"},
	}
	gardes := withoutDirectoryKeys(membres)
	if len(gardes) != 3 {
		t.Fatalf("gardes = %d, attendu 3 fichiers", len(gardes))
	}
	for _, membre := range gardes {
		if !strings.Contains(membre.Path, ".") {
			t.Fatalf("un dossier est reste : %q", membre.Path)
		}
	}
}

// Et un fichier qui se trouve porter le nom d'un dossier n'est pas perdu.
//
// La regle est "prefixe d'une autre cle", pas "sans extension" : un DICOM
// s'appelle souvent 449313, et le confondre avec un dossier retirerait de
// l'etude des fichiers bien reels.
func TestUnFichierSansExtensionResteDansLExtrait(t *testing.T) {
	membres := []extractdomain.Member{
		{Path: "SELENA/S1/20260218/ANTERION/DICOM/MC27/449313"},
		{Path: "SELENA/S1/20260218/ANTERION/DICOM/MC27/449314"},
	}
	if len(withoutDirectoryKeys(membres)) != 2 {
		t.Fatal("un fichier sans extension a ete pris pour un dossier")
	}
}

// Un seul membre, ou aucun, passe sans y toucher.
func TestUnExtraitMinusculeNEstPasFiltre(t *testing.T) {
	if len(withoutDirectoryKeys(nil)) != 0 {
		t.Fatal("nil doit rester vide")
	}
	un := []extractdomain.Member{{Path: "a/b.csv"}}
	if len(withoutDirectoryKeys(un)) != 1 {
		t.Fatal("un membre unique ne peut pas etre un prefixe")
	}
}

// Cote scan, les deux conditions comptent.
//
// Zero octet, parce qu'un marqueur est vide par definition. Et prefixe de la
// cle suivante, parce qu'un fichier vide reste un fichier : sans cette
// seconde condition, un fichier de zero octet disparaitrait de l'etude.
func TestLeScanReconnaitUneCleDeDossier(t *testing.T) {
	dossier := minio.ObjectInfo{Key: "SELENA/S1/ANTERION", Size: 0}
	if !estUneCleDeDossier(dossier, "SELENA/S1/ANTERION/DICOM/f.dcm") {
		t.Fatal("une cle vide suivie de ce qu'elle prefixe est un dossier")
	}
	if estUneCleDeDossier(dossier, "SELENA/S1/ANTERIONBIS/f.dcm") {
		t.Fatal("un prefixe de chaine n'est pas un prefixe de chemin")
	}
	if estUneCleDeDossier(dossier, "SELENA/S2/f.dcm") {
		t.Fatal("une cle vide que rien ne prolonge est un fichier vide")
	}
	plein := minio.ObjectInfo{Key: "SELENA/S1/ANTERION", Size: 12}
	if estUneCleDeDossier(plein, "SELENA/S1/ANTERION/DICOM/f.dcm") {
		t.Fatal("une cle qui porte des octets est un fichier")
	}
}

// L'arbre se construit a cote des datasets, pas dans le volume du projet.
//
// Il vivait dans /mnt/extracts, reconstruit par un rm -rf a chaque demarrage :
// quelqu'un qui avait cree son propre /mnt/extracts le perdait. Et un extrait
// est une vue en lecture seule d'un bucket, donc sa place est a cote du
// bucket.
func TestLArbreDesExtraitsEstACoteDesDatasets(t *testing.T) {
	if workspaceExtractsPath != "/extracts" {
		t.Fatalf("chemin = %q, attendu /extracts", workspaceExtractsPath)
	}
	lignes := strings.Join(extractBootstrapLines("/mnt", true, 0, nil), "\n")
	// Ce qui est interdit, c'est d'y CONSTRUIRE. Le script mentionne encore
	// /mnt/extracts, et doit le faire : il y reste des arbres d'avant le
	// deplacement, qu'il ecarte une fois. Interdire la mention plutot que la
	// construction aurait interdit ce nettoyage.
	for _, construction := range []string{
		"mkdir -p '/mnt/extracts'", `cible='/mnt/extracts'`, `dir='/mnt/extracts'`,
		`ln -sfn "$target" '/mnt/extracts'`,
	} {
		if strings.Contains(lignes, construction) {
			t.Fatalf("l'arbre est encore construit dans le volume du projet (%q) :\n%s",
				construction, lignes)
		}
	}
	if !strings.Contains(lignes, "mv '/mnt/extracts'") {
		t.Fatalf("l'ancien arbre doit etre ecarte une fois :\n%s", lignes)
	}
	// L'arbre est reconstruit a chaque demarrage - en vidant le repertoire,
	// pas en le retirant.
	//
	// Ce test exigeait `rm -rf '/extracts'`, et cette commande n'a jamais
	// fonctionne : retirer /extracts demande le droit d'ecrire dans /, qui
	// appartient a root. Elle echouait a chaque demarrage en affichant
	// « Permission denied », et le test la tenait en place. Un test peut
	// maintenir un mecanisme casse aussi surement qu'il en protege un bon ;
	// celui-ci verifie desormais l'effet et non la commande.
	if !strings.Contains(lignes, "-mindepth 1 -delete") {
		t.Fatalf("l'arbre doit etre vide a chaque demarrage :\n%s", lignes)
	}
}
