import { Link } from 'react-router';
import { AlertTriangle, CheckCircle2, ShieldAlert } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { SkeletonText } from '@/components/ui/skeleton';
import { usePlatformHealth } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatRelative } from '@/lib/format';
import { cn } from '@/lib/utils';
import type { HealthAlert, HealthSeverity } from '@/lib/api/types';

/**
 * Platform health panel.
 *
 * The notifier posts to a webhook, which assumes an operator already runs an
 * alert collector. Most installations do not, and the platform then knows it
 * is unwell while telling nobody — the failure this whole thread started from,
 * where a backup reported success for months (ADR-034).
 *
 * This is the same information, delivered where an administrator already
 * looks.
 */

const SEVERITY_TONE: Record<HealthSeverity, 'danger' | 'warning' | 'neutral'> = {
  critical: 'danger',
  warning: 'warning',
  info: 'neutral',
};

const SEVERITY_STYLE: Record<HealthSeverity, string> = {
  critical: 'border-danger/40 bg-danger-subtle',
  warning: 'border-warning/40 bg-warning-subtle',
  info: 'border-border bg-surface-muted',
};

function severityLabel(severity: HealthSeverity, locale: 'fr' | 'en'): string {
  const labels: Record<HealthSeverity, { fr: string; en: string }> = {
    critical: { fr: 'Critique', en: 'Critical' },
    warning: { fr: 'Avertissement', en: 'Warning' },
    info: { fr: 'Information', en: 'Info' },
  };
  return labels[severity][locale];
}

export function HealthAlertRow({ alert }: { alert: HealthAlert }) {
  const { locale } = useI18n();
  const Icon = alert.severity === 'critical' ? ShieldAlert : AlertTriangle;

  return (
    <div
      className={cn(
        'flex items-start gap-3 rounded-lg border px-4 py-3',
        SEVERITY_STYLE[alert.severity],
      )}
    >
      <Icon
        className={cn(
          'mt-0.5 size-4 shrink-0',
          alert.severity === 'critical' ? 'text-danger' : 'text-warning',
        )}
        aria-hidden
      />
      <div className="min-w-0 flex-1 space-y-1">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-sm font-medium">{alert.summary}</p>
          <Badge tone={SEVERITY_TONE[alert.severity]}>{severityLabel(alert.severity, locale)}</Badge>
          <Badge tone="outline">{alert.source}</Badge>
        </div>
        {alert.detail ? (
          <p className="break-words text-xs leading-relaxed text-muted-foreground">{alert.detail}</p>
        ) : null}
        {alert.since ? (
          <p className="text-xs text-muted-foreground">{formatRelative(alert.since, locale)}</p>
        ) : null}
      </div>
      {alert.action ? (
        <Link
          to={`/admin/${alert.action}`}
          className="shrink-0 self-center text-xs font-medium text-brand underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
        >
          {locale === 'fr' ? 'Ouvrir' : 'Open'}
        </Link>
      ) : null}
    </div>
  );
}

export function PlatformHealthPanel({ enabled }: { enabled: boolean }) {
  const t = useT();
  const { locale } = useI18n();
  const health = usePlatformHealth(enabled);

  // A health endpoint that cannot be reached must not itself be reported as a
  // platform alert: that turns one outage into two.
  if (health.isError) return null;

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('health.title')}</CardTitle>
          <CardDescription>
            {health.data
              ? `${t('health.checkedAt')} ${formatRelative(health.data.generatedAt, locale)}`
              : t('health.subtitle')}
          </CardDescription>
        </CardHeaderText>
        {health.data ? (
          <Badge
            tone={
              health.data.status === 'critical'
                ? 'danger'
                : health.data.status === 'degraded'
                  ? 'warning'
                  : 'success'
            }
          >
            {health.data.status === 'critical'
              ? t('health.statusCritical')
              : health.data.status === 'degraded'
                ? t('health.statusDegraded')
                : t('health.statusHealthy')}
          </Badge>
        ) : null}
      </CardHeader>
      <CardContent>
        {health.isLoading ? (
          <SkeletonText lines={2} />
        ) : health.data && health.data.alerts.length > 0 ? (
          <div className="space-y-2">
            {health.data.alerts.map((alert, index) => (
              <HealthAlertRow key={`${alert.source}-${index}`} alert={alert} />
            ))}
          </div>
        ) : (
          <div className="flex items-center gap-2.5 text-sm text-muted-foreground">
            <CheckCircle2 className="size-4 text-success" aria-hidden />
            {t('health.allClear')}
          </div>
        )}
      </CardContent>
    </Card>
  );
}
