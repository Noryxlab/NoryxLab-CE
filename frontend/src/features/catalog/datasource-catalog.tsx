import * as React from 'react';
import { AccessPanel, type AccessEntry } from '@/components/common/access-panel';
import { useMutation } from '@tanstack/react-query';
import { Plug, Plus, RotateCw, ShieldCheck, Trash2, UserRoundCog } from 'lucide-react';
import { OwnerTransfer } from '@/components/common/owner';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { LogViewer } from '@/components/common/log-viewer';
import { useConfirm } from '@/components/common/confirm-dialog';
import { SectionHeader } from '@/components/common/page-header';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { Badge, StatusBadge } from '@/components/ui/badge';
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
import { useQuery } from '@tanstack/react-query';
import {
  qk,
  useCan,
  useDatasourceAccess,
  useDatasourceDefinitions,
  useDatasources,
  useHardwareTiers,
  useInvalidate,
  useSecrets,
} from '@/lib/api/queries';
import { datasourcesApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { formatRelative } from '@/lib/format';
import type { Datasource } from '@/lib/api/types';

const SSL_MODES = ['disable', 'require', 'verify-ca', 'verify-full'];

/** Ceder un connecteur, dans la meme forme que partout ailleurs.
 *
 *  C'est l'objet du catalogue qui survit le plus souvent a la personne qui l'a
 *  declare : il pointe vers un systeme que l'organisation fait tourner, et il
 *  partait avec son auteur. */
/** Qui peut utiliser ce connecteur.
 *
 *  Le dernier objet du catalogue sans partage : une equipe travaillant sur la
 *  meme base en declarait un chacun, meme hote et memes identifiants recopies
 *  a la main. Pose sous la propriete, comme sur un dataset, parce que c'est
 *  une seule conversation - qui le detient, et qui peut s'en servir. */
function DatasourcePermissions({ datasource }: { datasource: Datasource }) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const access = useDatasourceAccess(datasource.id);
  const grant = useMutation({
    mutationFn: (input: { subjectType: string; subjectId: string; role: string }) => datasourcesApi.grant(datasource.id, input.subjectType, input.subjectId, input.role),
    onSuccess: () => invalidate(qk.datasourceAccess(datasource.id)),
    onError: (error) => toast.error(error, t('access.grant')),
  });
  const revoke = useMutation({
    mutationFn: (entry: AccessEntry) => datasourcesApi.revoke(datasource.id, entry.subjectType, entry.subjectId),
    onSuccess: () => invalidate(qk.datasourceAccess(datasource.id)),
    onError: (error) => toast.error(error, t('access.revoke')),
  });
  return (
    <AccessPanel
      title={t('access.title')}
      hint={t('datasources.permissionsHint')}
      rule="access.connectorHint"
      owner={{ type: datasource.ownerType, id: datasource.ownerId || datasource.ownerUserId, name: datasource.ownerName }}
      entries={access.data}
      isLoading={access.isLoading}
      isError={access.isError}
      error={access.error}
      onRetry={() => void access.refetch()}
      
      onGrant={(subjectType, subjectId, role) => grant.mutate({ subjectType, subjectId, role })}
      granting={grant.isPending}
      onRevoke={(entry) => revoke.mutate(entry)}
    />
  );
}

/** Changing what a connector points at: Write on it, or its ownership. A
 *  database the platform runs keeps its own connection details, so only its
 *  name is offered there. */
