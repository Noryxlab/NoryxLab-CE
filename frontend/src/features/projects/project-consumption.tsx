import { Card, CardContent, CardDescription, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { Stat, StatGrid } from '@/components/common/stat';
import { useProjectUsage } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatNumber } from '@/lib/format';

/**
 * What the project actually consumed.
 *
 * The platform could say what a project is allowed to run and what it is
 * running now; what it ran last month is the question behind every conversation
 * about cost and capacity, and the samples answering it have been collected
 * every five minutes for a hundred days with nothing on any screen reading
 * them. The endpoint existed and no screen called it.
 *
 * vCPU-hours rather than a peak: two cores for three hours is six, and that is
 * the number that compares across machines of different sizes. The count of
 * samples is shown beside it, because a total built from three measurements
 * must not read like one built from a month of them.
 */
export function ProjectConsumptionCard({ projectId }: { projectId: string }) {
  const t = useT();
  const { locale } = useI18n();
  const usage = useProjectUsage(projectId);
  const total = usage.data?.total;

  return (
    <Card>
      <CardHeader>
        <CardHeaderText>
          <CardTitle>{t('usage.consumption')}</CardTitle>
          <CardDescription>{t('usage.consumptionHint')}</CardDescription>
        </CardHeaderText>
      </CardHeader>
      <CardContent className="space-y-3">
        <StatGrid className="sm:grid-cols-3">
          <Stat
            label={t('usage.vcpuHours')}
            value={total ? total.vcpuHours.toFixed(1) : '—'}
            loading={usage.isLoading}
          />
          <Stat
            label={t('usage.memoryHours')}
            value={total ? total.memoryGibHours.toFixed(1) : '—'}
            loading={usage.isLoading}
          />
          <Stat
            label={t('usage.peak')}
            value={total ? `${total.peakVcpu.toFixed(1)} vCPU` : '—'}
            hint={total ? `${total.peakMemoryGib.toFixed(1)} GiB` : undefined}
            loading={usage.isLoading}
          />
        </StatGrid>
        {total ? (
          <p className="text-xs text-muted-foreground">
            {t('usage.restsOn', { samples: formatNumber(total.samples, locale) })}
          </p>
        ) : null}
      </CardContent>
    </Card>
  );
}
