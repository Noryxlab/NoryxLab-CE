import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { AlertTriangle, Network, Search, Trash2 } from 'lucide-react';
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
import { useOntologies, useOntologyFreshness, qk, useInvalidate } from '@/lib/api/queries';
import { ontologiesApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatNumber, formatRelative } from '@/lib/format';
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
      {dialog}
    </div>
  );
}
