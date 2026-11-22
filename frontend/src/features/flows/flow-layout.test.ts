import { describe, expect, it } from 'vitest';
import { flowColumns, flowRanks } from './flow-layout';
import type { FlowStep } from '@/lib/api/types';

const step = (key: string, dependsOn?: string[]): FlowStep => ({
  key,
  image: 'noryx-runtime:0.1.0',
  ...(dependsOn ? { dependsOn } : {}),
});

describe('flowRanks', () => {
  it('puts a step with no dependency in the first column', () => {
    const ranks = flowRanks([step('a'), step('b')]);
    expect(ranks.get('a')).toBe(0);
    expect(ranks.get('b')).toBe(0);
  });

  // L'embranchement est la propriete que la vue existe pour montrer : deux
  // etapes qui attendent la meme chose doivent se retrouver cote a cote.
  it('places a fan-out side by side', () => {
    const columns = flowColumns([
      step('geometrie'),
      step('maillage', ['geometrie']),
      step('simulation', ['maillage']),
      step('rendu', ['maillage']),
    ]);
    expect(columns).toHaveLength(3);
    expect(columns[2]?.map((item) => item.key)).toEqual(['simulation', 'rendu']);
  });

  /* Le plus long chemin, pas le plus court.
   *
   *  `rapport` attend `geometrie` (rang 0) et `rendu` (rang 2). Au plus court
   *  il s'afficherait au rang 1, a cote de `maillage` - donc a cote d'une
   *  etape dont il depend indirectement, ce qui se lirait comme « ces deux-la
   *  tournent ensemble » alors que l'un attend l'autre. */
  it('ranks a step by its longest path, not its shortest', () => {
    const ranks = flowRanks([
      step('geometrie'),
      step('maillage', ['geometrie']),
      step('rendu', ['maillage']),
      step('rapport', ['geometrie', 'rendu']),
    ]);
    expect(ranks.get('rapport')).toBe(3);
  });

  // Le formulaire appelle ces fonctions a chaque frappe : une dependance vers
  // une etape que l'auteur n'a pas encore ajoutee ne doit pas casser l'apercu.
  it('ignores a dependency that names no step', () => {
    const ranks = flowRanks([step('a', ['pas-encore-ecrite'])]);
    expect(ranks.get('a')).toBe(0);
  });

  // Et un cycle non plus : le serveur les refuse a l'enregistrement, mais il
  // en existe un le temps de deux frappes.
  it('does not loop on a cycle', () => {
    const ranks = flowRanks([step('a', ['b']), step('b', ['a'])]);
    expect(ranks.size).toBe(2);
  });

  it('has no column for an empty graph', () => {
    expect(flowColumns([])).toEqual([]);
  });
});
