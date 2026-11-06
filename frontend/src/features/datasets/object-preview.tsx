import * as React from 'react';
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { ErrorState } from '@/components/common/states';
import { Skeleton } from '@/components/ui/skeleton';
import { encodeObjectPath, request } from '@/lib/api/client';
import { formatBytes, formatNumber } from '@/lib/format';
import { useI18n, useT } from '@/lib/i18n';
import type { Dataset } from '@/lib/api/types';

/* Lire un fichier sans le sortir.
 *
 * L'explorateur listait, deposait, renommait et supprimait : rien n'ouvrait
 * un fichier. Cedric Folignoni, le 03/11/2026 : « une visu des csv, etc.
 * depuis l'explorateur de fichiers ». Le detour etait de telecharger pour
 * regarder, c'est-a-dire de sortir la donnee du perimetre pour decider si
 * elle valait la peine - exactement le geste qu'on vient de fermer ailleurs.
 *
 * L'apercu passe par la meme route objet que le reste, donc par les memes
 * droits, le meme refus HDS sur les types non previsualisables, et la meme
 * ligne d'audit `dataset.object.download`. Rien n'est ajoute au modele : on
 * affiche ce que la plateforme accepte deja de servir. */

/** Au-dela, on ne previsualise pas : la route objet recopie le fichier entier
 *  et le navigateur le garderait en memoire. Un CSV de clinique fait quelques
 *  mega-octets ; une serie DICOM, elle, n'a rien a faire ici. */
const PREVIEW_LIMIT_BYTES = 2 * 1024 * 1024;

/** Lignes rendues. Au-dela l'ecran rame pour rien : on regarde un CSV pour
 *  savoir ce qu'il contient, pas pour le lire en entier. */
const PREVIEW_ROWS = 200;

type PreviewKind = 'table' | 'text' | 'image' | 'video';

const TABLE_SUFFIXES = ['.csv', '.tsv'];
const TEXT_SUFFIXES = ['.txt', '.json', '.md', '.log', '.yaml', '.yml', '.xml'];
const IMAGE_SUFFIXES = ['.png', '.jpg', '.jpeg', '.gif', '.webp', '.svg'];
/* Ce qu'un navigateur sait lire sans rien installer. Le lecteur de la page
   est celui du navigateur : il n'y a plus de VLC dans un onglet depuis la fin
   des greffons, et une bibliotheque JavaScript n'ajoute pas un codec, elle
   habille celui qui est deja la. Un MKV ou un AVI ne se liront pas ici ; ils
   demandent soit un transcodage, soit un bureau distant ou VLC existe. */
const VIDEO_SUFFIXES = ['.mp4', '.m4v', '.webm', '.ogv', '.mov'];

export function previewKindOf(path: string): PreviewKind | null {
  const lower = path.toLowerCase();
  if (TABLE_SUFFIXES.some((suffix) => lower.endsWith(suffix))) return 'table';
  if (TEXT_SUFFIXES.some((suffix) => lower.endsWith(suffix))) return 'text';
  if (IMAGE_SUFFIXES.some((suffix) => lower.endsWith(suffix))) return 'image';
  if (VIDEO_SUFFIXES.some((suffix) => lower.endsWith(suffix))) return 'video';
  return null;
}

/** Le separateur, devine sur la premiere ligne.
 *
 *  Un « CSV » francais est tres souvent separe par des points-virgules, parce
 *  qu'Excel en francais ecrit ca. Deviner sur une ligne coute une comparaison
 *  et evite une colonne unique illisible. */
function delimiterOf(firstLine: string): string {
  const counts: [string, number][] = [
    ['\t', (firstLine.match(/\t/g) ?? []).length],
    [';', (firstLine.match(/;/g) ?? []).length],
    [',', (firstLine.match(/,/g) ?? []).length],
  ];
  counts.sort((left, right) => right[1] - left[1]);
  const best = counts[0];
  return best && best[1] > 0 ? best[0] : ',';
}

/** Un analyseur qui respecte les guillemets.
 *
 *  Un decoupage naif sur la virgule casse des la premiere colonne contenant
 *  « Dupont, Jean » ou une mesure avec une virgule decimale entre guillemets,
 *  ce qui est le quotidien d'un export clinique. Trente lignes valent mieux
 *  qu'un tableau faux. */
export function parseDelimited(raw: string, limit = PREVIEW_ROWS): { rows: string[][]; total: number } {
  const text = raw.replace(/\r\n/g, '\n').replace(/\r/g, '\n');
  const firstBreak = text.indexOf('\n');
  const delimiter = delimiterOf(firstBreak === -1 ? text : text.slice(0, firstBreak));

  const rows: string[][] = [];
  let row: string[] = [];
  let field = '';
  let quoted = false;
  let total = 0;

  const endField = () => {
    row.push(field);
    field = '';
  };
  const endRow = () => {
    endField();
    total += 1;
    if (rows.length < limit) rows.push(row);
    row = [];
  };

  for (let index = 0; index < text.length; index += 1) {
    const char = text[index];
    if (quoted) {
      if (char === '"') {
        if (text[index + 1] === '"') {
          field += '"';
          index += 1;
        } else {
          quoted = false;
        }
      } else {
        field += char;
      }
      continue;
    }
    if (char === '"') {
      quoted = true;
    } else if (char === delimiter) {
      endField();
    } else if (char === '\n') {
      endRow();
    } else {
      field += char;
    }
  }
  if (field !== '' || row.length > 0) endRow();

  return { rows, total };
}

