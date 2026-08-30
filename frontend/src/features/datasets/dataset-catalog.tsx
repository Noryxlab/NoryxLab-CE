import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Database, Plus, Trash2 } from 'lucide-react';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { SearchInput } from '@/components/common/search-input';
import { useConfirm } from '@/components/common/confirm-dialog';
import { SectionHeader } from '@/components/common/page-header';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardHeaderText, CardTitle, CardDescription } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { Field } from '@/components/ui/field';
import { Input, Textarea } from '@/components/ui/input';
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
import { useDatasetAccess, useDatasets, useOrganizations, useAdminUsers, qk, useInvalidate } from '@/lib/api/queries';
import { datasetsApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatRelative } from '@/lib/format';
import { presentRole } from '@/lib/presenters';
import { useAuth } from '@/lib/auth';
import { DatasetExplorer } from './dataset-explorer';
import type { Dataset, DatasetAccess } from '@/lib/api/types';

function CreateDatasetSheet({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { isAdmin } = useAuth();

  const [mode, setMode] = React.useState<'local' | 'external'>('local');
  const [classification, setClassification] = React.useState('non-hds');
  const [name, setName] = React.useState('');
  const [description, setDescription] = React.useState('');
  const [endpoint, setEndpoint] = React.useState('');
  const [region, setRegion] = React.useState('');
  const [bucket, setBucket] = React.useState('');
  const [prefix, setPrefix] = React.useState('');
  const [accessKey, setAccessKey] = React.useState('');
  const [secretKey, setSecretKey] = React.useState('');
  const [touched, setTouched] = React.useState(false);

  React.useEffect(() => {
    if (!open) {
      setName('');
      setDescription('');
      setBucket('');
      setAccessKey('');
      setSecretKey('');
      setTouched(false);
    }
  }, [open]);

  const nameError = touched && !name.trim() ? t('common.required') : undefined;
  const bucketError = touched && mode === 'external' && !bucket.trim() ? t('common.required') : undefined;
  const valid = Boolean(name.trim()) && (mode === 'local' || bucket.trim());

  const mutation = useMutation({
    mutationFn: () =>
      datasetsApi.create({
        name: name.trim(),
        description: description.trim(),
        classification,
        ...(mode === 'external'
          ? {
              provider: 's3',
              endpoint: endpoint.trim(),
              region: region.trim(),
              bucket: bucket.trim(),
              prefix: prefix.trim(),
              accessKey,
              secretKey,
            }
          : {}),
      }),
    onSuccess: () => {
      invalidate(qk.datasets);
      onOpenChange(false);
      toast.success(name.trim(), t('datasets.create'));
    },
    onError: (error) => toast.error(error, t('datasets.createTitle')),
  });

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent size="lg" aria-describedby={undefined}>
        <form
          onSubmit={(event) => {
            event.preventDefault();
            setTouched(true);
            if (valid) mutation.mutate();
          }}
          className="flex min-h-0 flex-1 flex-col"
        >
          <SheetHeader>
            <SheetTitle>{t('datasets.createTitle')}</SheetTitle>
            <SheetDescription>{t('datasets.createHint')}</SheetDescription>
          </SheetHeader>

          <SheetBody>
            <Field label={t('datasets.modeLabel')}>
              <Select
                value={mode}
                onValueChange={(value) => setMode(value === 'external' ? 'external' : 'local')}
                options={[
                  { value: 'local', label: t('datasets.modeLocal'), hint: t('datasets.modeLocalHint') },
                  {
                    value: 'external',
                    label: t('datasets.modeExternal'),
                    hint: t('datasets.modeExternalHint'),
                  },
                ]}
              />
            </Field>

            <Field label={t('datasets.createTitle')} error={nameError} required htmlFor="dataset-name">
              <Input
                value={name}
                onChange={(event) => setName(event.target.value)}
                onBlur={() => setTouched(true)}
                maxLength={120}
                autoFocus
              />
            </Field>

            <Field label={t('common.description')}>
              <Textarea
                value={description}
                onChange={(event) => setDescription(event.target.value)}
                rows={2}
                maxLength={500}
              />
            </Field>

            {/* HDS creation is administrator-only (ADR-013), so the control is
                hidden rather than shown and then rejected by the API. */}
            {isAdmin ? (
              <Field
                label={t('datasets.classificationLabel')}
                description={t('datasets.classificationHint')}
              >
                <Select
                  value={classification}
                  onValueChange={setClassification}
                  options={[
                    { value: 'non-hds', label: t('datasets.classificationStandard') },
                    { value: 'hds', label: t('datasets.classificationHds') },
                  ]}
                />
              </Field>
            ) : null}

            {classification === 'hds' ? (
              <p className="rounded-md border border-warning/40 bg-warning-subtle px-3 py-2 text-xs leading-relaxed text-warning-foreground">
                {t('datasets.hdsWarning')}
              </p>
            ) : null}

            {mode === 'external' ? (
              <>
                <Field label={t('datasets.endpointLabel')} required>
                  <Input
                    value={endpoint}
                    onChange={(event) => setEndpoint(event.target.value)}
                    placeholder="https://s3.example.com"
                    inputMode="url"
                  />
                </Field>
                <div className="grid gap-4 sm:grid-cols-2">
                  <Field label={t('datasets.bucketLabel')} error={bucketError} required>
                    <Input value={bucket} onChange={(event) => setBucket(event.target.value)} />
                  </Field>
                  <Field label={t('datasets.regionLabel')}>
                    <Input
                      value={region}
                      onChange={(event) => setRegion(event.target.value)}
                      placeholder="eu-west-3"
                    />
                  </Field>
                </div>
                <Field label={t('datasets.prefixLabel')} description={t('datasets.prefixHint')}>
                  <Input value={prefix} onChange={(event) => setPrefix(event.target.value)} />
                </Field>
                <div className="grid gap-4 sm:grid-cols-2">
                  <Field label={t('datasets.accessKeyLabel')} required>
                    <Input
                      value={accessKey}
                      onChange={(event) => setAccessKey(event.target.value)}
                      autoComplete="off"
                    />
                  </Field>
                  <Field label={t('datasets.secretKeyLabel')} required>
                    <Input
                      type="password"
                      value={secretKey}
                      onChange={(event) => setSecretKey(event.target.value)}
                      autoComplete="new-password"
                    />
                  </Field>
                </div>
                <p className="text-xs leading-relaxed text-muted-foreground">
                  {t('datasets.credentialsHint')}
                </p>
              </>
            ) : null}
          </SheetBody>

          <SheetFooter>
            <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" variant="primary" loading={mutation.isPending} disabled={!valid}>
              {t('common.create')}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  );
}

