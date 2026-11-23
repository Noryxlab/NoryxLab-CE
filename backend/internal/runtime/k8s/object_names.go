package k8s

import (
	"fmt"
	"regexp"
	"strings"
)

// Les noms d'objets, verifies ici plutot que par le cluster.
//
// Chaque creation verifiait que le nom n'etait pas vide, ce qui ne protege de
// rien quand le nom est assemble : `record.PodName + "-user-secrets"` sur un
// enregistrement dont le nom de pod est vide donne `-user-secrets`, qui n'est
// pas vide et que Kubernetes refuse. Ce qu'une personne a vu en cliquant
// « Redemarrer » sur l'application de comptabilite, le 2026-11-23 :
//
//	failed to restart: user secret: kubernetes api .../secrets failed:
//	status=422 body={... "a lowercase RFC 1123 subdomain must consist of
//	lower case alphanumeric characters, '-' or '.', and must start and end
//	with an alphanumeric character ... regex used for validation is
//	'[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*'"}
//
// Une expression reguliere de l'API Kubernetes recopiee dans une interface est
// la definition d'un message qui ne dit rien a qui le lit. Et c'est le mauvais
// endroit pour decouvrir le probleme : la requete est partie, le cluster a
// repondu, et la seule information utile - un enregistrement incomplet - n'est
// nulle part dans la reponse.
//
// Donc la verification est celle de Kubernetes, faite avant d'ecrire, et son
// message dit le nom fautif.

// rfc1123Subdomain est la regle que Kubernetes applique au nom de la plupart
// des objets : la meme expression, recopiee de son message d'erreur.
var rfc1123Subdomain = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)

// maxObjectNameLength est la limite de Kubernetes pour un sous-domaine RFC
// 1123. Verifiee ici parce qu'un nom trop long echoue avec le meme message
// illisible qu'un nom mal forme.
const maxObjectNameLength = 253

// validateObjectName answers whether a name is one Kubernetes will accept, and
// says what is wrong with it when it is not.
//
// `kind` names the object in the message - "secret", "pod" - because a caller
// assembling three names from one record needs to know which one broke.
func validateObjectName(kind, name string) error {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return fmt.Errorf("%s name is required", kind)
	}
	if len(trimmed) > maxObjectNameLength {
		return fmt.Errorf("%s name %q is %d characters, over the %d Kubernetes allows",
			kind, trimmed, len(trimmed), maxObjectNameLength)
	}
	if !rfc1123Subdomain.MatchString(trimmed) {
		// Le cas de loin le plus courant, et le seul qui vienne d'un defaut
		// de la plateforme plutot que d'une saisie : un nom assemble dont une
		// partie manquait. Le dire separement evite de faire relire une
		// regle generale a quelqu'un dont le probleme est precis.
		if strings.HasPrefix(trimmed, "-") || strings.HasPrefix(trimmed, ".") {
			return fmt.Errorf("%s name %q starts with %q, which means it was assembled from a value the platform did not have: the record it comes from is incomplete",
				kind, trimmed, trimmed[:1])
		}
		return fmt.Errorf("%s name %q is not a valid Kubernetes name: lower-case letters, digits, '-' and '.', starting and ending with a letter or digit",
			kind, trimmed)
	}
	return nil
}
