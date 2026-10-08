import { describe, expect, it } from 'vitest';

/* La ligne ouverte doit se distinguer des autres.
 *
 *  Huit ecrans ouvrent un panneau de detail sous le tableau quand on clique
 *  une ligne, et aucun ne disait laquelle : on lisait une fiche sans savoir a
 *  quoi elle se rapportait. Pire sur une liste triee par date de mise a jour,
 *  ou la ligne cliquee a souvent bouge entre-temps.
 *
 *  Verifie sur la regle plutot que sur un rendu : ce qui s'est trompe ici
 *  n'est pas la couleur, c'est de comparer la mauvaise chose. */
function estSelectionnee(cle: string, selection: string | null | undefined): boolean {
  return selection != null && cle === selection;
}

describe('ligne selectionnee', () => {
  it('marque la ligne dont le detail est ouvert', () => {
    expect(estSelectionnee('ds-1', 'ds-1')).toBe(true);
    expect(estSelectionnee('ds-2', 'ds-1')).toBe(false);
  });

  // Rien d'ouvert ne doit surligner personne - et surtout pas la premiere
  // ligne, ce qu'un `selection == undefined` compare a une cle absente
  // finirait par faire.
  it('ne marque rien quand rien n est ouvert', () => {
    expect(estSelectionnee('ds-1', null)).toBe(false);
    expect(estSelectionnee('ds-1', undefined)).toBe(false);
  });

  // Une cle vide est une cle : si une ligne la porte vraiment, elle se
  // marque, et sinon personne ne se marque.
  it('traite une cle vide comme une cle', () => {
    expect(estSelectionnee('', '')).toBe(true);
    expect(estSelectionnee('ds-1', '')).toBe(false);
  });
});
