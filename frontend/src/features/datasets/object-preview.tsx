import * as React from 'react';
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { DialogFooter } from '@/components/ui/dialog';
import { ErrorState } from '@/components/common/states';
import { useToast } from '@/components/ui/toast';
import { datasetsApi } from '@/lib/api/endpoints';
import { getAuthHeaders } from '@/lib/api/client';
import { Skeleton } from '@/components/ui/skeleton';
import { encodeObjectPath, request } from '@/lib/api/client';
import { openBackendSession } from '@/lib/auth';
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

/** Un classeur et un PDF sont lus en entier eux aussi, mais ils sont souvent
 *  plus lourds qu'un CSV sans etre pour autant hors de propos : un compte
 *  rendu scanne pese dix mega-octets et reste un document qu'on regarde. */
const DOCUMENT_LIMIT_BYTES = 25 * 1024 * 1024;

type PreviewKind = 'table' | 'sheet' | 'sheet-foreign' | 'pdf' | 'text' | 'image' | 'video' | 'video-foreign';

const TABLE_SUFFIXES = ['.csv', '.tsv'];
/* Un classeur se lit dans le navigateur, pas sur le serveur : un fichier
   depose par un tiers y est analyse dans l'onglet de la personne et non dans
   le processus qui sert la plateforme. La bibliotheque est chargee a la
   demande, donc elle ne pese rien pour qui n'ouvre jamais de tableur. */
const SHEET_SUFFIXES = ['.xlsx', '.xlsm'];
/* Les classeurs qu'on reconnait sans savoir les lire : l'ancien format binaire
   d'Excel et celui d'OpenDocument. Les nommer permet de dire quoi faire. */
const FOREIGN_SHEET_SUFFIXES = ['.xls', '.ods'];
const PDF_SUFFIXES = ['.pdf'];
const TEXT_SUFFIXES = ['.txt', '.json', '.md', '.log', '.yaml', '.yml', '.xml'];
const IMAGE_SUFFIXES = ['.png', '.jpg', '.jpeg', '.gif', '.webp', '.svg'];
/* Ce qu'un navigateur sait lire sans rien installer. Le lecteur de la page
   est celui du navigateur : il n'y a plus de VLC dans un onglet depuis la fin
   des greffons, et une bibliotheque JavaScript n'ajoute pas un codec, elle
   habille celui qui est deja la. Un MKV ou un AVI ne se liront pas ici ; ils
   demandent soit un transcodage, soit un bureau distant ou VLC existe. */
const VIDEO_SUFFIXES = ['.mp4', '.m4v', '.webm', '.ogv', '.mov'];
/* Les conteneurs video qu'on reconnait sans savoir les lire. Ils existent
   pour qu'un clic reponde quelque chose : sans eux, cliquer un MKV ne faisait
   rien du tout, ce qui se lit comme une panne et pas comme une limite. */
const FOREIGN_VIDEO_SUFFIXES = ['.mkv', '.avi', '.wmv', '.flv', '.mpg', '.mpeg', '.m2ts', '.ts', '.3gp'];

