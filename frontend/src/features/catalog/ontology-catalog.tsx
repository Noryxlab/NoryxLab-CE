import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { AlertTriangle, Network, Radar, Search, Trash2 } from 'lucide-react';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { useConfirm } from '@/components/common/confirm-dialog';
import { SectionHeader } from '@/components/common/page-header';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardHeaderText,
  CardTitle,
} from '@/components/ui/card';
import { Badge, StatusBadge } from '@/components/ui/badge';
import { Field } from '@/components/ui/field';
import { Select } from '@/components/ui/select';
import { Input } from '@/components/ui/input';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
  TableWrapper,
} from '@/components/ui/table';
import { useToast } from '@/components/ui/toast';
import {
  useOntologies,
  useOntologyFreshness,
  useOntologyCompleteness,
  useOntologyCohorts,
  useProjects,
  useProjectDatasets,
  qk,
  useInvalidate,
} from '@/lib/api/queries';
import { ontologiesApi, projectsApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatBytes, formatNumber, formatRelative } from '@/lib/format';
import type { OntologyQueryItem, Ontology } from '@/lib/api/types';

/**
 * Ontology catalogue (ADR-025).
 *
 * The natural-language query surface keeps the guardrail the ADR asks for:
 * the generated result is presented as data to inspect, never as an
 * authoritative answer, and the ontology's own manifest stays visible.
 */
