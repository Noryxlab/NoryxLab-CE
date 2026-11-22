import { Card, CardContent, CardDescription, CardHeader, CardHeaderText, CardTitle } from '@/components/ui/card';
import { Stat, StatGrid } from '@/components/common/stat';
import { Badge } from '@/components/ui/badge';
import { Progress } from '@/components/ui/progress';
import { useProjectBudget, useProjectUsage } from '@/lib/api/queries';
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
  /* La dotation du mois, a cote de ce qu'elle borne.
   *
   *  Dans cette carte et pas dans une carte a elle : la question est « on a
   *  pris combien, sur combien », et la couper en deux obligerait a lire deux
   *  endroits pour y repondre. Lisible par tout membre - une equipe qui ne
   *  voit pas sa consommation ne peut pas la changer - alors que poser la
   *  dotation reste un acte d'administration.  */
  const budget = useProjectBudget(projectId);
  const allowance = budget.data?.vcpuHours ?? 0;
  const percent = budget.data?.percent ?? 0;

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
        {allowance > 0 ? (
          <div className="space-y-1.5 rounded-md border border-border bg-surface-muted p-3">
            <div className="flex flex-wrap items-baseline justify-between gap-2">
              <p className="text-xs font-medium">
                {t('budgets.allowance')} · {t('budgets.period')}
              </p>
              <p className="text-xs tabular-nums text-muted-foreground">
                {(budget.data?.consumed ?? 0).toFixed(1)} / {formatNumber(allowance, locale)}{' '}
                {t('usage.vcpuHours')} · {percent.toFixed(0)} %
                {percent >= 100 ? (
                  <Badge tone="danger" className="ml-1.5">
                    {t('budgets.overBudget')}
                  </Badge>
                ) : null}
              </p>
            </div>
            <Progress
              value={Math.min(100, percent)}
              label={`${percent.toFixed(0)} %`}
              className={
                percent >= 100 ? '[&>div]:bg-danger' : percent >= 75 ? '[&>div]:bg-warning' : undefined
              }
            />
            {/* Dit a l'equipe, pas seulement a l'administrateur : une barre
              *  rouge se lit comme une coupure, et personne ne doit partir
              *  chercher pourquoi son calcul aurait ete suspendu. */}
            <p className="text-xs leading-relaxed text-muted-foreground">
              {t('budgets.doesNotStopHint')}
            </p>
          </div>
        ) : null}
        {total ? (
          <p className="text-xs text-muted-foreground">
            {t('usage.restsOn', { samples: formatNumber(total.samples, locale) })}
          </p>
        ) : null}
      </CardContent>
    </Card>
  );
}
