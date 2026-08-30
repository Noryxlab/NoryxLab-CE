import { Link } from 'react-router';
import { AlertTriangle, ShieldAlert } from 'lucide-react';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { usePlatformHealth } from '@/lib/api/queries';
import { useAuth } from '@/lib/auth';
import { useI18n, useT } from '@/lib/i18n';
import { formatRelative } from '@/lib/format';
import { cn } from '@/lib/utils';

/**
 * Persistent health indicator in the top bar.
 *
 * The panel in Administration only helps an administrator who thinks to look.
 * This is the part that surfaces a problem without being asked, which is what
 * an alert is for. It appears only when something is actually wrong, so it
 * carries signal rather than becoming furniture.
 */
export function HealthIndicator() {
  const t = useT();
  const { locale } = useI18n();
  const { isAdmin } = useAuth();
  const health = usePlatformHealth(isAdmin);

  const alerts = health.data?.alerts ?? [];
  const critical = alerts.filter((alert) => alert.severity === 'critical').length;

  // Nothing to say, nothing shown: an always-present green badge trains people
  // to stop reading it.
  if (!isAdmin || health.isError || alerts.length === 0) return null;

  const Icon = critical > 0 ? ShieldAlert : AlertTriangle;

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button
          variant="ghost"
          size="sm"
          className={cn('gap-1.5', critical > 0 ? 'text-danger' : 'text-warning')}
          aria-label={`${t('health.indicator')}: ${alerts.length}`}
        >
          <Icon aria-hidden />
          <span className="tabular-nums">{alerts.length}</span>
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent className="w-96">
        <DropdownMenuLabel className="flex items-center justify-between gap-2 normal-case">
          <span className="text-sm font-medium text-foreground">{t('health.title')}</span>
          {health.data ? (
            <Badge tone={critical > 0 ? 'danger' : 'warning'}>
              {critical > 0 ? t('health.statusCritical') : t('health.statusDegraded')}
            </Badge>
          ) : null}
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <div className="scrollbar-thin max-h-80 space-y-2 overflow-y-auto p-2">
          {alerts.map((alert, index) => (
            <div key={`${alert.source}-${index}`} className="space-y-0.5">
              <p className="text-xs font-medium leading-relaxed">{alert.summary}</p>
              <p className="text-xs text-muted-foreground">
                {alert.source}
                {alert.since ? ` · ${formatRelative(alert.since, locale)}` : ''}
              </p>
            </div>
          ))}
        </div>
        <DropdownMenuSeparator />
        <Link
          to="/admin/overview"
          className="block rounded-sm px-2 py-1.5 text-xs font-medium text-brand hover:bg-surface-muted focus-visible:outline-2 focus-visible:outline-offset-[-2px] focus-visible:outline-ring"
        >
          {t('admin.title')}
        </Link>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