function DatasetPermissions({ dataset }: { dataset: Dataset }) {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const access = useDatasetAccess(dataset.id);
  const users = useAdminUsers();
  const organizations = useOrganizations();

  const [subjectType, setSubjectType] = React.useState<'user' | 'organization'>('user');
  const [subjectId, setSubjectId] = React.useState('');
  const [role, setRole] = React.useState('reader');

  const grant = useMutation({
    mutationFn: () => datasetsApi.grant(dataset.id, subjectType, subjectId, role),
    onSuccess: () => {
      invalidate(qk.datasetAccess(dataset.id));
      setSubjectId('');
    },
    onError: (error) => toast.error(error, t('datasets.grant')),
  });

  const revoke = useMutation({
    mutationFn: (entry: DatasetAccess) =>
      datasetsApi.revoke(dataset.id, entry.subjectType, entry.subjectId),
    onSuccess: () => invalidate(qk.datasetAccess(dataset.id)),
    onError: (error) => toast.error(error, t('datasets.revoke')),
  });

  const columns: Column<DatasetAccess>[] = [
    {
      id: 'subject',
      header: t('rbac.subject'),
      cell: (entry) => <span className="font-medium">{entry.subjectId || entry.userId}</span>,
    },
    {
      id: 'type',
      header: t('common.type'),
      cell: (entry) => (
        <Badge tone="outline">
          {entry.subjectType === 'organization' ? t('common.organization') : t('common.user')}
        </Badge>
      ),
    },
    {
      id: 'role',
      header: t('common.role'),
      cell: (entry) => <Badge tone="brand">{presentRole(entry.role, locale)}</Badge>,
    },
  ];

  const subjectOptions =
    subjectType === 'organization'
      ? (organizations.data ?? []).map((organization) => ({
          value: organization.alias ?? organization.id,
          label: organization.name,
        }))
      : (users.data ?? []).map((user) => ({
          value: user.username ?? user.id,
          label: user.username ?? user.id,
          hint: user.email ?? undefined,
        }));

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('datasets.permissions')}</CardTitle>
          <CardDescription>{t('datasets.permissionsHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-[1fr_1fr_1fr_auto] sm:items-end">
          <Field label={t('common.type')}>
            <Select
              value={subjectType}
              onValueChange={(value) => {
                setSubjectType(value === 'organization' ? 'organization' : 'user');
                setSubjectId('');
              }}
              options={[
                { value: 'user', label: t('common.user') },
                { value: 'organization', label: t('common.organization') },
              ]}
            />
          </Field>
          <Field label={t('rbac.subject')}>
            <Select
              value={subjectId}
              onValueChange={setSubjectId}
              options={subjectOptions}
              placeholder={t('common.search')}
            />
          </Field>
          <Field label={t('common.role')}>
            <Select
              value={role}
              onValueChange={setRole}
              options={[
                { value: 'reader', label: presentRole('reader', locale) },
                { value: 'writer', label: presentRole('writer', locale) },
              ]}
            />
          </Field>
          <Button
            variant="primary"
            disabled={!subjectId}
            loading={grant.isPending}
            onClick={() => grant.mutate()}
          >
            {t('datasets.grant')}
          </Button>
        </div>

        <DataTable
          data={access.data}
          columns={columns}
          rowKey={(entry) => `${entry.subjectType}:${entry.subjectId || entry.userId}`}
          isLoading={access.isLoading}
          isError={access.isError}
          error={access.error}
          onRetry={() => void access.refetch()}
          emptyState={
            <EmptyState compact title={t('datasets.permissions')} description={t('datasets.permissionsHint')} />
          }
          rowActions={(entry) => (
            <DropdownMenuItem destructive onSelect={() => revoke.mutate(entry)}>
              <Trash2 aria-hidden />
              {t('datasets.revoke')}
            </DropdownMenuItem>
          )}
        />
      </CardContent>
    </Card>
  );
}

