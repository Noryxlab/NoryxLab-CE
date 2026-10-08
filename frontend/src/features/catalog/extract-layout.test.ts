import { describe, expect, it } from 'vitest';
import { reordonner } from './extract-layout';

const COMPLET = ['subject', 'visit', 'modality'];

describe('reordonner', () => {
  // Le geste que la premiere version rendait impossible : trier par date.
  //
  // Chaque menu n'offrait que les niveaux encore libres, donc avec les trois
  // places aucun n'etait libre et chaque menu proposait sa propre valeur -
  // trois listes a un seul choix. Et meme forcee, la mise a jour
  // dedupliquait au lieu d'echanger, ce qui effacait un niveau.
  it('met la periode en premier en echangeant avec ce qui y etait', () => {
    expect(reordonner(COMPLET, 0, 'visit')).toEqual(['visit', 'subject', 'modality']);
  });

  it('garde toujours les trois niveaux quand il y en avait trois', () => {
    for (let rang = 0; rang < 3; rang += 1) {
      for (const level of COMPLET) {
        const apres = reordonner(COMPLET, rang, level);
        expect([...apres].sort()).toEqual([...COMPLET].sort());
      }
    }
  });

  it('atteint les six ordres', () => {
    const vus = new Set<string>();
    const explorer = (etat: string[], profondeur: number) => {
      vus.add(etat.join('/'));
      if (profondeur === 0) return;
      for (let rang = 0; rang < etat.length; rang += 1) {
        for (const level of COMPLET) {
          explorer(reordonner(etat, rang, level), profondeur - 1);
        }
      }
    };
    explorer(COMPLET, 3);
    expect(vus.size).toBe(6);
  });

  it('ne bouge pas quand on rechoisit ce qui est deja la', () => {
    expect(reordonner(COMPLET, 1, 'visit')).toBe(COMPLET);
  });

  // Disposition courte : le niveau demande remplace l'occupant, qui sort de
  // l'arbre. C'est ce qu'on a demande, et l'apercu le montre aussitot.
  it('remplace sur une disposition courte', () => {
    expect(reordonner(['subject', 'modality'], 0, 'visit')).toEqual(['visit', 'modality']);
  });

  it('ignore un rang hors de la liste', () => {
    expect(reordonner(COMPLET, 7, 'visit')).toBe(COMPLET);
  });
});
