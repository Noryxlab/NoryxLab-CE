import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Bot, Copy, KeyRound, Plus, UserMinus } from 'lucide-react';
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardHeaderText,
  CardTitle,
} from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState, ErrorState } from '@/components/common/states';
import { SectionHeader } from '@/components/common/page-header';
import { useConfirm } from '@/components/common/confirm-dialog';
import { useToast } from '@/components/ui/toast';
import { useAdminUsers, useOrganizations, useServiceAccounts, qk, useInvalidate } from '@/lib/api/queries';
import { adminApi } from '@/lib/api/endpoints';
import { useT } from '@/lib/i18n';
import type { ServiceAccount } from '@/lib/api/types';

/**
 * Service accounts: principals that are not people (ADR-039).
 *
 * The question this exists for: an app belongs to FOR and goes into
 * production, the person who launched it leaves, and who at FOR answers for
 * it? Naming a successor moves the responsibility from one human to the next,
 * every time, on the worst possible day. A service account holds it instead -
 * it belongs to an organization, it can own what must outlive the people who
 * set it up, and it does not resign.
 *
 * It cannot sign in. Its credentials are API tokens carrying its own name, so
 * what it does is audited under that name, in the same column as a person's.
 * Which is exactly why the screen insists on the one thing a directory cannot
 * infer: who answers for it.
 */

const EXPIRY_CHOICES = ['90', '365', '0'] as const;

