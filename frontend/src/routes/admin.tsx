import * as React from 'react';
import { useNavigate, useParams } from 'react-router';
import { useMutation } from '@tanstack/react-query';
import {
  Activity,
  Boxes,
  Check,
  Cpu,
  Database,
  Download,
  HardDrive,
  MemoryStick,
  Play,
  Plus,
  ScrollText,
  ShieldCheck,
  Square,
  Trash2,
  Users,
  X,
} from 'lucide-react';
import { PageHeader, SectionHeader } from '@/components/common/page-header';
import { Stat, StatGrid } from '@/components/common/stat';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { SearchInput } from '@/components/common/search-input';
import { useConfirm } from '@/components/common/confirm-dialog';
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
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import { useToast } from '@/components/ui/toast';
import {
  useAdminExecutions,
  useAdminOrganizations,
  useAdminOverview,
  useAdminUsers,
  useAuditEvents,
  useBackupRuns,
  useBackupStatus,
  useDataUsage,
  useEgressRules,
  useOrganizationMembers,
  useProjects,
  useRbacMatrix,
  useStorageEndpoints,
  qk,
  useInvalidate,
} from '@/lib/api/queries';
import { adminApi, egressApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatBytes, formatCpu, formatDateTime, formatNumber, formatRelative } from '@/lib/format';
import { isEnterprise } from '@/lib/config';
import { useExtensions } from '@/lib/extensions';
import { ExtensionSlot } from '@/components/common/extension-slot';
import { useAuth } from '@/lib/auth';
import type {
  AuditEvent,
  DataUsageEdge,
  EgressRule,
  Execution,
  Organization,
  PlatformUser,
  RbacMatrixEntry,
  StorageEndpoint,
  BackupRun,
} from '@/lib/api/types';

const SECTIONS = [
  'overview',
  'identity',
  'activity',
  'data',
  'network',
  'rbac',
  'storage',
  'backups',
  'audit',
] as const;

/* -- overview -------------------------------------------------------------- */

function OverviewSection() {
  const t = useT();
  const { locale } = useI18n();
  const overview = useAdminOverview();
  const projects = useProjects();

  return (
    <div className="space-y-4">
      <SectionHeader title={t('admin.overview')} description={t('admin.subtitle')} />
      <StatGrid>
        <Stat
          icon={Users}
          label={t('home.users')}
          loading={overview.isLoading}
          value={formatNumber(overview.data?.users, locale)}
        />
        <Stat
          icon={Boxes}
          label={t('home.projects')}
          loading={overview.isLoading || projects.isLoading}
          value={formatNumber(overview.data?.projects ?? projects.data?.length, locale)}
        />
        <Stat
          icon={Cpu}
          label={t('activity.cpuReserved')}
          loading={overview.isLoading}
          value={formatCpu(overview.data?.cpuMillicores, locale)}
        />
        <Stat
          icon={MemoryStick}
          label={t('activity.memoryReserved')}
          loading={overview.isLoading}
          value={formatBytes(overview.data?.memoryBytes, locale)}
        />
      </StatGrid>
    </div>
  );
}

/* -- identity -------------------------------------------------------------- */

