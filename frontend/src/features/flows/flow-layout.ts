import type { FlowStep } from '@/lib/api/types';

/**
 * Ranger un graphe en colonnes.
 *
 * Le rang d'une etape est le plus long chemin qui mene a elle : une etape qui
 * attend `maillage`, lequel attend `geometrie`, est au rang 2 quoi qu'elle
 * attende par ailleurs. Le plus long et non le plus court, parce que c'est
 * celui qui dit quand elle peut reellement commencer - avec le plus court,
 * une etape apparaitrait a cote de quelque chose qu'elle attend encore.
 *
 * Deux etapes au meme rang sont des etapes qui peuvent tourner ensemble, et
 * c'est tout l'interet de la vue : l'embranchement se voit.
 */

/** Une dependance qui ne designe aucune etape est ignoree plutot que fatale.
 *  Le serveur valide le graphe a l'enregistrement, donc ce cas ne vient pas
 *  d'une saisie - il vient d'un graphe en cours d'ecriture dans le formulaire,
 *  ou l'auteur a nomme une etape qu'il n'a pas encore ajoutee. Faire planter
 *  l'apercu pendant la frappe serait le pire moment. */
export function flowRanks(steps: FlowStep[]): Map<string, number> {
  const byKey = new Map(steps.map((step) => [step.key, step]));
  const ranks = new Map<string, number>();
  // Le marquage protege d'un cycle : le serveur les refuse, mais cette
  // fonction sert aussi l'apercu d'un graphe pas encore enregistre, ou un
  // cycle existe le temps de deux frappes.
  const visiting = new Set<string>();

  const rankOf = (key: string): number => {
    const known = ranks.get(key);
    if (known !== undefined) return known;
    if (visiting.has(key)) return 0;
    const step = byKey.get(key);
    if (!step) return 0;
    visiting.add(key);
    const parents = (step.dependsOn ?? []).filter((parent) => byKey.has(parent));
    const rank = parents.length === 0 ? 0 : Math.max(...parents.map((parent) => rankOf(parent) + 1));
    visiting.delete(key);
    ranks.set(key, rank);
    return rank;
  };

  for (const step of steps) rankOf(step.key);
  return ranks;
}

/** Les colonnes, dans l'ordre, chacune avec ses etapes dans l'ordre declare.
 *  L'ordre declare et non l'ordre alphabetique : l'auteur a ecrit ses etapes
 *  dans un ordre qui veut dire quelque chose pour lui. */
export function flowColumns(steps: FlowStep[]): FlowStep[][] {
  const ranks = flowRanks(steps);
  const depth = steps.length === 0 ? 0 : Math.max(...steps.map((step) => ranks.get(step.key) ?? 0)) + 1;
  const columns: FlowStep[][] = Array.from({ length: depth }, () => []);
  for (const step of steps) columns[ranks.get(step.key) ?? 0]?.push(step);
  return columns;
}
