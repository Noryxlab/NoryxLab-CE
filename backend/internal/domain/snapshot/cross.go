package snapshot

import (
	"path"
	"sort"
	"strconv"
	"strings"
)

// Le croisement d'empreintes : un objet reglemente qui reapparait ailleurs.
//
// Decide le 2026-11-25, en echange de la fonction qui leve la regle de
// co-montage. Une ecriture a travers un montage FUSE ne passe pas par l'API,
// donc rien ne la journalise ; mais une copie laisse l'objet, et un objet se
// reconnait. On compare ce que porte un dataset reglemente a ce que porte un
// dataset non classe du meme projet, et on nomme ce qui se retrouve des deux
// cotes.
//
// Ce n'est pas une prevention et ne se presente pas comme telle : on constate
// le lendemain matin, la copie existe deja. C'est le prix d'un montage
// inscriptible, assume.
const (
	// Identique : meme ETag. Pour un envoi en une seule partie, l'ETag est le
	// MD5 du contenu - donc deux objets qui le partagent portent les memes
	// octets, et c'est une affirmation, pas un soupcon.
	CopyIdentical = "identical"
	// Probable : meme taille exacte et meme nom de fichier, ETag different ou
	// absent. C'est le cas d'une copie en plusieurs parties, qui recalcule un
	// ETag par morceau et n'a donc aucune chance de retomber sur le meme -
	// autrement dit precisement les gros fichiers, ceux qui comptent. Deux
	// signaux, pas un : la taille seule rapproche des milliers de fichiers.
	CopyProbable = "probable"
	// En dessous de quoi on ne soupconne pas sur la taille et le nom. Un
	// envoi en une seule partie couvre tout ce qui est petit, donc l'ETag
	// repond deja pour ces objets ; en dessous de cette limite, « meme nom,
	// meme taille » ne designe que des README et des __init__.py vides.
	//
	// Mesure sur le DC le 25/11/2026, a travers un montage csi-s3/geesefs :
	// un objet de 1 Mio ressort avec un ETag egal au MD5 du contenu, un de
	// 8 Mio avec « ...-2 » et un de 64 Mio avec « ...-13 » - donc des parties
	// d'environ 5 Mio, et la bascule se situe entre 1 et 8 Mio. Cette limite
	// a 1 Mio couvre donc toute la bande multipart avec de la marge ; entre
	// 1 et 5 Mio les deux signaux repondent, ce qui ne coute rien.
	copyProbableMinSize = 1 << 20
)

// Copy is one object found on both sides.
type Copy struct {
	// RegulatedPath is where it sits in the regulated dataset, OpenPath where
	// it was found. Both named: "three objects match" sends somebody looking,
	// and the person reading the alert needs the pair to decide.
	RegulatedPath string `json:"regulatedPath"`
	OpenPath      string `json:"openPath"`
	Size          int64  `json:"size"`
	Confidence    string `json:"confidence"`
}

// CrossMatch finds the objects of an unclassified dataset that look like
// objects of a regulated one.
//
// Ce qu'il faut savoir de sa portee avant de s'y fier : au-dela d'environ
// 5 Mio une copie est envoyee en plusieurs parties et son ETag ne peut pas
// retomber sur celui de la source si les deux cotes n'ont pas ete ecrits par
// le meme client avec la meme taille de partie. Le gros fichier ne se
// reconnait donc que par « meme taille exacte et meme nom », et **une copie
// renommee lui echappe**. C'est assume : le signal qui resterait, la taille
// seule, est precisement celui qui ment sur de l'imagerie medicale, ou une
// cohorte entiere d'examens aux memes dimensions pese exactement pareil.
//
// Order is deterministic - by the path found in the unclassified dataset -
// because this feeds an alert a person reads and compares with yesterday's.
func CrossMatch(regulated, open []Entry) []Copy {
	if len(regulated) == 0 || len(open) == 0 {
		return []Copy{}
	}
	sorted := append([]Entry(nil), regulated...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	byETag := make(map[string]Entry, len(sorted))
	byNameAndSize := make(map[string]Entry, len(sorted))
	for _, item := range sorted {
		etag := normaliseETag(item.ETag)
		if etag != "" {
			if _, taken := byETag[etag]; !taken {
				byETag[etag] = item
			}
		}
		if item.Size >= copyProbableMinSize {
			key := nameAndSizeKey(item)
			if _, taken := byNameAndSize[key]; !taken {
				byNameAndSize[key] = item
			}
		}
	}

	found := []Copy{}
	for _, item := range open {
		etag := normaliseETag(item.ETag)
		if etag != "" {
			if source, matched := byETag[etag]; matched {
				found = append(found, Copy{
					RegulatedPath: source.Path,
					OpenPath:      item.Path,
					Size:          item.Size,
					Confidence:    CopyIdentical,
				})
				continue
			}
		}
		if item.Size < copyProbableMinSize {
			continue
		}
		if source, matched := byNameAndSize[nameAndSizeKey(item)]; matched {
			found = append(found, Copy{
				RegulatedPath: source.Path,
				OpenPath:      item.Path,
				Size:          item.Size,
				Confidence:    CopyProbable,
			})
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].OpenPath < found[j].OpenPath })
	return found
}

// normaliseETag drops the quotes an S3 implementation may or may not send.
// Two stores disagreeing on quoting would otherwise read as two different
// objects, which is the quiet way this check stops working.
func normaliseETag(value string) string {
	return strings.ToLower(strings.Trim(strings.TrimSpace(value), `"`))
}

// nameAndSizeKey ignores the folders. A copy is rarely laid down at the same
// path - it lands in `resultats/` or at the root of somebody's dataset - so
// comparing full paths would miss the normal case.
func nameAndSizeKey(item Entry) string {
	return strings.ToLower(path.Base(item.Path)) + "\x00" + strconv.FormatInt(item.Size, 10)
}
