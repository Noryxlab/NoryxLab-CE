import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Network, Search, Trash2 } from 'lucide-react';
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
import { useOntologies, qk, useInvalidate } from '@/lib/api/queries';
import { ontologiesApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatRelative } from '@/lib/format';
import type { Ontology } from '@/lib/api/types';

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
  const [result, setResult] = React.useState<{ columns?: string[]; rows?: unknown[][] } | null>(null);

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

        {result?.rows?.length ? (
          <TableWrapper className="rounded-md border border-border">
            <Table>
              <TableHeader>
                <TableRow>
                  {(result.columns ?? []).map((column) => (
                    <TableHead key={column}>{column}</TableHead>
                  ))}
                </TableRow>
              </TableHeader>
              <TableBody>
                {result.rows.slice(0, 100).map((row, rowIndex) => (
                  <TableRow key={rowIndex}>
                    {row.map((cell, cellIndex) => (
                      <TableCell key={cellIndex} className="font-mono text-xs">
                        {cell === null || cell === undefined ? '—' : String(cell)}
                      </TableCell>
                    ))}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </TableWrapper>
        ) : null}
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
