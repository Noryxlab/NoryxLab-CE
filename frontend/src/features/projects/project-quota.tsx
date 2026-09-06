import * as React from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { Gauge } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardFooter, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { useToast } from '@/components/ui/toast';
import { adminApi, projectsApi } from '@/lib/api/endpoints';
import { useInvalidate } from '@/lib/api/queries';
import { useT } from '@/lib/i18n';
import type { ProjectQuota } from '@/lib/api/types';

/**
 * What this project may run at once, against what it is running.
 *
 * Shown to every member and editable by an administrator, deliberately: the
 * person whose launch was refused is the one who needs to see the number, and
 * making them ask an administrator turns a limit into a support ticket.
 *
 * A dimension left at 0 is unlimited. That reads oddly as a form field and is
 * the honest model: a project without a quota is not a project with a limit of
 * zero, and the two must never be the same value in a database.
 */
export function ProjectQuotaCard({ projectId, canEdit }: { projectId: string; canEdit: boolean }) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const key = ['projects', projectId, 'quota'] as const;

  const state = useQuery({ queryKey: key, queryFn: () => projectsApi.quota(projectId) });
  const [draft, setDraft] = React.useState<ProjectQuota | null>(null);

  React.useEffect(() => {
    if (state.data?.quota) setDraft(state.data.quota);
  }, [state.data]);

  const save = useMutation({
    mutationFn: () => adminApi.setProjectQuota(projectId, draft ?? {}),
    onSuccess: () => {
      invalidate(key);
      toast.success(t('common.save'), t('quota.title'));
    },
    onError: (error) => toast.error(error, t('quota.title')),
  });

  if (state.isLoading) return <Skeleton className="h-40 w-full" />;

  const usage = state.data?.usage;
  const quota = state.data?.quota;
  const dimensions: Array<{ label: string; used: number; limit: number; unit?: string }> = [
    { label: t('quota.vcpu'), used: usage?.vcpu ?? 0, limit: quota?.maxVcpu ?? 0 },
    { label: t('quota.memory'), used: usage?.memoryGib ?? 0, limit: quota?.maxMemoryGib ?? 0, unit: 'Gi' },
    { label: t('quota.workspaces'), used: usage?.workspaces ?? 0, limit: quota?.maxWorkspaces ?? 0 },
    { label: t('quota.jobs'), used: usage?.jobs ?? 0, limit: quota?.maxJobs ?? 0 },
  ];

  const set = (field: keyof ProjectQuota, value: string) =>
    setDraft((current) =>
      current ? { ...current, [field]: Number(value) || 0 } : current,
    );

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle className="flex items-center gap-2">
            <Gauge className="size-4" aria-hidden />
            {t('quota.title')}
          </CardTitle>
          <CardDescription>
            {state.data?.limited ? t('quota.hint') : t('quota.unlimited')}
          </CardDescription>
        </CardHeaderText>
      </CardHeader>

      <CardContent className="space-y-4">
        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {dimensions.map((dimension) => (
            <div key={dimension.label} className="rounded-md border border-border p-3">
              <p className="text-xs text-muted-foreground">{dimension.label}</p>
              <p className="text-sm font-medium tabular-nums">
                {dimension.used}
                {dimension.unit ?? ''}
                <span className="text-muted-foreground">
                  {' / '}
                  {dimension.limit > 0 ? `${dimension.limit}${dimension.unit ?? ''}` : t('quota.noLimit')}
                </span>
              </p>
            </div>
          ))}
        </div>

        {canEdit && draft ? (
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
            <Field label={t('quota.vcpu')} description={t('quota.zeroMeansNoLimit')}>
              <Input value={String(draft.maxVcpu)} onChange={(event) => set('maxVcpu', event.target.value)} inputMode="decimal" />
            </Field>
            <Field label={`${t('quota.memory')} (Gi)`}>
              <Input value={String(draft.maxMemoryGib)} onChange={(event) => set('maxMemoryGib', event.target.value)} inputMode="decimal" />
            </Field>
            <Field label={t('quota.workspaces')}>
              <Input value={String(draft.maxWorkspaces)} onChange={(event) => set('maxWorkspaces', event.target.value)} inputMode="numeric" />
            </Field>
            <Field label={t('quota.jobs')}>
              <Input value={String(draft.maxJobs)} onChange={(event) => set('maxJobs', event.target.value)} inputMode="numeric" />
            </Field>
          </div>
        ) : null}
      </CardContent>

      {canEdit ? (
        <CardFooter className="justify-end">
          <Button variant="primary" loading={save.isPending} onClick={() => save.mutate()}>
            {t('common.save')}
          </Button>
        </CardFooter>
      ) : null}
    </Card>
  );
}
