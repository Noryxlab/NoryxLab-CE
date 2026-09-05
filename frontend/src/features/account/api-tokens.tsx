import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Copy, KeyRound, Plus, Trash2 } from 'lucide-react';
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
import { SkeletonText } from '@/components/ui/skeleton';
import { ErrorState } from '@/components/common/states';
import { SectionHeader } from '@/components/common/page-header';
import { useToast } from '@/components/ui/toast';
import { useApiTokens, qk, useInvalidate } from '@/lib/api/queries';
import { platformApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatDateTime, formatRelative } from '@/lib/format';
import type { ApiToken } from '@/lib/api/types';

/**
 * Personal API tokens.
 *
 * A researcher calling the API from a CI job or a notebook had no honest
 * option: the platform authenticates people through Keycloak, and a pipeline
 * has no browser. What people reach for instead is worse — sharing a password,
 * or copying a short-lived browser token and wondering why it stops working an
 * hour later.
 *
 * A token acts as its owner and holds no rights of its own, so a leak costs one
 * account rather than the platform.
 */

const EXPIRY_CHOICES = ['30', '90', '365', '0'] as const;

function tokenState(token: ApiToken, now: Date): 'revoked' | 'expired' | 'active' {
  if (token.revokedAt) return 'revoked';
  if (token.expiresAt && new Date(token.expiresAt) <= now) return 'expired';
  return 'active';
}

export function ApiTokensSection() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const tokens = useApiTokens();

  const [name, setName] = React.useState('');
  const [expiresIn, setExpiresIn] = React.useState<string>('90');
  // Least dangerous first, and 'read' preselected: the common case for a token
  // is a dashboard or a report, and the default should be the one that cannot
  // delete anything.
  const [scope, setScope] = React.useState<string>('read');
  // Held in state, never refetched: the secret exists in the creation response
  // and nowhere else, ever again.
  const [issued, setIssued] = React.useState<string | null>(null);

  const create = useMutation({
    mutationFn: () =>
      platformApi.createApiToken({
        name: name.trim(),
        expiresInDays: Number(expiresIn) || undefined,
        scopes: [scope],
      }),
    onSuccess: (result) => {
      setIssued(result.secret);
      setName('');
      invalidate(qk.apiTokens);
    },
    onError: (error) => toast.error(error, t('tokens.title')),
  });

  const revoke = useMutation({
    mutationFn: (tokenId: string) => platformApi.revokeApiToken(tokenId),
    onSuccess: () => invalidate(qk.apiTokens),
    onError: (error) => toast.error(error, t('tokens.title')),
  });

  const now = new Date();
  const items = tokens.data ?? [];

  return (
    <div className="space-y-4">
      <SectionHeader title={t('tokens.title')} description={t('tokens.subtitle')} />

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
                  toast.success(t('tokens.copied'), t('tokens.title'));
                }}
              >
                <Copy aria-hidden />
                {t('tokens.copy')}
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
            <CardTitle>{t('tokens.createTitle')}</CardTitle>
            <CardDescription>{t('tokens.createHint')}</CardDescription>
          </CardHeaderText>
        </CardHeader>
        <CardContent>
          <form
            className="flex flex-wrap items-end gap-2"
            onSubmit={(event) => {
              event.preventDefault();
              if (name.trim()) create.mutate();
            }}
          >
            <Field label={t('tokens.nameLabel')} description={t('tokens.nameHint')} className="min-w-56 flex-1">
              <Input
                value={name}
                onChange={(event) => setName(event.target.value)}
                placeholder={t('tokens.namePlaceholder')}
                maxLength={80}
              />
            </Field>
            <Field
              label={t('tokens.scopeLabel')}
              description={t('tokens.scopeHint')}
              className="min-w-48"
            >
              <Select
                value={scope}
                onValueChange={setScope}
                options={[
                  { value: 'read', label: t('tokens.scopeRead'), hint: t('tokens.scopeReadHint') },
                  { value: 'workspaces', label: t('tokens.scopeWorkspaces'), hint: t('tokens.scopeWorkspacesHint') },
                  { value: 'jobs', label: t('tokens.scopeJobs'), hint: t('tokens.scopeJobsHint') },
                  { value: 'full', label: t('tokens.scopeFull'), hint: t('tokens.scopeFullHint') },
                ]}
              />
            </Field>
            <Field label={t('tokens.expiryLabel')} className="min-w-40">
              <Select
                value={expiresIn}
                onValueChange={setExpiresIn}
                options={EXPIRY_CHOICES.map((value) => ({
                  value,
                  label: value === '0' ? t('tokens.expiryNever') : t('tokens.expiryDays', { days: value }),
                }))}
              />
            </Field>
            <Button type="submit" variant="primary" disabled={!name.trim()} loading={create.isPending}>
              <Plus aria-hidden />
              {t('tokens.create')}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardContent className="py-0">
          {tokens.isLoading ? (
            <div className="py-4">
              <SkeletonText lines={3} />
            </div>
          ) : tokens.isError ? (
            <ErrorState error={tokens.error} onRetry={() => void tokens.refetch()} />
          ) : items.length === 0 ? (
            <p className="py-6 text-center text-sm text-muted-foreground">{t('tokens.empty')}</p>
          ) : (
            <ul className="divide-y divide-border">
              {items.map((token) => {
                const state = tokenState(token, now);
                return (
                  <li key={token.id} className="flex flex-wrap items-center justify-between gap-2 py-3">
                    <span className="min-w-0">
                      <span className="flex items-center gap-2">
                        <KeyRound className="size-3.5 shrink-0 text-muted-foreground" aria-hidden />
                        <span className="truncate text-sm font-medium">{token.name}</span>
                        {state === 'revoked' ? (
                          <Badge tone="outline">{t('tokens.revoked')}</Badge>
                        ) : state === 'expired' ? (
                          <Badge tone="warning">{t('tokens.expired')}</Badge>
                        ) : null}
                        {/* An unrestricted token is worth seeing at a glance:
                            it is the one whose leak costs the most, and the
                            one worth replacing with a narrower one. */}
                        {(token.scopes ?? []).map((scope) => (
                          <Badge key={scope} tone={scope === 'full' ? 'warning' : 'neutral'}>
                            {scope === 'read'
                              ? t('tokens.scopeRead')
                              : scope === 'workspaces'
                                ? t('tokens.scopeWorkspaces')
                                : scope === 'jobs'
                                  ? t('tokens.scopeJobs')
                                  : t('tokens.scopeFull')}
                          </Badge>
                        ))}
                      </span>
                      <span className="block text-xs text-muted-foreground">
                        {t('tokens.created')} {formatDateTime(token.createdAt, locale)}
                        {token.expiresAt
                          ? ` · ${t('tokens.expires')} ${formatDateTime(token.expiresAt, locale)}`
                          : ` · ${t('tokens.expiryNever')}`}
                        {/* Last use answers "is this still needed", which is the
                            question that gets a stale credential deleted. */}
                        {token.lastUsedAt
                          ? ` · ${t('tokens.lastUsed')} ${formatRelative(token.lastUsedAt, locale)}`
                          : ` · ${t('tokens.neverUsed')}`}
                      </span>
                    </span>
                    {state === 'active' ? (
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        aria-label={t('tokens.revoke')}
                        loading={revoke.isPending}
                        onClick={() => revoke.mutate(token.id)}
                      >
                        <Trash2 aria-hidden />
                      </Button>
                    ) : null}
                  </li>
                );
              })}
            </ul>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
