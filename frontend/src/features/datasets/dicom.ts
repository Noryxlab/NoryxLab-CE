/* Lire une coupe DICOM non compressee, sans dependance.
 *
 * Le choix merite d'etre explique, parce que la bibliotheque de reference
 * existe. Le decodeur de Cornerstone lit toutes les syntaxes de transfert, y
 * compris JPEG 2000 et JPEG-LS, et il pese 871 Ko compresses parce qu'il
 * importe son noyau de rendu 3D, vtk.js compris : mesure le 06/11/2026 sur un
 * paquet d'essai. C'est defendable sur une plateforme d'imagerie, et c'est une
 * quinzaine de paquets de plus dans l'arbre et dans les scans.
 *
 * Ce fichier couvre ce qui se lit sans rien de tout ca : VR explicite ou
 * implicite petit-boutien, 8 ou 16 bits, signe ou non, monochrome ou RGB. Ce
 * qui est compresse n'est pas devine : la syntaxe est nommee, et l'ecran dit
 * quoi faire. Si les images d'Essilor s'averent compressees, on ajoute le gros
 * decodeur derriere la meme interface, sans toucher a l'ecran.
 *
 * Ce n'est pas un lecteur de diagnostic et ne cherche pas a l'etre. Relire une
 * serie se fait dans Slicer, qui est un bureau distant sans canal de fichier.
 */

/** Ce qu'une coupe rend quand elle est lisible. */
export interface DicomSlice {
  kind: 'image';
  rows: number;
  columns: number;
  /** Niveaux, apres application de la pente et de l'ordonnee a l'origine. */
  values: Float32Array;
  /** Vrai pour MONOCHROME1, ou le blanc est la valeur basse. */
  inverted: boolean;
  /** Trois composantes par pixel plutot qu'une. */
  colour: boolean;
  windowCenter: number | null;
  windowWidth: number | null;
  modality: string;
  minimum: number;
  maximum: number;
}

/** Ce qu'elle rend quand elle ne l'est pas, avec de quoi le dire. */
export interface DicomRefusal {
  kind: 'refused';
  reason: 'compressed' | 'unsupported' | 'malformed';
  /** Nom lisible de la syntaxe de transfert, quand elle a pu etre lue. */
  syntax?: string;
}

export type DicomRead = DicomSlice | DicomRefusal;

const EXPLICIT_LITTLE = '1.2.840.10008.1.2.1';
const IMPLICIT_LITTLE = '1.2.840.10008.1.2';

/** Les syntaxes nommees, pour que le refus dise laquelle plutot que « autre ». */
const SYNTAX_NAMES: Record<string, string> = {
  '1.2.840.10008.1.2': 'VR implicite, petit-boutien',
  '1.2.840.10008.1.2.1': 'VR explicite, petit-boutien',
  '1.2.840.10008.1.2.1.99': 'VR explicite compressé (deflate)',
  '1.2.840.10008.1.2.2': 'VR explicite, gros-boutien',
  '1.2.840.10008.1.2.4.50': 'JPEG de base',
  '1.2.840.10008.1.2.4.51': 'JPEG étendu',
  '1.2.840.10008.1.2.4.57': 'JPEG sans perte',
  '1.2.840.10008.1.2.4.70': 'JPEG sans perte, première sélection',
  '1.2.840.10008.1.2.4.80': 'JPEG-LS sans perte',
  '1.2.840.10008.1.2.4.81': 'JPEG-LS quasi sans perte',
  '1.2.840.10008.1.2.4.90': 'JPEG 2000 sans perte',
  '1.2.840.10008.1.2.4.91': 'JPEG 2000',
  '1.2.840.10008.1.2.5': 'RLE',
};

export function syntaxName(uid: string): string {
  return SYNTAX_NAMES[uid] ?? uid;
}

/** Les VR dont la longueur tient sur quatre octets apres deux reserves. */
const LONG_VR = new Set(['OB', 'OW', 'OF', 'OD', 'OL', 'SQ', 'UT', 'UN', 'UC', 'UR']);

interface Element {
  group: number;
  element: number;
  vr: string;
  offset: number;
  length: number;
}

