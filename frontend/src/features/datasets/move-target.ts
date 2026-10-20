/**
 * Ou va un objet depose sur un dossier.
 *
 * Trois reponses, et les deux premieres evitent un aller-retour au serveur
 * pour un geste qui ne veut rien dire :
 *
 *  - `null` : le geste est sans effet, l'objet est deja dans ce dossier ;
 *  - `"refused"` : un dossier depose sur lui-meme ou sur l'un de ses propres
 *    descendants, ce qui le ferait disparaitre dans sa propre arborescence ;
 *  - sinon, la clef de destination.
 *
 * Le dossier d'arrivee est donne avec sa barre finale, comme partout dans
 * l'explorateur ; la racine est la chaine vide.
 */
export function moveTarget(key: string, folder: string): string | null | 'refused' {
  const isFolder = key.endsWith('/');
  const bare = isFolder ? key.slice(0, -1) : key;
  const name = bare.split('/').pop() ?? bare;
  if (name === '') return 'refused';

  // Deja la : `a/b/fichier` depose sur `a/b/`.
  const parent = bare.slice(0, bare.length - name.length);
  if (parent === folder) return null;

  // Un dossier ne rentre pas dans lui-meme, ni plus bas dans sa propre
  // branche : `a/b/` depose sur `a/b/c/` ferait `a/b/c/b`, qui est dedans.
  if (isFolder && (folder === key || folder.startsWith(key))) return 'refused';

  return `${folder}${name}`;
}
