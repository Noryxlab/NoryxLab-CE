import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Scissors, Trash2, UserRoundCog } from 'lucide-react';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { useConfirm } from '@/components/common/confirm-dialog';
import { Card } from '@/components/ui/card';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import { useToast } from '@/components/ui/toast';
import { ontologiesApi } from '@/lib/api/endpoints';
import { qk, useExtracts, useInvalidate, useOntologies } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatBytes, formatNumber, formatRelative } from '@/lib/format';
import type { Extract } from '@/lib/api/types';
import { ExtractOwnershipSheet } from './ontology-catalog';

/**
 * Les extraits du catalogue, tous, sans passer par leur ontologie.
 *
 * Un extrait ne se voyait que sous l'ontologie dont il est tire : pour en
 * retrouver un, il fallait deja savoir laquelle - et pour savoir a qui il
 * appartient, l'ouvrir. Un dataset, une source de donnees et une ontologie ont
 * chacun leur entree de catalogue ; l'extrait est le meme genre d'objet, se
 * cede de la meme facon et se rattache a un projet de la meme facon, donc il a
 * la meme entree.
 *
 * Ce qui est visible ici suit l'ontologie et non le proprietaire de l'extrait :
 * le nom d'un extrait et son n decrivent le contenu de l'ontologie, donc un
 * extrait tire d'une ontologie qu'on ne peut pas lire n'apparait pas, meme
 * confie a notre equipe.
 */
export function ExtractCatalog() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();
  const extracts = useExtracts();
  // Pour nommer l'ontologie d'origine : l'extrait ne porte que son
  // identifiant, et un identifiant ne dit rien a personne.
  const ontologies = useOntologies();
  const [owned, setOwned] = React.useState<Extract | null>(null);

  const nomOntologie = (ontologyId: string) =>
    ontologies.data?.find((item) => item.id === ontologyId)?.name ?? ontologyId;

  const remove = useMutation({
    mutationFn: (extractId: string) => ontologiesApi.deleteExtract(extractId),
    onSuccess: () => {
      invalidate(qk.extracts);
      invalidate(qk.ontologies);
      toast.success(t('ontologies.extractDeleted'));
    },
    onError: (error) => toast.error(error, t('ontologies.extractDelete')),
  });

  const columns: Column<Extract>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (extract) => extract.name,
      searchValue: (extract) => `${extract.name} ${extract.description}`,
      cell: (extract) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{extract.name}</p>
          {extract.description ? (
            <p className="truncate text-xs text-muted-foreground">{extract.description}</p>
          ) : null}
        </div>
      ),
    },
    {
      id: 'ontology',
      header: t('nav.ontologies'),
      sortValue: (extract) => nomOntologie(extract.ontologyId),
      searchValue: (extract) => nomOntologie(extract.ontologyId),
      cell: (extract) => (
        <span className="text-xs text-muted-foreground">{nomOntologie(extract.ontologyId)}</span>
      ),
    },
    {
      id: 'objects',
      header: t('ontologies.objects'),
      align: 'right',
      sortValue: (extract) => extract.objectCount,
      cell: (extract) => (
        <span className="text-xs tabular-nums">{formatNumber(extract.objectCount, locale)}</span>
      ),
    },
    {
      id: 'size',
      header: t('common.size'),
      align: 'right',
      sortValue: (extract) => extract.totalBytes,
      cell: (extract) => (
        <span className="text-xs tabular-nums">{formatBytes(extract.totalBytes, locale)}</span>
      ),
    },
    {
      id: 'owner',
      header: t('common.owner'),
      sortValue: (extract) => extract.ownerName ?? extract.ownerId ?? '',
      cell: (extract) => (
        <span className="text-xs text-muted-foreground">
          {extract.ownerName || extract.ownerId || extract.ownerUserId || '—'}
        </span>
      ),
    },
    {
      id: 'createdAt',
      header: t('common.createdAt'),
      sortValue: (extract) => extract.createdAt,
      cell: (extract) => (
        <span className="text-xs text-muted-foreground">
          {formatRelative(extract.createdAt, locale)}
        </span>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <Card>
        <DataTable
          data={extracts.data}
          columns={columns}
          rowKey={(extract) => extract.id}
          isLoading={extracts.isLoading}
          isError={extracts.isError}
          error={extracts.error}
          onRetry={() => void extracts.refetch()}
          defaultSort={{ columnId: 'createdAt', direction: 'desc' }}
          emptyState={
            <EmptyState
              icon={Scissors}
              title={t('ontologies.extractCatalogEmpty')}
              description={t('ontologies.extractCatalogHint')}
            />
          }
          rowActions={(extract) => (
            <>
              <DropdownMenuItem onSelect={() => setOwned(extract)}>
                <UserRoundCog aria-hidden />
                {t('projects.transferOwnership')}
              </DropdownMenuItem>
              <DropdownMenuItem
                destructive
                onSelect={() =>
                  ask({
                    title: t('ontologies.extractDelete'),
                    description: t('ontologies.extractDeleteWarning'),
                    confirmLabel: t('common.delete'),
                    destructive: true,
                    onConfirm: () => remove.mutateAsync(extract.id),
                  })
                }
              >
                <Trash2 aria-hidden />
                {t('common.delete')}
              </DropdownMenuItem>
            </>
          )}
        />
      </Card>

      <ExtractOwnershipSheet
        extract={owned}
        open={Boolean(owned)}
        onOpenChange={(open) => {
          if (!open) setOwned(null);
        }}
      />
      {dialog}
    </div>
  );
}
