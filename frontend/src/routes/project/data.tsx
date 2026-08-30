import * as React from 'react';
import { Link, useNavigate, useParams } from 'react-router';
import { useMutation } from '@tanstack/react-query';
import { Database, FolderGit2, Link2, Network, Plug, Unlink } from 'lucide-react';
import { PageHeader } from '@/components/common/page-header';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Select } from '@/components/ui/select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import { useToast } from '@/components/ui/toast';
import {
  useDatasets,
  useDatasources,
  useOntologies,
  useProjectDatasets,
  useProjectDatasources,
  useProjectOntologies,
  useProjectRepositories,
  useRepositories,
  qk,
  useInvalidate,
} from '@/lib/api/queries';
import { projectsApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatRelative } from '@/lib/format';
import type { Dataset, Datasource, Ontology, Repository } from '@/lib/api/types';

const SECTIONS = ['datasets', 'datasources', 'ontologies', 'repositories'] as const;
type Section = (typeof SECTIONS)[number];

/**
 * Attach/detach bar.
 *
 * Project data resources are references into the global catalogue, not copies,
 * which is what ADR-016 and ADR-017 describe. The picker only lists what is
 * not attached yet, so the action can never be a no-op.
 */
function AttachBar<T extends { id: string; name: string }>({
  available,
  attached,
  label,
  onAttach,
  pending,
}: {
  available: T[];
  attached: T[];
  label: string;
  onAttach: (id: string) => void;
  pending: boolean;
}) {
  const t = useT();
  const [selection, setSelection] = React.useState('');
  const attachedIds = new Set(attached.map((item) => item.id));
  const options = available
    .filter((item) => !attachedIds.has(item.id))
    .map((item) => ({ value: item.id, label: item.name }));

  return (
    <div className="flex flex-wrap items-end gap-2">
      <div className="min-w-56 flex-1">
        <label className="mb-1.5 block text-xs font-medium" htmlFor="attach-select">
          {label}
        </label>
        <Select
          value={selection}
          onValueChange={setSelection}
          options={options}
          placeholder={options.length ? t('common.search') : t('common.none')}
          aria-label={label}
        />
      </div>
      <Button
        variant="primary"
        disabled={!selection || pending}
        loading={pending}
        onClick={() => {
          onAttach(selection);
          setSelection('');
        }}
      >
        <Link2 aria-hidden />
        {t('datasources.attach')}
      </Button>
    </div>
  );
}

