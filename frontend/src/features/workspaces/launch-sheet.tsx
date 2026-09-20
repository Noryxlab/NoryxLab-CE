import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import {
  Sheet,
  SheetBody,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { Skeleton } from '@/components/ui/skeleton';
import { useToast } from '@/components/ui/toast';
import {
  useEnvironments,
  useHardwareTiers,
  useProjectDatasets,
  qk,
  useInvalidate,
} from '@/lib/api/queries';
import { workspacesApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { presentIde, presentTier } from '@/lib/presenters';

/**
 * Workspace launch.
 *
 * Three changes from the previous `<details>` row of six bare inputs:
 *
 *  - hardware tiers are presented by size name and specs, rather than the
 *    `1x4` id that needed a help note to explain it;
 *  - storage is gone from this form entirely. It was a preset list that read
 *    the same on every launch, and an Essilor engineer asked for it to go:
 *    capacity is infrastructure, decided once for the project, not a question
 *    to put to whoever is opening a notebook. It now lives in the project's
 *    settings and defaults to 10 Go;
 *  - the IDE is shown as a read-only property of the environment instead of a
 *    `<select disabled>`, which advertised a choice the user did not have.
 */
export function LaunchWorkspaceSheet({
  projectId,
  open,
  onOpenChange,
}: {
  projectId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();

  const environments = useEnvironments();
  const tiers = useHardwareTiers();
  // A dataset the installation labelled as health data. The assistant is
  // withheld from a workspace that mounts one, so the sheet says so here
  // rather than letting somebody find it missing later.
  const datasets = useProjectDatasets(projectId);
  const regulatedDatasets = (datasets.data ?? []).filter(
    (dataset) => dataset.classification === 'hds',
  );

  const [environmentId, setEnvironmentId] = React.useState('');
  const [tierId, setTierId] = React.useState('');
  const [name, setName] = React.useState('');

  const ready = environments.data ?? [];
  const usable = React.useMemo(
    () => ready.filter((environment) => environment.destinationImage),
    [ready],
  );

  // Preselect the first usable environment and the tier the backend marks as
  // default, so the common case is one click.
  React.useEffect(() => {
    if (usable.length === 0) return;
    // A selection kept from a previous opening is only kept while it still
    // names something. The sheet does not reset it on close, and an
    // environment can disappear between two openings - a rebuild changes the
    // reference, a kind is withdrawn - after which the form holds an id that
    // matches nothing and submits an environment nobody chose.
    const stillThere = usable.some((candidate) => candidate.id === environmentId);
    if (!environmentId || !stillThere) setEnvironmentId(usable[0]?.id ?? '');
  }, [usable, environmentId]);

  React.useEffect(() => {
    if (tierId || !tiers.data?.length) return;
    setTierId((tiers.data.find((tier) => tier.default) ?? tiers.data[0])?.id ?? '');
  }, [tiers.data, tierId]);

  React.useEffect(() => {
    if (!open) {
      setName('');
    }
  }, [open]);

  const environment = usable.find((candidate) => candidate.id === environmentId);

  const mutation = useMutation({
    mutationFn: () =>
      workspacesApi.create({
        projectId,
        // The image alone. The kind is a property of the environment, and the
        // platform derives it from the image - sending both invited them to
        // disagree, and they did: this form once showed one environment while
        // holding another and sent a pair nobody had chosen.
        image: environment?.destinationImage,
        name: name.trim() || undefined,
        hardwareTier: tierId || undefined,
      }),
    onSuccess: () => {
      invalidate(qk.workspaces(projectId), qk.projects);
      onOpenChange(false);
      toast.success(t('workspaces.launched'), t('workspaces.title'));
    },
    onError: (error) => toast.error(error, t('workspaces.createTitle')),
  });

  const loading = environments.isLoading || tiers.isLoading;

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent aria-describedby={undefined}>
        <form
          onSubmit={(event) => {
            event.preventDefault();
            if (environmentId) mutation.mutate();
          }}
          className="flex min-h-0 flex-1 flex-col"
        >
          <SheetHeader>
            <SheetTitle>{t('workspaces.createTitle')}</SheetTitle>
            <SheetDescription>{t('workspaces.createHint')}</SheetDescription>
          </SheetHeader>

          <SheetBody>
            {loading ? (
              <div className="space-y-4">
                {Array.from({ length: 4 }, (_, index) => (
                  <div key={index} className="space-y-1.5">
                    <Skeleton className="h-3 w-24" />
                    <Skeleton className="h-9 w-full" />
                  </div>
                ))}
              </div>
            ) : (
              <>
                {/* Said before launching, not discovered afterwards.
                    The assistant stays available on regulated data - taking
                    it away exactly where the work happens would remove the
                    product to protect it - so what the platform owes the
                    person is the fact, at the moment they can still act on
                    it. Not a warning to dismiss: a description of what the
                    tool does. */}
                {regulatedDatasets.length > 0 ? (
                  <p className="rounded-md border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-xs text-foreground">
                    {t('workspaces.assistantSendsDataOffSite', {
                      datasets: regulatedDatasets.map((dataset) => dataset.name).join(', '),
                    })}
                  </p>
                ) : null}
                <Field
                  label={t('workspaces.environmentLabel')}
                  description={t('workspaces.environmentHint')}
                  required
                >
                  <Select
                    value={environmentId}
                    onValueChange={setEnvironmentId}
                    placeholder={t('environments.title')}
                    options={usable.map((item) => ({
                      value: item.id,
                      label: item.name,
                      hint: item.workspaceIdes?.map(presentIde).join(', ') || undefined,
                    }))}
                  />
                </Field>

                {/* No IDE field.
                  *
                  * It restated what the environment name already says, and a
                  * second rendering of one fact is a second thing that can be
                  * wrong: it read JupyterLab under an environment called
                  * noryx-vscode for a whole afternoon. Each option in the list
                  * above carries its kinds as a hint, which is where the
                  * question is actually asked. */}

                <Field label={t('workspaces.tierLabel')} description={t('workspaces.tierHint')} required>
                  <Select
                    value={tierId}
                    onValueChange={setTierId}
                    options={(tiers.data ?? []).map((tier) => {
                      const presented = presentTier(tier, locale);
                      return { value: tier.id, label: presented.name, hint: presented.specs };
                    })}
                  />
                </Field>

                <Field label={t('workspaces.nameLabel')} description={t('workspaces.nameHint')}>
                  <Input
                    value={name}
                    onChange={(event) => setName(event.target.value)}
                    placeholder={t('workspaces.namePlaceholder')}
                    maxLength={80}
                  />
                </Field>
              </>
            )}
          </SheetBody>

          <SheetFooter>
            <Button type="button" variant="secondary" onClick={() => onOpenChange(false)}>
              {t('common.cancel')}
            </Button>
            <Button
              type="submit"
              variant="primary"
              loading={mutation.isPending}
              disabled={!environmentId || !tierId}
            >
              {t('workspaces.create')}
            </Button>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  );
}