function DatasourceDefinitionForm({ datasource }: { datasource: Datasource }) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const secrets = useSecrets();
  const managed = datasource.source === 'internal';
  const [draft, setDraft] = React.useState({
    name: datasource.name,
    host: datasource.host ?? '',
    port: String(datasource.port ?? ''),
    database: datasource.database ?? '',
    username: datasource.username ?? '',
    passwordSecret: datasource.passwordSecret ?? '',
    sslMode: datasource.sslMode ?? 'disable',
  });
  const save = useMutation({
    mutationFn: () =>
      datasourcesApi.update(
        datasource.id,
        managed
          ? { name: draft.name.trim() }
          : {
              name: draft.name.trim(),
              host: draft.host.trim(),
              port: Number(draft.port) || 0,
              database: draft.database.trim(),
              username: draft.username.trim(),
              passwordSecret: draft.passwordSecret,
              sslMode: draft.sslMode,
            },
      ),
    onSuccess: () => {
      invalidate(qk.datasources);
      toast.success(draft.name.trim(), t('datasources.definitionSaved'));
    },
    onError: (error) => toast.error(error, t('datasources.definition')),
  });
  const set = (key: keyof typeof draft) => (event: React.ChangeEvent<HTMLInputElement>) =>
    setDraft((previous) => ({ ...previous, [key]: event.target.value }));

  return (
    <div className="space-y-3">
      <SectionHeader
        title={t('datasources.definition')}
        description={managed ? t('datasources.definitionManagedHint') : t('datasources.definitionHint')}
      />
      <Field label={t('common.name')} required>
        <Input value={draft.name} onChange={set('name')} />
      </Field>
      {managed ? null : (
        <>
          <div className="grid gap-3 sm:grid-cols-[2fr_1fr]">
            <Field label={t('common.host')} required>
              <Input value={draft.host} onChange={set('host')} />
            </Field>
            <Field label={t('common.port')}>
              <Input value={draft.port} onChange={set('port')} />
            </Field>
          </div>
          <Field label={t('datasources.databaseLabel')}>
            <Input value={draft.database} onChange={set('database')} />
          </Field>
          <Field label={t('datasources.usernameLabel')}>
            <Input value={draft.username} onChange={set('username')} />
          </Field>
          <Field label={t('datasources.passwordSecretLabel')} description={t('datasources.passwordSecretHint')}>
            <Select
              value={draft.passwordSecret}
              onValueChange={(value) => setDraft((previous) => ({ ...previous, passwordSecret: value }))}
              options={(secrets.data ?? []).map((secret) => ({ value: secret.name, label: secret.name }))}
              placeholder={t('datasources.passwordSecretLabel')}
            />
          </Field>
        </>
      )}
      <div className="flex justify-end">
        <Button variant="primary" loading={save.isPending} disabled={!draft.name.trim()} onClick={() => save.mutate()}>
          {t('common.save')}
        </Button>
      </div>
    </div>
  );
}

function DatasourceOwnershipSheet({
  datasource,
  open,
  onOpenChange,
}: {
  datasource: Datasource | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();

  const transfer = useMutation({
    mutationFn: (input: { ownerType: string; ownerId: string }) =>
      datasourcesApi.setOwner(datasource?.id ?? '', input),
    onSuccess: () => {
      invalidate(qk.datasources);
      toast.success(t('projects.transferOwnership'));
      onOpenChange(false);
    },
    onError: (error) => toast.error(error, t('projects.transferOwnership')),
  });

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent title={datasource?.name ?? t('datasources.manage')}>
        <SheetBody>
          {datasource ? (
            <div className="space-y-6">
              <DatasourceDefinitionForm datasource={datasource} />
              <OwnerTransfer
                owner={datasource}
                pending={transfer.isPending}
                onTransfer={(input) => transfer.mutate(input)}
                label={t('projects.transferOwnership')}
              />
              <DatasourcePermissions datasource={datasource} />
            </div>
          ) : null}
        </SheetBody>
      </SheetContent>
    </Sheet>
  );
}

