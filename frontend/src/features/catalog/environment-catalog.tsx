import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Box, Hammer, Plus, Trash2 } from 'lucide-react';
import { PageHeader } from '@/components/common/page-header';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { useConfirm } from '@/components/common/confirm-dialog';
import { SearchInput } from '@/components/common/search-input';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardHeaderText,
  CardTitle,
} from '@/components/ui/card';
import { Badge, StatusBadge } from '@/components/ui/badge';
import { Field } from '@/components/ui/field';
import { Input, CodeTextarea } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import { useToast } from '@/components/ui/toast';
import { useDockerfile, useEnvironments, useProjects, qk, useInvalidate } from '@/lib/api/queries';
import { environmentsApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatRelative } from '@/lib/format';
import { presentIdes } from '@/lib/presenters';
import { slugify } from '@/lib/format';
import type { Environment, EnvironmentRevision, Project } from '@/lib/api/types';

// English, like the Dockerfile it seeds: this is file content the user edits
// and ships, not interface copy, so it does not belong in the translation
// catalogue and must not be French there either.
const BLANK_DOCKERFILE = `# Noryx environment
# Add your libraries below, then build a revision.
FROM noryx-python:latest

RUN pip install --no-cache-dir \\
    pandas \\
    scikit-learn
`;

/**
 * Where a build runs. Required, and never silently defaulted to the first
 * project: a build consumes that project's quota and its result is attributed
 * to it, so guessing would spend someone's budget on their behalf.
 */
function BuildProjectField({
  projects,
  value,
  onChange,
}: {
  projects: Project[];
  value: string;
  onChange: (value: string) => void;
}) {
  const t = useT();
  return (
    <Field
      label={t('environments.buildProject')}
      description={t('environments.buildProjectHint')}
      required
    >
      <Select
        value={value}
        onValueChange={onChange}
        placeholder={t('environments.buildProjectPlaceholder')}
        options={projects.map((project) => ({ value: project.id, label: project.name }))}
      />
    </Field>
  );
}

function CreateEnvironmentSheet({
  projects,
  projectId,
  onProjectChange,
  environments,
  open,
  onOpenChange,
}: {
  projects: Project[];
  projectId: string;
  onProjectChange: (value: string) => void;
  environments: Environment[];
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();

  const [name, setName] = React.useState('');
  const [baseId, setBaseId] = React.useState('');
  const [dockerfile, setDockerfile] = React.useState(BLANK_DOCKERFILE);
  const [touched, setTouched] = React.useState(false);

  const base = environments.find((environment) => environment.id === baseId);
  const baseDockerfile = useDockerfile(base?.latestBuildId);

  // Forking an environment pulls its definition in, so the user starts from
  // the libraries they already have rather than a blank file.
  React.useEffect(() => {
    if (baseId && baseDockerfile.data) setDockerfile(baseDockerfile.data);
    if (!baseId) setDockerfile(BLANK_DOCKERFILE);
  }, [baseId, baseDockerfile.data]);

  React.useEffect(() => {
    if (!open) {
      setName('');
      setBaseId('');
      setTouched(false);
    }
  }, [open]);

  const slug = slugify(name);
  const nameError = touched && !slug ? t('common.required') : undefined;
  const projectError = touched && !projectId ? t('common.required') : undefined;

  const mutation = useMutation({
    mutationFn: () =>
      environmentsApi.createBuild({
        projectId,
        name: slug,
        dockerfileContent: dockerfile,
        contextPath: '.',
      }),
    onSuccess: () => {
      invalidate(qk.environments(), qk.builds(projectId));
      onOpenChange(false);
      toast.success(t('environments.buildStarted'), t('environments.title'));
    },
    onError: (error) => toast.error(error, t('environments.createTitle')),
  });

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent size="lg" aria-describedby={undefined}>
        <form
          onSubmit={(event) => {
            event.preventDefault();
            setTouched(true);
            if (slug && projectId) mutation.mutate();
          }}
          className="flex min-h-0 flex-1 flex-col"
        >
          <SheetHeader>
            <SheetTitle>{t('environments.createTitle')}</SheetTitle>
            <SheetDescription>{t('environments.createHint')}</SheetDescription>
          </SheetHeader>
          <SheetBody>
            <BuildProjectField projects={projects} value={projectId} onChange={onProjectChange} />
            {projectError ? (
              <p className="text-xs text-danger">{projectError}</p>
            ) : null}
            <Field
              label={t('environments.nameLabel')}
              description={t('environments.nameHint')}
              error={nameError}
              required
            >
              <Input
                value={name}
                onChange={(event) => setName(event.target.value)}
                onBlur={() => setTouched(true)}
                placeholder={t('environments.namePlaceholder')}
                maxLength={63}
                autoFocus
              />
            </Field>
            <Field label={t('environments.baseLabel')} description={t('environments.baseHint')}>
              <Select
                value={baseId}
                onValueChange={setBaseId}
                placeholder={t('environments.baseEmpty')}
                options={[
                  { value: '', label: t('environments.baseEmpty') },
                  ...environments
                    .filter((environment) => environment.latestBuildId)
                    .map((environment) => ({ value: environment.id, label: environment.name })),
                ]}
              />
            </Field>
            <Field label={t('environments.definition')} description={t('environments.definitionHint')}>
              <CodeTextarea
                value={dockerfile}
                onChange={(event) => setDockerfile(event.target.value)}
              />
            </Field>
          </SheetBody>
          <SheetFooter>
            <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" variant="primary" loading={mutation.isPending} disabled={!slug || !projectId}>
              <Hammer aria-hidden />
              {t('environments.build')}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  );
}