export function ServiceAccountsSection() {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { ask } = useConfirm();
  const accounts = useServiceAccounts();
  const organizations = useOrganizations();
  const people = useAdminUsers();

  const [username, setUsername] = React.useState('');
  const [purpose, setPurpose] = React.useState('');
  const [organizationId, setOrganizationId] = React.useState('');
  const [responsible, setResponsible] = React.useState('');
  const [issued, setIssued] = React.useState<string | null>(null);
  const [tokenScope, setTokenScope] = React.useState('read');
  const [tokenExpiry, setTokenExpiry] = React.useState<string>('365');

  const create = useMutation({
    mutationFn: () =>
      adminApi.createServiceAccount({
        username: username.trim(),
        purpose: purpose.trim() || undefined,
        organizationId: organizationId || undefined,
        responsibleUserId: responsible,
      }),
    onSuccess: () => {
      setUsername('');
      setPurpose('');
      invalidate(qk.serviceAccounts);
      toast.success(t('serviceAccounts.created'), t('serviceAccounts.title'));
    },
    onError: (error) => toast.error(error, t('serviceAccounts.title')),
  });

  const issue = useMutation({
    mutationFn: (account: ServiceAccount) =>
      adminApi.createServiceAccountToken(account.username, {
        name: account.username,
        scopes: [tokenScope],
        expiresInDays: Number(tokenExpiry) || undefined,
      }),
    onSuccess: (result) => {
      setIssued(result.secret);
      invalidate(qk.serviceAccounts);
    },
    onError: (error) => toast.error(error, t('serviceAccounts.title')),
  });

  const disable = useMutation({
    mutationFn: (username: string) => adminApi.disableServiceAccount(username),
    onSuccess: (result) => {
      invalidate(qk.serviceAccounts);
      toast.success(
        t('serviceAccounts.disabled', { count: result.tokensRevoked }),
        t('serviceAccounts.title'),
      );
    },
    onError: (error) => toast.error(error, t('serviceAccounts.title')),
  });

  const columns: Column<ServiceAccount>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (account) => account.username,
      searchValue: (account) => `${account.username} ${account.purpose ?? ''}`,
      cell: (account) => (
        <div className="min-w-0">
          <div className="flex items-center gap-1.5">
            <Bot className="size-3.5 shrink-0 text-muted-foreground" aria-hidden />
            <span className="truncate font-medium">{account.username}</span>
            {account.enabled ? null : <Badge tone="neutral">{t('people.deactivated')}</Badge>}
          </div>
          {account.purpose ? (
            <div className="truncate text-xs text-muted-foreground">{account.purpose}</div>
          ) : null}
        </div>
      ),
    },
    {
      id: 'organization',
      header: t('common.organization'),
      cell: (account) => (account.organizations ?? []).join(', ') || '—',
    },
    {
      id: 'responsible',
      // The column that makes the whole thing worth having: an account that
      // answers to nobody is how a regulated dataset ends up unowned.
      header: t('serviceAccounts.responsible'),
      sortValue: (account) => account.responsible ?? '',
      cell: (account) =>
        account.responsible ? (
          account.responsible
        ) : (
          <Badge tone="warning">{t('serviceAccounts.noResponsible')}</Badge>
        ),
    },
    {
      id: 'tokens',
      header: t('serviceAccounts.credentials'),
      align: 'right',
      sortValue: (account) => account.tokens,
      cell: (account) =>
        account.tokens === 0 ? (
          <span className="text-muted-foreground">{t('serviceAccounts.noCredential')}</span>
        ) : (
          <span className="tabular-nums">{account.tokens}</span>
        ),
    },
    {
      id: 'actions',
      header: '',
      align: 'right',
      cell: (account) =>
        account.enabled ? (
          <div className="flex justify-end gap-1">
            <Button variant="ghost" size="sm" loading={issue.isPending} onClick={() => issue.mutate(account)}>
              <KeyRound aria-hidden />
              {t('serviceAccounts.issue')}
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() =>
                ask({
                  title: t('serviceAccounts.disableTitle'),
                  description: t('serviceAccounts.disableWarning', { name: account.username }),
                  confirmLabel: t('serviceAccounts.disableTitle'),
                  destructive: true,
                  onConfirm: () => disable.mutateAsync(account.username),
                })
              }
            >
              <UserMinus aria-hidden />
              {t('serviceAccounts.disableTitle')}
            </Button>
          </div>
        ) : null,
    },
  ];

  const peopleOptions = (people.data ?? [])
    .filter((person) => !person.serviceAccount)
    .map((person) => ({
      value: person.username ?? person.id,
      label: person.username ?? person.id,
      hint: person.email ?? undefined,
    }));

  return (
    <div className="space-y-4">
      <SectionHeader title={t('serviceAccounts.title')} description={t('serviceAccounts.subtitle')} />

      {issued ? (
        <Card className="border-brand/40 bg-brand-subtle/40">
          <CardHeader>
            <CardHeaderText>
              <CardTitle>{t('tokens.issuedTitle')}</CardTitle>
              <CardDescription>{t('componentCredentials.howToHint')}</CardDescription>
            </CardHeaderText>
          </CardHeader>
          <CardContent className="space-y-2">
            <p className="break-all rounded-md bg-surface p-2 font-mono text-xs">{issued}</p>
            <pre className="overflow-x-auto rounded-md bg-surface p-2 font-mono text-xs">
{`curl -H "Authorization: Bearer ${issued}" \\
  ${window.location.origin}/api/v1/projects`}
            </pre>
            <div className="flex flex-wrap gap-2">
              <Button
                variant="secondary"
                size="sm"
                onClick={() => {
                  void navigator.clipboard?.writeText(issued);
                  toast.success(t('tokens.copied'), t('serviceAccounts.title'));
                }}
              >
                <Copy aria-hidden />
                {t('common.copy')}
              </Button>
              <Button variant="ghost" size="sm" onClick={() => setIssued(null)}>
                {t('tokens.dismiss')}
              </Button>
            </div>
          </CardContent>
        </Card>
      ) : null}

      <Card>
        <CardHeader>
          <CardHeaderText>
            <CardTitle>{t('serviceAccounts.createTitle')}</CardTitle>
            <CardDescription>{t('serviceAccounts.createHint')}</CardDescription>
          </CardHeaderText>
        </CardHeader>
        <CardContent>
          <form
            className="flex flex-wrap items-end gap-2"
            onSubmit={(event) => {
              event.preventDefault();
              if (username.trim() && responsible) create.mutate();
            }}
          >
            <Field label={t('common.name')} description={t('serviceAccounts.nameHint')} className="min-w-48 flex-1">
              <Input
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                placeholder={t('serviceAccounts.namePlaceholder')}
                maxLength={64}
              />
            </Field>
            <Field label={t('serviceAccounts.purpose')} className="min-w-48 flex-1">
              <Input
                value={purpose}
                onChange={(event) => setPurpose(event.target.value)}
                placeholder={t('serviceAccounts.purposePlaceholder')}
                maxLength={120}
              />
            </Field>
            <Field label={t('common.organization')} className="min-w-40">
              <Select
                value={organizationId}
                onValueChange={setOrganizationId}
                options={(organizations.data ?? []).map((organization) => ({
                  value: organization.id,
                  label: organization.name,
                }))}
                placeholder={t('common.search')}
              />
            </Field>
            <Field
              label={t('serviceAccounts.responsible')}
              description={t('serviceAccounts.responsibleHint')}
              className="min-w-44"
            >
              <Select
                value={responsible}
                onValueChange={setResponsible}
                options={peopleOptions}
                placeholder={t('common.search')}
              />
            </Field>
            <Button
              type="submit"
              variant="primary"
              disabled={!username.trim() || !responsible}
              loading={create.isPending}
            >
              <Plus aria-hidden />
              {t('common.create')}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="flex flex-wrap items-end gap-2 p-3">
          {/* The credential a click on "issue" will produce, decided before the
              click rather than in a dialog after it. */}
          <Field label={t('serviceAccounts.nextCredential')} className="min-w-40">
            <Select
              value={tokenScope}
              onValueChange={setTokenScope}
              options={['read', 'datasets', 'workspaces', 'jobs', 'operate', 'full'].map((value) => ({
                value,
                label: value,
              }))}
            />
          </Field>
          <Field label={t('common.expiresAt')} className="min-w-36">
            <Select
              value={tokenExpiry}
              onValueChange={setTokenExpiry}
              options={EXPIRY_CHOICES.map((value) => ({
                value,
                label: value === '0' ? t('tokens.expiryNever') : t('tokens.expiryDays', { days: value }),
              }))}
            />
          </Field>
        </CardContent>
      </Card>

      {accounts.isLoading ? (
        <Skeleton className="h-40 w-full" />
      ) : accounts.isError ? (
        <ErrorState error={accounts.error} onRetry={() => void accounts.refetch()} />
      ) : (
        <DataTable
          data={accounts.data ?? []}
          columns={columns}
          rowKey={(account) => account.username}
          defaultSort={{ columnId: 'name', direction: 'asc' }}
          emptyState={
            <EmptyState
              icon={Bot}
              title={t('serviceAccounts.empty')}
              description={t('serviceAccounts.emptyHint')}
            />
          }
        />
      )}
    </div>
  );
}
