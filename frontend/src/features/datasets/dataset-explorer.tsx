import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import {
  ChevronRight,
  Download,
  File,
  Folder,
  FolderPlus,
  Home,
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
import { useDatasetObjects, qk, useInvalidate } from '@/lib/api/queries';
import { datasetsApi } from '@/lib/api/endpoints';
import { getAuthHeaders } from '@/lib/api/client';
import { useI18n, useT } from '@/lib/i18n';
import { formatBytes, formatDateTime } from '@/lib/format';
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

  const objects = useDatasetObjects(dataset.id, prefix);

  // Changing dataset or folder must clear a selection that no longer applies.
  React.useEffect(() => {
    setSelected(new Set());
  }, [dataset.id, prefix]);

  const isHds = dataset.classification === 'hds';

  const entries = React.useMemo(
    () =>
      (objects.data ?? [])
        .map((object) => ({ object, ...describeObject(object, prefix) }))
        .filter((entry) => entry.name.length > 0),
    [objects.data, prefix],
  );

  const totalSize = entries.reduce((sum, entry) => sum + (entry.object.size ?? 0), 0);
  const fileCount = entries.filter((entry) => !entry.isFolder).length;

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

  const removeObjects = useMutation({
    mutationFn: async (keys: string[]) => {
      for (const key of keys) await datasetsApi.deleteObject(dataset.id, key);
    },
    onSuccess: () => {
      invalidate(qk.datasetObjects(dataset.id, prefix));
      setSelected(new Set());
    },
    onError: (error) => toast.error(error, t('datasets.deleteSelection')),
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

      <CardContent className="space-y-4">
        <StatGrid className="sm:grid-cols-2 lg:grid-cols-2">
          <Stat
            label={t('datasets.filesCount')}
            value={fileCount}
            loading={objects.isLoading}
          />
          <Stat
            label={t('datasets.totalSize')}
            value={formatBytes(totalSize, locale)}
            loading={objects.isLoading}
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
                  the classification rules in ADR-013. */}
              <Button
                variant="secondary"
                size="sm"
                disabled={isHds}
                title={isHds ? t('datasets.hdsWarning') : undefined}
                onClick={() =>
                  void datasetsApi
                    .downloadArchive(dataset.id, [...selected], `${dataset.name}.zip`)
                    .catch((error: unknown) => toast.error(error, t('datasets.downloadSelection')))
                }
              >
                <Download aria-hidden />
                {t('datasets.downloadSelection')}
              </Button>
              <Button
                variant="danger-outline"
                size="sm"
                onClick={() =>
                  ask({
                    title: t('datasets.deleteObjectsTitle'),
                    description: t('datasets.deleteObjectsWarning'),
                    confirmLabel: t('common.delete'),
                    destructive: true,
                    onConfirm: () => removeObjects.mutateAsync([...selected]),
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
