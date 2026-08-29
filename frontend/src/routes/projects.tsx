import * as React from 'react';
import { Link } from 'react-router';
import { AppWindow, Boxes, LayoutGrid, List, Plus, Terminal } from 'lucide-react';
import { PageHeader } from '@/components/common/page-header';
import { SearchInput } from '@/components/common/search-input';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState, ErrorState, NoResultsState } from '@/components/common/states';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { SkeletonCards } from '@/components/ui/skeleton';
import { BareSelect } from '@/components/ui/select';
import { useProjects } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatRelative } from '@/lib/format';
import { CreateProjectSheet } from '@/features/projects/project-form';
import type { Project } from '@/lib/api/types';

const VIEW_KEY = 'noryx.projects.view';

function ResourceCount({ icon: Icon, count, label }: { icon: React.ComponentType<{ className?: string }>; count: number; label: string }) {
  if (!count) return null;
  return (
    <span className="inline-flex items-center gap-1 text-xs text-muted-foreground" title={label}>
      <Icon className="size-3.5" aria-hidden />
      <span className="tabular-nums">{count}</span>
      <span className="sr-only">{label}</span>
    </span>
  );
}

function ProjectCard({ project }: { project: Project }) {
  const t = useT();
  const { locale } = useI18n();
  const active = project.runningWorkspaces + project.runningJobs + project.runningApps;

  return (
    <Card className="group relative flex flex-col transition-colors hover:border-brand/50">
      <div className="flex-1 p-4">
        <div className="flex items-start justify-between gap-2">
          <h3 className="min-w-0 text-sm font-semibold">
            {/* Stretched link: the whole card is the target, but only one
                focusable element ends up in the tab order. */}
            <Link
              to={`/projects/${project.id}`}
              className="after:absolute after:inset-0 after:rounded-lg focus-visible:outline-none group-focus-within:after:outline-2 group-focus-within:after:outline-offset-2 group-focus-within:after:outline-ring"
            >
              <span className="line-clamp-2">{project.name}</span>
            </Link>
          </h3>
          {active > 0 ? (
            <Badge tone="success" className="shrink-0">
              {active}
            </Badge>
          ) : null}
        </div>
        {project.description ? (
          <p className="mt-1.5 line-clamp-2 text-xs leading-relaxed text-muted-foreground">
            {project.description}
          </p>
        ) : null}
      </div>
      <div className="flex flex-wrap items-center gap-3 border-t border-border px-4 py-2.5">
        <ResourceCount icon={Terminal} count={project.runningWorkspaces} label={t('nav.workspaces')} />
        <ResourceCount icon={LayoutGrid} count={project.runningJobs} label={t('nav.jobs')} />
        <ResourceCount icon={AppWindow} count={project.runningApps} label={t('nav.apps')} />
        <span className="ml-auto text-xs text-muted-foreground">
          {formatRelative(project.lastActivityAt || project.updatedAt, locale)}
        </span>
      </div>
    </Card>
  );
}

