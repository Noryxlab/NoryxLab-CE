import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Cpu, Plus, Trash2 } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { SectionHeader } from '@/components/common/page-header';
import { useConfirm } from '@/components/common/confirm-dialog';
import { useToast } from '@/components/ui/toast';
import { Skeleton } from '@/components/ui/skeleton';
import { adminApi } from '@/lib/api/endpoints';
import { qk, useAdminHardwareTiers, useInvalidate } from '@/lib/api/queries';
import { useT } from '@/lib/i18n';
import type { AdminHardwareTier } from '@/lib/api/types';

/**
 * The machine sizes, as an administrator maintains them.
 *
 * Four sizes used to be compiled into the binary, with their display names
 * hardcoded a second time in the interface's translation catalogues. Adding a
 * fifth, or naming them the way a customer names their machines, took a
 * release.
 *
 * Request and limit are shown as two separate columns because they answer two
 * different questions - what the scheduler sets aside, and what the machine may
 * reach - and an administrator who cannot see both cannot tell an oversubscribed
 * cluster from a wasted one.
 */

const BLANK: AdminHardwareTier = {
  id: '',
  name: '',
  description: '',
  cpuRequest: '100m',
  cpuLimit: '1',
  memoryRequest: '64Mi',
  memoryLimit: '4Gi',
  ephemeralStorageRequest: '64Mi',
  ephemeralStorageLimit: '8Gi',
  default: false,
  position: 0,
};

export function HardwareTiersSection() {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();
  const tiers = useAdminHardwareTiers();
  const [draft, setDraft] = React.useState<AdminHardwareTier | null>(null);

  const save = useMutation({
    mutationFn: (tier: AdminHardwareTier) => adminApi.saveHardwareTier(tier),
    onSuccess: () => {
      // Both lists: the one this screen shows and the one the launch form
      // reads. Refreshing only this one leaves a user picking from sizes that
      // no longer exist until their next reload.
      invalidate(qk.adminHardwareTiers, qk.hardwareTiers);
      setDraft(null);
      toast.success(t('admin.tierSaved'), t('admin.hardwareTiers'));
    },
    onError: (error) => toast.error(error, t('admin.hardwareTiers')),
  });

  const remove = useMutation({
    mutationFn: (tierId: string) => adminApi.removeHardwareTier(tierId),
    onSuccess: () => {
      invalidate(qk.adminHardwareTiers, qk.hardwareTiers);
      toast.success(t('admin.tierRemoved'), t('admin.hardwareTiers'));
    },
    onError: (error) => toast.error(error, t('admin.hardwareTiers')),
  });

  const items = tiers.data ?? [];

  return (
    <section className="space-y-3">
      {dialog}
      <SectionHeader
        title={t('admin.hardwareTiers')}
        description={t('admin.hardwareTiersHint')}
        actions={
          <Button
            variant="secondary"
            onClick={() => setDraft({ ...BLANK, position: items.length + 1 })}
          >
            <Plus className="size-4" />
            {t('admin.tierAdd')}
          </Button>
        }
      />

      {tiers.isLoading ? (
        <Skeleton className="h-40 w-full" />
      ) : (
        <div className="space-y-2">
          {items.map((tier) => (
            <TierRow
              key={tier.id}
              tier={tier}
              saving={save.isPending}
              onSave={(updated) => save.mutate(updated)}
              onRemove={() =>
                ask({
                  title: tier.name,
                  description: t('admin.tierRemoveConfirm'),
                  confirmLabel: t('common.delete'),
                  destructive: true,
                  onConfirm: () => remove.mutateAsync(tier.id),
                })
              }
            />
          ))}
          {draft ? (
            <TierRow
              tier={draft}
              creating
              saving={save.isPending}
              onSave={(created) => save.mutate(created)}
              onRemove={() => setDraft(null)}
            />
          ) : null}
        </div>
      )}

      <p className="text-xs text-muted-foreground">{t('admin.tierRunningUnaffected')}</p>
    </section>
  );
}

function TierRow({
  tier,
  creating = false,
  saving,
  onSave,
  onRemove,
}: {
  tier: AdminHardwareTier;
  creating?: boolean;
  saving: boolean;
  onSave: (tier: AdminHardwareTier) => void;
  onRemove: () => void;
}) {
  const t = useT();
  const [form, setForm] = React.useState(tier);
  React.useEffect(() => setForm(tier), [tier]);

  const set = <K extends keyof AdminHardwareTier>(key: K, value: AdminHardwareTier[K]) =>
    setForm((current) => ({ ...current, [key]: value }));

  const dirty = JSON.stringify(form) !== JSON.stringify(tier);
  const complete = form.id.trim() !== '' && form.name.trim() !== '';

  return (
    <Card>
      <CardContent className="grid gap-3 py-4 sm:grid-cols-2 lg:grid-cols-4">
        <Field label={t('admin.tierName')} required>
          <Input value={form.name} onChange={(event) => set('name', event.target.value)} maxLength={60} />
        </Field>
        <Field label={t('admin.tierId')} description={creating ? undefined : t('admin.tierIdHint')}>
          <Input
            value={form.id}
            onChange={(event) => set('id', event.target.value)}
            // The id is a reference held by running workspaces and by anything
            // that scripted a launch. Renaming it here would silently orphan
            // them, so it is set once, when the tier is created.
            disabled={!creating}
            maxLength={63}
          />
        </Field>
        <Field label={`${t('admin.tierRequest')} · CPU`} description={t('admin.tierRequestHint')}>
          <Input value={form.cpuRequest} onChange={(event) => set('cpuRequest', event.target.value)} />
        </Field>
        <Field label={`${t('admin.tierLimit')} · CPU`} description={t('admin.tierLimitHint')}>
          <Input value={form.cpuLimit} onChange={(event) => set('cpuLimit', event.target.value)} />
        </Field>
        <Field label={`${t('admin.tierRequest')} · RAM`}>
          <Input value={form.memoryRequest} onChange={(event) => set('memoryRequest', event.target.value)} />
        </Field>
        <Field label={`${t('admin.tierLimit')} · RAM`}>
          <Input value={form.memoryLimit} onChange={(event) => set('memoryLimit', event.target.value)} />
        </Field>

        <div className="flex items-end gap-2">
          <label className="flex items-center gap-2 text-sm">
            <input
              type="checkbox"
              checked={form.default}
              onChange={(event) => set('default', event.target.checked)}
              className="size-4 accent-[var(--noryx-brand)]"
            />
            {t('admin.tierDefault')}
          </label>
          {tier.default && !form.default ? null : tier.default ? (
            <Badge tone="neutral">
              <Cpu className="size-3" />
            </Badge>
          ) : null}
        </div>

        <div className="flex items-end justify-end gap-2">
          <Button variant="ghost" onClick={onRemove} aria-label={t('common.delete')}>
            <Trash2 className="size-4" />
          </Button>
          <Button
            variant="primary"
            loading={saving}
            disabled={!dirty || !complete}
            onClick={() => onSave({ ...form, id: form.id.trim(), name: form.name.trim() })}
          >
            {t('common.save')}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
