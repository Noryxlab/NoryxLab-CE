import type { Environment } from '@/lib/api/types';
import type { TranslationKey } from '@/lib/i18n';

/**
 * Ranger les environnements par ce qu'on y fait.
 *
 * « Systeme » et « Personnalise » disaient qui a publie une image, pas ce
 * qu'elle est : Slicer, ParaView et le banc mecanique se trouvaient dans la
 * meme liste plate qu'un runtime Python nu, et choisir demandait de connaitre
 * les noms d'avance. La forme vient du serveur, qui est le seul a la savoir.
 *
 * L'ordre des groupes n'est pas alphabetique : il va du plus courant au plus
 * particulier, parce que la liste sert surtout a quelqu'un qui cherche un
 * endroit pour ecrire du code.
 */
export const SHAPE_ORDER = ['notebook', 'editor', 'r', 'desktop', 'runtime'] as const;

export type Shape = (typeof SHAPE_ORDER)[number];

const LABELS: Record<Shape, TranslationKey> = {
  notebook: 'environments.shapeNotebook',
  editor: 'environments.shapeEditor',
  r: 'environments.shapeR',
  desktop: 'environments.shapeDesktop',
  runtime: 'environments.shapeRuntime',
};

const HINTS: Record<Shape, TranslationKey> = {
  notebook: 'environments.shapeNotebookHint',
  editor: 'environments.shapeEditorHint',
  r: 'environments.shapeRHint',
  desktop: 'environments.shapeDesktopHint',
  runtime: 'environments.shapeRuntimeHint',
};

/** Une forme que cette interface ne connait pas est rangee avec les images
 *  sans rien a ouvrir plutot que cachee : un environnement absent de l'ecran
 *  n'existe pour personne, et c'est le defaut qu'on repare. */
export function shapeOf(environment: Environment): Shape {
  const declared = (environment.shape ?? '').trim().toLowerCase();
  return (SHAPE_ORDER as readonly string[]).includes(declared) ? (declared as Shape) : 'runtime';
}

export function shapeLabelKey(shape: Shape): TranslationKey {
  return LABELS[shape];
}

export function shapeHintKey(shape: Shape): TranslationKey {
  return HINTS[shape];
}

export interface ShapeGroup {
  shape: Shape;
  items: Environment[];
}

/** Les groupes, dans l'ordre declare, et seulement ceux qui ont du contenu :
 *  un intitule suivi de rien fait croire a un filtre actif. */
export function groupByShape(environments: Environment[]): ShapeGroup[] {
  const buckets = new Map<Shape, Environment[]>();
  for (const environment of environments) {
    const shape = shapeOf(environment);
    const held = buckets.get(shape);
    if (held) held.push(environment);
    else buckets.set(shape, [environment]);
  }
  return SHAPE_ORDER.filter((shape) => buckets.has(shape)).map((shape) => ({
    shape,
    items: buckets.get(shape) ?? [],
  }));
}