export function ProjectDataPage() {
  const t = useT();
  const { locale } = useI18n();
  const navigate = useNavigate();
  const { projectId, section } = useParams<{ projectId: string; section?: string }>();
  const toast = useToast();
  const invalidate = useInvalidate();

  const active: Section = SECTIONS.includes(section as Section) ? (section as Section) : 'datasets';

  const projectDatasets = useProjectDatasets(projectId);
  const projectDatasources = useProjectDatasources(projectId);
  const projectOntologies = useProjectOntologies(projectId);
  const projectRepositories = useProjectRepositories(projectId);

  const allDatasets = useDatasets();
  const allDatasources = useDatasources();
  const allOntologies = useOntologies();
  const allRepositories = useRepositories();

  function refresh() {
    if (!projectId) return;
    invalidate(
      qk.projectDatasets(projectId),
      qk.projectDatasources(projectId),
      qk.projectOntologies(projectId),
      qk.projectRepositories(projectId),
    );
  }

  const attach = useMutation({
    mutationFn: (input: { kind: Section; id: string }) => {
      const id = projectId as string;
      switch (input.kind) {
        case 'datasets':
          return projectsApi.attachDataset(id, input.id);
        case 'datasources':
          return projectsApi.attachDatasource(id, input.id);
        case 'ontologies':
          return projectsApi.attachOntology(id, input.id);
        case 'repositories':
          return projectsApi.attachRepository(id, input.id);
      }
    },
    onSuccess: refresh,
    onError: (error) => toast.error(error, t('datasources.attach')),
  });

  const detach = useMutation({
    mutationFn: (input: { kind: Section; id: string }) => {
      const id = projectId as string;
      switch (input.kind) {
        case 'datasets':
          return projectsApi.detachDataset(id, input.id);
        case 'datasources':
          return projectsApi.detachDatasource(id, input.id);
        case 'ontologies':
          return projectsApi.detachOntology(id, input.id);
        case 'repositories':
          return projectsApi.detachRepository(id, input.id);
      }
    },
    onSuccess: refresh,
    onError: (error) => toast.error(error, t('datasources.detach')),
  });

  const datasetColumns: Column<Dataset>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (dataset) => dataset.name,
      cell: (dataset) => (
        <Link
          to={`/catalog/datasets/${dataset.id}`}
          className="font-medium hover:text-brand hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
        >
          {dataset.name}
        </Link>
      ),
    },
    {
      id: 'classification',
      header: t('datasets.classification'),
      cell: (dataset) =>
        dataset.classification === 'hds' ? (
          <Badge tone="warning">{t('datasets.classificationHds')}</Badge>
        ) : (
          <Badge tone="outline">{t('datasets.classificationStandard')}</Badge>
        ),
    },
    {
      id: 'mount',
      header: t('datasets.storage'),
      cell: (dataset) => (
        <span className="font-mono text-xs text-muted-foreground">/datasets/{dataset.name}</span>
      ),
    },
  ];

  const datasourceColumns: Column<Datasource>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (datasource) => datasource.name,
      cell: (datasource) => <span className="font-medium">{datasource.name}</span>,
    },
    {
      id: 'type',
      header: t('common.type'),
      cell: (datasource) => <Badge tone="outline">{datasource.type}</Badge>,
    },
    {
      id: 'host',
      header: t('datasources.hostLabel'),
      cell: (datasource) => (
        <span className="truncate font-mono text-xs text-muted-foreground">
          {datasource.host}
          {datasource.port ? `:${datasource.port}` : ''}
        </span>
      ),
    },
  ];

  const ontologyColumns: Column<Ontology>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (ontology) => ontology.name,
      cell: (ontology) => <span className="font-medium">{ontology.name}</span>,
    },
    {
      id: 'source',
      header: t('ontologies.source'),
      cell: (ontology) => (
        <span className="text-xs text-muted-foreground">{ontology.sourceName || '—'}</span>
      ),
    },
  ];

  const repositoryColumns: Column<Repository>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (repository) => repository.name,
      cell: (repository) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{repository.name}</p>
          <p className="truncate font-mono text-xs text-muted-foreground">{repository.url}</p>
        </div>
      ),
    },
    {
      id: 'mount',
      header: t('repositories.title'),
      cell: (repository) => (
        <span className="font-mono text-xs text-muted-foreground">/repos/{repository.name}</span>
      ),
    },
    {
      id: 'updatedAt',
      header: t('common.updatedAt'),
      sortValue: (repository) => repository.updatedAt,
      cell: (repository) => (
        <span className="text-xs text-muted-foreground">
          {formatRelative(repository.updatedAt, locale)}
        </span>
      ),
    },
  ];

  function detachAction(kind: Section, id: string) {
    return (
      <DropdownMenuItem destructive onSelect={() => detach.mutate({ kind, id })}>
        <Unlink aria-hidden />
        {t('datasources.detach')}
      </DropdownMenuItem>
    );
  }

  return (
    <div className="space-y-5">
      <PageHeader
        title={t('nav.data')}
        description={t('datasets.subtitle')}
        actions={
          <Button variant="secondary" asChild>
            <Link to="/catalog">{t('nav.catalog')}</Link>
          </Button>
        }
      />

      <Tabs
        value={active}
        onValueChange={(value) => navigate(`/projects/${projectId}/data/${value}`)}
      >
        <TabsList>
          <TabsTrigger value="datasets">{t('nav.datasets')}</TabsTrigger>
          <TabsTrigger value="datasources">{t('nav.datasources')}</TabsTrigger>
          <TabsTrigger value="ontologies">{t('nav.ontologies')}</TabsTrigger>
          <TabsTrigger value="repositories">{t('nav.repositories')}</TabsTrigger>
        </TabsList>

        <TabsContent value="datasets" className="space-y-4">
          <AttachBar
            available={allDatasets.data ?? []}
            attached={projectDatasets.data ?? []}
            label={t('nav.datasets')}
            pending={attach.isPending}
            onAttach={(id) => attach.mutate({ kind: 'datasets', id })}
          />
          <Card>
            <DataTable
              data={projectDatasets.data}
              columns={datasetColumns}
              rowKey={(dataset) => dataset.id}
              isLoading={projectDatasets.isLoading}
              isError={projectDatasets.isError}
              error={projectDatasets.error}
              onRetry={() => void projectDatasets.refetch()}
              emptyState={
                <EmptyState
                  icon={Database}
                  title={t('datasets.empty')}
                  description={t('datasets.emptyHint')}
                />
              }
              rowActions={(dataset) => detachAction('datasets', dataset.id)}
            />
          </Card>
        </TabsContent>

        <TabsContent value="datasources" className="space-y-4">
          <AttachBar
            available={allDatasources.data ?? []}
            attached={projectDatasources.data ?? []}
            label={t('nav.datasources')}
            pending={attach.isPending}
            onAttach={(id) => attach.mutate({ kind: 'datasources', id })}
          />
          <Card>
            <DataTable
              data={projectDatasources.data}
              columns={datasourceColumns}
              rowKey={(datasource) => datasource.id}
              isLoading={projectDatasources.isLoading}
              isError={projectDatasources.isError}
              error={projectDatasources.error}
              onRetry={() => void projectDatasources.refetch()}
              emptyState={
                <EmptyState
                  icon={Plug}
                  title={t('datasources.empty')}
                  description={t('datasources.emptyHint')}
                />
              }
              rowActions={(datasource) => detachAction('datasources', datasource.id)}
            />
          </Card>
        </TabsContent>

        <TabsContent value="ontologies" className="space-y-4">
          <AttachBar
            available={allOntologies.data ?? []}
            attached={projectOntologies.data ?? []}
            label={t('nav.ontologies')}
            pending={attach.isPending}
            onAttach={(id) => attach.mutate({ kind: 'ontologies', id })}
          />
          <Card>
            <DataTable
              data={projectOntologies.data}
              columns={ontologyColumns}
              rowKey={(ontology) => ontology.id}
              isLoading={projectOntologies.isLoading}
              isError={projectOntologies.isError}
              error={projectOntologies.error}
              onRetry={() => void projectOntologies.refetch()}
              emptyState={
                <EmptyState
                  icon={Network}
                  title={t('ontologies.empty')}
                  description={t('ontologies.emptyHint')}
                />
              }
              rowActions={(ontology) => detachAction('ontologies', ontology.id)}
            />
          </Card>
        </TabsContent>

        <TabsContent value="repositories" className="space-y-4">
          <AttachBar
            available={allRepositories.data ?? []}
            attached={projectRepositories.data ?? []}
            label={t('nav.repositories')}
            pending={attach.isPending}
            onAttach={(id) => attach.mutate({ kind: 'repositories', id })}
          />
          <Card>
            <DataTable
              data={projectRepositories.data}
              columns={repositoryColumns}
              rowKey={(repository) => repository.id}
              isLoading={projectRepositories.isLoading}
              isError={projectRepositories.isError}
              error={projectRepositories.error}
              onRetry={() => void projectRepositories.refetch()}
              emptyState={
                <EmptyState
                  icon={FolderGit2}
                  title={t('repositories.empty')}
                  description={t('repositories.emptyHint')}
                />
              }
              rowActions={(repository) => detachAction('repositories', repository.id)}
            />
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
}