function IdentitySection() {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();

  const users = useAdminUsers();
  const organizations = useAdminOrganizations();
  const [selectedOrganizationId, setSelectedOrganizationId] = React.useState<string | null>(null);
  const members = useOrganizationMembers(selectedOrganizationId ?? undefined);

  const [search, setSearch] = React.useState('');
  const [organizationName, setOrganizationName] = React.useState('');
  const [organizationAlias, setOrganizationAlias] = React.useState('');
  const [memberToAdd, setMemberToAdd] = React.useState('');

  const createOrganization = useMutation({
    mutationFn: () =>
      adminApi.createOrganization({ name: organizationName.trim(), alias: organizationAlias.trim() }),
    onSuccess: () => {
      invalidate(qk.adminOrganizations, qk.organizations);
      setOrganizationName('');
      setOrganizationAlias('');
    },
    onError: (error) => toast.error(error, t('rbac.createOrganization')),
  });

  const removeOrganization = useMutation({
    mutationFn: (organizationId: string) => adminApi.removeOrganization(organizationId),
    onSuccess: () => {
      invalidate(qk.adminOrganizations, qk.organizations);
      setSelectedOrganizationId(null);
    },
    onError: (error) => toast.error(error, t('common.delete')),
  });

  const addMember = useMutation({
    mutationFn: () =>
      adminApi.addOrganizationMember(selectedOrganizationId as string, memberToAdd),
    onSuccess: () => {
      invalidate(qk.adminOrganizationMembers(selectedOrganizationId ?? ''));
      setMemberToAdd('');
    },
    onError: (error) => toast.error(error, t('rbac.addMember')),
  });

  const removeMember = useMutation({
    mutationFn: (userId: string) =>
      adminApi.removeOrganizationMember(selectedOrganizationId as string, userId),
    onSuccess: () => invalidate(qk.adminOrganizationMembers(selectedOrganizationId ?? '')),
    onError: (error) => toast.error(error, t('common.delete')),
  });

  const userColumns: Column<PlatformUser>[] = [
    {
      id: 'user',
      header: t('common.user'),
      sortValue: (user) => user.username ?? user.id,
      searchValue: (user) => `${user.username ?? ''} ${user.email ?? ''} ${user.id}`,
      cell: (user) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{user.username ?? user.id}</p>
          {[user.firstName, user.lastName].filter(Boolean).length > 0 ? (
            <p className="truncate text-xs text-muted-foreground">
              {[user.firstName, user.lastName].filter(Boolean).join(' ')}
            </p>
          ) : null}
        </div>
      ),
    },
    {
      id: 'email',
      header: 'Email',
      sortValue: (user) => user.email ?? null,
      cell: (user) => <span className="truncate text-xs text-muted-foreground">{user.email ?? '—'}</span>,
    },
    {
      id: 'status',
      header: t('common.status'),
      cell: (user) =>
        user.enabled === false ? (
          <Badge tone="danger">{t('common.no')}</Badge>
        ) : (
          <Badge tone="success">{t('common.yes')}</Badge>
        ),
    },
  ];

  const organizationColumns: Column<Organization>[] = [
    {
      id: 'name',
      header: t('common.organization'),
      sortValue: (organization) => organization.name,
      cell: (organization) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{organization.name}</p>
          {organization.alias ? (
            <p className="truncate font-mono text-xs text-muted-foreground">{organization.alias}</p>
          ) : null}
        </div>
      ),
    },
  ];

  return (
    <div className="space-y-5">
      <SectionHeader
        title={t('admin.users')}
        description={t('admin.usersHint')}
        actions={
          <SearchInput
            value={search}
            onValueChange={setSearch}
            label={t('common.search')}
            className="w-56"
          />
        }
      />
      <Card>
        <DataTable
          data={users.data}
          columns={userColumns}
          rowKey={(user) => user.id}
          isLoading={users.isLoading}
          isError={users.isError}
          error={users.error}
          onRetry={() => void users.refetch()}
          search={search}
          onResetSearch={() => setSearch('')}
          defaultSort={{ columnId: 'user', direction: 'asc' }}
          emptyState={<EmptyState icon={Users} title={t('admin.users')} description={t('admin.usersHint')} />}
        />
      </Card>

      <SectionHeader title={t('rbac.organizations')} description={t('rbac.organizationsHint')} />
      <div className="grid gap-4 lg:grid-cols-2">
        <Card>
          <CardHeader>
            <CardHeaderText>
              <CardTitle>{t('rbac.createOrganization')}</CardTitle>
            </CardHeaderText>
          </CardHeader>
          <CardContent className="grid gap-3 sm:grid-cols-2">
            <Field label={t('rbac.organizationNameLabel')} required>
              <Input
                value={organizationName}
                onChange={(event) => setOrganizationName(event.target.value)}
              />
            </Field>
            <Field label={t('rbac.organizationAliasLabel')} description={t('rbac.organizationAliasHint')}>
              <Input
                value={organizationAlias}
                onChange={(event) => setOrganizationAlias(event.target.value.toLowerCase())}
                className="font-mono text-xs"
              />
            </Field>
          </CardContent>
          <CardFooter className="justify-end">
            <Button
              variant="primary"
              disabled={!organizationName.trim()}
              loading={createOrganization.isPending}
              onClick={() => createOrganization.mutate()}
            >
              <Plus aria-hidden />
              {t('common.create')}
            </Button>
          </CardFooter>
          <DataTable
            data={organizations.data}
            columns={organizationColumns}
            rowKey={(organization) => organization.id}
            isLoading={organizations.isLoading}
            isError={organizations.isError}
            error={organizations.error}
            onRetry={() => void organizations.refetch()}
            onRowClick={(organization) => setSelectedOrganizationId(organization.id)}
            emptyState={
              <EmptyState
                compact
                title={t('rbac.organizations')}
                description={t('rbac.organizationsHint')}
              />
            }
            rowActions={(organization) => (
              <DropdownMenuItem
                destructive
                onSelect={() =>
                  ask({
                    title: t('common.delete'),
                    description: t('rbac.organizationsHint'),
                    confirmLabel: t('common.delete'),
                    destructive: true,
                    confirmationValue: organization.name,
                    onConfirm: () => removeOrganization.mutateAsync(organization.id),
                  })
                }
              >
                <Trash2 aria-hidden />
                {t('common.delete')}
              </DropdownMenuItem>
            )}
          />
        </Card>

        <Card>
          <CardHeader>
            <CardHeaderText>
              <CardTitle>{t('rbac.members')}</CardTitle>
              <CardDescription>
                {selectedOrganizationId
                  ? organizations.data?.find((item) => item.id === selectedOrganizationId)?.name
                  : t('rbac.noOrganizationSelected')}
              </CardDescription>
            </CardHeaderText>
          </CardHeader>
          {selectedOrganizationId ? (
            <>
              <CardContent className="flex items-end gap-2">
                <Field label={t('rbac.addMember')} className="flex-1">
                  <Select
                    value={memberToAdd}
                    onValueChange={setMemberToAdd}
                    placeholder={t('common.search')}
                    options={(users.data ?? []).map((user) => ({
                      value: user.username ?? user.id,
                      label: user.username ?? user.id,
                      hint: user.email ?? undefined,
                    }))}
                  />
                </Field>
                <Button
                  variant="primary"
                  disabled={!memberToAdd}
                  loading={addMember.isPending}
                  onClick={() => addMember.mutate()}
                >
                  {t('common.add')}
                </Button>
              </CardContent>
              <DataTable
                data={members.data}
                columns={[
                  {
                    id: 'member',
                    header: t('common.user'),
                    cell: (member) => (
                      <span className="font-medium">{member.username ?? member.userId}</span>
                    ),
                  },
                ]}
                rowKey={(member) => member.userId}
                isLoading={members.isLoading}
                isError={members.isError}
                error={members.error}
                onRetry={() => void members.refetch()}
                emptyState={<EmptyState compact title={t('members.empty')} />}
                rowActions={(member) => (
                  <DropdownMenuItem destructive onSelect={() => removeMember.mutate(member.userId)}>
                    <Trash2 aria-hidden />
                    {t('common.delete')}
                  </DropdownMenuItem>
                )}
              />
            </>
          ) : (
            <EmptyState compact title={t('rbac.noOrganizationSelected')} />
          )}
        </Card>
      </div>

      {dialog}
    </div>
  );
}

