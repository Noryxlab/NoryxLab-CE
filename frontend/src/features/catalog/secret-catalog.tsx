import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { KeyRound, Plus, Trash2 } from 'lucide-react';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { useConfirm } from '@/components/common/confirm-dialog';
import { SectionHeader } from '@/components/common/page-header';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { DropdownMenuItem } from '@/components/ui/dropdown-menu';
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
import { useSecrets, qk, useInvalidate } from '@/lib/api/queries';
import { secretsApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatDate, formatRelative } from '@/lib/format';
import type { Secret } from '@/lib/api/types';

/** Secret names become environment variables, so they are normalised to the
 *  shape the runtime actually injects rather than rejected after the fact. */
function normaliseSecretName(value: string): string {
  return value
    .toUpperCase()
    .replace(/[^A-Z0-9_]/g, '_')
    .replace(/^_+/, '')
    .slice(0, 64);
}

const EXPIRY_WARNING_DAYS = 14;

function expiryTone(secret: Secret): 'danger' | 'warning' | null {
  if (!secret.expiresAt) return null;
  const remaining = new Date(secret.expiresAt).getTime() - Date.now();
  if (remaining <= 0) return 'danger';
  if (remaining < EXPIRY_WARNING_DAYS * 86_400_000) return 'warning';
  return null;
}

export function SecretCatalog() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();

  const secrets = useSecrets();
  const [creating, setCreating] = React.useState(false);
  const [name, setName] = React.useState('');
  const [value, setValue] = React.useState('');
  const [expiresAt, setExpiresAt] = React.useState('');
  const [touched, setTouched] = React.useState(false);

  React.useEffect(() => {
    if (!creating) {
      setName('');
      setValue('');
      setExpiresAt('');
      setTouched(false);
    }
  }, [creating]);

  const create = useMutation({
    mutationFn: () =>
      secretsApi.create({
        name,
        value,
        expiresAt: expiresAt ? new Date(expiresAt).toISOString() : null,
      }),
    onSuccess: () => {
      invalidate(qk.secrets);
      setCreating(false);
      toast.success(t('secrets.created'), name);
    },
    onError: (error) => toast.error(error, t('secrets.createTitle')),
  });

  const remove = useMutation({
    mutationFn: (secretName: string) => secretsApi.remove(secretName),
    onSuccess: () => invalidate(qk.secrets),
    onError: (error) => toast.error(error, t('secrets.deleteTitle')),
  });

  const columns: Column<Secret>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (secret) => secret.name,
      searchValue: (secret) => secret.name,
      cell: (secret) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{secret.name}</p>
          <p className="truncate font-mono text-xs text-muted-foreground">
            NORYX_SECRET_{secret.name}
          </p>
        </div>
      ),
    },
    {
      id: 'expiry',
      header: t('secrets.expiresAt'),
      sortValue: (secret) => secret.expiresAt ?? null,
      cell: (secret) => {
        const tone = expiryTone(secret);
        if (!secret.expiresAt) return <span className="text-xs text-muted-foreground">—</span>;
        return (
          <span className="flex items-center gap-2">
            <span className="text-xs text-muted-foreground">{formatDate(secret.expiresAt, locale)}</span>
            {tone ? (
              <Badge tone={tone}>{tone === 'danger' ? t('secrets.expired') : t('secrets.expiringSoon')}</Badge>
            ) : null}
          </span>
        );
      },
    },
    {
      id: 'updatedAt',
      header: t('common.updatedAt'),
      sortValue: (secret) => secret.updatedAt,
      cell: (secret) => (
        <span className="text-xs text-muted-foreground">{formatRelative(secret.updatedAt, locale)}</span>
      ),
    },
  ];

  const nameError = touched && !name ? t('common.required') : undefined;
  const valueError = touched && !value ? t('common.required') : undefined;

  return (
    <div className="space-y-4">
      <SectionHeader
        title={t('secrets.title')}
        description={t('secrets.subtitle')}
        actions={
          <Button variant="primary" onClick={() => setCreating(true)}>
            <Plus aria-hidden />
            {t('secrets.create')}
          </Button>
        }
      />

      <Card>
        <DataTable
          data={secrets.data}
          columns={columns}
          rowKey={(secret) => secret.id || secret.name}
          isLoading={secrets.isLoading}
          isError={secrets.isError}
          error={secrets.error}
          onRetry={() => void secrets.refetch()}
          defaultSort={{ columnId: 'name', direction: 'asc' }}
          emptyState={
            <EmptyState
              icon={KeyRound}
              title={t('secrets.empty')}
              description={t('secrets.emptyHint')}
              action={
                <Button variant="primary" onClick={() => setCreating(true)}>
                  <Plus aria-hidden />
                  {t('secrets.create')}
                </Button>
              }
            />
          }
          rowActions={(secret) => (
            <DropdownMenuItem
              destructive
              onSelect={() =>
                ask({
                  title: t('secrets.deleteTitle'),
                  description: t('secrets.deleteWarning'),
                  confirmLabel: t('common.delete'),
                  destructive: true,
                  onConfirm: () => remove.mutateAsync(secret.name),
                })
              }
            >
              <Trash2 aria-hidden />
              {t('common.delete')}
            </DropdownMenuItem>
          )}
        />
      </Card>

      <Sheet open={creating} onOpenChange={setCreating}>
        <SheetContent aria-describedby={undefined}>
          <form
            onSubmit={(event) => {
              event.preventDefault();
              setTouched(true);
              if (name && value) create.mutate();
            }}
            className="flex min-h-0 flex-1 flex-col"
          >
            <SheetHeader>
              <SheetTitle>{t('secrets.createTitle')}</SheetTitle>
              <SheetDescription>{t('secrets.createHint')}</SheetDescription>
            </SheetHeader>
            <SheetBody>
              <Field
                label={t('secrets.nameLabel')}
                description={t('secrets.nameHint', { name: name || 'NOM' })}
                error={nameError}
                required
              >
                <Input
                  value={name}
                  onChange={(event) => setName(normaliseSecretName(event.target.value))}
                  onBlur={() => setTouched(true)}
                  placeholder={t('secrets.namePlaceholder')}
                  className="font-mono text-xs"
                  autoFocus
                />
              </Field>
              <Field
                label={t('secrets.valueLabel')}
                description={t('secrets.valueHint')}
                error={valueError}
                required
              >
                <Input
                  type="password"
                  value={value}
                  onChange={(event) => setValue(event.target.value)}
                  onBlur={() => setTouched(true)}
                  autoComplete="new-password"
                />
              </Field>
              <Field label={t('secrets.expiresLabel')} description={t('secrets.expiresHint')}>
                <Input
                  type="date"
                  value={expiresAt}
                  onChange={(event) => setExpiresAt(event.target.value)}
                />
              </Field>
            </SheetBody>
            <SheetFooter>
              <Button type="button" variant="secondary" onClick={() => setCreating(false)}>
                {t('common.cancel')}
              </Button>
              <Button
                type="submit"
                variant="primary"
                loading={create.isPending}
                disabled={!name || !value}
              >
                {t('common.save')}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      {dialog}
    </div>
  );
}
