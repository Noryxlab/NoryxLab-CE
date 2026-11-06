import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { describe, expect, it } from 'vitest';
import { defaultWindow, paintSlice, readDicom, syntaxName, type DicomSlice } from './dicom';

/* Les quatre fichiers d'epreuve sont ecrits par pydicom, pas par nous : un
 * analyseur valide par des fichiers de sa propre main ne prouve rien. Ils
 * couvrent les formes qui arrivent vraiment - huit bits, seize bits signes
 * avec pente, MONOCHROME1, et une syntaxe compressee. */
function fixture(name: string): ArrayBuffer {
  const path = fileURLToPath(new URL(`./__fixtures__/${name}`, import.meta.url));
  const file = readFileSync(path);
  return file.buffer.slice(file.byteOffset, file.byteOffset + file.byteLength) as ArrayBuffer;
}

describe('readDicom', () => {
  it('lit une coupe huit bits en VR explicite', () => {
    const read = readDicom(fixture('huit-bits-explicite.dcm'));
    expect(read.kind).toBe('image');
    const slice = read as DicomSlice;
    expect([slice.rows, slice.columns]).toEqual([8, 8]);
    expect(slice.values.length).toBe(64);
    expect(slice.values[0]).toBe(0);
    expect(slice.values[63]).toBe(63);
    expect(slice.windowCenter).toBe(128);
    expect(slice.windowWidth).toBe(256);
    expect(slice.modality).toBe('OT');
    expect(slice.inverted).toBe(false);
  });

  it('lit du seize bits signe en VR implicite, pente et ordonnee appliquees', () => {
    const read = readDicom(fixture('seize-bits-signe-implicite.dcm'));
    expect(read.kind).toBe('image');
    const slice = read as DicomSlice;
    // Les pixels vont de -32 a 31 ; avec une pente de 2 et -1000 d'ordonnee,
    // la premiere valeur vaut -1064. Sans la pente on afficherait une image
    // juste, en unites fausses - ce qui ne se voit pas a l'ecran.
    expect(slice.values[0]).toBe(-1064);
    expect(slice.values[63]).toBe(-938);
    expect(slice.minimum).toBe(-1064);
    expect(slice.maximum).toBe(-938);
  });

  it('reconnait MONOCHROME1, ou le blanc est la valeur basse', () => {
    const read = readDicom(fixture('monochrome1.dcm'));
    expect((read as DicomSlice).inverted).toBe(true);
  });

  it('refuse une syntaxe compressee en la nommant', () => {
    const read = readDicom(fixture('jpeg2000.dcm'));
    expect(read.kind).toBe('refused');
    if (read.kind !== 'refused') return;
    expect(read.reason).toBe('compressed');
    expect(read.syntax).toContain('JPEG 2000');
  });

  it('refuse ce qui n\'est pas un DICOM, sans lancer', () => {
    const notDicom = new TextEncoder().encode('bonjour'.repeat(40)).buffer;
    expect(readDicom(notDicom as ArrayBuffer).kind).toBe('refused');
    expect(readDicom(new ArrayBuffer(4)).kind).toBe('refused');
  });
});

describe('la fenetre', () => {
  it('prend celle du fichier quand il en porte une', () => {
    const slice = readDicom(fixture('huit-bits-explicite.dcm')) as DicomSlice;
    expect(defaultWindow(slice)).toEqual({ centre: 128, width: 256 });
  });

  it('retombe sur l\'etendue des valeurs quand le fichier n\'en porte pas', () => {
    const slice = readDicom(fixture('seize-bits-signe-implicite.dcm')) as DicomSlice;
    const { centre, width } = defaultWindow(slice);
    expect(width).toBe(126);
    expect(centre).toBe(-1001);
  });
});

describe('paintSlice', () => {
  it('etale la fenetre sur les 256 niveaux de l\'ecran', () => {
    const slice = readDicom(fixture('huit-bits-explicite.dcm')) as DicomSlice;
    const target = { data: new Uint8ClampedArray(64 * 4), width: 8, height: 8 } as ImageData;
    // Fenetre exactement a cheval sur les valeurs presentes, 0 a 63, pour
    // que les deux extremites tombent rond : centre 31,5 et largeur 63.
    paintSlice(slice, 31.5, 63, target);
    expect(target.data[0]).toBe(0);
    expect(target.data[63 * 4]).toBe(255);
    expect(target.data[3]).toBe(255); // opacite

    // Une fenetre deux fois trop large ecrase le contraste sans rien couper :
    // le pixel le plus clair tombe aux trois quarts du blanc au lieu du blanc.
    // C'est le reglage qu'on corrige a la main devant une image terne.
    paintSlice(slice, 31.5, 126, target);
    expect(target.data[63 * 4]).toBe(191);
  });

  it('inverse pour MONOCHROME1', () => {
    const slice = readDicom(fixture('monochrome1.dcm')) as DicomSlice;
    const target = { data: new Uint8ClampedArray(64 * 4), width: 8, height: 8 } as ImageData;
    paintSlice(slice, 31.5, 63, target);
    expect(target.data[0]).toBe(255);
    expect(target.data[63 * 4]).toBe(0);
  });
});

describe('syntaxName', () => {
  it('nomme ce qu\'elle connait et rend l\'identifiant sinon', () => {
    expect(syntaxName('1.2.840.10008.1.2.4.91')).toBe('JPEG 2000');
    expect(syntaxName('1.2.3.4.inconnu')).toBe('1.2.3.4.inconnu');
  });
});
