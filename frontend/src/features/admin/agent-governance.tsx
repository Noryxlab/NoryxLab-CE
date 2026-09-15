import { Bot, Download } from 'lucide-react';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { SectionHeader } from '@/components/common/page-header';
import { Stat, StatGrid } from '@/components/common/stat';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { adminApi } from '@/lib/api/endpoints';
import { useAgentGovernance } from '@/lib/api/queries';
import { useI18n, useT } from '@/lib/i18n';
import { formatNumber, formatRelative } from '@/lib/format';
import type { AgentGovernanceMandate, AgentGovernanceRow } from '@/lib/api/types';

/**
 * Ce que les agents de cette installation ont le droit de faire.
 *
 * Repondre demandait d'ouvrir chaque agent et chaque equipe, ce que personne
 * ne fait - et la personne a qui on le demande n'est pas celle qui les a
 * construits. Qui deploie des agents se verra poser la question par sa propre
 * direction des risques, et il lui faut quelque chose a emporter dans cette
 * reunion, pas une demonstration.
 *
 * L'activite affichee vient de la liste d'actions relevee par la plateforme,
 * jamais des rapports : un rapport est du texte produit par un modele et peut
 * affirmer une action qui n'a pas eu lieu.
 */
export function AgentGovernanceSection() {
  const t = useT();
  const { locale } = useI18n();
  const report = useAgentGovernance();

  const summary = report.data?.summary;

  const columns: Column<AgentGovernanceRow>[] = [
    {
      id: 'name',
      header: t('common.name'),
      sortValue: (row) => row.name,
      cell: (row) => (
        <div className="min-w-0">
          <p className="truncate font-medium">{row.name}</p>
          <p className="truncate text-xs text-muted-foreground">
            {row.project}
            {row.team ? ` · ${row.team}` : ''}
          </p>
        </div>
      ),
    },
    {
      id: 'role',
      header: t('agents.roleLabel'),
      sortValue: (row) => row.role,
      cell: (row) => <Badge tone="neutral">{t(`agents.role${capitalise(row.role)}` as never)}</Badge>,
    },
    {
      id: 'mayDo',
      header: t('agentGovernance.mayDo'),
      // Le droit d'agir est la colonne qu'on vient lire : elle est nommee en
      // clair, et son absence l'est aussi.
      cell: (row) =>
        row.mayDo.length > 0 ? (
          <Badge tone="warning">{t('agents.mayRestart')}</Badge>
        ) : (
          <span className="text-xs text-muted-foreground">{t('agents.readsOnly')}</span>
        ),
    },
    {
      id: 'state',
      header: t('agentGovernance.state'),
      cell: (row) => (
        <span className="text-xs text-muted-foreground">
          {row.enabled ? t('agents.atWork') : t('agents.paused')} · {t(scheduleKeyOf(row.schedule))}
        </span>
      ),
    },
    {
      id: 'activity',
      header: t('agentGovernance.activity'),
      sortValue: (row) => row.acted,
      cell: (row) => (
        <span className="text-xs tabular-nums">
          {t('agentGovernance.runsAndActions')
            .replace('{runs}', formatNumber(row.runs, locale))
            .replace('{actions}', formatNumber(row.acted, locale))}
        </span>
      ),
    },
    {
      id: 'lastRun',
      header: t('agentGovernance.lastRun'),
      sortValue: (row) => row.lastRunAt ?? '',
      cell: (row) => (
        <span className="text-xs text-muted-foreground">
          {row.lastRunAt ? formatRelative(row.lastRunAt, locale) : t('agents.never')}
        </span>
      ),
    },
  ];

  return (
    <div className="space-y-5">
      <SectionHeader
        title={t('agentGovernance.title')}
        description={t('agentGovernance.intro')}
        actions={
          <Button variant="secondary" size="sm" onClick={() => void adminApi.exportAgentGovernanceCSV()}>
            <Download className="size-4" aria-hidden />
            {t('admin.exportCsv')}
          </Button>
        }
      />

      <StatGrid>
        <Stat label={t('agentGovernance.agents')} loading={report.isLoading} value={formatNumber(summary?.agents, locale)} />
        {/* Combien peuvent changer quelque chose : ce n'est pas le nombre
            d'agents, et c'est le chiffre qu'une direction des risques demande
            en premier. */}
        <Stat
          label={t('agentGovernance.canAct')}
          loading={report.isLoading}
          value={formatNumber(summary?.canAct, locale)}
          hint={t('agentGovernance.canActHint')}
        />
        <Stat label={t('agentGovernance.mandates')} loading={report.isLoading} value={formatNumber(summary?.mandates, locale)} />
        <Stat
          label={t('agentGovernance.actions')}
          loading={report.isLoading}
          value={formatNumber(summary?.actions, locale)}
          hint={t('agentGovernance.window').replace('{days}', String(report.data?.windowDays ?? 30))}
        />
      </StatGrid>

      <Card>
        <DataTable
          data={report.data?.agents}
          columns={columns}
          rowKey={(row) => row.agentId}
          isLoading={report.isLoading}
          isError={report.isError}
          error={report.error}
          onRetry={() => void report.refetch()}
          emptyState={
            <EmptyState
              icon={Bot}
              title={t('agentGovernance.noAgents')}
              description={t('agentGovernance.noAgentsHint')}
            />
          }
        />
      </Card>

      <Organisation mandates={report.data?.mandates ?? []} loading={report.isLoading} />
    </div>
  );
}

