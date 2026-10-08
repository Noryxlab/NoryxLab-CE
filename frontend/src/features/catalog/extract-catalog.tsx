import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { FolderOpen, Pencil, Scissors, Trash2, UserRoundCog } from 'lucide-react';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { useConfirm } from '@/components/common/confirm-dialog';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import { useToast } from '@/components/ui/toast';
import { ontologiesApi } from '@/lib/api/endpoints';
import { qk, useExtracts, useInvalidate, useOntologies } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatBytes, formatNumber, formatRelative } from '@/lib/format';
import type { Extract } from '@/lib/api/types';
import { MountedProjects, ProjectMountsSheet } from '@/components/common/project-mounts';
import { ExtractDetail } from './extract-detail';
import { ExtractDeclareForm } from './extract-declare';
import { ExtractOwnershipSheet, ExtractRenameSheet } from './ontology-catalog';
import { Card, CardContent, CardDescription, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { Field } from '@/components/ui/field';
import { Select } from '@/components/ui/select';

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
  const [renamed, setRenamed] = React.useState<Extract | null>(null);
  const [mounted, setMounted] = React.useState<Extract | null>(null);
  const [opened, setOpened] = React.useState<Extract | null>(null);

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
      id: 'layout',
      header: t('ontologies.extractLayout'),
      cell: (extract) => (
        <span className="text-xs text-muted-foreground">
          {(extract.layout ?? ['subject', 'visit', 'modality'])
            .map((level) => t(`ontologies.level_${level}` as 'ontologies.level_subject'))
            .join(' › ')}
        </span>
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
      id: 'projects',
      header: t('mounts.title'),
      cell: (extract) => <MountedProjects kind="extract" resourceId={extract.id} />,
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
      <DeclareExtract />
      <Card>
        <DataTable
          data={extracts.data}
          columns={columns}
          rowKey={(extract) => extract.id}
          isLoading={extracts.isLoading}
          isError={extracts.isError}
          error={extracts.error}
          onRetry={() => void extracts.refetch()}
          onRowClick={(extract) => setOpened(extract)}
          selectedKey={opened?.id ?? null}
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
              <DropdownMenuItem onSelect={() => setMounted(extract)}>
                <FolderOpen aria-hidden />
                {t('mounts.title')}
              </DropdownMenuItem>
              <DropdownMenuItem onSelect={() => setRenamed(extract)}>
                <Pencil aria-hidden />
                {t('ontologies.extractRename')}
              </DropdownMenuItem>
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
      {opened ? <ExtractDetail extract={opened} /> : null}

      <ProjectMountsSheet
        kind="extract"
        resourceId={mounted?.id ?? null}
        resourceName={mounted?.name}
        open={Boolean(mounted)}
        onOpenChange={(open) => {
          if (!open) setMounted(null);
        }}
      />
      <ExtractRenameSheet
        extract={renamed}
        open={Boolean(renamed)}
        onOpenChange={(open) => {
          if (!open) setRenamed(null);
        }}
      />
      {dialog}
    </div>
  );
}

/* Declarer un extrait, depuis l'entree de catalogue qui porte les extraits.
 *
 *  Le formulaire vivait sur la page d'une ontologie. C'est la page de l'objet
 *  qui decrit - pas de celui qu'on emporte - et quelqu'un qui l'ouvrait pour
 *  comprendre son jeu de donnees y trouvait un atelier de decoupe. Ici,
 *  l'ontologie est le premier champ du formulaire, exactement comme on
 *  choisit un dataset avant de le scanner. */
function DeclareExtract() {
  const t = useT();
  const ontologies = useOntologies();
  const [ontologyId, setOntologyId] = React.useState('');
  const choisie = ontologies.data?.find((item) => item.id === ontologyId);

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('ontologies.extractDeclare')}</CardTitle>
          <CardDescription>{t('ontologies.extractsHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent className="space-y-3">
        <Field label={t('nav.ontologies')} className="sm:max-w-sm">
          <Select
            value={ontologyId}
            onValueChange={setOntologyId}
            placeholder={t('ontologies.extractPickOntology')}
            options={(ontologies.data ?? []).map((item) => ({
              value: item.id,
              label: item.name,
              hint: item.sourceName || undefined,
            }))}
          />
        </Field>
        {/* Rien tant qu'aucune ontologie n'est choisie : un formulaire de
          *  decoupe sans objet a decouper propose des categories qui
          *  n'existent pas. */}
        {choisie ? <ExtractDeclareForm ontology={choisie} /> : null}
      </CardContent>
    </Card>
  );
}
