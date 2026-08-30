import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Lock, RotateCcw, Save } from 'lucide-react';
import { Card, CardContent, CardHeader, CardHeaderText, CardTitle, CardDescription } from '@/components/ui/card';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import { SkeletonText } from '@/components/ui/skeleton';
import { ErrorState } from '@/components/common/states';
import { SectionHeader } from '@/components/common/page-header';
import { useToast } from '@/components/ui/toast';
import { usePlatformSettings, qk, useInvalidate } from '@/lib/api/queries';
import { adminApi } from '@/lib/api/endpoints';
import { useI18n, useT, type TranslationKey } from '@/lib/i18n';
import type { EffectiveSetting } from '@/lib/api/types';

/**
 * Platform settings.
 *
 * Values that used to require editing a manifest and rolling the deployment.
 * Each row shows where its current value comes from, because "why is this 48h
 * when the manifest says 12h" is the question an operator actually has, and
 * configuration split across a manifest, an environment and a live deployment
 * is precisely what drifts (ADR-034 follow-up).
 */

// The backend supplies an English label as a fallback. Translating by key
// means a French session reads French and an English one reads English,
// instead of whatever language the backend happened to be written in.
const SETTING_LABEL: Record<string, TranslationKey> = {
  'workspace.max_lifetime': 'settings.key_workspace_max_lifetime',
  'alert.webhook_url': 'settings.key_alert_webhook_url',
  'alert.instance_name': 'settings.key_alert_instance_name',
  'ui.default_theme': 'settings.key_ui_default_theme',
  'platform.backend_version': 'settings.key_platform_backend_version',
  'platform.edition': 'settings.key_platform_edition',
  'platform.namespace': 'settings.key_platform_namespace',
};

function sourceLabel(source: string, locale: 'fr' | 'en'): string {
  const labels: Record<string, { fr: string; en: string }> = {
    stored: { fr: 'Défini ici', en: 'Set here' },
    environment: { fr: 'Variable d’environnement', en: 'Environment variable' },
    default: { fr: 'Valeur par défaut', en: 'Default' },
    build: { fr: 'Déterminé à la compilation', en: 'Set at build time' },
  };
  return labels[source]?.[locale] ?? source;
}

function SettingRow({ setting }: { setting: EffectiveSetting }) {
  const t = useT();
  const { locale } = useI18n();
  const toast = useToast();
  const invalidate = useInvalidate();

  const labelKey = SETTING_LABEL[setting.key];
  const label = labelKey ? t(labelKey) : setting.label;

  const [draft, setDraft] = React.useState(setting.value);
  React.useEffect(() => setDraft(setting.value), [setting.value]);

  const save = useMutation({
    mutationFn: (value: string) => adminApi.updateSetting(setting.key, value),
    onSuccess: () => {
      invalidate(qk.adminSettings, qk.adminHealth);
      toast.success(label, t('settings.saved'));
    },
    onError: (error) => toast.error(error, label),
  });

  const dirty = draft.trim() !== setting.value.trim();

  // A fact is shown, never offered for editing. Rendering a disabled input
  // would advertise a control that does not exist, which is the pattern the
  // frontend rewrite removed elsewhere.
  if (setting.readOnly) {
    return (
      <div className="space-y-1 border-b border-border py-4 last:border-0">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div className="min-w-0">
            <p className="text-sm font-medium">{label}</p>
            <p className="font-mono text-xs text-muted-foreground">{setting.key}</p>
          </div>
          <span className="inline-flex items-center gap-1.5">
            <Lock className="size-3 text-muted-foreground" aria-hidden />
            <Badge tone="outline">{sourceLabel(setting.source, locale)}</Badge>
          </span>
        </div>
        <p className="font-mono text-sm">{setting.value || '—'}</p>
        <p className="text-xs leading-relaxed text-muted-foreground">{setting.description}</p>
      </div>
    );
  }

  return (
    <div className="space-y-2 border-b border-border py-4 last:border-0">
      <div className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <p className="text-sm font-medium">{label}</p>
          <p className="font-mono text-xs text-muted-foreground">{setting.key}</p>
        </div>
        <Badge tone={setting.source === 'stored' ? 'brand' : 'outline'}>
          {sourceLabel(setting.source, locale)}
        </Badge>
      </div>

      <div className="flex flex-wrap items-end gap-2">
        <Field label={label} hideLabel description={setting.description} className="min-w-64 flex-1">
          {setting.kind === 'enum' ? (
            <Select
              value={draft}
              onValueChange={setDraft}
              options={(setting.values ?? []).map((value) => ({
                value,
                label: value === '' ? t('settings.unset') : value,
              }))}
            />
          ) : (
            <Input
              value={draft}
              onChange={(event) => setDraft(event.target.value)}
              placeholder={setting.fallback || t('settings.unset')}
              className={setting.kind === 'url' ? 'font-mono text-xs' : undefined}
              inputMode={setting.kind === 'url' ? 'url' : undefined}
            />
          )}
        </Field>
        <div className="flex items-center gap-1.5">
          {dirty ? (
            <Button variant="ghost" size="icon-sm" aria-label={t('common.cancel')} onClick={() => setDraft(setting.value)}>
              <RotateCcw aria-hidden />
            </Button>
          ) : null}
          <Button
            variant="primary"
            size="sm"
            disabled={!dirty}
            loading={save.isPending}
            onClick={() => save.mutate(draft)}
          >
            <Save aria-hidden />
            {t('common.save')}
          </Button>
        </div>
      </div>
    </div>
  );
}

export function PlatformSettingsSection() {
  const t = useT();
  const settings = usePlatformSettings();

  return (
    <div className="space-y-4">
      <SectionHeader title={t('settings.title')} description={t('settings.subtitle')} />
      <Card>
        <CardHeader>
          <CardHeaderText>
            <CardTitle>{t('settings.title')}</CardTitle>
            <CardDescription>{t('settings.precedence')}</CardDescription>
          </CardHeaderText>
        </CardHeader>
        <CardContent className="py-0">
          {settings.isLoading ? (
            <div className="py-4">
              <SkeletonText lines={4} />
            </div>
          ) : settings.isError ? (
            <ErrorState error={settings.error} onRetry={() => void settings.refetch()} />
          ) : (
            (settings.data ?? []).map((setting) => <SettingRow key={setting.key} setting={setting} />)
          )}
        </CardContent>
      </Card>
    </div>
  );
}
