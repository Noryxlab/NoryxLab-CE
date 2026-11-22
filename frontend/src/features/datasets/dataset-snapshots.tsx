import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Camera, FileDiff, Info, Moon } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { useToast } from '@/components/ui/toast';
import { datasetsApi } from '@/lib/api/endpoints';
import { qk, useDatasetSnapshots, useInvalidate, useSnapshotChanges } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatBytes, formatDateTime, formatNumber } from '@/lib/format';
import type { Dataset, DatasetSnapshot, SnapshotChange } from '@/lib/api/types';

/**
 * Les instantanes d'un dataset.
 *
 * Un instantane n'est pas une copie, et l'ecran le dit plutot que de le
 * laisser deviner : « instantane » evoque une sauvegarde, et quelqu'un qui
 * croirait en tenir une restaurerait un jour quelque chose qui n'existe pas.
 * Ce qui est garde est un manifeste - chemin, taille, date, ETag - et les
 * octets restent ou ils sont.
 *
 * Le deuxieme fait est aussi dit ici : prendre un instantane inscrit le
 * dataset au controle de nuit. C'est la seule facon de le demander, et sans
 * cette phrase le controle serait un service que personne ne sait activer.
 */

function ChangeKind({ kind }: { kind: SnapshotChange['kind'] }) {
  const t = useT();
  if (kind === 'added') return <Badge tone="success">{t('snapshots.added')}</Badge>;
  if (kind === 'removed') return <Badge tone="danger">{t('snapshots.removed')}</Badge>;
  return <Badge tone="warning">{t('snapshots.modified')}</Badge>;
}

function TakeSnapshotSheet({
  dataset,
  open,
  onOpenChange,
}: {
  dataset: Dataset;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const [label, setLabel] = React.useState('');

  React.useEffect(() => {
    if (open) setLabel('');
  }, [open]);

  const take = useMutation({
    mutationFn: () => datasetsApi.takeSnapshot(dataset.id, label.trim()),
    onSuccess: (snapshot) => {
      invalidate(qk.datasetSnapshots(dataset.id));
      onOpenChange(false);
      toast.success(t('snapshots.taken'), `${snapshot.objects} ${t('snapshots.objects').toLowerCase()}`);
    },
    onError: (error) => toast.error(error, t('snapshots.take')),
  });

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent aria-describedby={undefined}>
        <form
          onSubmit={(event) => {
            event.preventDefault();
            take.mutate();
          }}
          className="flex min-h-0 flex-1 flex-col"
        >
          <SheetHeader>
            <SheetTitle>{t('snapshots.take')}</SheetTitle>
            <SheetDescription>{t('snapshots.hint')}</SheetDescription>
          </SheetHeader>

          <SheetBody>
            <Field label={t('snapshots.labelLabel')} description={t('snapshots.labelHint')}>
              <Input
                value={label}
                onChange={(event) => setLabel(event.target.value)}
                placeholder={t('snapshots.labelPlaceholder')}
                maxLength={120}
              />
            </Field>

            <div className="space-y-2 rounded-md border border-border bg-surface-muted p-3">
              <p className="text-xs font-medium">{t('snapshots.whyNotACopy')}</p>
              <p className="text-xs leading-relaxed text-muted-foreground">{t('snapshots.whyNotACopyHint')}</p>
              <p className="flex items-center gap-1.5 pt-1 text-xs font-medium">
                <Moon className="size-3.5" aria-hidden />
                {t('snapshots.enrollsInScan')}
              </p>
              <p className="text-xs leading-relaxed text-muted-foreground">{t('snapshots.enrollsInScanHint')}</p>
            </div>
          </SheetBody>

          <SheetFooter>
            <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
              {t('common.cancel')}
            </Button>
            {/* Le libelle du bouton dit ce qui se passe pendant : le listage
              *  d'un gros bucket prend une minute, et un bouton qui tourne
              *  sans rien dire se lit comme une interface figee. */}
            <Button type="submit" variant="primary" loading={take.isPending}>
              {take.isPending ? t('snapshots.taking') : t('snapshots.take')}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  );
}

function SnapshotChanges({ datasetId, snapshot }: { datasetId: string; snapshot: DatasetSnapshot }) {
  const t = useT();
  const { locale } = useI18n();
  const changes = useSnapshotChanges(datasetId, snapshot.id);

  const columns: Column<SnapshotChange>[] = [
    {
      id: 'kind',
      header: t('common.status'),
      sortValue: (change) => change.kind,
      cell: (change) => <ChangeKind kind={change.kind} />,
    },
    {
      id: 'path',
      header: t('common.name'),
      sortValue: (change) => change.path,
      searchValue: (change) => change.path,
      cell: (change) => <code className="break-all font-mono text-xs">{change.path}</code>,
    },
    {
      id: 'size',
      header: t('snapshots.totalSize'),
      // La taille des deux cotes quand elle a bouge : « 4,2 Mio » seul ne dit
      // pas si le fichier a grossi ou s'il a ete remplace.
      cell: (change) =>
        change.was && change.now ? (
          <span className="text-xs tabular-nums text-muted-foreground">
            {formatBytes(change.was.size, locale)} → {formatBytes(change.now.size, locale)}
          </span>
        ) : (
          <span className="text-xs tabular-nums text-muted-foreground">
            {formatBytes((change.now ?? change.was)?.size ?? 0, locale)}
          </span>
        ),
    },
  ];

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('snapshots.changes')}</CardTitle>
          <CardDescription>
            {snapshot.label || formatDateTime(snapshot.takenAt, locale)} — {t('snapshots.changesHint')}
          </CardDescription>
        </CardHeaderText>
      </CardHeader>

      {changes.isLoading ? (
        <CardContent>
          <p className="text-xs text-muted-foreground">{t('snapshots.comparing')}</p>
        </CardContent>
      ) : changes.data?.identical ? (
        <EmptyState
          compact
          icon={FileDiff}
          title={t('snapshots.identical')}
          description={t('snapshots.identicalHint')}
        />
      ) : (
        <>
          <DataTable
            data={changes.data?.changes ?? []}
            columns={columns}
            rowKey={(change) => `${change.kind}:${change.path}`}
            isError={changes.isError}
            error={changes.error}
            onRetry={() => void changes.refetch()}
            defaultSort={{ columnId: 'path', direction: 'asc' }}
          />
          {/* La reserve sur l'ETag, affichee avec les ecarts et pas ailleurs :
            *  c'est en lisant « modifie » qu'on a besoin de savoir ce que ce
            *  mot garantit, et ce qu'il ne garantit pas. */}
          <CardContent className="border-t border-border pt-3">
            <p className="text-xs font-medium">{t('snapshots.etagCaveat')}</p>
            <p className="text-xs leading-relaxed text-muted-foreground">{t('snapshots.etagCaveatHint')}</p>
          </CardContent>
        </>
      )}
    </Card>
  );
}

