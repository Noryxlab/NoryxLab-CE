import { describe, expect, it } from 'vitest';
import { moveTarget } from './move-target';

describe('la destination d’un objet déposé', () => {
  it('place le fichier dans le dossier visé, en gardant son nom', () => {
    expect(moveTarget('etude/patient1.dcm', 'etude/anterion/')).toBe('etude/anterion/patient1.dcm');
    expect(moveTarget('a/b/c.txt', '')).toBe('c.txt');
  });

  it('déplace un dossier entier sous son nouveau parent', () => {
    expect(moveTarget('etude/patient1/', 'archive/')).toBe('archive/patient1');
  });

  it('ne fait rien quand l’objet est déjà dans ce dossier', () => {
    expect(moveTarget('etude/patient1.dcm', 'etude/')).toBeNull();
    expect(moveTarget('racine.txt', '')).toBeNull();
    expect(moveTarget('etude/patient1/', 'etude/')).toBeNull();
  });

  it('refuse un dossier déposé sur lui-même ou dans sa propre branche', () => {
    expect(moveTarget('etude/patient1/', 'etude/patient1/')).toBe('refused');
    expect(moveTarget('etude/patient1/', 'etude/patient1/visite2/')).toBe('refused');
    // Un nom qui commence pareil sans être dedans reste permis : `patient10`
    // n'est pas sous `patient1`.
    expect(moveTarget('etude/patient1/', 'etude/patient10/')).toBe('etude/patient10/patient1');
  });
});
