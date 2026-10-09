import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import {
  ChevronRight,
  Download,
  File,
  Folder,
  FolderPlus,
  Home,
  PenLine,
  Trash2,
  Upload,
} from 'lucide-react';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { useConfirm } from '@/components/common/confirm-dialog';
import { Stat, StatGrid } from '@/components/common/stat';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardHeaderText, CardTitle, CardDescription } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Progress } from '@/components/ui/progress';
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useToast } from '@/components/ui/toast';
import { useDatasetObjects, useDatasetUsage, useVersion, qk, useInvalidate } from '@/lib/api/queries';
import { datasetsApi } from '@/lib/api/endpoints';
import { getAuthHeaders } from '@/lib/api/client';
import { useI18n, useT } from '@/lib/i18n';
import { formatBytes, formatDateTime, formatNumber } from '@/lib/format';
import { cn } from '@/lib/utils';
import type { Dataset, StorageObject } from '@/lib/api/types';

/** Derives the display name and folder-ness of an S3 key under a prefix. */
function describeObject(object: StorageObject, prefix: string): { name: string; isFolder: boolean } {
  const key = object.key ?? object.name ?? '';
  const relative = prefix && key.startsWith(prefix) ? key.slice(prefix.length) : key;
  const trimmed = relative.replace(/^\/+/, '');
  const isFolder = object.isPrefix === true || trimmed.endsWith('/');
  return { name: trimmed.replace(/\/$/, ''), isFolder };
}

type DroppedFile = { file: File; path: string };

/** Aplatit un depot en fichiers portant leur chemin relatif.
 *
 *  Les dossiers comptent autant que les fichiers : une serie DICOM est une
 *  arborescence, et la deposer en perdant sa structure ne sert a rien.
 *
 *  `webkitGetAsEntry` doit etre appele avant que le gestionnaire ne rende la
 *  main : la liste d'elements est videe des le retour de l'evenement. Les
 *  entrees sont donc collectees d'abord, parcourues ensuite. */
async function flattenDrop(items: DataTransferItemList): Promise<DroppedFile[]> {
  const roots: FileSystemEntry[] = [];
  for (const item of Array.from(items)) {
    const entry = item.webkitGetAsEntry?.();
    if (entry) roots.push(entry);
  }

  const out: DroppedFile[] = [];
  async function walk(entry: FileSystemEntry, prefix: string): Promise<void> {
    if (entry.isFile) {
      const file = await new Promise<File>((resolve, reject) =>
        (entry as FileSystemFileEntry).file(resolve, reject),
      );
      out.push({ file, path: `${prefix}${file.name}` });
      return;
    }
    const reader = (entry as FileSystemDirectoryEntry).createReader();
    // readEntries rend une page a la fois et signale la fin par une page vide.
    // Un seul appel tronquerait un dossier vers la centaine d'entrees - soit,
    // sur une serie DICOM, l'essentiel du contenu, et sans rien dire.
    for (;;) {
      const page = await new Promise<FileSystemEntry[]>((resolve, reject) =>
        reader.readEntries(resolve, reject),
      );
      if (page.length === 0) break;
      for (const child of page) await walk(child, `${prefix}${entry.name}/`);
    }
  }
  for (const root of roots) await walk(root, '');
  return out;
}

