import { useMutation } from '@tanstack/react-query';
import {
  Check,
  ShieldCheck,
  X,
} from 'lucide-react';
import { SectionHeader } from '@/components/common/page-header';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import {
  Card,
} from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import { useToast } from '@/components/ui/toast';
import {
  useEgressRules,
  useProjects,
  qk,
  useInvalidate,
} from '@/lib/api/queries';
import { egressApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatRelative } from '@/lib/format';
import type {
  EgressRule,
} from '@/lib/api/types';

export function NetworkSection() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const rules = useEgressRules();
  const projects = useProjects();

  const decide = useMutation({
    mutationFn: (input: { ruleId: string; status: string }) =>
      egressApi.decide(input.ruleId, { status: input.status }),
    onSuccess: () => invalidate(qk.egressRules),
    onError: (error) => toast.error(error, t('network.title')),
  });

  const columns: Column<EgressRule>[] = [
    {
      id: 'destination',
      header: t('network.destinationLabel'),
      sortValue: (rule) => rule.destination,
      searchValue: (rule) => `${rule.destination} ${rule.subjectId}`,
      cell: (rule) => (
        <div className="min-w-0">
          <p className="truncate font-mono text-xs font-medium">{rule.destination}</p>
          <p className="text-xs text-muted-foreground">
            {rule.protocol}
            {rule.port ? `:${rule.port}` : ''}
          </p>
        </div>
      ),
    },
    {
      id: 'subject',
      header: t('rbac.subject'),
      cell: (rule) => (
        <span className="truncate text-xs text-muted-foreground">
          {rule.subjectType} · {rule.subjectId}
        </span>
      ),
    },
    {
      id: 'project',
      header: t('common.project'),
      cell: (rule) => (
        <span className="truncate text-xs text-muted-foreground">
          {projects.data?.find((project) => project.id === rule.projectId)?.name ?? rule.projectId}
        </span>
      ),
    },
    {
      id: 'profile',
      header: t('network.profileLabel'),
      cell: (rule) => <Badge tone="outline">{rule.profile}</Badge>,
    },
    {
      id: 'justification',
      header: t('network.justificationLabel'),
      cell: (rule) => (
        <span className="line-clamp-2 max-w-xs text-xs text-muted-foreground">
          {rule.justification || '—'}
        </span>
      ),
    },
    {
      id: 'status',
      header: t('common.status'),
      sortValue: (rule) => rule.status,
      cell: (rule) => {
        const value = rule.status.toLowerCase();
        const tone = value === 'approved' ? 'success' : value === 'rejected' ? 'danger' : 'warning';
        const label =
          value === 'approved'
            ? t('network.approved')
            : value === 'rejected'
              ? t('network.rejected')
              : t('network.pending');
        return <Badge tone={tone}>{label}</Badge>;
      },
    },
    {
      id: 'createdAt',
      header: t('common.createdAt'),
      sortValue: (rule) => rule.createdAt,
      cell: (rule) => (
        <span className="text-xs text-muted-foreground">{formatRelative(rule.createdAt, locale)}</span>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <SectionHeader title={t('network.title')} description={t('network.subtitle')} />
      <Card>
        <DataTable
          data={rules.data}
          columns={columns}
          rowKey={(rule) => rule.id}
          isLoading={rules.isLoading}
          isError={rules.isError}
          error={rules.error}
          onRetry={() => void rules.refetch()}
          defaultSort={{ columnId: 'createdAt', direction: 'desc' }}
          emptyState={
            <EmptyState icon={ShieldCheck} title={t('network.empty')} description={t('network.emptyHint')} />
          }
          rowActions={(rule) => (
            <>
              <DropdownMenuItem
                onSelect={() => decide.mutate({ ruleId: rule.id, status: 'approved' })}
                disabled={rule.status.toLowerCase() === 'approved'}
              >
                <Check aria-hidden />
                {t('network.approve')}
              </DropdownMenuItem>
              <DropdownMenuItem
                destructive
                onSelect={() => decide.mutate({ ruleId: rule.id, status: 'rejected' })}
                disabled={rule.status.toLowerCase() === 'rejected'}
              >
                <X aria-hidden />
                {t('network.reject')}
              </DropdownMenuItem>
            </>
          )}
        />
      </Card>
    </div>
  );
}