function readString(view: DataView, offset: number, length: number): string {
  let out = '';
  for (let index = 0; index < length; index += 1) {
    const code = view.getUint8(offset + index);
    if (code === 0) break;
    out += String.fromCharCode(code);
  }
  return out.trim();
}

/** Parcourt un groupe d'elements et appelle `visit` pour chacun. */
function walk(
  view: DataView,
  start: number,
  end: number,
  explicit: boolean,
  visit: (element: Element) => void,
): void {
  let offset = start;
  while (offset + 8 <= end) {
    const group = view.getUint16(offset, true);
    const element = view.getUint16(offset + 2, true);
    let vr = '';
    let length: number;
    let headerLength = 8;

    if (explicit) {
      vr = String.fromCharCode(view.getUint8(offset + 4), view.getUint8(offset + 5));
      if (LONG_VR.has(vr)) {
        length = view.getUint32(offset + 8, true);
        headerLength = 12;
      } else {
        length = view.getUint16(offset + 6, true);
      }
    } else {
      length = view.getUint32(offset + 4, true);
    }

    const valueOffset = offset + headerLength;
    // Une longueur indefinie signale une sequence ou des donnees encapsulees :
    // les deux veulent dire compresse ou structure, et aucune des deux ne se
    // lit ici. On s'arrete plutot que de lire n'importe quoi.
    if (length === 0xffffffff) {
      visit({ group, element, vr, offset: valueOffset, length: -1 });
      return;
    }
    if (valueOffset + length > end) return;
    visit({ group, element, vr, offset: valueOffset, length });
    offset = valueOffset + length;
  }
}

function numbers(view: DataView, element: Element): number[] {
  return readString(view, element.offset, element.length)
    .split('\\')
    .map((part) => Number(part.trim()))
    .filter((value) => Number.isFinite(value));
}

/**
 * Lit une coupe. Ne lance jamais : un fichier abime est un refus motive, pas
 * une exception qui remonte dans un composant.
 */
