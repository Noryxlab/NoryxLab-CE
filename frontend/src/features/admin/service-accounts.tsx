import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Copy, Plus, Trash2 } from 'lucide-react';
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
import { useComponentTokens, qk, useInvalidate } from '@/lib/api/queries';
import { adminApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatDateTime, formatRelative } from '@/lib/format';
import type { ApiToken } from '@/lib/api/types';

/**
 * Service accounts.
 *
 * The platform could already issue a credential to a component - the backup
 * runner, the validator, a customer's pipeline - and only through the API,
 * which needs an administrator's session to call. A credential that can only
 * be born from an API call is one nobody can list, and nobody remembers to
 * revoke: seven `backup-runner` tokens were found on the DC on 2026-09-26,
 * one created per redeploy, none revoked, none used since September.
 *
 * So the screen exists to make them visible, not merely creatable. What it
 * shows first is what an administrator needs to decide: which of these is
 * still being used, and by whom.
 *
 * One scope per account, like a personal token. An account needing two things
 * is two accounts, which is also what makes an audit line readable.
 */

const EXPIRY_CHOICES = ['30', '90', '365', '0'] as const;

export function accountState(token: ApiToken, now: Date): 'revoked' | 'expired' | 'idle' | 'active' {
  if (token.revokedAt) return 'revoked';
  if (token.expiresAt && new Date(token.expiresAt) <= now) return 'expired';
  if (!token.lastUsedAt) return 'idle';
  return 'active';
}

function expiryDate(days: string): string | undefined {
  const count = Number(days);
  if (!count) return undefined;
  const when = new Date();
  when.setDate(when.getDate() + count);
  return when.toISOString();
}

