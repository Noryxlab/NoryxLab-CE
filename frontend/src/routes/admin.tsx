import * as React from 'react';
import { useNavigate, useParams } from 'react-router';
import { useMutation } from '@tanstack/react-query';
import {
  Activity,
  AlertTriangle,
  Boxes,
  Check,
  Copy,
  Cpu,
  Database,
  Download,
  HardDrive,
  KeyRound,
  Mail,
  UserPlus,
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
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
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
  useSmtp,
} from '@/lib/api/queries';
import { adminApi, egressApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatBytes, formatCpu, formatDateTime, formatNumber, formatRelative } from '@/lib/format';
import { presentRole } from '@/lib/presenters';
import { isEnterprise } from '@/lib/config';
import { useExtensions } from '@/lib/extensions';
import { ExtensionSlot } from '@/components/common/extension-slot';
import { useAuth } from '@/lib/auth';
import { PlatformHealthPanel } from '@/features/admin/platform-health';
import { SoftwareInventorySection } from '@/features/admin/software-inventory';
import { AccessGraph } from '@/features/admin/access-graph';
import { HardwareTiersSection } from '@/features/admin/hardware-tiers';
import { SmtpSettingsSection } from '@/features/admin/smtp-settings';
import { PlatformSettingsSection } from '@/features/admin/platform-settings';
import type {
  AuditEvent,
  DataUsageEdge,
  DataUsageNode,
  EgressRule,
  Execution,
  Organization,
  PlatformUser,
  RbacCell,
  StorageEndpoint,
  BackupReport,
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
  'inventory',
  'backups',
  'audit',
  'settings',
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
      <PlatformHealthPanel enabled />
      <StatGrid>
        <Stat
          icon={Users}
          label={t('home.users')}
          loading={overview.isLoading}
          value={formatNumber(overview.data?.counts.users, locale)}
        />
        <Stat
          icon={Boxes}
          label={t('home.projects')}
          loading={overview.isLoading || projects.isLoading}
          value={formatNumber(overview.data?.counts.projects ?? projects.data?.length, locale)}
        />
        <Stat
          icon={Cpu}
          label={t('activity.cpuReserved')}
          loading={overview.isLoading}
          value={formatCpu(overview.data?.workloadMetrics.cpuRequestMillicores, locale)}
        />
        <Stat
          icon={MemoryStick}
          label={t('activity.memoryReserved')}
          loading={overview.isLoading}
          value={formatBytes(overview.data?.workloadMetrics.memoryRequestBytes, locale)}
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
  const [creatingUser, setCreatingUser] = React.useState(false);
  const [newUser, setNewUser] = React.useState({
    username: '', email: '', firstName: '', lastName: '', organizationId: '',
  });
  // Held in state and never refetched: the password exists in the creation
  // response and nowhere else, ever again.
  const [issuedPassword, setIssuedPassword] = React.useState<{ who: string; password: string } | null>(null);

  const createUser = useMutation({
    mutationFn: () => adminApi.createUser(newUser),
    onSuccess: (result) => {
      setIssuedPassword({ who: result.username ?? newUser.username, password: result.temporaryPassword });
      setNewUser({ username: '', email: '', firstName: '', lastName: '', organizationId: '' });
      setCreatingUser(false);
      invalidate(qk.adminUsers);
    },
    onError: (error) => toast.error(error, t('identity.createUser')),
  });

  const smtp = useSmtp();

  const resetByEmail = useMutation({
    mutationFn: (user: PlatformUser) => adminApi.sendPasswordResetEmail(user.id),
    onSuccess: () => toast.success(t('admin.resetByEmailSent'), t('identity.resetPassword')),
    onError: (error) => toast.error(error, t('admin.resetByEmail')),
  });

  const resetPassword = useMutation({
    mutationFn: (user: PlatformUser) => adminApi.resetUserPassword(user.id),
    onSuccess: (result, user) => {
      setIssuedPassword({ who: user.username || user.id, password: result.temporaryPassword });
    },
    onError: (error) => toast.error(error, t('identity.resetPassword')),
  });
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
      sortValue: (user) => user.username || user.id,
      searchValue: (user) => `${user.username} ${user.email} ${user.id}`,
      cell: (user) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{user.username || user.id}</p>
        </div>
      ),
    },
    {
      id: 'email',
      header: 'Email',
      sortValue: (user) => user.email || null,
      cell: (user) => <span className="truncate text-xs text-muted-foreground">{user.email || '—'}</span>,
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
    {
      id: 'actions',
      header: '',
      cell: (user) => (
        <div className="flex items-center justify-end gap-1">
        {/* Offered only when the realm can actually send: a button that fails
            at the click teaches an administrator to distrust the screen. */}
        {smtp.data?.configured ? (
          <Button
            variant="ghost"
            size="sm"
            loading={resetByEmail.isPending}
            onClick={() =>
              ask({
                title: t('admin.resetByEmail'),
                description: t('admin.resetByEmailHint', { user: user.username || user.id }),
                confirmLabel: t('admin.resetByEmail'),
                onConfirm: () => resetByEmail.mutateAsync(user),
              })
            }
          >
            <Mail aria-hidden />
            {t('admin.resetByEmail')}
          </Button>
        ) : null}
        <Button
          variant="ghost"
          size="sm"
          loading={resetPassword.isPending}
          onClick={() =>
            ask({
              title: t('identity.resetPassword'),
              description: t('identity.resetPasswordHint', { user: user.username || user.id }),
              confirmLabel: t('identity.resetPassword'),
              onConfirm: () => resetPassword.mutate(user),
            })
          }
        >
          <KeyRound aria-hidden />
          {t('identity.resetPassword')}
        </Button>
        </div>
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
          {organization.enabled ? null : (
            <Badge tone="danger">{t('common.no')}</Badge>
          )}
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
          <span className="flex items-center gap-2">
            <SearchInput
              value={search}
              onValueChange={setSearch}
              label={t('common.search')}
              className="w-56"
            />
            <Button variant="primary" onClick={() => setCreatingUser(true)}>
              <UserPlus aria-hidden />
              {t('identity.createUser')}
            </Button>
          </span>
        }
      />

      {/* The password exists here and nowhere else, ever again, so it stays on
          screen until dismissed rather than in a toast that vanishes. */}
      {issuedPassword ? (
        <Card className="border-brand/40 bg-brand-subtle/40">
          <CardHeader>
            <CardHeaderText>
              <CardTitle>{t('identity.passwordFor', { user: issuedPassword.who })}</CardTitle>
              <CardDescription>{t('identity.passwordHint')}</CardDescription>
            </CardHeaderText>
          </CardHeader>
          <CardContent className="space-y-2">
            <p className="break-all rounded-md bg-surface p-2 font-mono text-sm">{issuedPassword.password}</p>
            <div className="flex flex-wrap gap-2">
              <Button
                variant="secondary"
                size="sm"
                onClick={() => {
                  void navigator.clipboard?.writeText(issuedPassword.password);
                  toast.success(t('identity.passwordCopied'), t('identity.createUser'));
                }}
              >
                <Copy aria-hidden />
                {t('tokens.copy')}
              </Button>
              <Button variant="ghost" size="sm" onClick={() => setIssuedPassword(null)}>
                {t('tokens.dismiss')}
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : null}

      <Sheet open={creatingUser} onOpenChange={setCreatingUser}>
        <SheetContent aria-describedby={undefined}>
          <form
            className="flex min-h-0 flex-1 flex-col"
            onSubmit={(event) => {
              event.preventDefault();
              if (newUser.username.trim()) createUser.mutate();
            }}
          >
            <SheetHeader>
              <SheetTitle>{t('identity.createUser')}</SheetTitle>
              <SheetDescription>{t('identity.createUserHint')}</SheetDescription>
            </SheetHeader>
            <SheetBody>
              <Field label={t('identity.username')} required>
                <Input
                  value={newUser.username}
                  onChange={(event) => setNewUser({ ...newUser, username: event.target.value })}
                  autoFocus
                  maxLength={64}
                />
              </Field>
              <Field label="Email">
                <Input
                  type="email"
                  value={newUser.email}
                  onChange={(event) => setNewUser({ ...newUser, email: event.target.value })}
                />
              </Field>
              <div className="flex gap-2">
                <Field label={t('identity.firstName')} className="flex-1">
                  <Input
                    value={newUser.firstName}
                    onChange={(event) => setNewUser({ ...newUser, firstName: event.target.value })}
                  />
                </Field>
                <Field label={t('identity.lastName')} className="flex-1">
                  <Input
                    value={newUser.lastName}
                    onChange={(event) => setNewUser({ ...newUser, lastName: event.target.value })}
                  />
                </Field>
              </div>
              {/* Asked here rather than discovered later: on an installation
                  that requires membership, an account without an organization
                  signs in and can do nothing. */}
              <Field
                label={t('identity.organization')}
                description={t('identity.organizationHint')}
                required={isEnterprise()}
              >
                <Select
                  value={newUser.organizationId}
                  onValueChange={(value) => setNewUser({ ...newUser, organizationId: value })}
                  placeholder={t('identity.organizationPlaceholder')}
                  options={(organizations.data ?? []).map((organization) => ({
                    value: organization.id,
                    label: organization.name ?? organization.id,
                  }))}
                />
              </Field>
            </SheetBody>
            <SheetFooter>
              <Button variant="secondary" type="button" onClick={() => setCreatingUser(false)}>
                {t('common.cancel')}
              </Button>
              <Button
                type="submit"
                variant="primary"
                loading={createUser.isPending}
                disabled={!newUser.username.trim() || (isEnterprise() && !newUser.organizationId)}
              >
                {t('identity.createUser')}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

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
      sortValue: (execution) => execution.projectName || execution.projectId,
      cell: (execution) => (
        <span className="truncate text-xs text-muted-foreground">
          {execution.projectName || execution.projectId}
        </span>
      ),
    },
    {
      id: 'runtime',
      header: t('activity.pods'),
      cell: (execution) => (
        <span className="truncate font-mono text-xs text-muted-foreground">
          {execution.runtimeName || '—'}
        </span>
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
          value={formatNumber(overview.data?.workloadMetrics.pods, locale)}
        />
        <Stat
          icon={Cpu}
          label={t('activity.cpuReserved')}
          loading={overview.isLoading}
          value={formatCpu(overview.data?.workloadMetrics.cpuRequestMillicores, locale)}
        />
        <Stat
          icon={MemoryStick}
          label={t('activity.memoryReserved')}
          loading={overview.isLoading}
          value={formatBytes(overview.data?.workloadMetrics.memoryRequestBytes, locale)}
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
  const { locale } = useI18n();
  const toast = useToast();
  const usage = useDataUsage();
  const [search, setSearch] = React.useState('');

  // The endpoint returns a graph: edges reference nodes by id, so labels are
  // resolved here rather than repeated on every edge.
  const nodesById = React.useMemo(() => {
    const index = new Map<string, DataUsageNode>();
    for (const node of usage.data?.nodes ?? []) index.set(node.id, node);
    return index;
  }, [usage.data]);

  const label = React.useCallback(
    (id: string) => nodesById.get(id)?.label ?? id,
    [nodesById],
  );

  const columns: Column<DataUsageEdge>[] = [
    {
      id: 'from',
      header: t('rbac.subject'),
      sortValue: (edge) => label(edge.from),
      searchValue: (edge) => `${label(edge.from)} ${label(edge.to)}`,
      cell: (edge) => {
        const node = nodesById.get(edge.from);
        return (
          <div className="min-w-0">
            <p className="truncate font-medium">{node?.label ?? edge.from}</p>
            {node?.subLabel ? (
              <p className="truncate text-xs text-muted-foreground">{node.subLabel}</p>
            ) : null}
          </div>
        );
      },
    },
    {
      id: 'relation',
      header: t('rbac.matrix'),
      sortValue: (edge) => edge.relation,
      cell: (edge) => <Badge tone="outline">{edge.relation}</Badge>,
    },
    {
      id: 'to',
      header: t('rbac.resource'),
      sortValue: (edge) => label(edge.to),
      cell: (edge) => {
        const node = nodesById.get(edge.to);
        return (
          <div className="min-w-0">
            <p className="truncate">{node?.label ?? edge.to}</p>
            {node?.kind ? <p className="text-xs text-muted-foreground">{node.kind}</p> : null}
          </div>
        );
      },
    },
    {
      id: 'project',
      header: t('common.project'),
      cell: (edge) => (
        <span className="truncate text-xs text-muted-foreground">
          {edge.projectId ? label(edge.projectId) : '—'}
        </span>
      ),
    },
  ];

  const summary = usage.data?.summary;

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

      <StatGrid>
        <Stat
          icon={Database}
          label={t('nav.datasets')}
          loading={usage.isLoading}
          value={formatNumber(summary?.datasets, locale)}
          hint={
            summary?.hdsDatasets
              ? `${formatNumber(summary.hdsDatasets, locale)} ${t('datasets.classificationHds')}`
              : undefined
          }
        />
        <Stat
          icon={Boxes}
          label={t('home.projects')}
          loading={usage.isLoading}
          value={formatNumber(summary?.projects, locale)}
        />
        <Stat
          icon={Users}
          label={t('home.users')}
          loading={usage.isLoading}
          value={formatNumber(summary?.users, locale)}
          hint={
            summary?.organizations
              ? `${formatNumber(summary.organizations, locale)} ${t('rbac.organizations')}`
              : undefined
          }
        />
        <Stat
          icon={Activity}
          label={t('home.workloads')}
          loading={usage.isLoading}
          value={formatNumber(summary?.workloads, locale)}
        />
      </StatGrid>

      <Card>
        <DataTable
          data={usage.data?.edges}
          columns={columns}
          rowKey={(edge) => `${edge.from}:${edge.relation}:${edge.to}:${edge.projectId}`}
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

  // Cells are self-describing: subject, resource, role, and whether the grant
  // is direct or inherited from an organisation or an ownership.
  const columns: Column<RbacCell>[] = [
    {
      id: 'subject',
      header: t('rbac.subject'),
      sortValue: (cell) => cell.subjectName || cell.subjectId,
      searchValue: (cell) => `${cell.subjectName} ${cell.subjectId} ${cell.resourceName}`,
      cell: (cell) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{cell.subjectName || cell.subjectId}</p>
          <p className="text-xs text-muted-foreground">{cell.subjectType}</p>
        </div>
      ),
    },
    {
      id: 'resource',
      header: t('rbac.resource'),
      sortValue: (cell) => cell.resourceName || cell.resourceId,
      cell: (cell) => (
        <div className="min-w-0">
          <p className="truncate">{cell.resourceName || cell.resourceId}</p>
          <p className="text-xs text-muted-foreground">{cell.resourceType}</p>
        </div>
      ),
    },
    {
      id: 'role',
      header: t('common.role'),
      sortValue: (cell) => cell.role,
      cell: (cell) => <Badge tone="brand">{presentRole(cell.role, locale)}</Badge>,
    },
    {
      id: 'source',
      header: t('ontologies.source'),
      sortValue: (cell) => cell.source,
      cell: (cell) => (
        <span className="flex items-center gap-1.5">
          <Badge tone="outline">{cell.source}</Badge>
          {cell.inherited ? <Badge tone="neutral">{locale === 'fr' ? 'hérité' : 'inherited'}</Badge> : null}
        </span>
      ),
    },
  ];

  const summary = matrix.data?.summary;

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

      <StatGrid>
        <Stat
          icon={Users}
          label={t('home.users')}
          loading={matrix.isLoading}
          value={formatNumber(summary?.users, locale)}
        />
        <Stat
          icon={ShieldCheck}
          label={t('rbac.organizations')}
          loading={matrix.isLoading}
          value={formatNumber(summary?.organizations, locale)}
        />
        <Stat
          icon={Boxes}
          label={t('home.projects')}
          loading={matrix.isLoading}
          value={formatNumber(summary?.projects, locale)}
        />
        <Stat
          label={t('rbac.matrix')}
          loading={matrix.isLoading}
          value={formatNumber(summary?.grants, locale)}
          hint={
            summary?.inherited
              ? `${formatNumber(summary.inherited, locale)} ${locale === 'fr' ? 'hérités' : 'inherited'}`
              : undefined
          }
        />
      </StatGrid>

      {/* The table answers "who has access"; an audit asks "why", which is a
          question about paths. Drawn only once the data is filtered enough to
          be readable - see AccessGraph. */}
      {matrix.data ? <AccessGraph report={matrix.data} /> : null}

      <Card>
        <DataTable
          data={matrix.data?.cells}
          columns={columns}
          rowKey={(cell) =>
            `${cell.subjectType}:${cell.subjectId}:${cell.resourceType}:${cell.resourceId}:${cell.role}`
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

  // The report is a JSON string, and its `warnings` array is how a run
  // declares that it succeeded without actually copying anything. Surfacing
  // it is the difference between a backup and the belief that there is one.
  function parseReport(backup: BackupRun): BackupReport | null {
    if (!backup.report) return null;
    try {
      return JSON.parse(backup.report) as BackupReport;
    } catch {
      return null;
    }
  }

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
      cell: (backup) => {
        const report = parseReport(backup);
        const warnings = report?.warnings ?? [];
        return (
          <span className="flex flex-wrap items-center gap-1.5">
            <StatusBadge status={backup.status} locale={locale} />
            {warnings.length > 0 ? (
              <Badge tone="warning" title={warnings.join('\n')}>
                <AlertTriangle className="size-3" aria-hidden />
                {warnings.length}
              </Badge>
            ) : null}
          </span>
        );
      },
    },
    {
      id: 'size',
      header: t('common.size'),
      align: 'right',
      sortValue: (backup) => parseReport(backup)?.bytes ?? 0,
      cell: (backup) => (
        <span className="tabular-nums text-muted-foreground">
          {formatBytes(parseReport(backup)?.bytes, locale)}
        </span>
      ),
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

  // Warnings from the most recent run, hoisted into a banner: a run that
  // reports success while copying nothing is the failure mode that matters.
  const latestWarnings = React.useMemo(() => {
    const latest = [...(runs.data ?? [])].sort(
      (a, b) => new Date(b.startedAt).getTime() - new Date(a.startedAt).getTime(),
    )[0];
    if (!latest) return [];
    return parseReport(latest)?.warnings ?? [];
  }, [runs.data]);

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

      {latestWarnings.length > 0 ? (
        <div
          role="alert"
          className="flex items-start gap-2.5 rounded-lg border border-warning/40 bg-warning-subtle px-4 py-3"
        >
          <AlertTriangle className="mt-0.5 size-4 shrink-0 text-warning" aria-hidden />
          <div className="min-w-0 space-y-1">
            <p className="text-sm font-medium text-warning-foreground">
              {t('admin.backupIncomplete')}
            </p>
            <ul className="list-inside list-disc space-y-0.5 text-xs leading-relaxed text-warning-foreground">
              {latestWarnings.map((warning) => (
                <li key={warning}>{warning}</li>
              ))}
            </ul>
          </div>
        </div>
      ) : null}

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
          <TabsTrigger value="rbac">{t('nav.rbac')}</TabsTrigger>
          <TabsTrigger value="activity">{t('nav.activity')}</TabsTrigger>
          <TabsTrigger value="network">{t('nav.network')}</TabsTrigger>
          <TabsTrigger value="storage">{t('nav.storage')}</TabsTrigger>
          <TabsTrigger value="inventory">{t('nav.inventory')}</TabsTrigger>
          <TabsTrigger value="audit">{t('nav.audit')}</TabsTrigger>
          <TabsTrigger value="settings">{t('common.settings')}</TabsTrigger>
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
        <TabsContent value="inventory">
          <SoftwareInventorySection />
        </TabsContent>

        <TabsContent value="settings">
          <div className="space-y-8">
            <PlatformSettingsSection />
            <SmtpSettingsSection />
            {/* Machine sizes sit with the platform settings rather than in a
                tab of their own: an administrator arrives here to say how the
                installation behaves, and the sizes it offers are part of that. */}
            <HardwareTiersSection />
          </div>
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
