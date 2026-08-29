import * as React from 'react';
import { useParams } from 'react-router';
import { useMutation } from '@tanstack/react-query';
import { UserPlus, Users } from 'lucide-react';
import { PageHeader } from '@/components/common/page-header';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
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
import { useToast } from '@/components/ui/toast';
import { useAdminUsers, useProject, qk, useInvalidate } from '@/lib/api/queries';
import { projectsApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { presentRole } from '@/lib/presenters';
import type { ProjectMember } from '@/lib/api/types';

const ROLES = ['viewer', 'editor', 'admin'] as const;

export function ProjectMembersPage() {
  const t = useT();
  const { locale } = useI18n();
  const { projectId } = useParams<{ projectId: string }>();
  const toast = useToast();
  const invalidate = useInvalidate();

  const project = useProject(projectId);
  const users = useAdminUsers();
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