export function ServiceAccountsSection() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { ask } = useConfirm();
  const tokens = useComponentTokens();

  const [component, setComponent] = React.useState('');
  const [scope, setScope] = React.useState('read');
  const [expiresIn, setExpiresIn] = React.useState<string>('365');
  // Held in state and never refetched: the secret exists in the creation
  // response and nowhere else, ever again.
  const [issued, setIssued] = React.useState<string | null>(null);

  const create = useMutation({
    mutationFn: () =>
      adminApi.createComponentToken({
        component: component.trim(),
        name: component.trim(),
        scopes: [scope],
        expiresAt: expiryDate(expiresIn),
      }),
    onSuccess: (result) => {
      setIssued(result.secret);
      setComponent('');
      invalidate(qk.componentTokens);
    },
    onError: (error) => toast.error(error, t('serviceAccounts.title')),
  });

  const revoke = useMutation({
    mutationFn: (tokenId: string) => adminApi.revokeComponentToken(tokenId),
    onSuccess: () => invalidate(qk.componentTokens),
    onError: (error) => toast.error(error, t('serviceAccounts.title')),
  });

  const now = new Date();
  const items = tokens.data?.items ?? [];

  const columns: Column<ApiToken>[] = [
    {
      id: 'component',
      header: t('serviceAccounts.component'),
      sortValue: (token) => (token.component ?? '').toLowerCase(),
      searchValue: (token) => `${token.component ?? ''} ${token.name}`,
      cell: (token) => (
        <div className="min-w-0">
          <div className="truncate font-medium">{token.component || token.name}</div>
          {token.name && token.name !== token.component ? (
            <div className="truncate text-xs text-muted-foreground">{token.name}</div>
          ) : null}
        </div>
      ),
    },
    {
      id: 'scope',
      header: t('tokens.scopeLabel'),
      cell: (token) => <Badge tone="outline">{(token.scopes ?? ['full']).join(', ')}</Badge>,
    },
    {
      id: 'state',
      header: t('common.status'),
      sortValue: (token) => accountState(token, now),
      cell: (token) => {
        const state = accountState(token, now);
        // "Never used" is its own state rather than an empty cell: it is the
        // one that says a credential was created and forgotten.
        const tone = state === 'active' ? 'success' : state === 'idle' ? 'warning' : 'neutral';
        return <Badge tone={tone}>{t(`serviceAccounts.state_${state}` as 'serviceAccounts.state_active')}</Badge>;
      },
    },
    {
      id: 'lastUsed',
      header: t('serviceAccounts.lastUsed'),
      align: 'right',
      sortValue: (token) => (token.lastUsedAt ? new Date(token.lastUsedAt).getTime() : 0),
      cell: (token) =>
        token.lastUsedAt ? (
          <span title={formatDateTime(token.lastUsedAt)}>{formatRelative(token.lastUsedAt, locale)}</span>
        ) : (
          <span className="text-muted-foreground">{t('common.neverUsed')}</span>
        ),
    },
    {
      id: 'expires',
      header: t('common.expiresAt'),
      align: 'right',
      sortValue: (token) => (token.expiresAt ? new Date(token.expiresAt).getTime() : Infinity),
      cell: (token) =>
        token.expiresAt ? formatDateTime(token.expiresAt) : t('tokens.expiryNever'),
    },
    {
      id: 'actions',
      header: '',
      align: 'right',
      cell: (token) =>
        token.revokedAt ? null : (
          <Button
            variant="ghost"
            size="sm"
            onClick={() =>
              ask({
                title: t('common.revoke'),
                description: t('serviceAccounts.revokeWarning', {
                  name: token.component || token.name,
                }),
                confirmLabel: t('common.revoke'),
                destructive: true,
                onConfirm: () => revoke.mutateAsync(token.id),
              })
            }
          >
            <Trash2 aria-hidden />
            {t('common.revoke')}
          </Button>
        ),
    },
  ];

  const scopeOptions = (tokens.data?.scopes ?? ['read', 'full']).map((value) => ({
    value,
    label: value,
  }));

  return (
    <div className="space-y-4">
      <SectionHeader title={t('serviceAccounts.title')} description={t('serviceAccounts.subtitle')} />

      {issued ? (
        <Card className="border-brand/40 bg-brand-subtle/40">
          <CardHeader>
            <CardHeaderText>
              <CardTitle>{t('tokens.issuedTitle')}</CardTitle>
              <CardDescription>{t('tokens.issuedHint')}</CardDescription>
            </CardHeaderText>
          </CardHeader>
          <CardContent className="space-y-2">
            <p className="break-all rounded-md bg-surface p-2 font-mono text-xs">{issued}</p>
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
              if (component.trim()) create.mutate();
            }}
          >
            <Field
              label={t('serviceAccounts.component')}
              description={t('serviceAccounts.componentHint')}
              className="min-w-56 flex-1"
            >
              <Input
                value={component}
                onChange={(event) => setComponent(event.target.value)}
                placeholder={t('serviceAccounts.componentPlaceholder')}
                maxLength={80}
              />
            </Field>
            <Field label={t('tokens.scopeLabel')} description={t('serviceAccounts.scopeHint')} className="min-w-44">
              <Select value={scope} onValueChange={setScope} options={scopeOptions} />
            </Field>
            <Field label={t('common.expiresAt')} className="min-w-40">
              <Select
                value={expiresIn}
                onValueChange={setExpiresIn}
                options={EXPIRY_CHOICES.map((value) => ({
                  value,
                  label: value === '0' ? t('tokens.expiryNever') : t('tokens.expiryDays', { days: value }),
                }))}
              />
            </Field>
            <Button type="submit" variant="primary" disabled={!component.trim()} loading={create.isPending}>
              <Plus aria-hidden />
              {t('common.create')}
            </Button>
          </form>
        </CardContent>
      </Card>

      {tokens.isLoading ? (
        <Skeleton className="h-40 w-full" />
      ) : tokens.isError ? (
        <ErrorState error={tokens.error} onRetry={() => void tokens.refetch()} />
      ) : (
        <DataTable
          data={items}
          columns={columns}
          rowKey={(token) => token.id}
          defaultSort={{ columnId: 'state', direction: 'asc' }}
          emptyState={
            <EmptyState title={t('serviceAccounts.empty')} description={t('serviceAccounts.emptyHint')} />
          }
        />
      )}
    </div>
  );
}
