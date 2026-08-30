import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Button } from '@/components/ui/button';
import { Field, ReadOnlyValue } from '@/components/ui/field';
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
import { useEnvironments, useHardwareTiers, qk, useInvalidate } from '@/lib/api/queries';
import { workspacesApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { presentIde, presentStorage, presentTier, STORAGE_PRESETS } from '@/lib/presenters';

/**
 * Workspace launch.
 *
 * Three changes from the previous `<details>` row of six bare inputs:
 *
 *  - hardware tiers are presented by size name and specs, rather than the
 *    `1x4` id that needed a help note to explain it;
 *  - storage is a set of presets instead of a free-text field that expected a
 *    Kubernetes quantity ("Stockage (ex. 10Gi)");
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

  const [environmentId, setEnvironmentId] = React.useState('');
  const [tierId, setTierId] = React.useState('');
  const [name, setName] = React.useState('');
  const [storage, setStorage] = React.useState('10Gi');

  const ready = environments.data ?? [];
  const usable = React.useMemo(
    () => ready.filter((environment) => environment.destinationImage),
    [ready],
  );

  // Preselect the first usable environment and the tier the backend marks as
  // default, so the common case is one click.
  React.useEffect(() => {
    if (!environmentId && usable.length > 0) setEnvironmentId(usable[0]?.id ?? '');
  }, [usable, environmentId]);

  React.useEffect(() => {
    if (tierId || !tiers.data?.length) return;
    setTierId((tiers.data.find((tier) => tier.default) ?? tiers.data[0])?.id ?? '');
  }, [tiers.data, tierId]);

  React.useEffect(() => {
    if (!open) {
      setName('');
      setStorage('10Gi');
    }
  }, [open]);

  const environment = usable.find((candidate) => candidate.id === environmentId);
  const ide = environment?.workspaceIdes?.[0] ?? null;

  const mutation = useMutation({
    mutationFn: () =>
      workspacesApi.create({
        projectId,
        // The API has no environment id: it takes the image directly, and an
        // ide that selects the default image when none is supplied.
        image: environment?.destinationImage,
        ide: ide ?? undefined,
        name: name.trim() || undefined,
        hardwareTier: tierId || undefined,
        storageSize: storage,
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

                <ReadOnlyValue
                  label={t('workspaces.ideLabel')}
                  value={presentIde(ide)}
                  description={t('workspaces.ideHint')}
                />

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

                <Field label={t('workspaces.storageLabel')} description={t('workspaces.storageHint')}>
                  <Select
                    value={storage}
                    onValueChange={setStorage}
                    options={STORAGE_PRESETS.map((preset) => ({
                      value: preset.value,
                      label: presentStorage(preset.gib, locale),
                    }))}
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
