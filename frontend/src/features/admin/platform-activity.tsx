import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { Activity } from 'lucide-react';
import { Card } from '@/components/ui/card';
import { Select } from '@/components/ui/select';
import { SectionHeader } from '@/components/common/page-header';
import { EmptyState } from '@/components/common/states';
import { adminApi } from '@/lib/api/endpoints';
import { useT } from '@/lib/i18n';
import { formatDate, formatNumber } from '@/lib/format';

/**
 * Ce que la plateforme a servi : des personnes et des actions.
 *
 * Distinct de la section Usage, qui compte des vCPU-heures et repond a une
 * question de cout. Celle-ci repond a « qui s'en est servi, pour quoi, quand »,
 * c'est-a-dire la question posee a la fin d'un pilote.
 *
 * La barre quotidienne compte des *personnes*, jamais des evenements, et c'est
 * la decision qui porte tout le reste. Un import d'ontologie ecrit une ligne
 * d'audit par objet : 685 000 sur une seule apres-midi, par une seule personne.
 * Compte en evenements, cette apres-midi enterre une semaine de travail reel et
 * le graphique decrit l'import plutot que la plateforme. Compte en personnes,
 * le meme import vaut une personne sur un jour - ce qu'il etait.
 */
export function PlatformActivitySection() {
  const t = useT();
  const [window, setWindow] = useState('720h');
  const activity = useQuery({
    queryKey: ['admin', 'activity', window],
    queryFn: () => adminApi.activity(window),
  });

  const report = activity.data;
  const peak = Math.max(1, ...(report?.daily ?? []).map((day) => day.people));

  // La periode reellement couverte, quand elle commence apres celle demandee.
  // Sans cette phrase, un rapport sur trente jours affiche douze jours de
  // donnees et se lit comme un mois calme (ADR-034).
  const partial =
    report?.coversSince && new Date(report.coversSince) > new Date(report.since)
      ? formatDate(report.coversSince)
      : null;

  return (
    <section className="space-y-4">
      <SectionHeader
        title={t('admin.activity')}
        description={t('admin.activityHint')}
        actions={
          <Select
            value={window}
            onValueChange={setWindow}
            options={[
              { value: '168h', label: t('admin.window7d') },
              { value: '720h', label: t('admin.window30d') },
              { value: '2160h', label: t('admin.window90d') },
            ]}
          />
        }
      />

      {report && report.totalEvents === 0 ? (
        <EmptyState icon={Activity} title={t('admin.activity')} description={t('admin.activityEmpty')} />
      ) : null}

      {report && report.totalEvents > 0 ? (
        <>
          {partial ? (
            <p className="text-xs text-muted-foreground">{t('admin.activityCoverage', { since: partial })}</p>
          ) : null}

          <Card className="p-4">
            <p className="mb-3 text-xs font-medium uppercase tracking-wide text-muted-foreground">
              {t('admin.activePeoplePerDay')}
            </p>
            {/* Une barre par jour, hauteur proportionnelle au nombre de
                personnes. Le titre porte le detail : un graphique qu'on ne
                peut pas interroger ne vaut que ce qu'il montre. */}
            <div className="flex h-24 items-end gap-[3px] overflow-x-auto">
              {report.daily.map((day) => (
                <div
                  key={day.day}
                  className="min-w-[6px] flex-1 rounded-t bg-primary/70"
                  style={{ height: `${Math.max(4, (day.people / peak) * 100)}%` }}
                  title={`${formatDate(day.day)} — ${t('admin.peopleCount', { count: String(day.people) })}, ${t(
                    'admin.eventsCount',
                    { count: formatNumber(day.events) },
                  )}`}
                />
              ))}
            </div>
            <div className="mt-2 flex justify-between text-[11px] text-muted-foreground">
              <span>{formatDate(report.daily[0]?.day ?? report.since)}</span>
              <span>{formatDate(report.daily[report.daily.length - 1]?.day ?? report.until)}</span>
            </div>
          </Card>

          {report.organizations.length > 0 ? (
            <Card className="p-4">
              <p className="mb-3 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                {t('admin.byOrganization')}
              </p>
              <ul className="space-y-2">
                {report.organizations.map((row) => (
                  <li key={row.organization || 'none'} className="flex items-baseline justify-between gap-3 text-sm">
                    <span className="truncate">
                      {row.organization || <span className="italic text-muted-foreground">{t('admin.noOrganization')}</span>}
                    </span>
                    {/* People first, count second: events are dominated by
                        whoever automated something, and the question asked of
                        a pilot is how many people each party brought. */}
                    <span className="shrink-0 tabular-nums text-xs text-muted-foreground">
                      {t('admin.peopleCount', { count: String(row.people) })} ·{' '}
                      {t('admin.eventsCount', { count: formatNumber(row.events) })}
                    </span>
                  </li>
                ))}
              </ul>
            </Card>
          ) : null}

          <div className="grid gap-4 md:grid-cols-2">
            <Card className="p-4">
              <p className="mb-3 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                {t('admin.whoWasActive', { count: String(report.people.length) })}
              </p>
              <ul className="space-y-1.5">
                {report.people.slice(0, 12).map((person) => (
                  <li key={person.actor} className="flex items-baseline justify-between gap-3 text-sm">
                    <span className="min-w-0 truncate">
                      {person.actor}
                      {person.organization ? (
                        <span className="ml-2 text-xs text-muted-foreground">{person.organization}</span>
                      ) : null}
                    </span>
                    <span className="shrink-0 tabular-nums text-xs text-muted-foreground">
                      {formatNumber(person.events)}
                    </span>
                  </li>
                ))}
              </ul>
            </Card>

            <Card className="p-4">
              <p className="mb-3 text-xs font-medium uppercase tracking-wide text-muted-foreground">
                {t('admin.whatWasDone')}
              </p>
              <ul className="space-y-1.5">
                {report.actions.slice(0, 12).map((action) => (
                  <li key={action.action} className="flex items-baseline justify-between gap-3 text-sm">
                    <span className="truncate font-mono text-xs">{action.action}</span>
                    <span className="shrink-0 tabular-nums text-xs text-muted-foreground">
                      {formatNumber(action.count)}
                    </span>
                  </li>
                ))}
              </ul>
              {report.actionsTotal > report.actionsShown ? (
                <p className="mt-3 text-[11px] text-muted-foreground">
                  {t('admin.actionsTruncated', {
                    shown: String(report.actionsShown),
                    total: String(report.actionsTotal),
                  })}
                </p>
              ) : null}
            </Card>
          </div>
        </>
      ) : null}
    </section>
  );
}
