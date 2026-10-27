import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Plus, ShieldCheck, Trash2 } from 'lucide-react';
import { SectionHeader } from '@/components/common/page-header';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { useToast } from '@/components/ui/toast';
import { adminApi } from '@/lib/api/endpoints';
import { qk, useInvalidate, usePlatformRoles } from '@/lib/api/queries';
import { useT } from '@/lib/i18n';
import type { PlatformRole } from '@/lib/api/types';

/** Ce qu'un role peut creer (ADR-049).
 *
 *  Les fonctions en ligne, les roles en colonne, oui ou non dans les cases.
 *  Les trois lignes fournies se lisent et ne se modifient pas ; les lignes de
 *  l'installation se cochent. Ce que quelqu'un peut lire n'est pas ici : c'est
 *  un octroi, sur l'objet lui-meme, et le mettre en case reconstruirait
 *  l'heritage qu'on a retire. */
export function PlatformRolesMatrix() {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const matrix = usePlatformRoles();
  const [draft, setDraft] = React.useState<PlatformRole | null>(null);

  const capabilities = matrix.data?.capabilities ?? [];
  const roles = matrix.data?.roles ?? [];

  const save = useMutation({
    mutationFn: (role: PlatformRole) =>
      adminApi.upsertPlatformRole(role.key, {
        name: role.name,
        description: role.description,
        capabilities: role.capabilities,
      }),
    onSuccess: () => {
      invalidate(qk.adminPlatformRoles, qk.myCapabilities);
      setDraft(null);
      toast.success(t('platformRoles.saved'), t('platformRoles.title'));
    },
    onError: (error) => toast.error(error, t('platformRoles.title')),
  });
  const remove = useMutation({
    mutationFn: (key: string) => adminApi.deletePlatformRole(key),
    onSuccess: () => {
      invalidate(qk.adminPlatformRoles);
      toast.success(t('platformRoles.removed'), t('platformRoles.title'));
    },
    onError: (error) => toast.error(error, t('platformRoles.title')),
  });

  const label = (capability: string) => t(`platformRoles.${capability}` as never);
  const toggle = (role: PlatformRole, capability: string) => {
    const has = role.capabilities.includes(capability);
    const next = { ...role, capabilities: has ? role.capabilities.filter((c) => c !== capability) : [...role.capabilities, capability] };
    save.mutate(next);
  };

  const columns = draft ? [...roles, draft] : roles;

  return (
    <Card>
      <CardContent className="space-y-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <SectionHeader title={t('platformRoles.title')} description={t('platformRoles.intro')} />
          {draft === null ? (
            <Button variant="secondary" onClick={() => setDraft({ key: '', name: '', description: '', capabilities: [], locked: false, assignable: true })}>
              <Plus aria-hidden />
              {t('platformRoles.addRole')}
            </Button>
          ) : null}
        </div>

        {draft !== null ? (
          <div className="grid gap-3 rounded-md border border-border/60 p-3 sm:grid-cols-[1fr_1fr_2fr_auto] sm:items-end">
            <Field label="key">
              <Input value={draft.key} onChange={(e) => setDraft({ ...draft, key: e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-') })} placeholder={t('platformRoles.keyPlaceholder')} />
            </Field>
            <Field label={t('common.name')}>
              <Input value={draft.name} onChange={(e) => setDraft({ ...draft, name: e.target.value })} placeholder={t('platformRoles.namePlaceholder')} />
            </Field>
            <Field label={t('common.description')}>
              <Input value={draft.description} onChange={(e) => setDraft({ ...draft, description: e.target.value })} placeholder={t('platformRoles.descriptionPlaceholder')} />
            </Field>
            <div className="flex gap-2">
              <Button variant="primary" disabled={!draft.key || !draft.name} loading={save.isPending} onClick={() => save.mutate(draft)}>
                {t('platformRoles.save')}
              </Button>
              <Button variant="ghost" onClick={() => setDraft(null)}>{t('common.cancel')}</Button>
            </div>
          </div>
        ) : null}

        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border/60 text-left">
                <th className="px-3 py-2 font-medium">{t('platformRoles.function')}</th>
                {columns.map((role) => (
                  <th key={role.key || 'draft'} className="px-3 py-2 align-top font-medium">
                    <div className="flex items-center gap-1.5">
                      {role.name || t('platformRoles.role')}
                      {role.locked ? <Badge tone="neutral">{t('platformRoles.shipped')}</Badge> : null}
                      {!role.assignable ? <ShieldCheck className="size-3.5 text-muted-foreground" aria-hidden /> : null}
                    </div>
                    {role.heldBy !== undefined ? (
                      <div className="text-xs font-normal text-muted-foreground">
                        {t('platformRoles.heldBy').replace('{count}', String(role.heldBy))}
                      </div>
                    ) : null}
                    {!role.locked && role.key ? (
                      <Button variant="ghost" size="sm" aria-label={t('platformRoles.remove')} onClick={() => remove.mutate(role.key)}>
                        <Trash2 aria-hidden />
                      </Button>
                    ) : null}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {capabilities.map((capability) => (
                <tr key={capability} className="border-b border-border/40">
                  <td className="px-3 py-2">{label(capability)}</td>
                  {columns.map((role) => {
                    const checked = role.capabilities.includes(capability);
                    const editable = !role.locked;
                    return (
                      <td key={(role.key || 'draft') + capability} className="px-3 py-2">
                        <input
                          type="checkbox"
                          aria-label={`${role.name} ${label(capability)}`}
                          checked={checked}
                          disabled={!editable || save.isPending}
                          onChange={() => (role === draft ? setDraft({ ...draft, capabilities: checked ? draft.capabilities.filter((c) => c !== capability) : [...draft.capabilities, capability] }) : toggle(role, capability))}
                        />
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </CardContent>
    </Card>
  );
}