export function DatasetCatalog({
  selectedId,
  onSelect,
}: {
  selectedId: string | null;
  onSelect: (datasetId: string | null) => void;
}) {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();

  const datasets = useDatasets();
  const [search, setSearch] = React.useState('');
  const [creating, setCreating] = React.useState(false);

  const selected = datasets.data?.find((dataset) => dataset.id === selectedId) ?? null;

  const remove = useMutation({
    mutationFn: (datasetId: string) => datasetsApi.remove(datasetId),
    onSuccess: () => {
      invalidate(qk.datasets);
      onSelect(null);
    },
    onError: (error) => toast.error(error, t('datasets.deleteTitle')),
  });

  const columns: Column<Dataset>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (dataset) => dataset.name,
      searchValue: (dataset) => `${dataset.name} ${dataset.description} ${dataset.bucket}`,
      cell: (dataset) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{dataset.name}</p>
          {dataset.description ? (
            <p className="truncate text-xs text-muted-foreground">{dataset.description}</p>
          ) : null}
        </div>
      ),
    },
    {
      id: 'classification',
      header: t('datasets.classification'),
      sortValue: (dataset) => dataset.classification,
      cell: (dataset) =>
        dataset.classification === 'hds' ? (
          <Badge tone="warning">{t('datasets.classificationHds')}</Badge>
        ) : (
          <Badge tone="outline">{t('datasets.classificationStandard')}</Badge>
        ),
    },
    {
      id: 'storage',
      header: t('datasets.storage'),
      cell: (dataset) => (
        <span className="truncate font-mono text-xs text-muted-foreground">
          {dataset.bucket}
          {dataset.prefix ? `/${dataset.prefix}` : ''}
        </span>
      ),
    },
    {
      id: 'owner',
      header: t('common.owner'),
      sortValue: (dataset) => dataset.ownerId,
      cell: (dataset) => <span className="text-xs text-muted-foreground">{dataset.ownerId || '—'}</span>,
    },
    {
      id: 'updatedAt',
      header: t('common.updatedAt'),
      sortValue: (dataset) => dataset.updatedAt,
      cell: (dataset) => (
        <span className="text-xs text-muted-foreground">{formatRelative(dataset.updatedAt, locale)}</span>
      ),
    },
  ];

  return (
    <div className="space-y-4">
      <SectionHeader
        title={t('datasets.title')}
        description={t('datasets.subtitle')}
        actions={
          <div className="flex items-center gap-2">
            <SearchInput
              value={search}
              onValueChange={setSearch}
              label={t('common.search')}
              className="w-56"
            />
            <Button variant="primary" onClick={() => setCreating(true)}>
              <Plus aria-hidden />
              {t('datasets.create')}
            </Button>
          </div>
        }
      />

      <Card>
        <DataTable
          data={datasets.data}
          columns={columns}
          rowKey={(dataset) => dataset.id}
          isLoading={datasets.isLoading}
          isError={datasets.isError}
          error={datasets.error}
          onRetry={() => void datasets.refetch()}
          search={search}
          onResetSearch={() => setSearch('')}
          onRowClick={(dataset) => onSelect(dataset.id)}
          defaultSort={{ columnId: 'updatedAt', direction: 'desc' }}
          emptyState={
            <EmptyState
              icon={Database}
              title={t('datasets.empty')}
              description={t('datasets.emptyHint')}
              action={
                <Button variant="primary" onClick={() => setCreating(true)}>
                  <Plus aria-hidden />
                  {t('datasets.create')}
                </Button>
              }
            />
          }
          rowActions={(dataset) => (
            <>
              <DropdownMenuItem onSelect={() => onSelect(dataset.id)}>{t('common.open')}</DropdownMenuItem>
              <DropdownMenuItem
                destructive
                onSelect={() =>
                  ask({
                    title: t('datasets.deleteTitle'),
                    description: t('datasets.deleteWarning'),
                    confirmLabel: t('common.delete'),
                    destructive: true,
                    confirmationValue: dataset.name,
                    onConfirm: () => remove.mutateAsync(dataset.id),
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

      {selected ? (
        <>
          <DatasetExplorer dataset={selected} />
          <DatasetPermissions dataset={selected} />
        </>
      ) : null}

      <CreateDatasetSheet open={creating} onOpenChange={setCreating} />
      {dialog}
    </div>
  );
}