export function previewKindOf(path: string): PreviewKind | null {
  const lower = path.toLowerCase();
  if (TABLE_SUFFIXES.some((suffix) => lower.endsWith(suffix))) return 'table';
  if (SHEET_SUFFIXES.some((suffix) => lower.endsWith(suffix))) return 'sheet';
  if (FOREIGN_SHEET_SUFFIXES.some((suffix) => lower.endsWith(suffix))) return 'sheet-foreign';
  if (PDF_SUFFIXES.some((suffix) => lower.endsWith(suffix))) return 'pdf';
  if (TEXT_SUFFIXES.some((suffix) => lower.endsWith(suffix))) return 'text';
  if (IMAGE_SUFFIXES.some((suffix) => lower.endsWith(suffix))) return 'image';
  if (VIDEO_SUFFIXES.some((suffix) => lower.endsWith(suffix))) return 'video';
  if (FOREIGN_VIDEO_SUFFIXES.some((suffix) => lower.endsWith(suffix))) return 'video-foreign';
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

/** La valeur d'une cellule, rendue comme la personne l'a saisie.
 *
 *  Un classeur ne contient pas du texte : une date y est un nombre de jours
 *  depuis 1900 et un pourcentage un reel. La bibliotheque rend des objets
 *  Date et des nombres ; les afficher bruts donnerait « 45678 » a la place du
 *  15 janvier 2026, ce qui est exactement le genre de detail qui fait douter
 *  d'un apercu. */
export function cellText(value: unknown): string {
  if (value === null || value === undefined) return '';
  if (value instanceof Date) {
    const iso = value.toISOString();
    // Une date sans heure est une date : afficher « T00:00:00Z » n'apprend
    // rien et casse l'alignement de la colonne.
    return iso.endsWith('T00:00:00.000Z') ? iso.slice(0, 10) : iso.replace('.000Z', 'Z');
  }
  if (typeof value === 'boolean') return value ? 'true' : 'false';
  return String(value);
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
  downloadWithdrawn = false,
  onSaved,
  onClose,
}: {
  dataset: Dataset;
  path: string;
  size: number;
  /** Quand l'installation retire les telechargements, la route objet ne sert
   *  plus que les types previsualisables - et la video n'en fait pas partie.
   *  Le dire avant d'essayer vaut mieux qu'un lecteur noir. */
  downloadWithdrawn?: boolean;
  /** Pour que le listage reprenne la taille et la date apres un
   *  enregistrement : la fenetre ne connait pas la requete qui l'a remplie. */
  onSaved?: () => void;
  onClose: () => void;
}) {
  const t = useT();
  const toast = useToast();
  const { locale } = useI18n();
  const kind = previewKindOf(path);
  /* La video ne passe pas par la memoire : elle est diffusee par la route
     objet, qui repond desormais aux requetes partielles. La limite de taille
     ne vaut donc que pour ce qu'on lit en entier. */
  const streams = kind === 'video';
  const [playbackFailed, setPlaybackFailed] = React.useState(false);
  /* Un seul nouvel essai, apres avoir rouvert la session.
     Le cookie vit huit heures et l'onglet bien plus, donc l'echec le plus
     probable n'est pas le codec : c'est un 401 sur une session finie pendant
     que les jetons du reste de l'interface, eux, sont toujours valides. */
  const [attempt, setAttempt] = React.useState(0);
  /* L'edition, pour ce qui se reecrit sans rien perdre.
     Un fichier texte et un CSV se reenregistrent a l'identique, octet pour
     octet, sauf ce que la personne a change. Un classeur, non : le reecrire
     demande de le reconstruire, et ce que la bibliotheque n'a pas compris -
     formules, styles, macros, graphiques - ne revient pas. L'edition s'arrete
     donc la ou la fidelite s'arrete. */
  const [editing, setEditing] = React.useState(false);
  const [draft, setDraft] = React.useState('');
  const [saving, setSaving] = React.useState(false);
  const reads = kind === 'sheet' || kind === 'pdf' ? DOCUMENT_LIMIT_BYTES : PREVIEW_LIMIT_BYTES;
  const tooBig = !streams && size > reads;
  const regulated = dataset.classification === 'hds';

  const [text, setText] = React.useState<string | null>(null);
  const [imageUrl, setImageUrl] = React.useState<string | null>(null);
  const [documentUrl, setDocumentUrl] = React.useState<string | null>(null);
  const [sheet, setSheet] = React.useState<{
    rows: string[][];
    total: number;
    name: string;
    sheets: string[];
  } | null>(null);
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
        } else if (kind === 'pdf') {
          objectUrl = URL.createObjectURL(blob);
          setDocumentUrl(objectUrl);
        } else if (kind === 'sheet') {
          /* Chargee ici et nulle part ailleurs : qui n'ouvre jamais de
             classeur ne telecharge jamais ce morceau. */
          /* L'entree « browser » explicitement : le paquet n'expose pas de
             racine, et c'est celle qui lit un Blob. */
          const { default: readXlsxFile } = await import('read-excel-file/browser');
          const sheets = await readXlsxFile(blob);
          if (cancelled) return;
          /* La premiere feuille, et on dit qu'il y en a d'autres : un classeur
             de laboratoire en porte souvent cinq, et n'en montrer qu'une sans
             le dire fait conclure que le reste a disparu. */
          const first = sheets[0];
          setSheet({
            rows: (first?.data ?? []).slice(0, PREVIEW_ROWS).map((row) => row.map(cellText)),
            total: first?.data.length ?? 0,
            name: first?.sheet ?? '',
            sheets: sheets.map((item) => item.sheet),
          });
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

  const table = React.useMemo(() => {
    if (kind === 'table' && text !== null) return parseDelimited(text);
    if (kind === 'sheet') return sheet;
    return null;
  }, [kind, text, sheet]);

  const editable = (kind === 'text' || kind === 'table') && text !== null && !tooBig;

  async function save() {
    setSaving(true);
    try {
      const headers = await getAuthHeaders();
      const name = path.split('/').pop() ?? path;
      /* Le type part avec le fichier : sans lui l'objet est reenregistre en
         binaire et cesse d'etre previsualisable, ce qui transformerait une
         correction d'une ligne en fichier qu'on ne peut plus ouvrir. */
      const type = name.toLowerCase().endsWith('.csv')
        ? 'text/csv; charset=utf-8'
        : name.toLowerCase().endsWith('.json')
          ? 'application/json; charset=utf-8'
          : 'text/plain; charset=utf-8';
      await datasetsApi.upload(dataset.id, path, new File([draft], name, { type }), { headers });
      setText(draft);
      setEditing(false);
      onSaved?.();
      toast.success(t('datasets.previewSaved'), name);
    } catch (reason: unknown) {
      toast.error(reason, t('datasets.previewSave'));
    } finally {
      setSaving(false);
    }
  }

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
              {t('datasets.previewTooBig', { limit: formatBytes(reads, locale) })}
            </p>
          ) : loading ? (
            <div className="space-y-2">
              <Skeleton className="h-4 w-1/3" />
              <Skeleton className="h-32 w-full" />
            </div>
          ) : error ? (
            <ErrorState error={error} />
          ) : kind === 'sheet-foreign' ? (
            <p className="text-sm text-muted-foreground">{t('datasets.previewSheetForeign')}</p>
          ) : kind === 'pdf' && documentUrl ? (
            /* Le lecteur du navigateur, depuis un blob deja authentifie :
               un iframe qui irait chercher l'adresse lui-meme ne saurait pas
               dire pourquoi il echoue, alors qu'ici le refus du serveur
               remonte comme une erreur lisible. */
            <iframe
              title={path}
              src={documentUrl}
              className="h-[65vh] w-full rounded-md border border-border"
            />
          ) : kind === 'video-foreign' ? (
            <p className="text-sm text-muted-foreground">{t('datasets.previewVideoForeign')}</p>
          ) : kind === 'video' ? (
            regulated ? (
              <p className="text-sm text-muted-foreground">{t('datasets.previewVideoRegulated')}</p>
            ) : downloadWithdrawn ? (
              <p className="text-sm text-muted-foreground">{t('datasets.previewVideoWithdrawn')}</p>
            ) : playbackFailed ? (
              <p className="text-sm text-muted-foreground">{t('datasets.previewVideoFailed')}</p>
            ) : (
              /* L'adresse directe, pas un blob : le navigateur demande des
                 morceaux et on ne charge pas un film en memoire. Il porte le
                 cookie de session tout seul, l'interface et l'API partageant
                 la meme origine. */
              <video
                controls
                preload="metadata"
                className="mx-auto max-h-[60vh] w-full rounded-md bg-black"
                /* Le parametre ne sert qu'a forcer le navigateur a refaire
                   la requete apres la reouverture de session : sans lui il
                   rejoue l'echec qu'il a en cache. Le serveur l'ignore. */
                src={`/api/v1/datasets/${dataset.id}/objects/${encodeObjectPath(path)}${attempt > 0 ? `?retry=${attempt}` : ''}`}
                /* Un element video qui echoue ne dit rien : il reste noir.
                   Un refus du serveur, un codec que ce navigateur n'a pas, un
                   fichier tronque, et la personne conclut que la plateforme
                   est cassee. */
                onError={() => {
                  if (attempt > 0) {
                    setPlaybackFailed(true);
                    return;
                  }
                  void openBackendSession().then(() => setAttempt(1));
                }}
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
                {sheet && sheet.sheets.length > 1
                  ? ` ${t('datasets.previewSheetOf', {
                      name: sheet.name,
                      others: sheet.sheets.filter((other) => other !== sheet.name).join(', '),
                    })}`
                  : null}
              </p>
            </div>
          ) : editing ? (
            <textarea
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              spellCheck={false}
              className="h-[60vh] w-full rounded-md border border-border bg-surface p-3 font-mono text-xs"
            />
          ) : text !== null ? (
            <pre className="max-h-[60vh] overflow-auto whitespace-pre-wrap break-words rounded-md border border-border bg-surface-muted p-3 text-xs">
              {text.slice(0, 200_000)}
            </pre>
          ) : null}
        </DialogBody>
        {editable ? (
          <DialogFooter>
            {editing ? (
              <>
                <Button variant="secondary" onClick={() => setEditing(false)} disabled={saving}>
                  {t('common.cancel')}
                </Button>
                <Button variant="primary" onClick={() => void save()} disabled={saving}>
                  {t('datasets.previewSave')}
                </Button>
              </>
            ) : (
              <Button
                variant="secondary"
                onClick={() => {
                  setDraft(text ?? '');
                  setEditing(true);
                }}
              >
                {t('datasets.previewEdit')}
              </Button>
            )}
          </DialogFooter>
        ) : null}
      </DialogContent>
    </Dialog>
  );
}