function EnvironmentDetail({
  environment,
  projects,
  projectId,
  onProjectChange,
  onClose,
}: {
  environment: Environment;
  projects: Project[];
  projectId: string;
  onProjectChange: (value: string) => void;
  onClose: () => void;
}) {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const dockerfile = useDockerfile(environment.latestBuildId);
  const [draft, setDraft] = React.useState<string | null>(null);

  const rebuild = useMutation({
    mutationFn: () =>
      environmentsApi.createBuild({
        projectId,
        name: environment.name,
        dockerfileContent: draft ?? dockerfile.data ?? '',
        contextPath: environment.revisions?.[0]?.contextPath ?? '.',
      }),
    onSuccess: () => {
      invalidate(qk.environments(), qk.builds(projectId));
      setDraft(null);
      toast.success(t('environments.buildStarted'), environment.name);
    },
    onError: (error) => toast.error(error, t('environments.build')),
  });

  const revisionColumns: Column<EnvironmentRevision>[] = [
    {
      id: 'build',
      header: t('production.revision'),
      cell: (revision) => (
        <span className="flex items-center gap-2">
          <span className="font-mono text-xs">{revision.buildId.slice(0, 8)}</span>
          {revision.buildId === environment.latestBuildId ? (
            <Badge tone="success">{t('environments.activeRevision')}</Badge>
          ) : null}
        </span>
      ),
    },
    {
      id: 'status',
      header: t('common.status'),
      cell: (revision) => <StatusBadge status={revision.status} locale={locale} />,
    },
    {
      id: 'createdAt',
      header: t('common.createdAt'),
      sortValue: (revision) => revision.createdAt,
      cell: (revision) => (
        <span className="text-xs text-muted-foreground">
          {formatRelative(revision.createdAt, locale)}
        </span>
      ),
    },
  ];

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{environment.name}</CardTitle>
          <CardDescription>
            {presentIdes(environment.workspaceIdes)}
            {environment.latestImageSizeGiB ? ` · ${environment.latestImageSizeGiB} Gio` : ''}
          </CardDescription>
        </CardHeaderText>
        <div className="flex items-center gap-2">
          <StatusBadge status={environment.latestStatus} locale={locale} />
          <Button variant="ghost" size="sm" onClick={onClose}>
            {t('common.close')}
          </Button>
        </div>
      </CardHeader>

      <CardContent>
        <Tabs defaultValue="definition">
          <TabsList>
            <TabsTrigger value="definition">{t('environments.definition')}</TabsTrigger>
            <TabsTrigger value="revisions">{t('environments.revisions')}</TabsTrigger>
          </TabsList>

          <TabsContent value="definition" className="space-y-3">
            <Field label={t('environments.definition')} description={t('environments.definitionHint')}>
              <CodeTextarea
                value={draft ?? dockerfile.data ?? ''}
                onChange={(event) => setDraft(event.target.value)}
                readOnly={dockerfile.isLoading}
              />
            </Field>
            <BuildProjectField projects={projects} value={projectId} onChange={onProjectChange} />
            <div className="flex justify-end gap-2">
              {draft !== null ? (
                <Button variant="secondary" onClick={() => setDraft(null)}>
                  {t('common.cancel')}
                </Button>
              ) : null}
              <Button
                variant="primary"
                loading={rebuild.isPending}
                onClick={() => rebuild.mutate()}
                disabled={dockerfile.isLoading || !projectId}
              >
                <Hammer aria-hidden />
                {t('environments.build')}
              </Button>
            </div>
          </TabsContent>

          <TabsContent value="revisions">
            <DataTable
              data={environment.revisions ?? []}
              columns={revisionColumns}
              rowKey={(revision) => revision.buildId}
              defaultSort={{ columnId: 'createdAt', direction: 'desc' }}
              emptyState={
                <EmptyState
                  compact
                  title={t('environments.revisions')}
                  description={t('environments.revisionsHint')}
                />
              }
            />
          </TabsContent>
        </Tabs>
      </CardContent>

      <CardFooter className="justify-between text-xs text-muted-foreground">
        <span className="truncate font-mono" title={environment.destinationImage}>
          {environment.destinationImage || '—'}
        </span>
        <span>{formatRelative(environment.updatedAt, locale)}</span>
      </CardFooter>
    </Card>
  );
}

