import { Loader2, RotateCw } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { relativeTime, scheduleKey } from './agent-roster';
import { useT } from '@/lib/i18n';
import type { Agent, AgentRun } from '@/lib/api/types';

/**
 * Ce que l'agent a vu.
 *
 * Un fil chronologique et non un tableau : on lit ca comme on lit un carnet,
 * du plus recent vers le passe, et la valeur est dans la phrase, pas dans une
 * colonne. Les heures calmes restent affichees, en gris et sur une ligne : une
 * rangee d'heures sans rien est la seule facon de savoir qu'il etait la, et
 * les masquer donnerait une page vide pour un agent qui travaille tres bien.
 *
 * Ce qu'il a fait est affiche a partir de la liste d'actions relevee par la
 * plateforme, jamais depuis le rapport. Le rapport est du texte produit par un
 * modele, et c'est la seule partie de cette page qui peut se tromper avec
 * aplomb.
 */

interface AgentJournalProps {
  agent: Agent;
  runs: AgentRun[];
  loading: boolean;
  running: boolean;
  onRun: () => void;
  onToggle: (enabled: boolean) => void;
}

export function AgentJournal({ agent, runs, loading, running, onRun, onToggle }: AgentJournalProps) {
  const t = useT();

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <h2 className="text-sm font-semibold">{t('agents.whatTheySaw', { name: agent.name })}</h2>
          <p className="max-w-2xl text-sm leading-relaxed text-muted-foreground">{agent.mission}</p>
          <p className="text-xs text-placeholder">
            {t(scheduleKey(agent.schedule))}
            {agent.actions.length > 0 ? ` · ${t('agents.mayRestart')}` : ` · ${t('agents.readsOnly')}`}
          </p>
        </div>
        <div className="flex shrink-0 items-center gap-3">
          <label className="flex items-center gap-2 text-xs text-muted-foreground">
            <Switch checked={agent.enabled} onCheckedChange={onToggle} />
            {agent.enabled ? t('agents.atWork') : t('agents.paused')}
          </label>
          <Button variant="secondary" size="sm" onClick={onRun} disabled={running}>
            {running ? (
              <Loader2 className="size-4 animate-spin" aria-hidden />
            ) : (
              <RotateCw className="size-4" aria-hidden />
            )}
            {t('agents.runNow')}
          </Button>
        </div>
      </div>

      {loading && runs.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('agents.loadingRuns')}</p>
      ) : runs.length === 0 ? (
        <p className="rounded-lg border border-dashed border-border px-4 py-6 text-sm text-muted-foreground">
          {t('agents.neverRan')}
        </p>
      ) : (
        <ol className="relative space-y-0 border-l border-border pl-5">
          {runs.map((run) => (
            <RunEntry key={run.id} run={run} />
          ))}
        </ol>
      )}
    </section>
  );
}

function RunEntry({ run }: { run: AgentRun }) {
  const t = useT();
  const when = relativeTime(run.startedAt, t);

  // Trois sortes de lignes, et elles ne se ressemblent pas : une panne se
  // remarque, un rapport se lit, une heure calme se survole.
  if (run.error) {
    return (
      <li className="relative py-3">
        <Dot tone="danger" />
        <div className="flex flex-wrap items-baseline gap-2">
          <span className="text-xs text-placeholder tabular-nums">{when}</span>
          <Badge tone="danger">{t('agents.failed')}</Badge>
        </div>
        <p className="mt-1 text-sm leading-relaxed text-muted-foreground">{run.error}</p>
      </li>
    );
  }

  if (run.quiet) {
    return (
      <li className="relative py-1.5">
        <Dot tone="muted" />
        <div className="flex flex-wrap items-baseline gap-2">
          <span className="text-xs text-placeholder tabular-nums">{when}</span>
          <span className="text-xs text-placeholder">{t('agents.nothingToReport')}</span>
        </div>
      </li>
    );
  }

  return (
    <li className="relative py-3">
      <Dot tone="brand" />
      <span className="text-xs text-placeholder tabular-nums">{when}</span>
      <p className="mt-1 text-sm leading-relaxed whitespace-pre-line">{run.report}</p>
      {run.actions.length > 0 ? (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {run.actions.map((action, index) => (
            <Badge key={`${action}-${index}`} tone="success">
              {t('agents.actionRestartApp')}
            </Badge>
          ))}
        </div>
      ) : null}
    </li>
  );
}

function Dot({ tone }: { tone: 'brand' | 'danger' | 'muted' }) {
  const color =
    tone === 'danger'
      ? 'bg-[var(--noryx-danger)]'
      : tone === 'brand'
        ? 'bg-brand'
        : 'bg-border-strong';
  return (
    <span
      aria-hidden
      className={`absolute -left-[1.4rem] top-[1.15rem] size-2 rounded-full ring-2 ring-[var(--noryx-background)] ${color}`}
    />
  );
}
