package k8s

import "strings"

// L'empreinte reellement executee, telle que le cluster la rapporte.
//
// Kubernetes publie deux choses pour un conteneur : l'image demandee
// (`spec.containers[].image`, souvent une etiquette) et l'image resolue
// (`status.containerStatuses[].imageID`, toujours une empreinte). La seconde
// est la seule qui dise ce qui a tourne, et c'est elle qui manquait aux
// executions planifiees et aux applications : leur image restait une
// etiquette, qui a change de contenu sous l'application de comptabilite entre
// deux demarrages le 2026-11-23 sans que rien ne le dise.
//
// La forme varie selon le runtime de conteneurs, et c'est pour ca que cette
// extraction est une fonction plutot qu'un `strings.Split` recopie :
//
//	containerd : harbor.lan/noryx-environments/noryx-runtime@sha256:1eb7...
//	docker     : docker-pullable://harbor.lan/.../noryx-runtime@sha256:1eb7...
//	et parfois  : sha256:1eb7...  (une empreinte seule, sans depot)
//
// Ce qui est garde est l'empreinte, sans le depot : c'est ce que la colonne
// `image_digest` contient partout ailleurs dans la plateforme, et melanger les
// deux formes rendrait toute comparaison fausse.
func digestFromImageID(imageID string) string {
	imageID = strings.TrimSpace(imageID)
	if imageID == "" {
		return ""
	}
	// Un algorithme autre que sha256 existe dans la specification OCI. Chercher
	// le "@" d'abord et ne retomber sur "sha256:" qu'ensuite accepte les deux
	// sans avoir a les enumerer.
	if at := strings.LastIndex(imageID, "@"); at >= 0 && at < len(imageID)-1 {
		return strings.TrimSpace(imageID[at+1:])
	}
	if index := strings.Index(imageID, "sha256:"); index >= 0 {
		return strings.TrimSpace(imageID[index:])
	}
	// Ni "@" ni empreinte reconnaissable : un identifiant local que le cluster
	// a invente pour une image chargee a la main. Il ne designe rien dans un
	// registre, donc il ne vaut pas mieux que rien et on rend rien - une
	// provenance fausse serait pire qu'une provenance absente.
	return ""
}