async function fetchObject(datasetId: string, path: string): Promise<Blob> {
  const response = await request<Response>(
    `/api/v1/datasets/${datasetId}/objects/${encodeObjectPath(path)}`,
    { raw: true },
  );
  return response.blob();
}

export function ObjectPreview({
  dataset,
  path,
  size,
  onClose,
}: {
  dataset: Dataset;
  path: string;
  size: number;
  onClose: () => void;
}) {
  const t = useT();
  const { locale } = useI18n();
  const kind = previewKindOf(path);
  /* La video ne passe pas par la memoire : elle est diffusee par la route
     objet, qui repond desormais aux requetes partielles. La limite de taille
     ne vaut donc que pour ce qu'on lit en entier. */
  const streams = kind === 'video';
  const tooBig = !streams && size > PREVIEW_LIMIT_BYTES;
  const regulated = dataset.classification === 'hds';

  const [text, setText] = React.useState<string | null>(null);
  const [imageUrl, setImageUrl] = React.useState<string | null>(null);
  const [error, setError] = React.useState<unknown>(null);
  const [loading, setLoading] = React.useState(false);

  React.useEffect(() => {
    if (!kind || tooBig || streams) return;
    let cancelled = false;
    let objectUrl: string | null = null;
    setLoading(true);
    setError(null);
    fetchObject(dataset.id, path)
      .then(async (blob) => {
        if (cancelled) return;
        if (kind === 'image') {
          /* Un blob plutot qu'une adresse dans l'attribut `src` : en mode
             developpement l'interface s'authentifie par un en-tete et pas par
             un cookie, donc une balise qui va chercher l'octet toute seule
             recevrait un 401. Un seul chemin, qui marche dans les deux. */
          objectUrl = URL.createObjectURL(blob);
          setImageUrl(objectUrl);
        } else {
          setText(await blob.text());
        }
      })
      .catch((reason: unknown) => {
        if (!cancelled) setError(reason);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
    };
  }, [dataset.id, path, kind, tooBig, streams]);

  const table = React.useMemo(
    () => (kind === 'table' && text !== null ? parseDelimited(text) : null),
    [kind, text],
  );

  return (
    <Dialog open onOpenChange={(open) => (open ? undefined : onClose())}>
      <DialogContent size="lg">
        <DialogHeader>
          <DialogTitle className="truncate">{path.split('/').pop()}</DialogTitle>
          <DialogDescription>
            /{path} · {formatBytes(size, locale)}
          </DialogDescription>
        </DialogHeader>
        <DialogBody className="overflow-auto">
          {!kind ? (
            <p className="text-sm text-muted-foreground">{t('datasets.previewUnsupported')}</p>
          ) : tooBig ? (
            <p className="text-sm text-muted-foreground">
              {t('datasets.previewTooBig', { limit: formatBytes(PREVIEW_LIMIT_BYTES, locale) })}
            </p>
          ) : loading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-1/3" />
              <Skeleton className="h-32 w-full" />
            </div>
          ) : error ? (
            <ErrorState error={error} />
          ) : kind === 'video' ? (
            regulated ? (
              <p className="text-sm text-muted-foreground">{t('datasets.previewVideoRegulated')}</p>
            ) : (
              /* L'adresse directe, pas un blob : le navigateur demande des
                 morceaux et on ne charge pas un film en memoire. Il porte le
                 cookie de session tout seul, l'interface et l'API partageant
                 la meme origine. */
              <video
                controls
                preload="metadata"
                className="mx-auto max-h-[60vh] w-full rounded-md bg-black"
                src={`/api/v1/datasets/${dataset.id}/objects/${encodeObjectPath(path)}`}
              >
                {t('datasets.previewUnsupported')}
              </video>
            )
          ) : kind === 'image' && imageUrl ? (
            <img src={imageUrl} alt={path} className="mx-auto max-h-[60vh] max-w-full object-contain" />
          ) : table ? (
            <div className="space-y-2">
              <div className="overflow-x-auto rounded-md border border-border">
                <table className="w-full text-xs tabular-nums">
                  <tbody>
                    {table.rows.map((row, rowIndex) => (
                      <tr
                        key={rowIndex}
                        className={rowIndex === 0 ? 'bg-surface-muted font-medium' : 'border-t border-border'}
                      >
                        {row.map((cell, cellIndex) => (
                          <td key={cellIndex} className="max-w-[20rem] truncate px-2 py-1" title={cell}>
                            {cell}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <p className="text-xs text-muted-foreground">
                {t('datasets.previewRows', {
                  shown: formatNumber(table.rows.length, locale),
                  total: formatNumber(table.total, locale),
                })}
              </p>
            </div>
          ) : text !== null ? (
            <pre className="max-h-[60vh] overflow-auto whitespace-pre-wrap break-words rounded-md border border-border bg-surface-muted p-3 text-xs">
              {text.slice(0, 200_000)}
            </pre>
          ) : null}
        </DialogBody>
      </DialogContent>
    </Dialog>
  );
}
