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
import { useToast } from '@/components/ui/toast';
import { useEnvironments, useHardwareTiers, qk, useInvalidate } from '@/lib/api/queries';
import { appsApi, dashboardsApi } from '@/lib/api/endpoints';
import { useI18n, useT } from '@/lib/i18n';
import { findFramework, formatCommand, frameworkOptions, presentTier } from '@/lib/presenters';
import { slugify } from '@/lib/format';

/**
 * App and dashboard creation.
 *
 * The previous form put the launch command in front of the user as a
 * pre-filled shell string:
 *
 *   value="python3 -m http.server 9000 --bind 0.0.0.0 --directory /mnt"
 *
 * A data scientist publishing a Streamlit app has no reason to read that.
 * Here they pick a framework and a main file; the command and port are
 * derived, shown read-only as a preview, and still editable through the
 * "custom command" option for the cases that need it.
 */
export function CreateAppSheet({
  projectId,
  open,
  onOpenChange,
  variant = 'app',
}: {
  projectId: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
  variant?: 'app' | 'dashboard';
}) {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();
  const environments = useEnvironments();
  const tiers = useHardwareTiers();

  const [frameworkId, setFrameworkId] = React.useState('streamlit');
  const [name, setName] = React.useState('');
  const [slug, setSlug] = React.useState('');
  const [slugEdited, setSlugEdited] = React.useState(false);
  const [entrypoint, setEntrypoint] = React.useState('app.py');
  const [port, setPort] = React.useState('8501');
  const [customCommand, setCustomCommand] = React.useState('');
  const [environmentId, setEnvironmentId] = React.useState('');
  const [tierId, setTierId] = React.useState('');
  const [accessMode, setAccessMode] = React.useState('private');
  const [touched, setTouched] = React.useState(false);

  const framework = findFramework(frameworkId);
  const isCustom = frameworkId === 'custom';
  const usable = (environments.data ?? []).filter((environment) => environment.destinationImage);

  React.useEffect(() => {
    if (!environmentId && usable.length > 0) setEnvironmentId(usable[0]?.id ?? '');
  }, [usable, environmentId]);

  React.useEffect(() => {
    if (tierId || !tiers.data?.length) return;
    setTierId((tiers.data.find((tier) => tier.default) ?? tiers.data[0])?.id ?? '');
  }, [tiers.data, tierId]);

  // Switching framework resets the derived defaults, but never overwrites a
  // slug the user has edited by hand.
  React.useEffect(() => {
    if (!framework) return;
    setPort(String(framework.defaultPort));
    setEntrypoint(framework.defaultEntrypoint);
  }, [framework]);

  React.useEffect(() => {
    if (!slugEdited) setSlug(slugify(name));
  }, [name, slugEdited]);

  React.useEffect(() => {
    if (!open) {
      setName('');
      setSlug('');
      setSlugEdited(false);
      setCustomCommand('');
      setTouched(false);
    }
  }, [open]);

  const portNumber = Number(port) || framework?.defaultPort || 8080;
  const command = isCustom
    ? customCommand.trim().split(/\s+/).filter(Boolean)
    : (framework?.command(entrypoint, portNumber) ?? []);

  const nameError = touched && !name.trim() ? t('common.required') : undefined;
  const slugError = touched && !slug.trim() ? t('common.required') : undefined;
  const commandError = touched && command.length === 0 ? t('common.required') : undefined;

  const valid = Boolean(name.trim() && slug.trim() && command.length > 0 && environmentId);

  const mutation = useMutation({
    mutationFn: () => {
      const payload = {
        projectId,
        name: name.trim(),
        slug: slug.trim(),
        environmentId,
        command,
        port: portNumber,
        hardwareTier: tierId || undefined,
        accessMode,
        kind: variant,
      };
      return variant === 'dashboard'
        ? dashboardsApi.create({ ...payload, slug: slug.trim() })
        : appsApi.create(payload);
    },
    onSuccess: () => {
      invalidate(qk.apps(projectId), qk.dashboards(projectId), qk.projects, qk.production);
      onOpenChange(false);
      toast.success(name.trim(), variant === 'dashboard' ? t('dashboards.title') : t('apps.title'));
    },
    onError: (error) =>
      toast.error(error, variant === 'dashboard' ? t('dashboards.createTitle') : t('apps.createTitle')),
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
            <SheetTitle>
              {variant === 'dashboard' ? t('dashboards.createTitle') : t('apps.createTitle')}
            </SheetTitle>
            <SheetDescription>
              {variant === 'dashboard' ? t('dashboards.createHint') : t('apps.createHint')}
            </SheetDescription>
          </SheetHeader>

          <SheetBody>
            <Field label={t('apps.frameworkLabel')} description={t('apps.frameworkHint')} required>
              <Select
                value={frameworkId}
                onValueChange={setFrameworkId}
                options={frameworkOptions(locale)}
              />
            </Field>

            <Field label={t('apps.nameLabel')} error={nameError} required>
              <Input
                value={name}
                onChange={(event) => setName(event.target.value)}
                onBlur={() => setTouched(true)}
                placeholder={t('apps.namePlaceholder')}
                maxLength={80}
                autoFocus
              />
            </Field>

            <Field label={t('apps.slugLabel')} description={t('apps.slugHint')} error={slugError} required>
              <Input
                value={slug}
                onChange={(event) => {
                  setSlugEdited(true);
                  setSlug(slugify(event.target.value));
                }}
                className="font-mono text-xs"
                maxLength={63}
              />
            </Field>

            {isCustom ? (
              <Field
                label={t('apps.commandLabel')}
                description={t('apps.commandHint')}
                error={commandError}
                required
              >
                <Input
                  value={customCommand}
                  onChange={(event) => setCustomCommand(event.target.value)}
                  className="font-mono text-xs"
                />
              </Field>
            ) : (
              <>
                <Field label={t('apps.entrypointLabel')} description={t('apps.entrypointHint')}>
                  <Input
                    value={entrypoint}
                    onChange={(event) => setEntrypoint(event.target.value)}
                    className="font-mono text-xs"
                  />
                </Field>
                <div className="space-y-1.5">
                  <span className="text-xs font-medium text-foreground">{t('apps.commandLabel')}</span>
                  <pre className="scroll-x rounded-md border border-dashed border-border bg-surface-muted px-3 py-2 font-mono text-xs text-muted-foreground">
                    {formatCommand(command)}
                  </pre>
                  <p className="text-xs text-muted-foreground">{t('apps.commandHint')}</p>
                </div>
              </>
            )}

            <div className="grid gap-4 sm:grid-cols-2">
              <Field label={t('apps.portLabel')} description={t('apps.portHint')}>
                <Input
                  type="number"
                  inputMode="numeric"
                  min={1}
                  max={65535}
                  value={port}
                  onChange={(event) => setPort(event.target.value)}
                />
              </Field>
              <Field label={t('workspaces.tierLabel')}>
                <Select
                  value={tierId}
                  onValueChange={setTierId}
                  options={(tiers.data ?? []).map((tier) => {
                    const presented = presentTier(tier, locale);
                    return { value: tier.id, label: presented.name, hint: presented.specs };
                  })}
                />
              </Field>
            </div>

            <Field label={t('workspaces.environmentLabel')} required>
              <Select
                value={environmentId}
                onValueChange={setEnvironmentId}
                options={usable.map((item) => ({ value: item.id, label: item.name }))}
              />
            </Field>

            <Field label={t('apps.accessLabel')}>
              <Select
                value={accessMode}
                onValueChange={setAccessMode}
                options={[
                  { value: 'private', label: t('apps.accessPrivate') },
                  { value: 'organization', label: t('apps.accessOrganization') },
                  { value: 'authenticated', label: t('apps.accessPublic') },
                ]}
              />
            </Field>
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