/**
 * Environment catalogue.
 *
 * Environments moved out of the project because they never belonged to one:
 * `/api/v1/environments` is a platform endpoint with an optional project
 * filter, and an image built in one project is used by workspaces, jobs and
 * apps in others. Listing them per project made a shared asset look owned, and
 * hid every environment a user could actually launch.
 *
 * Building still happens somewhere, so the project is now chosen in the form
 * rather than inherited from the URL — the same decision, made visible.
 */
export function EnvironmentCatalog() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();

  const [search, setSearch] = React.useState('');
  const [creating, setCreating] = React.useState(false);
  const [selectedId, setSelectedId] = React.useState<string | null>(null);
  // Which project a build runs in. Remembered across sheets so a user who
  // builds several environments in a row does not re-answer every time.
  const [buildProjectId, setBuildProjectId] = React.useState('');

  const projects = useProjects();
  const environments = useEnvironments();
  const selected = environments.data?.find((environment) => environment.id === selectedId) ?? null;

  const remove = useMutation({
    mutationFn: (environmentId: string) => environmentsApi.remove(environmentId),
    onSuccess: () => {
      invalidate(qk.environments());
      setSelectedId(null);
    },
    onError: (error) => toast.error(error, t('environments.deleteTitle')),
  });

  const columns: Column<Environment>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (environment) => environment.name,
      searchValue: (environment) => environment.name,
      cell: (environment) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{environment.name}</p>
          <p className="truncate text-xs text-muted-foreground">
            {presentIdes(environment.workspaceIdes)}
          </p>
        </div>
      ),
    },
    {
      id: 'category',
      header: t('common.type'),
      sortValue: (environment) => environment.category,
      cell: (environment) => (
        <Badge tone={environment.category === 'system' ? 'neutral' : 'brand'}>
          {environment.category === 'system' ? 'Système' : 'Personnalisé'}
        </Badge>
      ),
    },
    {
      id: 'status',
      header: t('common.status'),
      sortValue: (environment) => environment.latestStatus,
      cell: (environment) => <StatusBadge status={environment.latestStatus} locale={locale} />,
    },
    {
      id: 'revisions',
      header: t('environments.revisions'),
      align: 'right',
      sortValue: (environment) => environment.revisions?.length ?? 0,
      cell: (environment) => (
        <span className="tabular-nums text-muted-foreground">{environment.revisions?.length ?? 0}</span>
      ),
    },
    {
      id: 'updatedAt',
      header: t('common.updatedAt'),
      sortValue: (environment) => environment.updatedAt,
      cell: (environment) => (
        <span className="text-xs text-muted-foreground">
          {formatRelative(environment.updatedAt, locale)}
        </span>
      ),
    },
  ];

  return (
    <div className="space-y-5">
      <PageHeader
        title={t('environments.title')}
        description={t('environments.subtitle')}
        actions={
          <Button variant="primary" onClick={() => setCreating(true)}>
            <Plus aria-hidden />
            {t('environments.create')}
          </Button>
        }
      />

      {environments.data && environments.data.length > 0 ? (
        <SearchInput
          value={search}
          onValueChange={setSearch}
          label={t('common.search')}
          className="max-w-xs"
        />
      ) : null}

      <Card>
        <DataTable
          data={environments.data}
          columns={columns}
          rowKey={(environment) => environment.id}
          isLoading={environments.isLoading}
          isError={environments.isError}
          error={environments.error}
          onRetry={() => void environments.refetch()}
          search={search}
          onResetSearch={() => setSearch('')}
          onRowClick={(environment) => setSelectedId(environment.id)}
          defaultSort={{ columnId: 'updatedAt', direction: 'desc' }}
          emptyState={
            <EmptyState
              icon={Box}
              title={t('environments.empty')}
              description={t('environments.emptyHint')}
              action={
                <Button variant="primary" onClick={() => setCreating(true)}>
                  <Plus aria-hidden />
                  {t('environments.create')}
                </Button>
              }
            />
          }
          rowActions={(environment) => (
            <>
              <DropdownMenuItem onSelect={() => setSelectedId(environment.id)}>
                {t('common.open')}
              </DropdownMenuItem>
              <DropdownMenuItem
                destructive
                disabled={environment.category === 'system'}
                onSelect={() =>
                  ask({
                    title: t('environments.deleteTitle'),
                    description: t('environments.deleteWarning'),
                    confirmLabel: t('common.delete'),
                    destructive: true,
                    confirmationValue: environment.name,
                    confirmationLabel: t('environments.nameLabel'),
                    onConfirm: () => remove.mutateAsync(environment.id),
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

      {selected ? (
        <EnvironmentDetail
          environment={selected}
          projects={projects.data ?? []}
          projectId={buildProjectId}
          onProjectChange={setBuildProjectId}
          onClose={() => setSelectedId(null)}
        />
      ) : null}

      <CreateEnvironmentSheet
        projects={projects.data ?? []}
        projectId={buildProjectId}
        onProjectChange={setBuildProjectId}
        environments={environments.data ?? []}
        open={creating}
        onOpenChange={setCreating}
      />
      {dialog}
    </div>
  );
}
