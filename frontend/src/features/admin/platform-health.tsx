import * as React from 'react';
import { Link } from 'react-router';
import { AlertTriangle, CheckCircle2, ShieldAlert } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { Badge } from '@/components/ui/badge';
import { SkeletonText } from '@/components/ui/skeleton';
import { Button } from '@/components/ui/button';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { usePlatformHealth, usePlatformHealthHistory } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatDuration, formatRelative } from '@/lib/format';
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

/**
 * History of platform conditions.
 *
 * The panel above shows the present, and the webhook is fire-and-forget: if
 * nobody was in the channel, the event is gone. Three nights without a backup
 * left no consultable trace anywhere — this is the record that survives both a
 * missing channel and a restart.
 *
 * Job failures are deliberately absent. They belong to whoever ran them and
 * have context on their own screen; interleaving them here would bury the
 * conditions that cost the business everything under the ones that cost one
 * person an afternoon.
 */
function HealthHistory({ enabled }: { enabled: boolean }) {
  const t = useT();
  const { locale } = useI18n();
  const [days, setDays] = React.useState(30);
  const history = usePlatformHealthHistory(days, enabled);

  if (history.isError) return null;

  const items = history.data?.items ?? [];

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-xs leading-relaxed text-muted-foreground">{t('health.historyHint')}</p>
        <div className="flex gap-1">
          {([30, 90, 365] as const).map((option) => (
            <Button
              key={option}
              size="sm"
              variant={days === option ? 'secondary' : 'ghost'}
              onClick={() => setDays(option)}
            >
              {option === 30 ? t('health.last30') : option === 90 ? t('health.last90') : t('health.last365')}
            </Button>
          ))}
        </div>
      </div>

      {history.isLoading ? (
        <SkeletonText lines={3} />
      ) : history.data && !history.data.recording ? (
        <p className="text-sm text-muted-foreground">{t('health.notRecording')}</p>
      ) : items.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('health.historyEmpty')}</p>
      ) : (
        <ul className="space-y-2">
          {items.map((event) => (
            <li
              key={event.id}
              className={cn(
                'rounded-lg border px-3 py-2',
                event.resolvedAt ? 'border-border bg-surface-muted' : SEVERITY_STYLE[event.severity],
              )}
            >
              <div className="flex flex-wrap items-start justify-between gap-2">
                <p className="min-w-0 text-sm font-medium">{event.summary}</p>
                <Badge tone={event.resolvedAt ? 'outline' : SEVERITY_TONE[event.severity]}>
                  {event.resolvedAt
                    ? `${t('health.resolvedAfter')} ${formatDuration(
                        (new Date(event.resolvedAt).getTime() - new Date(event.raisedAt).getTime()) / 1000,
                        locale,
                      )}`
                    : t('health.ongoing')}
                </Badge>
              </div>
              {event.detail ? (
                <p className="mt-1 break-words text-xs text-muted-foreground">{event.detail}</p>
              ) : null}
              <p className="mt-1 text-xs text-muted-foreground">
                {event.source} · {t('health.raised')} {formatRelative(event.raisedAt, locale)}
              </p>
            </li>
          ))}
        </ul>
      )}
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
        <Tabs defaultValue="current">
          <TabsList>
            <TabsTrigger value="current">{t('health.title')}</TabsTrigger>
            <TabsTrigger value="history">{t('health.history')}</TabsTrigger>
          </TabsList>

          <TabsContent value="current">
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
          </TabsContent>

          <TabsContent value="history">
            <HealthHistory enabled={enabled} />
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  );
}