/* -- activity -------------------------------------------------------------- */

function ActivitySection() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();

  const executions = useAdminExecutions();
  const overview = useAdminOverview();
  const projects = useProjects();

  const stop = useMutation({
    mutationFn: (execution: Execution) => adminApi.killExecution(execution.kind, execution.id),
    onSuccess: () => invalidate(qk.adminExecutions, qk.adminOverview),
    onError: (error) => toast.error(error, t('activity.stopTitle')),
  });

  const columns: Column<Execution>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (execution) => execution.name,
      searchValue: (execution) => execution.name,
      cell: (execution) => <span className="truncate font-medium">{execution.name}</span>,
    },
    {
      id: 'kind',
      header: t('activity.kind'),
      sortValue: (execution) => execution.kind,
      cell: (execution) => <Badge tone="outline">{execution.kind}</Badge>,
    },
    {
      id: 'project',
      header: t('common.project'),
      cell: (execution) => (
        <span className="truncate text-xs text-muted-foreground">
          {projects.data?.find((project) => project.id === execution.projectId)?.name ??
            execution.projectId}
        </span>
      ),
    },
    {
      id: 'owner',
      header: t('common.owner'),
      cell: (execution) => (
        <span className="text-xs text-muted-foreground">{execution.ownerUserId ?? '—'}</span>
      ),
    },
    {
      id: 'status',
      header: t('common.status'),
      cell: (execution) => <StatusBadge status={execution.status} locale={locale} />,
    },
    {
      id: 'createdAt',
      header: t('common.startedAt'),
      sortValue: (execution) => execution.createdAt,
      cell: (execution) => (
        <span className="text-xs text-muted-foreground">
          {formatRelative(execution.createdAt, locale)}
        </span>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <SectionHeader title={t('activity.title')} description={t('activity.subtitle')} />
      <StatGrid>
        <Stat
          icon={Activity}
          label={t('activity.executions')}
          loading={executions.isLoading}
          value={executions.data?.length ?? 0}
        />
        <Stat
          icon={Boxes}
          label={t('activity.pods')}
          loading={overview.isLoading}
          value={formatNumber(overview.data?.pods, locale)}
        />
        <Stat
          icon={Cpu}
          label={t('activity.cpuReserved')}
          loading={overview.isLoading}
          value={formatCpu(overview.data?.cpuMillicores, locale)}
        />
        <Stat
          icon={MemoryStick}
          label={t('activity.memoryReserved')}
          loading={overview.isLoading}
          value={formatBytes(overview.data?.memoryBytes, locale)}
        />
      </StatGrid>
      <Card>
        <DataTable
          data={executions.data}
          columns={columns}
          rowKey={(execution) => `${execution.kind}:${execution.id}`}
          isLoading={executions.isLoading}
          isError={executions.isError}
          error={executions.error}
          onRetry={() => void executions.refetch()}
          defaultSort={{ columnId: 'createdAt', direction: 'desc' }}
          emptyState={
            <EmptyState icon={Activity} title={t('activity.empty')} description={t('activity.emptyHint')} />
          }
          rowActions={(execution) => (
            <DropdownMenuItem
              destructive
              onSelect={() =>
                ask({
                  title: t('activity.stopTitle'),
                  description: t('activity.stopWarning'),
                  confirmLabel: t('workspaces.stop'),
                  destructive: true,
                  onConfirm: () => stop.mutateAsync(execution),
                })
              }
            >
              <Square aria-hidden />
              {t('workspaces.stop')}
            </DropdownMenuItem>
          )}
        />
      </Card>
      {dialog}
    </div>
  );
}

