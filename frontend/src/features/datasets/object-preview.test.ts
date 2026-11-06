import { describe, expect, it } from 'vitest';
import { parseDelimited, previewKindOf } from './object-preview';

describe('previewKindOf', () => {
  it('reconnait les trois familles et refuse le reste', () => {
    expect(previewKindOf('cohorte/mesures.CSV')).toBe('table');
    expect(previewKindOf('notes.txt')).toBe('text');
    expect(previewKindOf('coupe.PNG')).toBe('image');
    // Une serie DICOM n'a pas d'apercu : la refuser ici evite d'ouvrir une
    // fenetre vide sur chaque ligne d'un dossier qui n'en contient que.
    expect(previewKindOf('IM-0001-0001.dcm')).toBeNull();
    expect(previewKindOf('mesures.nrrd')).toBeNull();
  });
});

describe('parseDelimited', () => {
  it('respecte les guillemets, y compris une virgule dans un champ', () => {
    const { rows } = parseDelimited('nom,ville\n"Dupont, Jean",Saint-Etienne\n');
    expect(rows[1]).toEqual(['Dupont, Jean', 'Saint-Etienne']);
  });

  it('devine le point-virgule, qui est ce qu\'Excel en francais ecrit', () => {
    const { rows } = parseDelimited('sujet;acuite;date\nSELENA-01-001;0,8;20260115\n');
    expect(rows[1]).toEqual(['SELENA-01-001', '0,8', '20260115']);
  });

  it('devine la tabulation', () => {
    const { rows } = parseDelimited('a\tb\n1\t2\n');
    expect(rows[1]).toEqual(['1', '2']);
  });

  it('rend un guillemet double comme un seul', () => {
    const { rows } = parseDelimited('mesure\n"oeil ""droit"""\n');
    expect(rows[1]).toEqual(['oeil "droit"']);
  });

  it('compte toutes les lignes en n\'en rendant que la limite', () => {
    const corps = Array.from({ length: 50 }, (_, index) => `${index},valeur`).join('\n');
    const { rows, total } = parseDelimited(`index,colonne\n${corps}\n`, 10);
    expect(rows).toHaveLength(10);
    expect(total).toBe(51);
  });

  it('accepte une fin de ligne Windows et un fichier sans saut final', () => {
    const { rows, total } = parseDelimited('a,b\r\n1,2');
    expect(total).toBe(2);
    expect(rows[1]).toEqual(['1', '2']);
  });
});