export function ProjectsPage() {
  const t = useT();
  const { locale } = useI18n();
  const { data, isLoading, isError, error, refetch } = useProjects();

  const [search, setSearch] = React.useState('');
  const [creating, setCreating] = React.useState(false);
  const [view, setView] = React.useState<'grid' | 'list'>(() => {
    try {
      return localStorage.getItem(VIEW_KEY) === 'list' ? 'list' : 'grid';
    } catch {
      return 'grid';
    }
  });

  function changeView(next: string) {
    const mode = next === 'list' ? 'list' : 'grid';
    setView(mode);
    try {
      localStorage.setItem(VIEW_KEY, mode);
    } catch {
      /* ignore */
    }
  }

  const filtered = React.useMemo(() => {
    if (!data) return [];
    const query = search.trim().toLowerCase();
    if (!query) return data;
    return data.filter(
      (project) =>
        project.name.toLowerCase().includes(query) ||
        project.description.toLowerCase().includes(query),
    );
  }, [data, search]);

  const columns: Column<Project>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (project) => project.name,
      searchValue: (project) => `${project.name} ${project.description}`,
      cell: (project) => (
        <div className="min-w-0">
          <Link
            to={`/projects/${project.id}`}
            className="font-medium hover:text-brand hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
          >
            {project.name}
          </Link>
          {project.description ? (
            <p className="truncate text-xs text-muted-foreground">{project.description}</p>
          ) : null}
        </div>
      ),
    },
    {
      id: 'resources',
      header: t('projects.resources'),
      cell: (project) => (
        <div className="flex items-center gap-3">
          <ResourceCount icon={Terminal} count={project.runningWorkspaces} label={t('nav.workspaces')} />
          <ResourceCount icon={LayoutGrid} count={project.runningJobs} label={t('nav.jobs')} />
          <ResourceCount icon={AppWindow} count={project.runningApps} label={t('nav.apps')} />
          {project.runningWorkspaces + project.runningJobs + project.runningApps === 0 ? (
            <span className="text-xs text-muted-foreground">—</span>
          ) : null}
        </div>
      ),
    },
    {
      id: 'activity',
      header: t('projects.lastActivity'),
      sortValue: (project) => project.lastActivityAt || project.updatedAt,
      cell: (project) => (
        <span className="text-xs text-muted-foreground">
          {formatRelative(project.lastActivityAt || project.updatedAt, locale)}
        </span>
      ),
    },
  ];

  const emptyState = (
    <EmptyState
      icon={Boxes}
      title={t('projects.empty')}
      description={t('projects.emptyHint')}
      action={
        <Button variant="primary" onClick={() => setCreating(true)}>
          <Plus aria-hidden />
          {t('projects.create')}
        </Button>
      }
    />
  );

  return (
    <div className="space-y-5">
      <PageHeader
        title={t('projects.title')}
        description={t('projects.subtitle')}
        actions={
          <Button variant="primary" onClick={() => setCreating(true)}>
            <Plus aria-hidden />
            {t('projects.create')}
          </Button>
        }
      />

      {isError ? (
        <ErrorState error={error} onRetry={() => void refetch()} />
      ) : (
        <>
          {data && data.length > 0 ? (
            <div className="flex flex-wrap items-center gap-2">
              <SearchInput
                value={search}
                onValueChange={setSearch}
                label={t('projects.search')}
                className="max-w-xs flex-1"
              />
              <BareSelect
                value={view}
                onValueChange={changeView}
                aria-label={t('projects.viewMode')}
                className="w-36"
                options={[
                  { value: 'grid', label: t('projects.viewGrid') },
                  { value: 'list', label: t('projects.viewList') },
                ]}
              />
            </div>
          ) : null}

          {isLoading ? (
            <SkeletonCards />
          ) : data && data.length === 0 ? (
            <Card>{emptyState}</Card>
          ) : view === 'grid' ? (
            filtered.length === 0 ? (
              <Card>
                <NoResultsState onReset={() => setSearch('')} />
              </Card>
            ) : (
              <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                {filtered.map((project) => (
                  <ProjectCard key={project.id} project={project} />
                ))}
              </div>
            )
          ) : (
            <Card>
              <DataTable
                data={data}
                columns={columns}
                rowKey={(project) => project.id}
                search={search}
                onResetSearch={() => setSearch('')}
                defaultSort={{ columnId: 'activity', direction: 'desc' }}
                emptyState={emptyState}
              />
            </Card>
          )}

          {view === 'grid' && filtered.length > 0 ? (
            <div className="flex items-center gap-2 text-xs text-muted-foreground">
              <List className="size-3.5" aria-hidden />
              {filtered.length} / {data?.length ?? 0}
            </div>
          ) : null}
        </>
      )}

      <CreateProjectSheet open={creating} onOpenChange={setCreating} />
    </div>
  );
}
