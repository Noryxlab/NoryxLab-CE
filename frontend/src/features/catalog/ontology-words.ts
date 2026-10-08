import type { Ontology } from '@/lib/api/types';

/* Les mots du metier qui possede la donnee, et les petites regles de langue
 * qui vont avec.
 *
 * Partages parce que la meme page les contredisait : la card disait
 * « Entites », la regle de lecture « Sujet », la liste des formes « subject »,
 * et le tableau de couverture « entitys ». Un seul endroit, donc un seul mot.
 */

/** Ce que cette ontologie appelle ses trois niveaux.
 *
 *  Les noms viennent de la regle qui a pris CETTE photographie, jamais de la
 *  regle du dataset aujourd'hui (ADR-040) : un nom change depuis ferait
 *  decrire une vieille photographie dans un vocabulaire avec lequel elle n'a
 *  jamais ete lue. Vides, la plateforme parle sa propre langue - celle de
 *  l'ecran, pas celle du serveur. */
export function motsDuMetier(ontology: Ontology | null | undefined) {
  const regle = (ontology?.manifest as ManifestAvecRegle | undefined)?.readingRule;
  return {
    sujet: regle?.subjectName?.trim() || 'entité',
    visite: regle?.visitName?.trim() || 'période',
    modalite: regle?.modalityName?.trim() || 'catégorie',
  };
}

type ManifestAvecRegle = {
  readingRule?: { subjectName?: string; visitName?: string; modalityName?: string };
};

/** Le pluriel d'un mot que quelqu'un a ecrit lui-meme.
 *
 *  La version precedente collait un s, ce qui donnait « entitys » et
 *  « categorys » en toutes lettres dans un tableau - parce que le mot par
 *  defaut arrivait en anglais du serveur et que le repli francais n'etait
 *  jamais atteint. Le serveur ne l'envoie plus, mais la regle reste fausse des
 *  qu'on ecrit « bilan » ou « journal » dans la regle de lecture. */
export function pluriel(mot: string): string {
  if (/[sxz]$/.test(mot)) return mot;
  if (/(au|eu)$/.test(mot)) return `${mot}x`;
  if (/al$/.test(mot)) return `${mot.slice(0, -2)}aux`;
  return `${mot}s`;
}

export function capitaliser(mot: string): string {
  return mot.charAt(0).toUpperCase() + mot.slice(1);
}

/** Une date de visite telle qu'elle est ecrite dans les chemins.
 *
 *  Les cles portent AAAAMMJJ, et la card l'affichait tel quel : « 20260218 »
 *  se lit comme un numero, pas comme le 18 fevrier. Tout ce qui n'est pas huit
 *  chiffres est rendu inchange - c'est une chaine du dataset, pas une date que
 *  la plateforme a le droit d'interpreter. */
export function dateDeVisite(brut: string | undefined, locale: string): string {
  if (!brut) return '';
  if (!/^\d{8}$/.test(brut)) return brut;
  const jour = new Date(
    Number(brut.slice(0, 4)),
    Number(brut.slice(4, 6)) - 1,
    Number(brut.slice(6, 8)),
  );
  if (Number.isNaN(jour.getTime())) return brut;
  return jour.toLocaleDateString(locale, { year: 'numeric', month: 'long', day: 'numeric' });
}