/**
 * L'organisation ecrite, en phrases.
 *
 * "Ariane peut demander a Atlas de relancer une application arretee" se lit
 * par quelqu'un qui n'a jamais entendu le mot delegation. Trois colonnes
 * d'identifiants ne se lisent par personne.
 */
function Organisation({
  mandates,
  loading,
}: {
  mandates: AgentGovernanceMandate[];
  loading: boolean;
}) {
  const t = useT();
  const { locale } = useI18n();

  return (
    <section className="space-y-2">
      <h3 className="text-sm font-semibold">{t('agents.organisation')}</h3>
      <p className="max-w-prose text-sm text-muted-foreground">{t('agentGovernance.organisationIntro')}</p>
      {loading ? (
        <p className="text-sm text-muted-foreground">{t('common.loading')}</p>
      ) : mandates.length === 0 ? (
        <p className="rounded-lg border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
          {t('agentGovernance.noMandates')}
        </p>
      ) : (
        <ul className="space-y-2">
          {mandates.map((mandate, index) => (
            <li
              key={`${mandate.teamId}-${mandate.lead}-${mandate.member}-${index}`}
              className="rounded-lg bg-surface-muted px-4 py-3"
            >
              <p className="text-sm leading-relaxed">
                {t('agents.mandateSentence')
                  .replace('{lead}', mandate.lead)
                  .replace('{member}', mandate.member)
                  .replace('{action}', actionLabel(mandate.action, t))}
              </p>
              {/* Qui a autorise, et quand : premiere question posee sur toute
                  delegation. */}
              <p className="mt-1 text-xs text-muted-foreground">
                {t('agentGovernance.grantedBy')
                  .replace('{who}', mandate.grantedBy)
                  .replace('{when}', formatRelative(mandate.since, locale))}
                {mandate.team ? ` · ${mandate.team}` : ''}
              </p>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

type Translate = ReturnType<typeof useT>;

function actionLabel(action: string, t: Translate): string {
  if (action === 'restart_app') return t('agents.askActionRestartApp');
  return action;
}

function capitalise(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function scheduleKeyOf(schedule: AgentGovernanceRow['schedule']) {
  if (schedule === 'hourly') return 'agents.scheduleHourly' as const;
  if (schedule === 'daily') return 'agents.scheduleDaily' as const;
  return 'agents.scheduleManual' as const;
}
