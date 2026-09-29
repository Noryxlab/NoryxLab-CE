import * as React from 'react';
import { useNavigate, useParams } from 'react-router';
import { useMutation, useQuery } from '@tanstack/react-query';
import { AlertTriangle, Trash2 } from 'lucide-react';
import { PageHeader } from '@/components/common/page-header';
import { ProjectVariablesSection } from '@/features/projects/project-variables';
import { useConfirm } from '@/components/common/confirm-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardHeaderText, CardTitle, CardDescription, CardFooter } from '@/components/ui/card';
import { Field } from '@/components/ui/field';
import { Input, Textarea } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { useToast } from '@/components/ui/toast';
import { useAdminUsers, useOrganizations, useProject, qk, useInvalidate } from '@/lib/api/queries';
import { projectsApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { presentStorage, STORAGE_PRESETS } from '@/lib/presenters';
import { useAuth } from '@/lib/auth';
import { ProjectQuotaCard } from '@/features/projects/project-quota';

export function ProjectSettingsPage() {
  const t = useT();
  const { locale } = useI18n();
  const { projectId } = useParams<{ projectId: string }>();
  const navigate = useNavigate();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { isAdmin } = useAuth();
  const { dialog, ask } = useConfirm();

  const project = useProject(projectId);
  const organizations = useOrganizations();
  const users = useAdminUsers();

  const [name, setName] = React.useState('');
  const [description, setDescription] = React.useState('');
  const [storageSize, setStorageSize] = React.useState('default');
  const [ownerType, setOwnerType] = React.useState<'user' | 'organization'>('user');
  const [ownerId, setOwnerId] = React.useState('');

  // Seed the form once the project arrives, without clobbering edits in flight.
  React.useEffect(() => {
    if (!project.data) return;
    setName(project.data.name);
    setDescription(project.data.description);
    setStorageSize(project.data.workspaceStorageSize || 'default');
    setOwnerType(project.data.ownerType === 'organization' ? 'organization' : 'user');
    setOwnerId(project.data.ownerId);
  }, [project.data]);

  const save = useMutation({
    mutationFn: () =>
      projectsApi.update(projectId as string, {
        name: name.trim(),
        description: description.trim(),
        workspaceStorageSize: storageSize,
      }),
    onSuccess: () => {
      invalidate(qk.projects);
      toast.success(t('common.save'), t('projectOverview.settingsTitle'));
    },
    onError: (error) => toast.error(error, t('projectOverview.settingsTitle')),
  });

  const transfer = useMutation({
    mutationFn: () => projectsApi.setOwner(projectId as string, { ownerType, ownerId }),
    onSuccess: () => {
      invalidate(qk.projects);
      toast.success(t('projects.transferOwnership'));
    },
    onError: (error) => toast.error(error, t('projects.transferOwnership')),
  });

  /* Ce que la suppression detruirait, lu avant de demander quoi que ce soit.
   *
   *  La suppression effacait les espaces de travail en silence et laissait les
   *  applications et les travaux derriere elle. Une application n'est jamais
   *  arretee automatiquement : celle dont le projet a disparu continue de
   *  tourner contre un identifiant qui ne resout plus rien, elle coute une
   *  carte, elle sert une adresse, et elle n'apparait sur aucun ecran puisque
   *  tous la trouvent par le projet qu'elle n'a plus.
   *
   *  Decider de detruire suppose d'avoir vu. */
  const workloads = useQuery({
    queryKey: ['project-workloads', projectId],
    queryFn: () => projectsApi.workloads(projectId as string),
    enabled: Boolean(projectId),
  });
  const held = workloads.data?.items ?? [];
  // Nommes explicitement : une cle construite a la volee echappe au typage et
  // se casse en silence le jour ou une sorte s'ajoute.
  const kindLabel: Record<string, string> = {
    workspace: t('nav.workspaces'),
    app: t('nav.apps'),
    job: t('nav.jobs'),
  };
  const running = workloads.data?.running ?? 0;

  const remove = useMutation({
    mutationFn: () => projectsApi.remove(projectId as string, held.length > 0),
    onSuccess: () => {
      invalidate(qk.projects);
      navigate('/projects');
    },
    onError: (error) => toast.error(error, t('projects.deleteTitle')),
  });

  const canManageOwner = project.data?.canManageOwner || isAdmin;
  const ownerOptions =
    ownerType === 'organization'
      ? (organizations.data ?? []).map((organization) => ({
          value: organization.alias ?? organization.id,
          label: organization.name,
        }))
      : (users.data ?? []).map((user) => ({
          value: user.username ?? user.id,
          label: user.username ?? user.id,
          hint: user.email ?? undefined,
        }));

  if (project.isLoading) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-7 w-56" />
        <Skeleton className="h-52 w-full" />
      </div>
    );
  }

  return (
    <div className="space-y-5">
      <PageHeader
        title={t('projectOverview.settingsTitle')}
        description={t('projectOverview.settingsHint')}
      />

      <Card>
        <CardHeader>
          <CardHeaderText>
            <CardTitle>{t('common.settings')}</CardTitle>
          </CardHeaderText>
        </CardHeader>
        <CardContent className="max-w-xl space-y-4">
          <Field label={t('projects.nameLabel')} description={t('projects.nameHint')} required>
            <Input value={name} onChange={(event) => setName(event.target.value)} maxLength={120} />
          </Field>
          <Field label={t('common.description')} description={t('projects.descriptionHint')}>
            <Textarea
              value={description}
              onChange={(event) => setDescription(event.target.value)}
              rows={3}
              maxLength={500}
            />
          </Field>
          <Field
            label={t('projects.storageSizeLabel')}
            description={t('projects.storageSizeHint')}
          >
            <Select
              value={storageSize}
              onValueChange={setStorageSize}
              options={[
                { value: 'default', label: t('projects.storageSizeDefault') },
                ...STORAGE_PRESETS.map((preset) => ({
                  value: preset.value,
                  label: presentStorage(preset.gib, locale),
                })),
              ]}
            />
          </Field>
        </CardContent>
        <CardFooter className="justify-end">
          <Button
            variant="primary"
            loading={save.isPending}
            disabled={!name.trim()}
            onClick={() => save.mutate()}
          >
            {t('common.save')}
          </Button>
        </CardFooter>
      </Card>

      <ProjectVariablesSection projectId={projectId as string} />

      {/* Shown to every member: the person whose launch was refused is the one
          who needs to see the number. Editing stays with an administrator. */}
      <ProjectQuotaCard projectId={projectId as string} canEdit={isAdmin} />

      {canManageOwner ? (
        <Card>
          <CardHeader>
            <CardHeaderText>
              <CardTitle>{t('projects.transferOwnership')}</CardTitle>
              <CardDescription>{t('datasets.permissionsHint')}</CardDescription>
            </CardHeaderText>
          </CardHeader>
          <CardContent className="grid max-w-xl gap-4 sm:grid-cols-2">
            <Field label={t('projects.ownerTypeLabel')}>
              <Select
                value={ownerType}
                onValueChange={(value) => {
                  setOwnerType(value === 'organization' ? 'organization' : 'user');
                  setOwnerId('');
                }}
                options={[
                  { value: 'user', label: t('common.user') },
                  { value: 'organization', label: t('common.organization') },
                ]}
              />
            </Field>
            <Field label={t('projects.ownerLabel')}>
              <Select
                value={ownerId}
                onValueChange={setOwnerId}
                options={ownerOptions}
                placeholder={t('common.search')}
              />
            </Field>
          </CardContent>
          <CardFooter className="justify-end">
            <Button
              variant="secondary"
              loading={transfer.isPending}
              disabled={!ownerId}
              onClick={() => transfer.mutate()}
            >
              {t('projects.transferOwnership')}
            </Button>
          </CardFooter>
        </Card>
      ) : null}

      <Card className="border-danger/30">
        <CardHeader>
          <CardHeaderText>
            <CardTitle className="text-danger">{t('projects.deleteTitle')}</CardTitle>
            <CardDescription>{t('projects.deleteWarning')}</CardDescription>
          </CardHeaderText>
        </CardHeader>
        {held.length > 0 ? (
          <CardContent>
            {/* Nommees une par une, et non comptees. Onze objets est un
                chiffre ; "l'espace de travail de Katerina, en marche" est une
                decision. */}
            <div className="flex items-start gap-2 rounded-md border border-danger/30 bg-danger/5 p-3 text-sm">
              <AlertTriangle className="mt-0.5 size-4 shrink-0 text-danger" aria-hidden />
              <div className="min-w-0 space-y-2">
                <p className="font-medium text-danger">
                  {t('projects.deleteHoldsWarning', { count: held.length, running })}
                </p>
                <ul className="space-y-1">
                  {held.map((entry) => (
                    <li key={`${entry.kind}-${entry.id}`} className="flex items-center gap-2">
                      <Badge tone="outline">{kindLabel[entry.kind] ?? entry.kind}</Badge>
                      <span className="truncate">{entry.name}</span>
                      {entry.status ? (
                        <span className="text-xs text-muted-foreground">{entry.status}</span>
                      ) : null}
                    </li>
                  ))}
                </ul>
              </div>
            </div>
          </CardContent>
        ) : null}
        <CardFooter className="justify-end">
          <Button
            variant="danger-outline"
            onClick={() =>
              ask({
                title: t('projects.deleteTitle'),
                description:
                  held.length > 0
                    ? t('projects.deleteHoldsConfirm', { count: held.length, running })
                    : t('projects.deleteWarning'),
                confirmLabel: t('common.delete'),
                destructive: true,
                // Typing the project name is the guard for an action that
                // stops every workload the team has running.
                confirmationValue: project.data?.name ?? '',
                confirmationLabel: t('projects.nameLabel'),
                onConfirm: () => remove.mutateAsync(),
              })
            }
          >
            <Trash2 aria-hidden />
            {t('common.delete')}
          </Button>
        </CardFooter>
      </Card>

      {dialog}
    </div>
  );
}
