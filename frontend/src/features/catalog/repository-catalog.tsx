import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { CheckCircle2, FolderGit2, Plus, ShieldCheck, Trash2, XCircle } from 'lucide-react';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { useConfirm } from '@/components/common/confirm-dialog';
import { SectionHeader } from '@/components/common/page-header';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
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
import { useRepositories, useSecrets, qk, useInvalidate } from '@/lib/api/queries';
import { repositoriesApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatRelative } from '@/lib/format';
import type { Repository } from '@/lib/api/types';

export function RepositoryCatalog() {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();

  const repositories = useRepositories();
  const secrets = useSecrets();

  const [creating, setCreating] = React.useState(false);
  const [name, setName] = React.useState('');
  const [url, setUrl] = React.useState('');
  const [defaultRef, setDefaultRef] = React.useState('main');
  const [authType, setAuthType] = React.useState('none');
  const [authSecretName, setAuthSecretName] = React.useState('');
  const [authorName, setAuthorName] = React.useState('');
  const [authorEmail, setAuthorEmail] = React.useState('');
  const [touched, setTouched] = React.useState(false);

  React.useEffect(() => {
    if (!creating) {
      setName('');
      setUrl('');
      setDefaultRef('main');
      setAuthType('none');
      setAuthSecretName('');
      setTouched(false);
    }
  }, [creating]);

  const create = useMutation({
    mutationFn: () =>
      repositoriesApi.create({
        name: name.trim(),
        url: url.trim(),
        defaultRef: defaultRef.trim() || 'main',
        authType,
        authSecretName: authType === 'secret' ? authSecretName : '',
        gitAuthorName: authorName.trim(),
        gitAuthorEmail: authorEmail.trim(),
      }),
    onSuccess: () => {
      invalidate(qk.repositories);
      setCreating(false);
      toast.success(name.trim(), t('repositories.create'));
    },
    onError: (error) => toast.error(error, t('repositories.createTitle')),
  });

  const validate = useMutation({
    mutationFn: (repositoryId: string) => repositoriesApi.validate(repositoryId),
    onSuccess: (result) => {
      invalidate(qk.repositories);
      if (result.reachable) toast.success(t('repositories.reachable'), t('repositories.validate'));
      else toast.error(result.error ?? t('repositories.unreachable'), t('repositories.validate'));
    },
    onError: (error) => toast.error(error, t('repositories.validate')),
  });

  const remove = useMutation({
    mutationFn: (repositoryId: string) => repositoriesApi.remove(repositoryId),
    onSuccess: () => invalidate(qk.repositories),
    onError: (error) => toast.error(error, t('repositories.deleteTitle')),
  });

  const columns: Column<Repository>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (repository) => repository.name,
      searchValue: (repository) => `${repository.name} ${repository.url}`,
      cell: (repository) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{repository.name}</p>
          <p className="truncate font-mono text-xs text-muted-foreground">{repository.url}</p>
        </div>
      ),
    },
    {
      id: 'ref',
      header: t('repositories.defaultRefLabel'),
      cell: (repository) => (
        <Badge tone="outline" className="font-mono">
          {repository.defaultRef || 'main'}
        </Badge>
      ),
    },
    {
      id: 'reachable',
      header: t('common.status'),
      sortValue: (repository) => (repository.reachable ? 1 : 0),
      cell: (repository) =>
        repository.reachable ? (
          <span className="flex items-center gap-1.5 text-xs text-success">
            <CheckCircle2 className="size-3.5" aria-hidden />
            {t('repositories.reachable')}
          </span>
        ) : (
          <span
            className="flex items-center gap-1.5 text-xs text-danger"
            title={repository.validationError}
          >
            <XCircle className="size-3.5" aria-hidden />
            {t('repositories.unreachable')}
          </span>
        ),
    },
    {
      id: 'validated',
      header: t('repositories.lastValidated'),
      sortValue: (repository) => repository.lastValidatedAt ?? null,
      cell: (repository) => (
        <span className="text-xs text-muted-foreground">
          {repository.lastValidatedAt ? formatRelative(repository.lastValidatedAt, locale) : '—'}
        </span>
      ),
    },
  ];

  const valid = Boolean(name.trim() && url.trim());

  return (
    <div className="space-y-4">
      <SectionHeader
        title={t('repositories.title')}
        description={t('repositories.subtitle')}
        actions={
          <Button variant="primary" onClick={() => setCreating(true)}>
            <Plus aria-hidden />
            {t('repositories.create')}
          </Button>
        }
      />

      <Card>
        <DataTable
          data={repositories.data}
          columns={columns}
          rowKey={(repository) => repository.id}
          isLoading={repositories.isLoading}
          isError={repositories.isError}
          error={repositories.error}
          onRetry={() => void repositories.refetch()}
          defaultSort={{ columnId: 'name', direction: 'asc' }}
          emptyState={
            <EmptyState
              icon={FolderGit2}
              title={t('repositories.empty')}
              description={t('repositories.emptyHint')}
              action={
                <Button variant="primary" onClick={() => setCreating(true)}>
                  <Plus aria-hidden />
                  {t('repositories.create')}
                </Button>
              }
            />
          }
          rowActions={(repository) => (
            <>
              <DropdownMenuItem onSelect={() => validate.mutate(repository.id)}>
                <ShieldCheck aria-hidden />
                {t('repositories.validate')}
              </DropdownMenuItem>
              <DropdownMenuItem
                destructive
                onSelect={() =>
                  ask({
                    title: t('repositories.deleteTitle'),
                    description: t('repositories.deleteWarning'),
                    confirmLabel: t('common.delete'),
                    destructive: true,
                    onConfirm: () => remove.mutateAsync(repository.id),
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

      <Sheet open={creating} onOpenChange={setCreating}>
        <SheetContent aria-describedby={undefined}>
          <form
            onSubmit={(event) => {
              event.preventDefault();
              setTouched(true);
              if (valid) create.mutate();
            }}
            className="flex min-h-0 flex-1 flex-col"
          >
            <SheetHeader>
              <SheetTitle>{t('repositories.createTitle')}</SheetTitle>
              <SheetDescription>{t('repositories.createHint')}</SheetDescription>
            </SheetHeader>
            <SheetBody>
              <Field
                label={t('common.name')}
                error={touched && !name.trim() ? t('common.required') : undefined}
                required
              >
                <Input
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  onBlur={() => setTouched(true)}
                  autoFocus
                  maxLength={80}
                />
              </Field>
              <Field
                label={t('repositories.urlLabel')}
                description={t('repositories.urlHint')}
                error={touched && !url.trim() ? t('common.required') : undefined}
                required
              >
                <Input
                  value={url}
                  onChange={(event) => setUrl(event.target.value)}
                  onBlur={() => setTouched(true)}
                  className="font-mono text-xs"
                  placeholder="https://github.com/organisation/depot.git"
                />
              </Field>
              <Field label={t('repositories.defaultRefLabel')}>
                <Input
                  value={defaultRef}
                  onChange={(event) => setDefaultRef(event.target.value)}
                  className="font-mono text-xs"
                />
              </Field>
              <Field label={t('repositories.authTypeLabel')}>
                <Select
                  value={authType}
                  onValueChange={setAuthType}
                  options={[
                    { value: 'none', label: t('repositories.authNone') },
                    { value: 'secret', label: t('repositories.authSecret') },
                  ]}
                />
              </Field>
              {authType === 'secret' ? (
                <Field label={t('repositories.authSecretLabel')} required>
                  <Select
                    value={authSecretName}
                    onValueChange={setAuthSecretName}
                    placeholder={t('secrets.title')}
                    options={(secrets.data ?? []).map((secret) => ({
                      value: secret.name,
                      label: secret.name,
                    }))}
                  />
                </Field>
              ) : null}
              <div className="grid gap-4 sm:grid-cols-2">
                <Field label={t('repositories.authorNameLabel')}>
                  <Input value={authorName} onChange={(event) => setAuthorName(event.target.value)} />
                </Field>
                <Field label={t('repositories.authorEmailLabel')}>
                  <Input
                    type="email"
                    value={authorEmail}
                    onChange={(event) => setAuthorEmail(event.target.value)}
                  />
                </Field>
              </div>
            </SheetBody>
            <SheetFooter>
              <Button type="button" variant="secondary" onClick={() => setCreating(false)}>
                {t('common.cancel')}
              </Button>
              <Button type="submit" variant="primary" loading={create.isPending} disabled={!valid}>
                {t('common.create')}
              </Button>
            </SheetFooter>
          </form>
        </SheetContent>
      </Sheet>

      {dialog}
    </div>
  );
}
