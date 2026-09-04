import * as React from 'react';
import { useParams } from 'react-router';
import { useMutation } from '@tanstack/react-query';
import { Trash2, UserPlus, Users } from 'lucide-react';
import { PageHeader } from '@/components/common/page-header';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { Button } from '@/components/ui/button';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardHeaderText,
  CardTitle,
} from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Field } from '@/components/ui/field';
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
import { SkeletonText } from '@/components/ui/skeleton';
import { useToast } from '@/components/ui/toast';
import {
  useAdminUsers,
  useOrganizations,
  useProject,
  useProjectOrganizationRoles,
  qk,
  useInvalidate,
} from '@/lib/api/queries';
import { projectOrganizationRolesApi, projectsApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { presentRole } from '@/lib/presenters';
import type { ProjectMember } from '@/lib/api/types';

const ROLES = ['viewer', 'editor', 'admin'] as const;

/** The same wording in both tables, so one screen does not name a role twice. */
function roleLabel(role: string, locale: 'fr' | 'en'): string {
  return presentRole(role, locale);
}

export function ProjectMembersPage() {
  const t = useT();
  const { locale } = useI18n();
  const { projectId } = useParams<{ projectId: string }>();
  const toast = useToast();
  const invalidate = useInvalidate();

  const project = useProject(projectId);
  const users = useAdminUsers();
  // Decided by the backend, which is also what enforces it. Re-implementing
  // the rule here would eventually disagree with it, and a viewer seeing a
  // control that always fails on click is a worse answer than not seeing it.
  const canManage = Boolean(project.data?.canManageMembers);
  const [inviting, setInviting] = React.useState(false);
  const [userId, setUserId] = React.useState('');
  const [role, setRole] = React.useState<string>('editor');

  const roleOptions = ROLES.map((value) => ({
    value,
    label: presentRole(value, locale),
    hint:
      value === 'viewer'
        ? t('members.roleViewerHint')
        : value === 'editor'
          ? t('members.roleEditorHint')
          : t('members.roleAdminHint'),
  }));

  const invite = useMutation({
    mutationFn: () => projectsApi.invite(projectId as string, { userId, role }),
    onSuccess: () => {
      invalidate(qk.project(projectId ?? ''), qk.projects);
      setInviting(false);
      setUserId('');
      toast.success(userId, t('members.invite'));
    },
    onError: (error) => toast.error(error, t('members.inviteTitle')),
  });

  const changeRole = useMutation({
    mutationFn: (input: { userId: string; role: string }) =>
      projectsApi.setMemberRole(projectId as string, input.userId, input.role),
    onSuccess: () => invalidate(qk.project(projectId ?? ''), qk.projects),
    onError: (error) => toast.error(error, t('members.roleLabel')),
  });

  // The owner is always a member; the API exposes members through the project
  // itself rather than a dedicated collection endpoint.
  const members: ProjectMember[] = React.useMemo(() => {
    if (!project.data) return [];
    return [{ userId: project.data.ownerId, role: 'admin' as const }].filter((member) => member.userId);
  }, [project.data]);

  const columns: Column<ProjectMember>[] = [
    {
      id: 'user',
      header: t('members.userLabel'),
      sortValue: (member) => member.displayName ?? member.userId,
      searchValue: (member) => `${member.displayName ?? ''} ${member.userId}`,
      cell: (member) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{member.displayName ?? member.userId}</p>
          {member.email ? <p className="truncate text-xs text-muted-foreground">{member.email}</p> : null}
        </div>
      ),
    },
    {
      id: 'role',
      header: t('members.roleLabel'),
      cell: (member) =>
        member.userId === project.data?.ownerId ? (
          <Badge tone="brand">{t('common.owner')}</Badge>
        ) : (
          <Select
            value={member.role}
            onValueChange={(value) => changeRole.mutate({ userId: member.userId, role: value })}
            options={roleOptions}
            className="max-w-48"
            aria-label={t('members.roleLabel')}
          />
        ),
    },
  ];

  return (
    <div className="space-y-5">
      <PageHeader
        title={t('members.title')}
        description={t('members.subtitle')}
        actions={
          <Button variant="primary" onClick={() => setInviting(true)}>
            <UserPlus aria-hidden />
            {t('members.invite')}
          </Button>
        }
      />

      <Card>
        <DataTable
          data={members}
          columns={columns}
          rowKey={(member) => member.userId}
          isLoading={project.isLoading}
          isError={project.isError}
          error={project.error}
          onRetry={() => void project.refetch()}
          emptyState={
            <EmptyState
              icon={Users}
              title={t('members.empty')}
              description={t('members.emptyHint')}
              action={
                <Button variant="primary" onClick={() => setInviting(true)}>
                  <UserPlus aria-hidden />
                  {t('members.invite')}
                </Button>
              }
            />
          }
        />
      </Card>

      <OrganizationGrants projectId={projectId ?? ''} canManage={canManage} />

      <Sheet open={inviting} onOpenChange={setInviting}>
        <SheetContent aria-describedby={undefined}>
          <form
            onSubmit={(event) => {
              event.preventDefault();
              if (userId) invite.mutate();
            }}
            className="flex min-h-0 flex-1 flex-col"
          >
            <SheetHeader>
              <SheetTitle>{t('members.inviteTitle')}</SheetTitle>
              <SheetDescription>{t('members.inviteHint')}</SheetDescription>
            </SheetHeader>
            <SheetBody>
              <Field label={t('members.userLabel')} required>
                <Select
                  value={userId}
                  onValueChange={setUserId}
                  placeholder={t('common.search')}
                  options={(users.data ?? []).map((user) => ({
                    value: user.username ?? user.id,
                    label: user.username ?? user.id,
                    hint: user.email ?? undefined,
                  }))}
                />
              </Field>
              <Field label={t('members.roleLabel')}>
                <Select value={role} onValueChange={setRole} options={roleOptions} />
              </Field>
            </SheetBody>
            <SheetFooter>
              <Button type="button" variant="secondary" onClick={() => setInviting(false)}>
                {t('common.cancel')}
              </Button>
              <Button type="submit" variant="primary" loading={invite.isPending} disabled={!userId}>
                {t('members.invite')}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>
    </div>
  );
}