export function readDicom(buffer: ArrayBuffer): DicomRead {
  try {
    const view = new DataView(buffer);
    if (buffer.byteLength < 140) return { kind: 'refused', reason: 'malformed' };
    if (readString(view, 128, 4) !== 'DICM') return { kind: 'refused', reason: 'malformed' };

    // Le groupe de meta-informations est toujours en VR explicite petit-boutien,
    // et sa longueur est le premier element qu'il contient.
    let syntax = EXPLICIT_LITTLE;
    let metaEnd = buffer.byteLength;
    walk(view, 132, Math.min(buffer.byteLength, 132 + 200), true, (element) => {
      if (element.group === 0x0002 && element.element === 0x0000 && element.length === 4) {
        metaEnd = element.offset + 4 + view.getUint32(element.offset, true);
      }
    });
    walk(view, 132, metaEnd, true, (element) => {
      if (element.group === 0x0002 && element.element === 0x0010) {
        syntax = readString(view, element.offset, element.length);
      }
    });

    if (syntax !== EXPLICIT_LITTLE && syntax !== IMPLICIT_LITTLE) {
      return { kind: 'refused', reason: 'compressed', syntax: syntaxName(syntax) };
    }

    let rows = 0;
    let columns = 0;
    let bitsAllocated = 0;
    let pixelRepresentation = 0;
    let samplesPerPixel = 1;
    let photometric = 'MONOCHROME2';
    let modality = '';
    let slope = 1;
    let intercept = 0;
    let windowCenter: number | null = null;
    let windowWidth: number | null = null;
    let pixelOffset = -1;
    let pixelLength = 0;

    walk(view, metaEnd, buffer.byteLength, syntax === EXPLICIT_LITTLE, (element) => {
      if (element.length < 0) return;
      const tag = (element.group << 16) | element.element;
      switch (tag) {
        case 0x00080060:
          modality = readString(view, element.offset, element.length);
          break;
        case 0x00280002:
          samplesPerPixel = view.getUint16(element.offset, true);
          break;
        case 0x00280004:
          photometric = readString(view, element.offset, element.length);
          break;
        case 0x00280010:
          rows = view.getUint16(element.offset, true);
          break;
        case 0x00280011:
          columns = view.getUint16(element.offset, true);
          break;
        case 0x00280100:
          bitsAllocated = view.getUint16(element.offset, true);
          break;
        case 0x00280103:
          pixelRepresentation = view.getUint16(element.offset, true);
          break;
        case 0x00281052:
          intercept = numbers(view, element)[0] ?? 0;
          break;
        case 0x00281053:
          slope = numbers(view, element)[0] ?? 1;
          break;
        case 0x00281050:
          windowCenter = numbers(view, element)[0] ?? null;
          break;
        case 0x00281051:
          windowWidth = numbers(view, element)[0] ?? null;
          break;
        case 0x7fe00010:
          pixelOffset = element.offset;
          pixelLength = element.length;
          break;
        default:
          break;
      }
    });

    if (rows <= 0 || columns <= 0 || pixelOffset < 0) {
      return { kind: 'refused', reason: 'malformed' };
    }
    if (bitsAllocated !== 8 && bitsAllocated !== 16) {
      return { kind: 'refused', reason: 'unsupported' };
    }
    if (samplesPerPixel !== 1 && samplesPerPixel !== 3) {
      return { kind: 'refused', reason: 'unsupported' };
    }

    const count = rows * columns * samplesPerPixel;
    const expected = count * (bitsAllocated / 8);
    if (pixelLength < expected) return { kind: 'refused', reason: 'malformed' };

    const values = new Float32Array(count);
    let minimum = Number.POSITIVE_INFINITY;
    let maximum = Number.NEGATIVE_INFINITY;
    for (let index = 0; index < count; index += 1) {
      let raw: number;
      if (bitsAllocated === 8) {
        raw = view.getUint8(pixelOffset + index);
      } else if (pixelRepresentation === 1) {
        raw = view.getInt16(pixelOffset + index * 2, true);
      } else {
        raw = view.getUint16(pixelOffset + index * 2, true);
      }
      const value = raw * slope + intercept;
      values[index] = value;
      if (value < minimum) minimum = value;
      if (value > maximum) maximum = value;
    }

    return {
      kind: 'image',
      rows,
      columns,
      values,
      inverted: photometric === 'MONOCHROME1',
      colour: samplesPerPixel === 3,
      windowCenter,
      windowWidth,
      modality,
      minimum,
      maximum,
    };
  } catch {
    return { kind: 'refused', reason: 'malformed' };
  }
}

/**
 * Peint la coupe dans un ImageData, en appliquant la fenetre.
 *
 * La fenetre est ce qui rend une image lisible : un scanner couvre des
 * milliers de niveaux et un ecran en montre 256, donc sans centre ni largeur
 * on affiche du gris uniforme et on croit l'image vide.
 */
export function paintSlice(
  slice: DicomSlice,
  centre: number,
  width: number,
  target: ImageData,
): void {
  const data = target.data;
  const half = Math.max(width, 1) / 2;
  const low = centre - half;
  const span = Math.max(width, 1);

  if (slice.colour) {
    for (let index = 0, pixel = 0; index < slice.values.length; index += 3, pixel += 4) {
      data[pixel] = slice.values[index] ?? 0;
      data[pixel + 1] = slice.values[index + 1] ?? 0;
      data[pixel + 2] = slice.values[index + 2] ?? 0;
      data[pixel + 3] = 255;
    }
    return;
  }

  for (let index = 0, pixel = 0; index < slice.values.length; index += 1, pixel += 4) {
    let level = (((slice.values[index] ?? 0) - low) / span) * 255;
    if (level < 0) level = 0;
    if (level > 255) level = 255;
    if (slice.inverted) level = 255 - level;
    data[pixel] = level;
    data[pixel + 1] = level;
    data[pixel + 2] = level;
    data[pixel + 3] = 255;
  }
}

/** La fenetre par defaut : celle du fichier, sinon l'etendue des valeurs. */
export function defaultWindow(slice: DicomSlice): { centre: number; width: number } {
  if (slice.windowCenter !== null && slice.windowWidth !== null && slice.windowWidth > 0) {
    return { centre: slice.windowCenter, width: slice.windowWidth };
  }
  const width = Math.max(slice.maximum - slice.minimum, 1);
  return { centre: slice.minimum + width / 2, width };
}