function OntologyQuery({ ontology }: { ontology: Ontology }) {
  const t = useT();
  const toast = useToast();
  const [question, setQuestion] = React.useState('');
  const [result, setResult] = React.useState<{
    items?: OntologyQueryItem[];
    count?: number;
    limited?: boolean;
  } | null>(null);

  const run = useMutation({
    mutationFn: () => ontologiesApi.query(ontology.id, question.trim()),
    onSuccess: (response) => {
      if (response.error) {
        toast.error(response.error, t('ontologies.query'));
        setResult(null);
        return;
      }
      setResult(response);
    },
    onError: (error) => toast.error(error, t('ontologies.query')),
  });

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>
            {t('ontologies.query')} — {ontology.name}
          </CardTitle>
          <CardDescription>{t('ontologies.queryHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent className="space-y-4">
        <OntologyFreshnessNote ontologyId={ontology.id} />
        <form
          onSubmit={(event) => {
            event.preventDefault();
            if (question.trim()) run.mutate();
          }}
          className="flex items-end gap-2"
        >
          <Field label={t('ontologies.queryLabel')} className="flex-1">
            <Input
              value={question}
              onChange={(event) => setQuestion(event.target.value)}
              placeholder={t('ontologies.queryPlaceholder')}
            />
          </Field>
          <Button type="submit" variant="primary" loading={run.isPending} disabled={!question.trim()}>
            <Search aria-hidden />
            {t('ontologies.query')}
          </Button>
        </form>

        {result ? (
          result.items?.length ? (
            <>
              <p className="text-xs text-muted-foreground">
                {t('ontologies.matches', {
                  shown: String(result.items.length),
                  total: String(result.count ?? result.items.length),
                })}
              </p>
              <TableWrapper className="rounded-md border border-border">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t('ontologies.object')}</TableHead>
                      <TableHead>{t('ontologies.objectType')}</TableHead>
                      <TableHead>{t('ontologies.parent')}</TableHead>
                      <TableHead className="text-right">{t('ontologies.objects')}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {result.items.map((item) => (
                      <TableRow key={`${item.parent}/${item.object}/${item.type}`}>
                        <TableCell className="font-mono text-xs">{item.object}</TableCell>
                        <TableCell className="text-xs">{item.type}</TableCell>
                        <TableCell className="font-mono text-xs text-muted-foreground">
                          {item.parent || '—'}
                        </TableCell>
                        <TableCell className="text-right text-xs tabular-nums">{item.count}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TableWrapper>
            </>
          ) : (
            <p className="text-xs text-muted-foreground">{t('ontologies.noMatch')}</p>
          )
        ) : null}
      </CardContent>
    </Card>
  );
}

/**
 * Whether the ontology still describes its source.
 *
 * An ontology is a photograph, and the screen presented it as a fact: the June
 * scan of the study read "18,738 objects, 20 subjects" in exactly the same
 * typeface as this morning's 24,179 and 31, and nothing said the study had
 * recruited eleven subjects in between. Everything built on an ontology - a
 * cohort above all - silently inherits that gap, so the count is checked
 * against the source and the difference is stated in objects.
 */
function OntologyFreshnessNote({ ontologyId }: { ontologyId: string }) {
  const t = useT();
  const { locale } = useI18n();
  const freshness = useOntologyFreshness(ontologyId);
  const data = freshness.data;
  if (!data) return null;

  const measured =
    data.ageDays > 0
      ? t('ontologies.freshnessMeasured', {
          days: String(data.ageDays),
          objects: formatNumber(data.manifestObjects, locale),
        })
      : t('ontologies.freshnessMeasuredToday', {
          objects: formatNumber(data.manifestObjects, locale),
        });

  // A source that could not be reached leaves the count unknown rather than
  // zero: "24,179 fewer objects" would be a frightening lie.
  const drift =
    data.sourceObjects === 0 && data.manifestObjects > 0
      ? t('ontologies.freshnessUnknown')
      : data.drift > 0
        ? t('ontologies.freshnessGrown', { drift: formatNumber(data.drift, locale) })
        : data.drift < 0
          ? t('ontologies.freshnessShrunk', { drift: formatNumber(-data.drift, locale) })
          : t('ontologies.freshnessAligned');

  return (
    <div
      className={
        data.stale
          ? 'flex items-start gap-2 rounded-md border border-warning/40 bg-warning-subtle px-3 py-2'
          : 'flex items-start gap-2 rounded-md border border-border px-3 py-2'
      }
    >
      {data.stale ? (
        <AlertTriangle aria-hidden className="mt-0.5 size-4 shrink-0 text-warning-foreground" />
      ) : null}
      <p className={data.stale ? 'text-xs leading-relaxed text-warning-foreground' : 'text-xs text-muted-foreground'}>
        {data.stale ? (
          <span className="font-medium">{t('ontologies.freshnessStale')} — </span>
        ) : null}
        {measured} {drift}
      </p>
    </div>
  );
}

/**
 * Who the study covers, and who a cohort would leave out.
 *
 * "31 subjects" was the only number on the screen, and it averaged together
 * the subjects who carry a corneal wavefront and those who do not. Anyone
 * assembling a cohort by modality needs the second list by name, before they
 * publish an n.
 */
function OntologyCoverage({ ontologyId }: { ontologyId: string }) {
  const t = useT();
  const { locale } = useI18n();
  const coverage = useOntologyCompleteness(ontologyId);
  const data = coverage.data;
  if (!data || data.modalities.length === 0) return null;

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('ontologies.completeness')}</CardTitle>
          <CardDescription>{t('ontologies.completenessHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent className="space-y-3">
        <p className="text-xs text-muted-foreground">
          {t('ontologies.completenessSubjects', {
            complete: formatNumber(data.completeSubjects, locale),
            total: formatNumber(data.subjects, locale),
          })}
        </p>
        <TableWrapper className="rounded-md border border-border">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('ontologies.completenessModality')}</TableHead>
                <TableHead className="text-right">{t('ontologies.completenessHolders')}</TableHead>
                <TableHead className="text-right">{t('ontologies.objects')}</TableHead>
                <TableHead>{t('ontologies.completenessMissing')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {data.modalities.map((modality) => (
                <TableRow key={modality.name}>
                  <TableCell className="font-mono text-xs">{modality.name}</TableCell>
                  <TableCell className="text-right text-xs tabular-nums">
                    {formatNumber(modality.subjects, locale)} / {formatNumber(data.subjects, locale)}
                  </TableCell>
                  <TableCell className="text-right text-xs tabular-nums">
                    {formatNumber(modality.objects, locale)}
                  </TableCell>
                  <TableCell className="text-xs text-muted-foreground">
                    {modality.missingSubjects.length === 0 ? (
                      t('ontologies.completenessNoGap')
                    ) : (
                      <span className="font-mono">{modality.missingSubjects.join(', ')}</span>
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </TableWrapper>
      </CardContent>
    </Card>
  );
}

/**
 * Cohorts: the subset a study is actually run on.
 *
 * Declaring one resolves the selection to an explicit list of files and keeps
 * it. A cohort that re-ran its filter would return a different study every
 * month, and last month's n would stop being reproducible.
 *
 * Nothing is duplicated: the frozen paths point into the dataset where the data
 * already lives, and a workspace mounts the cohort as a tree of links over it.
 */
function OntologyCohorts({ ontology }: { ontology: Ontology }) {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();
  const cohorts = useOntologyCohorts(ontology.id);
  const [name, setName] = React.useState('');
  const [modalities, setModalities] = React.useState('');
  const [subjects, setSubjects] = React.useState('');

  const asList = (raw: string) =>
    raw
      .split(',')
      .map((value) => value.trim())
      .filter(Boolean);

  const create = useMutation({
    mutationFn: () =>
      ontologiesApi.createCohort(ontology.id, {
        name: name.trim(),
        // Left to the server when the ontology belongs to exactly one project;
        // it refuses with an explanation when the answer is ambiguous, which is
        // better than filing the cohort under a project that will never mount it.
        modalities: asList(modalities),
        subjects: asList(subjects),
      }),
    onSuccess: (created) => {
      toast.success(t('ontologies.cohortCreated', { count: formatNumber(created.objectCount, locale) }));
      setName('');
      setModalities('');
      setSubjects('');
      invalidate(qk.ontologyCohorts(ontology.id));
    },
    onError: (error) => toast.error(error, t('ontologies.cohortCreate')),
  });

  const remove = useMutation({
    mutationFn: (cohortId: string) => ontologiesApi.deleteCohort(cohortId),
    onSuccess: () => invalidate(qk.ontologyCohorts(ontology.id)),
    onError: (error) => toast.error(error, t('ontologies.cohortDeleteTitle')),
  });

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('ontologies.cohorts')}</CardTitle>
          <CardDescription>{t('ontologies.cohortsHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent className="space-y-4">
        <form
          onSubmit={(event) => {
            event.preventDefault();
            if (name.trim()) create.mutate();
          }}
          className="grid gap-2 sm:grid-cols-[1fr_1fr_1fr_auto] sm:items-end"
        >
          <Field label={t('ontologies.cohortName')}>
            <Input value={name} onChange={(event) => setName(event.target.value)} />
          </Field>
          <Field label={t('ontologies.cohortModalities')}>
            <Input
              value={modalities}
              onChange={(event) => setModalities(event.target.value)}
              placeholder="Cornea_Wavefront"
            />
          </Field>
          <Field label={t('ontologies.cohortSubjects')}>
            <Input
              value={subjects}
              onChange={(event) => setSubjects(event.target.value)}
              placeholder="PREMYOM1000-001"
            />
          </Field>
          <Button type="submit" variant="primary" loading={create.isPending} disabled={!name.trim()}>
            {t('ontologies.cohortCreate')}
          </Button>
        </form>

        {cohorts.data?.length ? (
          <TableWrapper className="rounded-md border border-border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('ontologies.cohortName')}</TableHead>
                  <TableHead className="text-right">{t('ontologies.objects')}</TableHead>
                  <TableHead className="text-right">{t('common.size')}</TableHead>
                  <TableHead>{t('common.createdAt')}</TableHead>
                  <TableHead />
                </TableRow>
              </TableHeader>
              <TableBody>
                {cohorts.data.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="text-xs font-medium">
                      {item.name}
                      {item.modalities.length ? (
                        <span className="ml-2 font-mono text-xs text-muted-foreground">
                          {item.modalities.join(', ')}
                        </span>
                      ) : null}
                    </TableCell>
                    <TableCell className="text-right text-xs tabular-nums">
                      {formatNumber(item.objectCount, locale)}
                    </TableCell>
                    <TableCell className="text-right text-xs tabular-nums">
                      {formatBytes(item.totalBytes, locale)}
                    </TableCell>
                    <TableCell className="text-xs text-muted-foreground">
                      {formatRelative(item.createdAt, locale)}
                    </TableCell>
                    <TableCell className="text-right">
                      <Button
                        variant="ghost"
                        onClick={() =>
                          ask({
                            title: t('ontologies.cohortDeleteTitle'),
                            description: t('ontologies.cohortDeleteWarning'),
                            confirmLabel: t('common.delete'),
                            destructive: true,
                            onConfirm: () => remove.mutateAsync(item.id),
                          })
                        }
                      >
                        <Trash2 aria-hidden />
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableWrapper>
        ) : (
          <p className="text-xs text-muted-foreground">{t('ontologies.cohortEmpty')}</p>
        )}
        <p className="text-xs text-muted-foreground">{t('ontologies.cohortMountHint')}</p>
      </CardContent>
      {dialog}
    </Card>
  );
}

/**
 * Launching a scan.
 *
 * The endpoint existed, the API client had a function for it, and no screen
 * called either - so an installation with no ontology had no way to make one
 * except by hand against the API, and the empty state said "no ontology" as if
 * that were a fact about the data rather than a missing button. The client also
 * sent no body, and the scan needs to be told which dataset to read.
 *
 * The scan reads object *paths*, never their content, and creates a new
 * ontology rather than replacing one: a scan is a photograph, and photographs
 * do not overwrite each other.
 */
function OntologyScan() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const projects = useProjects();
  const [projectId, setProjectId] = React.useState('');
  const [datasetId, setDatasetId] = React.useState('');
  const datasets = useProjectDatasets(projectId || undefined);

  const scan = useMutation({
    mutationFn: () => projectsApi.scanOntology(projectId, { datasetId }),
    onSuccess: (response) => {
      const summary = response.manifest?.summary;
      toast.success(
        t('ontologies.scanDone', {
          objects: formatNumber(summary?.objects ?? 0, locale),
          subjects: formatNumber(summary?.subjects ?? 0, locale),
        }),
      );
      invalidate(qk.ontologies);
    },
    onError: (error) => toast.error(error, t('ontologies.scanTitle')),
  });

  const datasetOptions = (datasets.data ?? []).map((item) => ({
    value: item.id,
    label: item.name,
    hint: item.bucket,
  }));

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('ontologies.scanTitle')}</CardTitle>
          <CardDescription>{t('ontologies.scanHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent className="space-y-3">
        <form
          onSubmit={(event) => {
            event.preventDefault();
            if (projectId && datasetId) scan.mutate();
          }}
          className="grid gap-2 sm:grid-cols-[1fr_1fr_auto] sm:items-end"
        >
          <Field label={t('ontologies.scanProject')}>
            <Select
              value={projectId}
              onValueChange={(value) => {
                setProjectId(value);
                setDatasetId('');
              }}
              placeholder={t('ontologies.scanProject')}
              options={(projects.data ?? []).map((project) => ({
                value: project.id,
                label: project.name,
              }))}
            />
          </Field>
          <Field label={t('ontologies.scanDataset')}>
            <Select
              value={datasetId}
              onValueChange={setDatasetId}
              placeholder={t('ontologies.scanDataset')}
              disabled={!projectId || datasetOptions.length === 0}
              options={datasetOptions}
            />
          </Field>
          <Button
            type="submit"
            variant="primary"
            loading={scan.isPending}
            disabled={!projectId || !datasetId}
          >
            <Radar aria-hidden />
            {t('ontologies.scan')}
          </Button>
        </form>
        {projectId && datasetOptions.length === 0 && !datasets.isLoading ? (
          <p className="text-xs text-muted-foreground">{t('ontologies.scanNoDataset')}</p>
        ) : null}
        {/* What the profile actually matches, where somebody can read it before
            wondering why half their objects came back unrecognised. */}
        <p className="text-xs text-muted-foreground">{t('ontologies.scanProfileHint')}</p>
      </CardContent>
    </Card>
  );
}

export function OntologyCatalog() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();

  const ontologies = useOntologies();
  const [selectedId, setSelectedId] = React.useState<string | null>(null);
  const selected = ontologies.data?.find((ontology) => ontology.id === selectedId) ?? null;

  const remove = useMutation({
    mutationFn: (ontologyId: string) => ontologiesApi.remove(ontologyId),
    onSuccess: () => {
      invalidate(qk.ontologies);
      setSelectedId(null);
    },
    onError: (error) => toast.error(error, t('ontologies.deleteTitle')),
  });

  const columns: Column<Ontology>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (ontology) => ontology.name,
      searchValue: (ontology) => `${ontology.name} ${ontology.description}`,
      cell: (ontology) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{ontology.name}</p>
          {ontology.description ? (
            <p className="truncate text-xs text-muted-foreground">{ontology.description}</p>
          ) : null}
        </div>
      ),
    },
    {
      id: 'source',
      header: t('ontologies.source'),
      cell: (ontology) => (
        <span className="flex items-center gap-1.5">
          <Badge tone="outline">{ontology.sourceType || '—'}</Badge>
          <span className="truncate text-xs text-muted-foreground">{ontology.sourceName}</span>
        </span>
      ),
    },
    {
      id: 'profile',
      header: t('ontologies.profile'),
      cell: (ontology) => (
        <span className="text-xs text-muted-foreground">{ontology.inferenceProfile || '—'}</span>
      ),
    },
    {
      id: 'status',
      header: t('common.status'),
      sortValue: (ontology) => ontology.status,
      cell: (ontology) => <StatusBadge status={ontology.status} locale={locale} />,
    },
    {
      id: 'updatedAt',
      header: t('common.updatedAt'),
      sortValue: (ontology) => ontology.updatedAt,
      cell: (ontology) => (
        <span className="text-xs text-muted-foreground">
          {formatRelative(ontology.updatedAt, locale)}
        </span>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <SectionHeader title={t('ontologies.title')} description={t('ontologies.subtitle')} />

      <OntologyScan />

      <Card>
        <DataTable
          data={ontologies.data}
          columns={columns}
          rowKey={(ontology) => ontology.id}
          isLoading={ontologies.isLoading}
          isError={ontologies.isError}
          error={ontologies.error}
          onRetry={() => void ontologies.refetch()}
          onRowClick={(ontology) => setSelectedId(ontology.id)}
          defaultSort={{ columnId: 'updatedAt', direction: 'desc' }}
          emptyState={
            <EmptyState
              icon={Network}
              title={t('ontologies.empty')}
              description={t('ontologies.emptyHint')}
            />
          }
          rowActions={(ontology) => (
            <>
              <DropdownMenuItem onSelect={() => setSelectedId(ontology.id)}>
                <Search aria-hidden />
                {t('ontologies.query')}
              </DropdownMenuItem>
              <DropdownMenuItem
                destructive
                onSelect={() =>
                  ask({
                    title: t('ontologies.deleteTitle'),
                    description: t('ontologies.deleteWarning'),
                    confirmLabel: t('common.delete'),
                    destructive: true,
                    onConfirm: () => remove.mutateAsync(ontology.id),
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

      {selected ? <OntologyQuery ontology={selected} /> : null}
      {selected ? <OntologyCoverage ontologyId={selected.id} /> : null}
      {selected ? <OntologyCohorts ontology={selected} /> : null}
      {dialog}
    </div>
  );
}
