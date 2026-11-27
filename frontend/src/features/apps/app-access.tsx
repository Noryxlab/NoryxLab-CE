import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { X } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Select } from '@/components/ui/select';
import { Sheet, SheetBody, SheetContent } from '@/components/ui/sheet';
import { useToast } from '@/components/ui/toast';
import { appsApi, type AppAccessMode } from '@/lib/api/endpoints';
import type { App } from '@/lib/api/types';
import { useOrganizations } from '@/lib/api/queries';
import { useT } from '@/lib/i18n';

/** Qui peut ouvrir une application deja publiee.
 *
 *  L'acces etait decide une fois, a la creation. Une application publiee pour
 *  une organisation ne pouvait pas s'ouvrir a une deuxieme sans etre
 *  supprimee et relancee - ce qu'on ne fait pas a une production qui tourne,
 *  avec son URL, ses revisions et son historique, pour ajouter une collegue.
 *
 *  La liste est modifiable ici, et pas seulement une organisation comme au
 *  formulaire de creation : une application sert souvent plusieurs parties,
 *  et c'est en ajoutant la deuxieme qu'on s'en apercoit. */
export function AppAccessSheet({
  app,
  open,
  onOpenChange,
  onDone,
}: {
  app: App | null;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onDone: () => void;
}) {
  const t = useT();
  const toast = useToast();
  const organizations = useOrganizations();
  const [mode, setMode] = React.useState<AppAccessMode>('private');
  const [orgs, setOrgs] = React.useState<string[]>([]);
  const [toAdd, setToAdd] = React.useState('');

  // Repart de l'application a chaque ouverture : la feuille montre ce qui est,
  // pas ce qu'on avait commence a taper la fois d'avant.
  React.useEffect(() => {
    if (!app) return;
    setMode((app.accessMode ?? 'private') as AppAccessMode);
    setOrgs(app.allowedOrganizations ?? []);
    setToAdd('');
  }, [app]);

  const save = useMutation({
    mutationFn: () =>
      appsApi.setAccess(app?.id ?? '', {
        accessMode: mode,
        ...(mode === 'organization' ? { allowedOrganizations: orgs } : {}),
      }),
    onSuccess: () => {
      onDone();
      toast.success(t('apps.accessChanged'), app?.name ?? '');
      onOpenChange(false);
    },
    onError: (error) => toast.error(error, t('apps.access')),
  });

  const known = organizations.data ?? [];
  const nameOf = (id: string) => known.find((o) => o.id === id)?.name ?? id;
  const candidates = known
    .filter((o) => !orgs.includes(o.id))
    .map((o) => ({ value: o.id, label: o.name }));

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent title={app?.name ?? t('apps.access')}>
        <SheetBody className="space-y-4">
          <Field label={t('apps.accessLabel')}>
            <Select
              value={mode}
              onValueChange={(value) => setMode(value as AppAccessMode)}
              options={[
                { value: 'private', label: t('apps.accessPrivate') },
                { value: 'organization', label: t('apps.accessOrganization') },
                { value: 'public', label: t('apps.accessPublic') },
              ]}
            />
          </Field>

          {mode === 'organization' ? (
            <div className="space-y-2">
              <div className="flex flex-wrap items-end gap-2">
                <Field label={t('apps.addOrganization')} className="min-w-64">
                  <Select value={toAdd} onValueChange={setToAdd} options={candidates} />
                </Field>
                <Button
                  variant="secondary"
                  disabled={!toAdd}
                  onClick={() => {
                    setOrgs((current) => [...current, toAdd]);
                    setToAdd('');
                  }}
                >
                  {t('common.add')}
                </Button>
              </div>
              {orgs.length === 0 ? (
                // Dire pourquoi c'est vide, et ce que ca produirait : une
                // liste vide n'est pas un acces ouvert, c'est un refus.
                <p className="text-sm text-muted-foreground">{t('apps.noOrganizationYet')}</p>
              ) : (
                <ul className="space-y-1">
                  {orgs.map((id) => (
                    <li key={id} className="flex items-center justify-between rounded-md border border-border px-3 py-1.5 text-sm">
                      <span>{nameOf(id)}</span>
                      <Button
                        variant="ghost"
                        size="icon-sm"
                        aria-label={t('common.delete')}
                        onClick={() => setOrgs((current) => current.filter((item) => item !== id))}
                      >
                        <X aria-hidden />
                      </Button>
                    </li>
                  ))}
                </ul>
              )}
            </div>
          ) : null}

          <Button
            variant="primary"
            loading={save.isPending}
            disabled={mode === 'organization' && orgs.length === 0}
            onClick={() => save.mutate()}
          >
            {t('common.save')}
          </Button>
        </SheetBody>
      </SheetContent>
    </Sheet>
  );
}
