import { Plus } from 'lucide-react';
import { AgentFace, moodOf } from './agent-face';
import { cn } from '@/lib/utils';
import { useT } from '@/lib/i18n';
import type { Agent } from '@/lib/api/types';

/**
 * La rangee de collegues.
 *
 * Une carte par agent, toutes de la meme taille, avec le meme element au meme
 * endroit : le visage, le nom, ce qu'il fait, et quand il a travaille pour la
 * derniere fois. C'est la ligne qu'on parcourt en arrivant, et elle doit se
 * lire sans rien ouvrir.
 *
 * La derniere ligne dit l'heure et pas l'etat. L'etat est deja sur le visage,
 * et l'ecrire une deuxieme fois en toutes lettres ferait deux fois le meme
 * travail avec le risque de se contredire.
 */

interface AgentRosterProps {
  agents: Agent[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  onRecruit: () => void;
}

export function AgentRoster({ agents, selectedId, onSelect, onRecruit }: AgentRosterProps) {
  const t = useT();
  return (
    <div className="flex flex-wrap gap-3">
      {agents.map((agent) => (
        <AgentTile
          key={agent.id}
          agent={agent}
          selected={agent.id === selectedId}
          onSelect={() => onSelect(agent.id)}
        />
      ))}
      <button
        type="button"
        onClick={onRecruit}
        className={cn(
          'flex w-44 flex-col items-center justify-center gap-2 rounded-xl border border-dashed',
          'border-border-strong/70 px-4 py-5 text-sm text-muted-foreground transition',
          'hover:border-brand hover:text-brand focus-visible:outline-2 focus-visible:outline-offset-2',
          'focus-visible:outline-[var(--noryx-ring)]',
        )}
      >
        <Plus className="size-5" aria-hidden />
        {t('agents.recruit')}
      </button>
    </div>
  );
}

function AgentTile({
  agent,
  selected,
  onSelect,
}: {
  agent: Agent;
  selected: boolean;
  onSelect: () => void;
}) {
  const t = useT();
  const mood = moodOf(agent);
  return (
    <button
      type="button"
      onClick={onSelect}
      aria-pressed={selected}
      className={cn(
        'flex w-44 flex-col items-center gap-2 rounded-xl border bg-surface px-4 py-5 text-center transition',
        'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--noryx-ring)]',
        selected
          ? 'border-brand shadow-[0_0_0_1px_var(--noryx-brand)]'
          : 'border-border hover:border-border-strong',
        !agent.enabled && 'opacity-65',
      )}
    >
      <AgentFace name={agent.name} mood={mood} size={52} />
      <span className="max-w-full truncate text-sm font-medium">{agent.name}</span>
      <span className="line-clamp-2 text-xs leading-snug text-muted-foreground">
        {firstSentence(agent.mission)}
      </span>
      <span className="text-[11px] text-placeholder">
        {agent.enabled ? relativeTime(agent.lastRunAt, t) : t('agents.paused')}
      </span>
    </button>
  );
}

/** La premiere phrase de la consigne, qui est presque toujours ce que la
 *  personne aurait mis comme intitule de poste si on le lui avait demande. */
function firstSentence(mission: string): string {
  const trimmed = mission.trim();
  const stop = trimmed.search(/[.!?\n]/);
  return stop > 0 ? trimmed.slice(0, stop) : trimmed;
}

type Translate = ReturnType<typeof useT>;

export function relativeTime(iso: string | undefined, t: Translate): string {
  if (!iso) return t('agents.never');
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return t('agents.never');
  const minutes = Math.round((Date.now() - then) / 60000);
  if (minutes < 1) return t('agents.justNow');
  if (minutes < 60) return t('agents.minutesAgo', { count: minutes });
  const hours = Math.round(minutes / 60);
  if (hours < 24) return t('agents.hoursAgo', { count: hours });
  return t('agents.daysAgo', { count: Math.round(hours / 24) });
}

/** Les clefs du catalogue sont plates : il n'accepte qu'un niveau
 *  d'imbrication, et un chemin compose ne serait pas typable. */
export function scheduleKey(schedule: 'manual' | 'hourly' | 'daily') {
  return ({
    manual: 'agents.scheduleManual',
    hourly: 'agents.scheduleHourly',
    daily: 'agents.scheduleDaily',
  } as const)[schedule];
}