/**
 * Roles held by every member of an organization.
 *
 * Assigning people one at a time is workable for a team of five and a reason
 * not to buy at fifty: an administrator adding a researcher had to remember
 * every project that person should reach, and someone leaving had to be
 * removed from each one by hand.
 *
 * Grants add up with personal roles rather than replacing them, so the strongest
 * applies. The hint says so, because an administrator who expects a grant to
 * *cap* somebody's access would otherwise be surprised in the wrong direction.
 */
function OrganizationGrants({ projectId, canManage }: { projectId: string; canManage: boolean }) {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const grants = useProjectOrganizationRoles(projectId);
  const organizations = useOrganizations();

  const [organizationId, setOrganizationId] = React.useState('');
  const [role, setRole] = React.useState<string>('editor');

  const grant = useMutation({
    mutationFn: () => projectOrganizationRolesApi.grant(projectId, organizationId, role),
    onSuccess: () => {
      invalidate(qk.projectOrganizationRoles(projectId));
      setOrganizationId('');
      toast.success(t('members.orgGranted'), t('members.orgTitle'));
    },
    onError: (error) => toast.error(error, t('members.orgTitle')),
  });

  const revoke = useMutation({
    mutationFn: (id: string) => projectOrganizationRolesApi.revoke(projectId, id),
    onSuccess: () => invalidate(qk.projectOrganizationRoles(projectId)),
    onError: (error) => toast.error(error, t('members.orgTitle')),
  });

  const items = grants.data ?? [];

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('members.orgTitle')}</CardTitle>
          <CardDescription>{t('members.orgHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent className="space-y-3">
        {grants.isLoading ? (
          <SkeletonText lines={2} />
        ) : items.length === 0 ? (
          <p className="text-sm text-muted-foreground">{t('members.orgEmpty')}</p>
        ) : (
          <ul className="divide-y divide-border">
            {items.map((item) => (
              <li key={item.organizationId} className="flex flex-wrap items-center justify-between gap-2 py-2">
                <span className="min-w-0">
                  <span className="block truncate text-sm font-medium">
                    {item.organizationName || item.organizationId}
                  </span>
                  <span className="block truncate font-mono text-xs text-muted-foreground">
                    {item.organizationId}
                  </span>
                </span>
                <span className="flex items-center gap-2">
                  <Badge tone="outline">{roleLabel(item.role, locale)}</Badge>
                  {canManage ? (
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label={t('common.delete')}
                      loading={revoke.isPending}
                      onClick={() => revoke.mutate(item.organizationId)}
                    >
                      <Trash2 aria-hidden />
                    </Button>
                  ) : null}
                </span>
              </li>
            ))}
          </ul>
        )}

        {canManage ? (
          <div className="flex flex-wrap items-end gap-2 border-t border-border pt-3">
            <Field label={t('members.orgLabel')} className="min-w-56 flex-1">
              <Select
                value={organizationId}
                onValueChange={setOrganizationId}
                placeholder={t('members.orgPlaceholder')}
                options={(organizations.data ?? []).map((organization) => ({
                  value: organization.id,
                  label: organization.name ?? organization.id,
                }))}
              />
            </Field>
            <Field label={t('members.roleLabel')} className="min-w-40">
              <Select
                value={role}
                onValueChange={setRole}
                options={ROLES.map((value) => ({ value, label: roleLabel(value, locale) }))}
              />
            </Field>
            <Button
              variant="secondary"
              disabled={!organizationId}
              loading={grant.isPending}
              onClick={() => grant.mutate()}
            >
              {t('members.orgGrant')}
            </Button>
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}