/* -- data governance ------------------------------------------------------- */

function DataGovernanceSection() {
  const t = useT();
  const toast = useToast();
  const usage = useDataUsage();
  const [search, setSearch] = React.useState('');

  const columns: Column<DataUsageEdge>[] = [
    {
      id: 'dataset',
      header: t('nav.datasets'),
      sortValue: (edge) => edge.datasetName,
      searchValue: (edge) => `${edge.datasetName} ${edge.targetName} ${edge.projectName ?? ''}`,
      cell: (edge) => <span className="truncate font-medium">{edge.datasetName}</span>,
    },
    {
      id: 'relation',
      header: t('rbac.matrix'),
      sortValue: (edge) => edge.relation,
      cell: (edge) => <Badge tone="outline">{edge.relation}</Badge>,
    },
    {
      id: 'target',
      header: t('rbac.subject'),
      cell: (edge) => (
        <span className="truncate text-xs text-muted-foreground">
          {edge.targetType} · {edge.targetName || edge.targetId}
        </span>
      ),
    },
    {
      id: 'project',
      header: t('common.project'),
      cell: (edge) => (
        <span className="truncate text-xs text-muted-foreground">{edge.projectName ?? '—'}</span>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <SectionHeader
        title={t('admin.dataUsage')}
        description={t('admin.dataUsageHint')}
        actions={
          <div className="flex items-center gap-2">
            <SearchInput
              value={search}
              onValueChange={setSearch}
              label={t('common.search')}
              className="w-56"
            />
            <Button
              variant="secondary"
              onClick={() =>
                void adminApi
                  .downloadDataUsage()
                  .catch((error: unknown) => toast.error(error, t('admin.exportCsv')))
              }
            >
              <Download aria-hidden />
              {t('admin.exportCsv')}
            </Button>
          </div>
        }
      />
      <Card>
        <DataTable
          data={usage.data}
          columns={columns}
          rowKey={(edge, ) => `${edge.datasetId}:${edge.relation}:${edge.targetId}:${edge.projectId ?? ''}`}
          isLoading={usage.isLoading}
          isError={usage.isError}
          error={usage.error}
          onRetry={() => void usage.refetch()}
          search={search}
          onResetSearch={() => setSearch('')}
          emptyState={
            <EmptyState icon={Database} title={t('admin.dataUsage')} description={t('admin.dataUsageHint')} />
          }
        />
      </Card>
    </div>
  );
}

/* -- network --------------------------------------------------------------- */

function NetworkSection() {
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

/* -- rbac ------------------------------------------------------------------ */

function RbacSection() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const matrix = useRbacMatrix();
  const [search, setSearch] = React.useState('');

  const columns: Column<RbacMatrixEntry>[] = [
    {
      id: 'subject',
      header: t('rbac.subject'),
      sortValue: (entry) => entry.subjectId,
      searchValue: (entry) => `${entry.subjectId} ${entry.resourceName ?? entry.resourceId}`,
      cell: (entry) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{entry.subjectId}</p>
          <p className="text-xs text-muted-foreground">{entry.subjectType}</p>
        </div>
      ),
    },
    {
      id: 'resource',
      header: t('rbac.resource'),
      sortValue: (entry) => entry.resourceName ?? entry.resourceId,
      cell: (entry) => (
        <div className="min-w-0">
          <p className="truncate">{entry.resourceName ?? entry.resourceId}</p>
          <p className="text-xs text-muted-foreground">{entry.resourceType}</p>
        </div>
      ),
    },
    {
      id: 'role',
      header: t('common.role'),
      sortValue: (entry) => entry.role,
      cell: (entry) => <Badge tone="brand">{entry.role}</Badge>,
    },
  ];

  return (
    <div className="space-y-4">
      <SectionHeader
        title={t('rbac.matrix')}
        description={t('rbac.matrixHint')}
        actions={
          <div className="flex items-center gap-2">
            <SearchInput
              value={search}
              onValueChange={setSearch}
              label={t('common.search')}
              className="w-56"
            />
            <Button
              variant="secondary"
              onClick={() =>
                void adminApi
                  .downloadRbacMatrix()
                  .catch((error: unknown) => toast.error(error, t('admin.exportCsv')))
              }
            >
              <Download aria-hidden />
              {t('admin.exportCsv')}
            </Button>
          </div>
        }
      />
      <Card>
        <DataTable
          data={matrix.data}
          columns={columns}
          rowKey={(entry) =>
            `${entry.subjectType}:${entry.subjectId}:${entry.resourceType}:${entry.resourceId}`
          }
          isLoading={matrix.isLoading}
          isError={matrix.isError}
          error={matrix.error}
          onRetry={() => void matrix.refetch()}
          search={search}
          onResetSearch={() => setSearch('')}
          emptyState={<EmptyState title={t('rbac.empty')} description={t('rbac.emptyHint')} />}
        />
      </Card>
      <p className="sr-only">{locale}</p>
    </div>
  );
}

/* -- storage endpoints ----------------------------------------------------- */

function StorageSection() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();
  const endpoints = useStorageEndpoints();

  const test = useMutation({
    mutationFn: (endpointId: string) => adminApi.testStorageEndpoint(endpointId),
    onSuccess: (result) => {
      invalidate(qk.adminStorageEndpoints);
      if (result.reachable) toast.success(t('datasources.testOk'), t('admin.testEndpoint'));
      else toast.error(result.error ?? t('datasources.testFailed'), t('admin.testEndpoint'));
    },
    onError: (error) => toast.error(error, t('admin.testEndpoint')),
  });

  const remove = useMutation({
    mutationFn: (endpointId: string) => adminApi.removeStorageEndpoint(endpointId),
    onSuccess: () => invalidate(qk.adminStorageEndpoints),
    onError: (error) => toast.error(error, t('common.delete')),
  });

  const columns: Column<StorageEndpoint>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (endpoint) => endpoint.name,
      cell: (endpoint) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{endpoint.name}</p>
          <p className="truncate font-mono text-xs text-muted-foreground">{endpoint.endpoint}</p>
        </div>
      ),
    },
    {
      id: 'purpose',
      header: t('common.type'),
      cell: (endpoint) => (
        <div className="flex flex-wrap gap-1.5">
          <Badge tone="outline">{endpoint.provider}</Badge>
          {endpoint.defaultDataset ? <Badge tone="brand">{t('nav.datasets')}</Badge> : null}
          {endpoint.defaultBackup ? <Badge tone="brand">{t('nav.backups')}</Badge> : null}
        </div>
      ),
    },
    {
      id: 'classification',
      header: t('datasets.classification'),
      cell: (endpoint) =>
        endpoint.classification === 'hds' ? (
          <Badge tone="warning">{t('datasets.classificationHds')}</Badge>
        ) : (
          <Badge tone="outline">{t('datasets.classificationStandard')}</Badge>
        ),
    },
    {
      id: 'status',
      header: t('common.status'),
      cell: (endpoint) => <StatusBadge status={endpoint.status} locale={locale} />,
    },
    {
      id: 'checked',
      header: t('repositories.lastValidated'),
      sortValue: (endpoint) => endpoint.lastCheckedAt ?? null,
      cell: (endpoint) => (
        <span className="text-xs text-muted-foreground">
          {endpoint.lastCheckedAt ? formatRelative(endpoint.lastCheckedAt, locale) : '—'}
        </span>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <SectionHeader title={t('admin.storageEndpoints')} description={t('admin.storageEndpointsHint')} />
      <Card>
        <DataTable
          data={endpoints.data}
          columns={columns}
          rowKey={(endpoint) => endpoint.id}
          isLoading={endpoints.isLoading}
          isError={endpoints.isError}
          error={endpoints.error}
          onRetry={() => void endpoints.refetch()}
          emptyState={
            <EmptyState
              icon={HardDrive}
              title={t('admin.storageEndpoints')}
              description={t('admin.storageEndpointsHint')}
            />
          }
          rowActions={(endpoint) => (
            <>
              <DropdownMenuItem onSelect={() => test.mutate(endpoint.id)}>
                <ShieldCheck aria-hidden />
                {t('admin.testEndpoint')}
              </DropdownMenuItem>
              <DropdownMenuItem
                destructive
                onSelect={() =>
                  ask({
                    title: t('common.delete'),
                    description: t('admin.storageEndpointsHint'),
                    confirmLabel: t('common.delete'),
                    destructive: true,
                    confirmationValue: endpoint.name,
                    onConfirm: () => remove.mutateAsync(endpoint.id),
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
      {dialog}
    </div>
  );
}

/* -- backups --------------------------------------------------------------- */

function BackupsSection() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();

  const status = useBackupStatus();
  const runs = useBackupRuns();

  const [endpoint, setEndpoint] = React.useState('');
  const [bucket, setBucket] = React.useState('');
  const [prefix, setPrefix] = React.useState('');
  const [region, setRegion] = React.useState('');
  const [accessKey, setAccessKey] = React.useState('');
  const [secretKey, setSecretKey] = React.useState('');
  const [encryptionKeyId, setEncryptionKeyId] = React.useState('');

  React.useEffect(() => {
    if (!status.data) return;
    setEndpoint(status.data.endpoint ?? '');
    setBucket(status.data.bucket ?? '');
    setPrefix(status.data.prefix ?? '');
    setRegion(status.data.region ?? '');
  }, [status.data]);

  const save = useMutation({
    mutationFn: () =>
      adminApi.saveBackupConfig({
        endpoint: endpoint.trim(),
        bucket: bucket.trim(),
        prefix: prefix.trim(),
        region: region.trim(),
        accessKey,
        secretKey,
        encryptionKeyId: encryptionKeyId.trim(),
      }),
    onSuccess: () => {
      invalidate(qk.adminBackupStatus);
      setAccessKey('');
      setSecretKey('');
      toast.success(t('common.save'), t('admin.backups'));
    },
    onError: (error) => toast.error(error, t('admin.backupConfigure')),
  });

  const run = useMutation({
    mutationFn: () => adminApi.runBackup(),
    onSuccess: () => {
      invalidate(qk.adminBackupRuns);
      toast.success(t('admin.backupRun'), t('admin.backups'));
    },
    onError: (error) => toast.error(error, t('admin.backupRun')),
  });

  const columns: Column<BackupRun>[] = [
    {
      id: 'startedAt',
      header: t('common.startedAt'),
      sortValue: (backup) => backup.startedAt,
      cell: (backup) => (
        <span className="text-xs text-muted-foreground">{formatDateTime(backup.startedAt, locale)}</span>
      ),
    },
    {
      id: 'status',
      header: t('common.status'),
      cell: (backup) => <StatusBadge status={backup.status} locale={locale} />,
    },
    {
      id: 'object',
      header: t('admin.bucket'),
      cell: (backup) => (
        <span className="truncate font-mono text-xs text-muted-foreground">
          {backup.bucket}/{backup.objectKey}
        </span>
      ),
    },
    {
      id: 'error',
      header: t('common.error'),
      cell: (backup) =>
        backup.error ? (
          <span className="line-clamp-2 max-w-xs text-xs text-danger">{backup.error}</span>
        ) : (
          <span className="text-xs text-muted-foreground">—</span>
        ),
    },
  ];

  const configured = status.data?.configured === true;

  return (
    <div className="space-y-4">
      <SectionHeader
        title={t('admin.backups')}
        description={t('admin.backupsHint')}
        actions={
          <Button
            variant="primary"
            disabled={!configured}
            loading={run.isPending}
            onClick={() => run.mutate()}
          >
            <Play aria-hidden />
            {t('admin.backupRun')}
          </Button>
        }
      />

      <StatGrid className="lg:grid-cols-3">
        <Stat
          label={t('common.status')}
          loading={status.isLoading}
          value={configured ? t('admin.backupConfigured') : t('admin.backupNotConfigured')}
        />
        <Stat label={t('admin.bucket')} loading={status.isLoading} value={status.data?.bucket ?? '—'} />
        <Stat
          label={t('admin.lastUpdated')}
          loading={status.isLoading}
          value={status.data?.updatedAt ? formatRelative(status.data.updatedAt, locale) : '—'}
        />
      </StatGrid>

      <Card>
        <CardHeader>
          <CardHeaderText>
            <CardTitle>{t('admin.backupConfigure')}</CardTitle>
            <CardDescription>{t('datasets.credentialsHint')}</CardDescription>
          </CardHeaderText>
        </CardHeader>
        <CardContent className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <Field label={t('admin.endpoint')} className="sm:col-span-2 lg:col-span-3" required>
            <Input
              value={endpoint}
              onChange={(event) => setEndpoint(event.target.value)}
              placeholder="https://s3.example.com"
              className="font-mono text-xs"
            />
          </Field>
          <Field label={t('admin.bucket')} required>
            <Input value={bucket} onChange={(event) => setBucket(event.target.value)} />
          </Field>
          <Field label={t('admin.prefix')}>
            <Input value={prefix} onChange={(event) => setPrefix(event.target.value)} />
          </Field>
          <Field label={t('admin.region')}>
            <Input value={region} onChange={(event) => setRegion(event.target.value)} />
          </Field>
          <Field label={t('admin.accessKey')}>
            <Input
              value={accessKey}
              onChange={(event) => setAccessKey(event.target.value)}
              autoComplete="off"
            />
          </Field>
          <Field label={t('admin.secretKey')} description={t('datasets.credentialsHint')}>
            <Input
              type="password"
              value={secretKey}
              onChange={(event) => setSecretKey(event.target.value)}
              autoComplete="new-password"
            />
          </Field>
          <Field label={t('admin.encryptionKey')}>
            <Input
              value={encryptionKeyId}
              onChange={(event) => setEncryptionKeyId(event.target.value)}
            />
          </Field>
        </CardContent>
        <CardFooter className="justify-end">
          <Button
            variant="primary"
            loading={save.isPending}
            disabled={!endpoint.trim() || !bucket.trim()}
            onClick={() => save.mutate()}
          >
            {t('common.save')}
          </Button>
        </CardFooter>
      </Card>

      <Card>
        <CardHeader>
          <CardHeaderText>
            <CardTitle>{t('admin.backupHistory')}</CardTitle>
          </CardHeaderText>
        </CardHeader>
        <DataTable
          data={runs.data}
          columns={columns}
          rowKey={(backup) => backup.id}
          isLoading={runs.isLoading}
          isError={runs.isError}
          error={runs.error}
          onRetry={() => void runs.refetch()}
          defaultSort={{ columnId: 'startedAt', direction: 'desc' }}
          emptyState={
            <EmptyState
              icon={HardDrive}
              title={t('admin.backupEmpty')}
              description={t('admin.backupEmptyHint')}
            />
          }
        />
      </Card>
    </div>
  );
}

/* -- audit ----------------------------------------------------------------- */

function AuditSection() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const events = useAuditEvents();
  const [search, setSearch] = React.useState('');

  const columns: Column<AuditEvent>[] = [
    {
      id: 'occurredAt',
      header: t('admin.occurredAt'),
      sortValue: (event) => event.occurredAt,
      cell: (event) => (
        <span className="whitespace-nowrap text-xs text-muted-foreground">
          {formatDateTime(event.occurredAt, locale)}
        </span>
      ),
    },
    {
      id: 'actor',
      header: t('admin.actor'),
      sortValue: (event) => event.actorUserId,
      searchValue: (event) => `${event.actorUserId} ${event.action} ${event.resourceType}`,
      cell: (event) => <span className="truncate font-medium">{event.actorUserId || '—'}</span>,
    },
    {
      id: 'action',
      header: t('admin.action'),
      sortValue: (event) => event.action,
      cell: (event) => <Badge tone="outline">{event.action}</Badge>,
    },
    {
      id: 'resource',
      header: t('rbac.resource'),
      cell: (event) => (
        <span className="truncate text-xs text-muted-foreground">
          {event.resourceType}
          {event.resourceId ? ` · ${event.resourceId.slice(0, 12)}` : ''}
        </span>
      ),
    },
    {
      id: 'outcome',
      header: t('admin.outcome'),
      sortValue: (event) => event.outcome,
      cell: (event) => (
        <Badge tone={event.outcome?.toLowerCase() === 'success' ? 'success' : 'danger'}>
          {event.outcome || '—'}
        </Badge>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <SectionHeader
        title={t('admin.audit')}
        description={t('admin.auditHint')}
        actions={
          <div className="flex items-center gap-2">
            <SearchInput
              value={search}
              onValueChange={setSearch}
              label={t('common.search')}
              className="w-56"
            />
            <Button
              variant="secondary"
              onClick={() =>
                void adminApi
                  .downloadAudit()
                  .catch((error: unknown) => toast.error(error, t('admin.exportCsv')))
              }
            >
              <Download aria-hidden />
              {t('admin.exportCsv')}
            </Button>
          </div>
        }
      />
      <Card>
        <DataTable
          data={events.data}
          columns={columns}
          rowKey={(event) => event.id}
          isLoading={events.isLoading}
          isError={events.isError}
          error={events.error}
          onRetry={() => void events.refetch()}
          search={search}
          onResetSearch={() => setSearch('')}
          defaultSort={{ columnId: 'occurredAt', direction: 'desc' }}
          emptyState={
            <EmptyState
              icon={ScrollText}
              title={t('admin.auditEmpty')}
              description={t('admin.auditEmptyHint')}
            />
          }
        />
      </Card>
    </div>
  );
}

/* -- page ------------------------------------------------------------------ */

export function AdminPage() {
  const t = useT();
  const { locale } = useI18n();
  const navigate = useNavigate();
  const { section } = useParams<{ section?: string }>();
  const { isAdmin } = useAuth();

  // Enterprise modules add their own admin sections through the declared
  // extension point rather than by patching this file's markup.
  const extensions = useExtensions('admin.section');

  const known = new Set<string>([...SECTIONS, ...extensions.map((module) => module.id)]);
  const active: string = known.has(section ?? '') ? (section as string) : 'overview';

  if (!isAdmin) {
    return (
      <Card className="mx-auto max-w-lg">
        <EmptyState
          icon={ShieldCheck}
          title={t('common.accessDenied')}
          description={t('errors.noOrganizationHint')}
        />
      </Card>
    );
  }

  return (
    <div className="space-y-5">
      <PageHeader title={t('admin.title')} description={t('admin.subtitle')} />

      <Tabs value={active} onValueChange={(value) => navigate(`/admin/${value}`)}>
        <TabsList>
          <TabsTrigger value="overview">{t('admin.overview')}</TabsTrigger>
          <TabsTrigger value="identity">{t('nav.identity')}</TabsTrigger>
          <TabsTrigger value="activity">{t('nav.activity')}</TabsTrigger>
          <TabsTrigger value="network">{t('nav.network')}</TabsTrigger>
          <TabsTrigger value="rbac">{t('nav.rbac')}</TabsTrigger>
          <TabsTrigger value="storage">{t('nav.storage')}</TabsTrigger>
          <TabsTrigger value="audit">{t('nav.audit')}</TabsTrigger>
          {/* Data-usage mapping and platform backups are Enterprise modules
              (ADR-026), so the tabs only exist where the module is deployed
              instead of rendering a section the API will refuse. */}
          {isEnterprise() ? (
            <>
              <TabsTrigger value="data">{t('nav.dataGovernance')}</TabsTrigger>
              <TabsTrigger value="backups">{t('nav.backups')}</TabsTrigger>
            </>
          ) : null}
          {extensions.map((module) => (
            <TabsTrigger key={module.id} value={module.id}>
              {module.title[locale]}
            </TabsTrigger>
          ))}
        </TabsList>

        <TabsContent value="overview">
          <OverviewSection />
        </TabsContent>
        <TabsContent value="identity">
          <IdentitySection />
        </TabsContent>
        <TabsContent value="activity">
          <ActivitySection />
        </TabsContent>
        <TabsContent value="network">
          <NetworkSection />
        </TabsContent>
        <TabsContent value="rbac">
          <RbacSection />
        </TabsContent>
        <TabsContent value="storage">
          <StorageSection />
        </TabsContent>
        <TabsContent value="audit">
          <AuditSection />
        </TabsContent>
        {isEnterprise() ? (
          <>
            <TabsContent value="data">
              <DataGovernanceSection />
            </TabsContent>
            <TabsContent value="backups">
              <BackupsSection />
            </TabsContent>
          </>
        ) : null}
        {extensions.map((module) => (
          <TabsContent key={module.id} value={module.id}>
            <ExtensionSlot module={module} />
          </TabsContent>
        ))}
      </Tabs>
    </div>
  );
}