function UploadDialog({
  dataset,
  prefix,
  open,
  onOpenChange,
}: {
  dataset: Dataset;
  prefix: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const [file, setFile] = React.useState<File | null>(null);
  const [percent, setPercent] = React.useState<number | null>(null);

  React.useEffect(() => {
    if (!open) {
      setFile(null);
      setPercent(null);
    }
  }, [open]);

  async function upload() {
    if (!file) return;
    setPercent(0);
    try {
      // Uploads go through XHR rather than fetch so the user gets real
      // progress on a multi-hundred-megabyte file instead of a frozen button.
      const headers = await getAuthHeaders();
      await datasetsApi.upload(dataset.id, `${prefix}${file.name}`, file, {
        headers,
        onProgress: setPercent,
      });
      invalidate(qk.datasetObjects(dataset.id, prefix));
      toast.success(t('datasets.uploadDone'), file.name);
      onOpenChange(false);
    } catch (error) {
      toast.error(error, t('datasets.uploadFailed'));
    } finally {
      setPercent(null);
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="sm">
        <DialogHeader>
          <DialogTitle>{t('datasets.uploadTitle')}</DialogTitle>
          <DialogDescription>
            {t('datasets.uploadPathLabel')} : /{prefix || ''}
          </DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <Field label={t('datasets.uploadFileLabel')} required>
            <Input
              type="file"
              onChange={(event) => setFile(event.target.files?.[0] ?? null)}
              className="file:mr-3 file:rounded file:border-0 file:bg-surface-muted file:px-2 file:py-1 file:text-xs"
            />
          </Field>
          {file ? (
            <p className="text-xs text-muted-foreground">
              {file.name} · {formatBytes(file.size)}
            </p>
          ) : null}
          {percent !== null ? <Progress value={percent} label={t('datasets.upload')} /> : null}
        </DialogBody>
        <DialogFooter>
          <Button variant="secondary" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
          <Button
            variant="primary"
            disabled={!file}
            loading={percent !== null}
            onClick={() => void upload()}
          >
            <Upload aria-hidden />
            {t('datasets.upload')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function DatasetExplorer({ dataset }: { dataset: Dataset }) {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();

  const [prefix, setPrefix] = React.useState('');
  const [selected, setSelected] = React.useState<Set<string>>(new Set());
  const [uploading, setUploading] = React.useState(false);
  const [creatingFolder, setCreatingFolder] = React.useState(false);
  const [folderName, setFolderName] = React.useState('');
  const [renaming, setRenaming] = React.useState<{ from: string; to: string } | null>(null);
  /* Un compteur et non un booleen : chaque enfant survole emet son propre
     dragleave, et un booleen fait clignoter la zone pendant qu'on traverse
     le tableau. */
  const [dragDepth, setDragDepth] = React.useState(0);
  const [dropping, setDropping] = React.useState<{ done: number; total: number; name: string } | null>(
    null,
  );

  const objects = useDatasetObjects(dataset.id, prefix);

  // Changing dataset or folder must clear a selection that no longer applies.
  React.useEffect(() => {
    setSelected(new Set());
  }, [dataset.id, prefix]);

  /* Deposer dans le dossier ouvert, en gardant l'arborescence deposee.
   *
   *  Sequentiel et non en parallele : un depot de serie DICOM fait des
   *  milliers de fichiers, et autant de requetes simultanees noient le
   *  navigateur comme la passerelle. Un fichier a la fois donne aussi un
   *  compteur qui veut dire quelque chose.
   *
   *  Rien n'est verifie ici sur le droit d'ecrire : c'est le serveur qui
   *  tranche, comme pour le bouton de televersement, et son refus remonte
   *  tel quel. Un controle cote navigateur en plus serait un deuxieme avis
   *  a maintenir, qui finirait par contredire le premier. */
  async function uploadDropped(files: DroppedFile[]) {
    const headers = await getAuthHeaders();
    const failed: string[] = [];
    for (const [index, item] of files.entries()) {
      setDropping({ done: index, total: files.length, name: item.path });
      try {
        await datasetsApi.upload(dataset.id, `${prefix}${item.path}`, item.file, { headers });
      } catch {
        failed.push(item.path);
      }
    }
    setDropping(null);
    invalidate(qk.datasetObjects(dataset.id, prefix));
    if (failed.length === 0) {
      toast.success(
        t('datasets.dropDone', { count: String(files.length) }),
        t('datasets.uploadTitle'),
      );
      return;
    }
    /* Les echecs sont nommes, et le nombre de reussites avec : « 3 fichiers
       n'ont pas pu etre deposes » sans dire lesquels oblige a tout comparer
       a la main. */
    toast.error(
      new Error(
        t('datasets.dropPartly', {
          ok: String(files.length - failed.length),
          failed: String(failed.length),
        }) + ' ' + failed.slice(0, 5).join(', '),
      ),
      t('datasets.uploadFailed'),
    );
  }

  async function onDrop(event: React.DragEvent) {
    event.preventDefault();
    setDragDepth(0);
    if (dropping) return;
    const files = await flattenDrop(event.dataTransfer.items);
    if (files.length === 0) return;
    await uploadDropped(files);
  }

  const { data: version } = useVersion();
  const isHds = dataset.classification === 'hds';
  /* Deux raisons distinctes de ne pas proposer le telechargement, et il faut
     les garder separees : la classification du dataset, et un reglage de la
     plateforme entiere. Les confondre dirait « HDS » a un utilisateur d'un
     dataset ordinaire sur une installation verrouillee. */
  const downloadWithdrawn = version?.datasetDownload === 'blocked';
  const noDownload = isHds || downloadWithdrawn;

  const entries = React.useMemo(
    () =>
      (objects.data ?? [])
        .map((object) => ({ object, ...describeObject(object, prefix) }))
        .filter((entry) => entry.name.length > 0),
    [objects.data, prefix],
  );

  // The tiles say "total", so they have to mean the dataset - and they come
  // from the same measurement the catalogue shows, or the two screens disagree
  // about the same bucket. They used to be summed from the listing, which was
  // the whole bucket until the explorer started fetching one directory at a
  // time; then they silently became "the files sitting in this folder" under a
  // heading that still said total: 2 files and 67.6 KB for a 381 GB dataset.
  const usage = useDatasetUsage(dataset.id);
  const folderFiles = entries.filter((entry) => !entry.isFolder).length;
  const folderSize = entries.reduce((sum, entry) => sum + (entry.object.size ?? 0), 0);

  // Ce qui est coche, avec sa nature : la cle seule ne dit pas si c'est un
  // dossier, et c'est exactement ce que la suppression doit savoir.
  const selectedTargets = React.useMemo(
    () =>
      entries
        .filter((entry) => selected.has(entry.object.key))
        .map((entry) => ({ key: entry.object.key, isFolder: entry.isFolder })),
    [entries, selected],
  );
  const selectedFolders = selectedTargets.filter((target) => target.isFolder);

  const segments = prefix.split('/').filter(Boolean);

  const createFolder = useMutation({
    mutationFn: () => datasetsApi.createFolder(dataset.id, `${prefix}${folderName.trim()}/`),
    onSuccess: () => {
      invalidate(qk.datasetObjects(dataset.id, prefix));
      setCreatingFolder(false);
      setFolderName('');
    },
    onError: (error) => toast.error(error, t('datasets.newFolder')),
  });

  /** Un dossier se supprime avec ce qu'il contient, ou ne se supprime pas.
   *
   *  La suppression partait sans `recursive`, donc sur un dossier elle
   *  effacait une cle qui n'existe pas, S3 repondait succes, et l'ecran
   *  annoncait une suppression qui n'avait rien supprime. Sur un bucket HDS
   *  c'est le mauvais sens de l'erreur : on croit la donnee partie. */
  const removeObjects = useMutation({
    mutationFn: async (targets: { key: string; isFolder: boolean }[]) => {
      for (const target of targets) {
        await datasetsApi.deleteObject(dataset.id, target.key, target.isFolder);
      }
    },
    onSuccess: () => {
      invalidate(qk.datasetObjects(dataset.id, prefix));
      setSelected(new Set());
    },
    onError: (error) => toast.error(error, t('datasets.deleteSelection')),
  });

  /** Renommer, qui n'ajoute aucun droit.
   *
   *  Un ecrivain peut deja le faire a la main - telecharger, renvoyer sous un
   *  autre nom, supprimer l'ancien - avec les memes autorisations. Ce bouton
   *  fait la meme chose en une action verifiee, sans la fenetre ou le fichier
   *  existe en double ou plus du tout. */
  const renameObject = useMutation({
    mutationFn: ({ from, to }: { from: string; to: string }) =>
      datasetsApi.renameObject(dataset.id, from, to),
    onSuccess: () => {
      invalidate(qk.datasetObjects(dataset.id, prefix));
      setSelected(new Set());
      setRenaming(null);
      toast.success(t('datasets.objectRenamed'), t('datasets.rename'));
    },
    onError: (error) => toast.error(error, t('datasets.rename')),
  });

  const columns: Column<(typeof entries)[number]>[] = [
    {
      id: 'select',
      header: '',
      headClassName: 'w-8',
      className: 'w-8',
      cell: (entry) => (
        <input
          type="checkbox"
          aria-label={entry.name}
          className="size-3.5 accent-[var(--noryx-brand)]"
          checked={selected.has(entry.object.key)}
          onClick={(event) => event.stopPropagation()}
          onChange={(event) => {
            setSelected((current) => {
              const next = new Set(current);
              if (event.target.checked) next.add(entry.object.key);
              else next.delete(entry.object.key);
              return next;
            });
          }}
        />
      ),
    },
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (entry) => `${entry.isFolder ? '0' : '1'}${entry.name}`,
      searchValue: (entry) => entry.name,
      cell: (entry) => (
        <span className="flex items-center gap-2">
          {entry.isFolder ? (
            <Folder className="size-4 shrink-0 text-brand" aria-hidden />
          ) : (
            <File className="size-4 shrink-0 text-muted-foreground" aria-hidden />
          )}
          <span className={cn('truncate', entry.isFolder && 'font-medium')}>{entry.name}</span>
        </span>
      ),
    },
    {
      id: 'size',
      header: t('common.size'),
      align: 'right',
      sortValue: (entry) => entry.object.size ?? 0,
      cell: (entry) => (
        <span className="tabular-nums text-muted-foreground">
          {entry.isFolder ? '—' : formatBytes(entry.object.size, locale)}
        </span>
      ),
    },
    {
      id: 'modified',
      header: t('common.updatedAt'),
      sortValue: (entry) => entry.object.lastModified ?? null,
      cell: (entry) => (
        <span className="text-xs text-muted-foreground">
          {entry.object.lastModified ? formatDateTime(entry.object.lastModified, locale) : '—'}
        </span>
      ),
    },
  ];

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('datasets.explorer')}</CardTitle>
          <CardDescription>
            <span className="font-mono">
              {dataset.bucket}
              {dataset.prefix ? `/${dataset.prefix}` : ''}
            </span>
          </CardDescription>
        </CardHeaderText>
        <div className="flex flex-wrap items-center gap-2">
          {isHds ? <Badge tone="warning">{t('datasets.classificationHds')}</Badge> : null}
          <Button variant="secondary" size="sm" onClick={() => setCreatingFolder(true)}>
            <FolderPlus aria-hidden />
            {t('datasets.newFolder')}
          </Button>
          <Button variant="primary" size="sm" onClick={() => setUploading(true)}>
            <Upload aria-hidden />
            {t('datasets.upload')}
          </Button>
        </div>
      </CardHeader>

      <CardContent
        className="relative space-y-4"
        onDragEnter={(event) => {
          /* Seulement pour un depot de fichiers : faire reagir la zone a une
             selection de texte traversee a la souris est du bruit. */
          if (!event.dataTransfer.types.includes('Files')) return;
          event.preventDefault();
          setDragDepth((depth) => depth + 1);
        }}
        onDragOver={(event) => {
          if (!event.dataTransfer.types.includes('Files')) return;
          // Sans preventDefault sur dragover, le navigateur refuse le depot et
          // ouvre le fichier dans un onglet - ce qui, sur un DICOM, le
          // telecharge.
          event.preventDefault();
          event.dataTransfer.dropEffect = 'copy';
        }}
        onDragLeave={() => setDragDepth((depth) => Math.max(0, depth - 1))}
        onDrop={(event) => void onDrop(event)}
      >
        {dragDepth > 0 && !dropping ? (
          <div className="pointer-events-none absolute inset-0 z-10 flex items-center justify-center rounded-lg border-2 border-dashed border-brand bg-brand-subtle/80">
            <p className="text-sm font-medium text-brand-subtle-foreground">
              {t('datasets.dropHere', { path: `/${prefix}` })}
            </p>
          </div>
        ) : null}
        {dropping ? (
          <Progress
            value={Math.round((dropping.done / Math.max(1, dropping.total)) * 100)}
            label={t('datasets.dropProgress', {
              done: String(dropping.done + 1),
              total: String(dropping.total),
              name: dropping.name,
            })}
          />
        ) : null}
        <StatGrid className="sm:grid-cols-2 lg:grid-cols-2">
          <Stat
            label={t('datasets.filesCount')}
            value={usage.data ? formatNumber(usage.data.objects, locale) : '—'}
            hint={t('datasets.inThisFolder', {
              files: formatNumber(folderFiles, locale),
              size: formatBytes(folderSize, locale),
            })}
            loading={usage.isLoading}
          />
          <Stat
            label={t('datasets.totalSize')}
            value={usage.data ? formatBytes(usage.data.totalBytes, locale) : '—'}
            loading={usage.isLoading}
          />
        </StatGrid>

        <nav aria-label={t('datasets.explorer')} className="flex flex-wrap items-center gap-1 text-xs">
          <button
            type="button"
            onClick={() => setPrefix('')}
            className="flex items-center gap-1 rounded px-1.5 py-1 text-muted-foreground transition-colors hover:bg-surface-muted hover:text-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
          >
            <Home className="size-3.5" aria-hidden />
            {dataset.name}
          </button>
          {segments.map((segment, index) => (
            <React.Fragment key={`${segment}-${index}`}>
              <ChevronRight className="size-3 text-muted-foreground" aria-hidden />
              <button
                type="button"
                onClick={() => setPrefix(`${segments.slice(0, index + 1).join('/')}/`)}
                className={cn(
                  'rounded px-1.5 py-1 transition-colors hover:bg-surface-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring',
                  index === segments.length - 1 ? 'font-medium text-foreground' : 'text-muted-foreground',
                )}
              >
                {segment}
              </button>
            </React.Fragment>
          ))}
        </nav>

        {selected.size > 0 ? (
          <div className="flex flex-wrap items-center gap-2 rounded-md border border-brand/40 bg-brand-subtle px-3 py-2">
            <span className="text-xs font-medium text-brand-subtle-foreground">
              {selected.size === 1
                ? t('datasets.selectedCount', { count: selected.size })
                : t('datasets.selectedCountPlural', { count: selected.size })}
            </span>
            <div className="ml-auto flex gap-2">
              {/* HDS datasets disable direct download and archive export, per
                  the classification rules in ADR-013; an installation may
                  withdraw it for every classification as well. */}
              <Button
                variant="secondary"
                size="sm"
                disabled={noDownload}
                title={
                  isHds
                    ? t('datasets.hdsWarning')
                    : downloadWithdrawn
                      ? t('datasets.downloadDisabled')
                      : undefined
                }
                onClick={() =>
                  void datasetsApi
                    .downloadArchive(dataset.id, [...selected], `${dataset.name}.zip`)
                    .catch((error: unknown) => toast.error(error, t('datasets.downloadSelection')))
                }
              >
                <Download aria-hidden />
                {t('datasets.downloadSelection')}
              </Button>
              {/* Un seul objet a la fois : renommer une selection n'a pas de
                  sens, il n'y a pas de nouveau nom commun a deux fichiers. */}
              {selected.size === 1 ? (
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => {
                    const from = [...selected][0];
                    if (!from) return;
                    setRenaming({ from, to: from.split('/').pop() ?? from });
                  }}
                >
                  <PenLine aria-hidden />
                  {t('datasets.rename')}
                </Button>
              ) : null}
              <Button
                variant="danger-outline"
                size="sm"
                onClick={() =>
                  ask({
                    title: t('datasets.deleteObjectsTitle'),
                    // Un dossier emporte tout ce qu'il contient, et le
                    // listage n'est pas recursif : on ne sait donc pas
                    // combien. On le dit plutot que d'avancer un chiffre
                    // qu'on n'a pas mesure.
                    description: selectedFolders.length
                      ? t('datasets.deleteFoldersWarning', {
                          folders: selectedFolders.length,
                          files: selectedTargets.length - selectedFolders.length,
                        })
                      : t('datasets.deleteObjectsWarning'),
                    confirmLabel: t('common.delete'),
                    destructive: true,
                    onConfirm: () => removeObjects.mutateAsync(selectedTargets),
                  })
                }
              >
                <Trash2 aria-hidden />
                {t('datasets.deleteSelection')}
              </Button>
            </div>
          </div>
        ) : null}

        <DataTable
          data={entries}
          columns={columns}
          rowKey={(entry) => entry.object.key}
          isLoading={objects.isLoading}
          isError={objects.isError}
          error={objects.error}
          onRetry={() => void objects.refetch()}
          defaultSort={{ columnId: 'name', direction: 'asc' }}
          onRowClick={(entry) => {
            if (entry.isFolder) setPrefix(`${prefix}${entry.name}/`);
          }}
          emptyState={
            <EmptyState
              compact
              icon={Folder}
              title={t('datasets.emptyFolder')}
              description={t('datasets.emptyFolderHint')}
              action={
                <Button variant="primary" size="sm" onClick={() => setUploading(true)}>
                  <Upload aria-hidden />
                  {t('datasets.upload')}
                </Button>
              }
            />
          }
        />
      </CardContent>

      <UploadDialog dataset={dataset} prefix={prefix} open={uploading} onOpenChange={setUploading} />

      {/* Le renommage.
          Seul le dernier segment est modifiable : deplacer un fichier ailleurs
          et le renommer sont deux gestes differents, et un champ qui accepte
          des barres obliques fait l'un en croyant faire l'autre. */}
      <Dialog open={renaming !== null} onOpenChange={(open) => !open && setRenaming(null)}>
        <DialogContent size="sm">
          <DialogHeader>
            <DialogTitle>{t('datasets.rename')}</DialogTitle>
          </DialogHeader>
          <DialogBody>
            <Field label={t('datasets.renameNewName')} required>
              <Input
                value={renaming?.to ?? ''}
                onChange={(event) =>
                  setRenaming((current) =>
                    current ? { ...current, to: event.target.value.replace(/\//g, '') } : current)
                }
                autoFocus
                maxLength={255}
              />
            </Field>
            <p className="mt-2 text-xs text-muted-foreground">{t('datasets.renameHint')}</p>
          </DialogBody>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setRenaming(null)}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="primary"
              disabled={
                !renaming?.to.trim() ||
                renaming.to.trim() === (renaming.from.split('/').pop() ?? '')
              }
              loading={renameObject.isPending}
              onClick={() => {
                if (!renaming) return;
                // Le nouveau nom reste dans le dossier courant : on recompose
                // le chemin a partir de celui d'origine plutot que du prefixe
                // affiche, qui peut avoir change sous les pieds de l'ecran.
                const segments = renaming.from.split('/');
                segments[segments.length - 1] = renaming.to.trim();
                renameObject.mutate({ from: renaming.from, to: segments.join('/') });
              }}
            >
              {t('common.save')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={creatingFolder} onOpenChange={setCreatingFolder}>
        <DialogContent size="sm">
          <DialogHeader>
            <DialogTitle>{t('datasets.newFolder')}</DialogTitle>
          </DialogHeader>
          <DialogBody>
            <Field label={t('datasets.folderNameLabel')} required>
              <Input
                value={folderName}
                onChange={(event) => setFolderName(event.target.value)}
                autoFocus
                maxLength={120}
              />
            </Field>
          </DialogBody>
          <DialogFooter>
            <Button variant="secondary" onClick={() => setCreatingFolder(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              variant="primary"
              disabled={!folderName.trim()}
              loading={createFolder.isPending}
              onClick={() => createFolder.mutate()}
            >
              {t('common.create')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {dialog}
    </Card>
  );
}
