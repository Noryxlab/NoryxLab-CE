/* Reordonner les dossiers d'un extrait.
 *
 *  La selection dit QUELS fichiers ; la disposition dit comment ils sont
 *  ranges pour travailler. « Que possede ce patient » et « montre-moi tous
 *  mes scans de cornee, peu importe a qui » sont deux questions sur le meme
 *  ensemble de fichiers, et la seconde veut la categorie en premier.
 *
 *  Sorti du composant parce que la premiere version etait fausse de deux
 *  facons a la fois et qu'aucune ne se voyait a la lecture du JSX. */

/** reordonner place un niveau a une position, en echangeant avec celui qui
 *  l'occupait deja ailleurs.
 *
 *  Echanger, et non dedupliquer. La version precedente posait le niveau puis
 *  filtrait les doublons, donc choisir « periode » au dossier 1 produisait
 *  [periode, periode, categorie] puis [periode, categorie] : l'entite
 *  disparaissait de l'arbre au lieu de descendre d'un cran.
 *
 *  Un niveau absent de la liste - parce que la disposition est courte -
 *  remplace simplement l'occupant, qui sort de l'arbre. C'est ce qu'on a
 *  demande : la liste montre ce qu'elle produit, et l'apercu avec. */
export function reordonner(niveaux: string[], rang: number, level: string): string[] {
  if (rang < 0 || rang >= niveaux.length) return niveaux;
  const suite = [...niveaux];
  const occupant = suite[rang];
  if (occupant === undefined || occupant === level) return niveaux;
  const ailleurs = suite.indexOf(level);
  if (ailleurs !== -1) {
    suite[ailleurs] = occupant;
  }
  suite[rang] = level;
  return suite;
}