export function DatasetSnapshots({ dataset }: { dataset: Dataset }) {
  const t = useT();
  const { locale } = useI18n();
  const snapshots = useDatasetSnapshots(dataset.id);
  const [taking, setTaking] = React.useState(false);
  const [comparing, setComparing] = React.useState<DatasetSnapshot | null>(null);

  // Un dataset sans bucket n'a rien a lister : l'API refuse, et le dire ici
  // vaut mieux qu'un bouton qui echoue.
  const listable = Boolean(dataset.bucket);

  const columns: Column<DatasetSnapshot>[] = [
    {
      id: 'label',
      header: t('snapshots.labelLabel'),
      sortValue: (item) => item.label ?? '',
      searchValue: (item) => item.label ?? '',
      cell: (item) =>
        item.label ? (
          <span className="font-medium">{item.label}</span>
        ) : (
          // Sans libelle, la date tient ce role : c'est ce qui distingue deux
          // instantanes dans une liste.
          <span className="text-muted-foreground">{formatDateTime(item.takenAt, locale)}</span>
        ),
    },
    {
      id: 'takenAt',
      header: t('snapshots.takenAt'),
      sortValue: (item) => item.takenAt,
      cell: (item) => (
        <span className="text-xs text-muted-foreground">{formatDateTime(item.takenAt, locale)}</span>
      ),
    },
    {
      id: 'takenBy',
      header: t('snapshots.takenBy'),
      sortValue: (item) => item.takenBy,
      cell: (item) => (
        <span className="text-xs text-muted-foreground">{item.takenBy || '—'}</span>
      ),
    },
    {
      id: 'objects',
      header: t('snapshots.objects'),
      sortValue: (item) => item.objects,
      cell: (item) => <span className="tabular-nums">{formatNumber(item.objects, locale)}</span>,
    },
    {
      id: 'totalSize',
      header: t('snapshots.totalSize'),
      sortValue: (item) => item.totalSize,
      cell: (item) => (
        <span className="tabular-nums text-muted-foreground">{formatBytes(item.totalSize, locale)}</span>
      ),
    },
    {
      id: 'compare',
      header: '',
      cell: (item) => (
        <Button variant="ghost" size="sm" onClick={() => setComparing(item)}>
          <FileDiff aria-hidden />
          {t('snapshots.compare')}
        </Button>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardHeaderText>
            <CardTitle>{t('snapshots.title')}</CardTitle>
            <CardDescription>{t('snapshots.hint')}</CardDescription>
          </CardHeaderText>
          {listable ? (
            <Button variant="secondary" onClick={() => setTaking(true)}>
              <Camera aria-hidden />
              {t('snapshots.take')}
            </Button>
          ) : null}
        </CardHeader>

        {listable ? (
          <>
            <DataTable
              data={snapshots.data}
              columns={columns}
              rowKey={(item) => item.id}
              isLoading={snapshots.isLoading}
              isError={snapshots.isError}
              error={snapshots.error}
              onRetry={() => void snapshots.refetch()}
              defaultSort={{ columnId: 'takenAt', direction: 'desc' }}
              emptyState={
                <EmptyState
                  compact
                  icon={Camera}
                  title={t('snapshots.empty')}
                  description={t('snapshots.emptyHint')}
                  action={
                    <Button variant="primary" onClick={() => setTaking(true)}>
                      <Camera aria-hidden />
                      {t('snapshots.take')}
                    </Button>
                  }
                />
              }
            />
            {/* Le controle de nuit, dit une fois que la liste existe : sur un
              *  dataset suivi, c'est la consequence la plus utile du geste. */}
            {(snapshots.data ?? []).length > 0 ? (
              <CardContent className="flex items-start gap-3 border-t border-border pt-3">
                <Moon className="mt-0.5 size-4 shrink-0 text-muted-foreground" aria-hidden />
                <div>
                  <p className="text-xs font-medium">{t('snapshots.scanAtNight')}</p>
                  <p className="text-xs leading-relaxed text-muted-foreground">
                    {t('snapshots.scanAtNightHint')}
                  </p>
                </div>
              </CardContent>
            ) : null}
          </>
        ) : (
          <EmptyState
            compact
            icon={Info}
            title={t('snapshots.needsLocalDataset')}
            description={t('snapshots.needsLocalDatasetHint')}
          />
        )}
      </Card>

      {comparing ? <SnapshotChanges datasetId={dataset.id} snapshot={comparing} /> : null}

      <TakeSnapshotSheet dataset={dataset} open={taking} onOpenChange={setTaking} />
    </div>
  );
}