export function DatasourceCatalog() {
  const t = useT();
  const can = useCan();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();

  const datasources = useDatasources();
  const definitions = useDatasourceDefinitions();
  const secrets = useSecrets();

  const [creating, setCreating] = React.useState(false);
  const [logsFor, setLogsFor] = React.useState<Datasource | null>(null);
  const [transferred, setTransferred] = React.useState<Datasource | null>(null);

  // Two acts, not one. The screen promised both - "connect an existing
  // database, or deploy a service the platform manages" - and only ever
  // called the connector route, with the engine catalogue as a mere type
  // menu: somebody picking PostgreSQL 17 believed they were creating a
  // database and was then asked for a host.
  const [act, setAct] = React.useState<'connect' | 'create'>('connect');
  const [storageSize, setStorageSize] = React.useState('10Gi');
  const [hardwareTier, setHardwareTier] = React.useState('');
  const tiers = useHardwareTiers();
  const [definitionId, setDefinitionId] = React.useState('');
  const [name, setName] = React.useState('');
  const [host, setHost] = React.useState('');
  const [port, setPort] = React.useState('5432');
  const [database, setDatabase] = React.useState('');
  const [username, setUsername] = React.useState('');
  const [passwordSecret, setPasswordSecret] = React.useState('');
  const [sslMode, setSslMode] = React.useState('require');
  const [touched, setTouched] = React.useState(false);

  const logs = useQuery({
    queryKey: ['datasources', logsFor?.id, 'logs'],
    queryFn: () => datasourcesApi.logs(logsFor?.id as string),
    enabled: Boolean(logsFor?.id),
  });

  const definition = definitions.data?.find((candidate) => candidate.id === definitionId);

  React.useEffect(() => {
    if (definition?.defaultPort) setPort(String(definition.defaultPort));
  }, [definition]);

  React.useEffect(() => {
    if (!creating) {
      setName('');
      setHost('');
      setDatabase('');
      setUsername('');
      setPasswordSecret('');
      setTouched(false);
    }
  }, [creating]);

  const create = useMutation({
    mutationFn: () =>
      act === 'create'
        ? datasourcesApi.createService({
            name: name.trim(),
            definitionId,
            database: database.trim(),
            username: username.trim(),
            storageSize: storageSize.trim(),
            hardwareTier,
          })
        : datasourcesApi.create({
            name: name.trim(),
            serviceDefinitionId: definitionId || undefined,
            type: definition?.type ?? 'postgresql',
            host: host.trim(),
            port: Number(port) || 5432,
            database: database.trim(),
            username: username.trim(),
            passwordSecret,
            sslMode,
          }),
    onSuccess: () => {
      invalidate(qk.datasources);
      setCreating(false);
      toast.success(name.trim(), t('datasources.create'));
    },
    onError: (error) => toast.error(error, t('datasources.createTitle')),
  });

  const validate = useMutation({
    mutationFn: (datasourceId: string) => datasourcesApi.validate(datasourceId),
    onSuccess: (result) => {
      invalidate(qk.datasources);
      if (result.reachable) toast.success(t('datasources.testOk'), t('datasources.test'));
      else toast.error(result.error ?? t('datasources.testFailed'), t('datasources.test'));
    },
    onError: (error) => toast.error(error, t('datasources.test')),
  });

  const restart = useMutation({
    mutationFn: (datasourceId: string) => datasourcesApi.restart(datasourceId),
    onSuccess: () => invalidate(qk.datasources),
    onError: (error) => toast.error(error, t('datasources.restart')),
  });

  const remove = useMutation({
    mutationFn: (datasourceId: string) => datasourcesApi.remove(datasourceId),
    onSuccess: () => {
      invalidate(qk.datasources);
      setLogsFor(null);
    },
    onError: (error) => toast.error(error, t('datasources.deleteTitle')),
  });

  const columns: Column<Datasource>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (datasource) => datasource.name,
      searchValue: (datasource) => `${datasource.name} ${datasource.host} ${datasource.database}`,
      cell: (datasource) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{datasource.name}</p>
          <p className="truncate font-mono text-xs text-muted-foreground">
            {datasource.host}
            {datasource.port ? `:${datasource.port}` : ''}
            {datasource.database ? `/${datasource.database}` : ''}
          </p>
        </div>
      ),
    },
    {
      id: 'type',
      header: t('common.type'),
      sortValue: (datasource) => datasource.type,
      cell: (datasource) => (
        <div className="flex flex-wrap items-center gap-1.5">
          <Badge tone="outline">{datasource.type}</Badge>
          <Badge tone={datasource.system ? 'brand' : 'neutral'}>
            {datasource.system ? t('datasources.managed') : t('datasources.external')}
          </Badge>
        </div>
      ),
    },
    {
      id: 'status',
      header: t('common.status'),
      sortValue: (datasource) => datasource.status ?? '',
      cell: (datasource) =>
        datasource.status ? (
          <StatusBadge status={datasource.status} locale={locale} />
        ) : (
          <span className="text-xs text-muted-foreground">—</span>
        ),
    },
    {
      id: 'projects',
      header: t('datasources.attached'),
      align: 'right',
      sortValue: (datasource) => datasource.attachedProjectIds?.length ?? 0,
      cell: (datasource) => (
        <span className="tabular-nums text-muted-foreground">
          {datasource.attachedProjectIds?.length ?? 0}
        </span>
      ),
    },
    {
      id: 'updatedAt',
      header: t('common.updatedAt'),
      sortValue: (datasource) => datasource.updatedAt,
      cell: (datasource) => (
        <span className="text-xs text-muted-foreground">
          {formatRelative(datasource.updatedAt, locale)}
        </span>
      ),
    },
  ];

  // A connector says where the database is; a creation says which engine and
  // how much disk, and the platform generates the credentials.
  const valid =
    act === 'create' ? Boolean(name.trim() && definitionId) : Boolean(name.trim() && host.trim());

  return (
    <div className="space-y-4">
      <SectionHeader
        title={t('datasources.title')}
        description={t('datasources.subtitle')}
        actions={can('declare-datasource') ? (<Button variant="primary" onClick={() => setCreating(true)}>
            <Plus aria-hidden />
            {t('datasources.create')}
          </Button>) : null}
      />

      <Card>
        <DataTable
          data={datasources.data}
          columns={columns}
          rowKey={(datasource) => datasource.id}
          isLoading={datasources.isLoading}
          isError={datasources.isError}
          error={datasources.error}
          onRetry={() => void datasources.refetch()}
          defaultSort={{ columnId: 'name', direction: 'asc' }}
          emptyState={
            <EmptyState
              icon={Plug}
              title={t('datasources.empty')}
              description={t('datasources.emptyHint')}
              action={can('declare-datasource') ? (<Button variant="primary" onClick={() => setCreating(true)}>
                  <Plus aria-hidden />
                  {t('datasources.create')}
                </Button>) : null}
            />
          }
          rowActions={(datasource) => (
            <>
              <DropdownMenuItem onSelect={() => validate.mutate(datasource.id)}>
                <ShieldCheck aria-hidden />
                {t('datasources.test')}
              </DropdownMenuItem>
              {datasource.system ? (
                <>
                  <DropdownMenuItem onSelect={() => setLogsFor(datasource)}>
                    {t('common.viewLogs')}
                  </DropdownMenuItem>
                  <DropdownMenuItem onSelect={() => restart.mutate(datasource.id)}>
                    <RotateCw aria-hidden />
                    {t('datasources.restart')}
                  </DropdownMenuItem>
                </>
              ) : null}
              <DropdownMenuItem onSelect={() => setTransferred(datasource)}>
                <UserRoundCog aria-hidden />
                {t('projects.transferOwnership')}
              </DropdownMenuItem>
              <DropdownMenuItem
                destructive
                onSelect={() =>
                  ask({
                    title: t('datasources.deleteTitle'),
                    description: t('datasources.deleteWarning'),
                    confirmLabel: t('common.delete'),
                    destructive: true,
                    confirmationValue: datasource.name,
                    onConfirm: () => remove.mutateAsync(datasource.id),
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

      {logsFor ? (
        <Card>
          <CardHeader>
            <CardHeaderText>
              <CardTitle>
                {t('common.logs')} — {logsFor.name}
              </CardTitle>
            </CardHeaderText>
            <Button variant="ghost" size="sm" onClick={() => setLogsFor(null)}>
              {t('common.close')}
            </Button>
          </CardHeader>
          <CardContent>
            <LogViewer
              content={logs.data}
              isLoading={logs.isLoading}
              downloadName={`${logsFor.name}.log`}
            />
          </CardContent>
        </Card>
      ) : null}

      <DatasourceOwnershipSheet
        datasource={transferred}
        open={transferred !== null}
        onOpenChange={(open) => {
          if (!open) setTransferred(null);
        }}
      />

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
              <SheetTitle>{t('datasources.createTitle')}</SheetTitle>
              <SheetDescription>{t('datasources.createHint')}</SheetDescription>
            </SheetHeader>
            <SheetBody>
              <Field label={t('datasources.actLabel')} description={t('datasources.actHint')}>
                <Select
                  value={act}
                  onValueChange={(value) => setAct(value === 'create' ? 'create' : 'connect')}
                  options={[
                    { value: 'connect', label: t('datasources.actConnect'), hint: t('datasources.actConnectHint') },
                    { value: 'create', label: t('datasources.actCreate'), hint: t('datasources.actCreateHint') },
                  ]}
                />
              </Field>
              <Field label={act === 'create' ? t('datasources.engineLabel') : t('datasources.typeLabel')}>
                <Select
                  value={definitionId}
                  onValueChange={setDefinitionId}
                  placeholder={t('datasources.typeLabel')}
                  options={(definitions.data ?? []).map((item) => ({
                    value: item.id,
                    label: item.name,
                    hint: item.description || item.type,
                  }))}
                />
              </Field>
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
              {act === 'create' ? null : (
              <div className="grid gap-4 sm:grid-cols-[2fr_1fr]">
                <Field
                  label={t('common.host')}
                  error={touched && !host.trim() ? t('common.required') : undefined}
                  required
                >
                  <Input
                    value={host}
                    onChange={(event) => setHost(event.target.value)}
                    onBlur={() => setTouched(true)}
                    className="font-mono text-xs"
                  />
                </Field>
                <Field label={t('common.port')}>
                  <Input
                    type="number"
                    inputMode="numeric"
                    value={port}
                    onChange={(event) => setPort(event.target.value)}
                  />
                </Field>
              </div>
              )}
              <Field label={t('datasources.databaseLabel')}>
                <Input value={database} onChange={(event) => setDatabase(event.target.value)} />
              </Field>
              <Field label={t('datasources.usernameLabel')}>
                <Input
                  value={username}
                  onChange={(event) => setUsername(event.target.value)}
                  autoComplete="off"
                />
              </Field>
              {act === 'create' ? (
                <>
                  <Field label={t('datasources.storageLabel')} description={t('datasources.storageHint')}>
                    <Input value={storageSize} onChange={(event) => setStorageSize(event.target.value)} />
                  </Field>
                  <Field label={t('datasources.tierLabel')} description={t('datasources.tierHint')}>
                    <Select
                      value={hardwareTier}
                      onValueChange={setHardwareTier}
                      placeholder={t('datasources.tierLabel')}
                      options={(tiers.data ?? []).map((tier) => ({ value: tier.id, label: tier.name }))}
                    />
                  </Field>
                </>
              ) : (
                <>
              <Field
                label={t('datasources.passwordSecretLabel')}
                description={t('datasources.passwordSecretHint')}
              >
                <Select
                  value={passwordSecret}
                  onValueChange={setPasswordSecret}
                  placeholder={t('secrets.title')}
                  options={(secrets.data ?? []).map((secret) => ({
                    value: secret.name,
                    label: secret.name,
                  }))}
                />
              </Field>
              <Field label={t('datasources.sslLabel')}>
                <Select
                  value={sslMode}
                  onValueChange={setSslMode}
                  options={SSL_MODES.map((mode) => ({ value: mode, label: mode }))}
                />
              </Field>
                </>
              )}
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
