import * as React from 'react';
import { Trash2 } from 'lucide-react';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
import { Field } from '@/components/ui/field';
import { Select } from '@/components/ui/select';
import { useAdminUsers, useAllTeams, useOrganizations } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { presentRole } from '@/lib/presenters';

/**
 * Who may do what with one object, the same on every object (ADR-051).
 *
 * Four screens held four copies of this panel, written at four moments, with
 * four vocabularies. This is the one: the owner first - who administers,
 * has write and shares - then the grants, Write or Read, to a person, a team
 * or an organization, with the subject named and the identifier under it.
 * An extract is frozen, so it offers Read only and says so.
 */

export type AccessSubjectType = 'user' | 'team' | 'organization';

export interface AccessEntry {
  subjectType: string;
  subjectId: string;
  subjectName?: string;
  userId?: string;
  role: string;
}

export interface AccessOwner {
  type?: string;
  id?: string;
  name?: string;
}

export function AccessPanel({
  title,
  hint,
  owner,
  entries,
  isLoading,
  isError,
  error,
  onRetry,
  roles = 'readWrite',
  onGrant,
  granting,
  onRevoke,
  canManage = true,
}: {
  title: string;
  hint: string;
  owner?: AccessOwner;
  entries: AccessEntry[] | undefined;
  isLoading?: boolean;
  isError?: boolean;
  error?: unknown;
  onRetry?: () => void;
  /** 'readOnly' for a frozen object: the right select disappears and the
   *  hint says why. */
  roles?: 'readWrite' | 'readOnly';
  onGrant: (subjectType: AccessSubjectType, subjectId: string, role: string) => void;
  granting?: boolean;
  onRevoke: (entry: AccessEntry) => void;
  canManage?: boolean;
}) {
  const t = useT();
  const { locale } = useI18n();
  const users = useAdminUsers();
  const organizations = useOrganizations();
  const teams = useAllTeams();

  const [subjectType, setSubjectType] = React.useState<AccessSubjectType>('user');
  const [subjectId, setSubjectId] = React.useState('');
  const [role, setRole] = React.useState('reader');

  const subjectOptions =
    subjectType === 'organization'
      ? (organizations.data ?? []).map((organization) => ({
          value: organization.alias ?? organization.id,
          label: organization.name,
        }))
      : subjectType === 'team'
        ? // Granted by identifier, never by name: two organizations may each
          // have a team called the same thing, so the name is only the label.
          teams.data.map((team) => ({ value: team.id, label: team.name, hint: team.organizationName || undefined }))
        : (users.data ?? []).map((user) => ({
            value: user.username ?? user.id,
            label: user.username ?? user.id,
            hint: user.email ?? undefined,
          }));

  const subjectTypeLabel = (type: string) =>
    type === 'organization' ? t('common.organization') : type === 'team' ? t('common.team') : t('common.user');

  const columns: Column<AccessEntry>[] = [
    {
      id: 'subject',
      header: t('rbac.subject'),
      cell: (entry) => {
        const name = entry.subjectName?.trim();
        const identifier = entry.subjectId || entry.userId || '';
        return (
          <div className="min-w-0">
            <span className="font-medium">{name || identifier}</span>
            {/* The identifier stays under the name: it is what goes into a
                command or a ticket. */}
            {name && name !== identifier ? (
              <div className="truncate font-mono text-xs text-muted-foreground">{identifier}</div>
            ) : null}
          </div>
        );
      },
    },
    {
      id: 'type',
      header: t('common.type'),
      cell: (entry) => <Badge tone="outline">{subjectTypeLabel(entry.subjectType)}</Badge>,
    },
    {
      id: 'role',
      header: t('access.right'),
      cell: (entry) => <Badge tone="brand">{presentRole(entry.role, locale)}</Badge>,
    },
  ];

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{title}</CardTitle>
          <CardDescription>
            {hint} {roles === 'readOnly' ? t('access.frozenHint') : t('access.rule')}
          </CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent className="space-y-4">
        {owner?.id ? (
          <dl className="flex flex-wrap items-center gap-x-3 gap-y-1 text-sm">
            <dt className="text-muted-foreground">{t('access.owner')}</dt>
            <dd className="flex flex-wrap items-center gap-2">
              <span className="font-medium">{owner.name || owner.id}</span>
              <Badge tone="outline">{subjectTypeLabel(owner.type ?? 'user')}</Badge>
              <span className="text-xs text-muted-foreground">
                {owner.type === 'organization' ? t('access.ownerOrganizationMeans') : t('access.ownerMeans')}
              </span>
            </dd>
          </dl>
        ) : null}

        {canManage ? (
          <div
            className={`grid gap-3 sm:items-end ${roles === 'readOnly' ? 'sm:grid-cols-[1fr_1fr_auto]' : 'sm:grid-cols-[1fr_1fr_1fr_auto]'}`}
          >
            <Field label={t('common.type')}>
              <Select
                value={subjectType}
                onValueChange={(value) => {
                  setSubjectType(value === 'organization' ? 'organization' : value === 'team' ? 'team' : 'user');
                  setSubjectId('');
                }}
                options={[
                  { value: 'user', label: t('common.user') },
                  { value: 'team', label: t('common.team') },
                  { value: 'organization', label: t('common.organization') },
                ]}
              />
            </Field>
            <Field label={t('rbac.subject')}>
              <Select value={subjectId} onValueChange={setSubjectId} options={subjectOptions} placeholder={t('common.search')} />
            </Field>
            {roles === 'readWrite' ? (
              <Field label={t('access.right')}>
                <Select
                  value={role}
                  onValueChange={setRole}
                  options={[
                    { value: 'reader', label: presentRole('reader', locale) },
                    { value: 'writer', label: presentRole('writer', locale) },
                  ]}
                />
              </Field>
            ) : null}
            <Button
              variant="primary"
              disabled={!subjectId}
              loading={granting}
              onClick={() => {
                onGrant(subjectType, subjectId, roles === 'readOnly' ? 'reader' : role);
                setSubjectId('');
              }}
            >
              {t('access.grant')}
            </Button>
          </div>
        ) : null}

        <DataTable
          data={entries}
          columns={columns}
          rowKey={(entry) => `${entry.subjectType}:${entry.subjectId || entry.userId}`}
          isLoading={isLoading}
          isError={isError}
          error={error}
          onRetry={onRetry}
          emptyState={<EmptyState compact title={t('access.empty')} description={t('access.emptyHint')} />}
          rowActions={
            canManage
              ? (entry) => (
                  <DropdownMenuItem destructive onSelect={() => onRevoke(entry)}>
                    <Trash2 aria-hidden />
                    {t('access.revoke')}
                  </DropdownMenuItem>
                )
              : undefined
          }
        />
      </CardContent>
    </Card>
  );
}
