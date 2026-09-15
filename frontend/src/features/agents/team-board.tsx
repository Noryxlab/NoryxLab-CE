import { useMutation } from '@tanstack/react-query';
import { Users, X } from 'lucide-react';
import { AgentFace, moodOf } from './agent-face';
import { Button } from '@/components/ui/button';
import { platformApi } from '@/lib/api/endpoints';
import { qk, useAgentMandates, useInvalidate } from '@/lib/api/queries';
import { useToast } from '@/components/ui/toast';
import { useT } from '@/lib/i18n';
import type { Agent, AgentMandate, AgentTeam } from '@/lib/api/types';

/**
 * Une equipe, et l'organisation ecrite entre ses membres.
 *
 * Le parti pris tient en une ligne : un mandat s'affiche en phrase, jamais en
 * matrice. "Charlotte peut demander a Camille de relancer une application"
 * se lit par quelqu'un qui n'a jamais entendu parler de delegation ; trois
 * colonnes d'identifiants ne se lisent par personne, et c'est pourtant la
 * forme que prend n'importe quel ecran de permissions laisse a lui-meme.
 *
 * L'organisation est ecrite d'avance, donc cet ecran *est* la reponse a "qui
 * peut faire quoi". Il n'y a pas de journal a depouiller a cote.
 */

interface TeamBoardProps {
  team: AgentTeam;
  agents: Agent[];
  onAllowAsk: (team: AgentTeam) => void;
}

export function TeamBoard({ team, agents, onAllowAsk }: TeamBoardProps) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const mandates = useAgentMandates(team.id);

  const members = agents.filter((agent) => agent.teamId === team.id);
  const leads = members.filter((agent) => agent.role === 'lead');

  const withdraw = useMutation({
    mutationFn: (mandateId: string) => platformApi.deleteAgentMandate(team.id, mandateId),
    onSuccess: () => {
      invalidate(qk.agentMandates(team.id));
      toast.success(t('agents.withdrawn'), t('agents.teamsTitle'));
    },
    onError: (error) => toast.error(error, t('agents.teamsTitle')),
  });

  return (
    <section className="rounded-xl border border-border bg-surface p-5">
      <header className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h3 className="flex items-center gap-2 text-sm font-semibold">
            <Users className="size-4 text-muted-foreground" aria-hidden />
            {team.name}
          </h3>
          {team.purpose ? (
            <p className="mt-1 max-w-prose text-sm text-muted-foreground">{team.purpose}</p>
          ) : null}
        </div>
        <Button
          variant="secondary"
          size="sm"
          disabled={leads.length === 0 || members.length < 2}
          onClick={() => onAllowAsk(team)}
        >
          {t('agents.allowAsk')}
        </Button>
      </header>

      <div className="mt-5 grid gap-5 lg:grid-cols-2">
        <div>
          <h4 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            {t('agents.teamMembers')}
          </h4>
          {members.length === 0 ? (
            <p className="mt-2 text-sm text-muted-foreground">{t('agents.teamNoMembers')}</p>
          ) : (
            <ul className="mt-3 space-y-2">
              {members.map((agent) => (
                <li key={agent.id} className="flex items-center gap-3">
                  <AgentFace name={agent.name} mood={moodOf(agent)} size={28} />
                  <span className="text-sm font-medium">{agent.name}</span>
                  <span className="text-xs text-muted-foreground">{roleLabel(agent.role, t)}</span>
                </li>
              ))}
            </ul>
          )}
        </div>

        <div>
          <h4 className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            {t('agents.organisation')}
          </h4>
          {mandates.isLoading ? (
            <p className="mt-2 text-sm text-muted-foreground">{t('common.loading')}</p>
          ) : (mandates.data ?? []).length === 0 ? (
            <p className="mt-2 max-w-prose text-sm text-muted-foreground">
              {leads.length === 0 ? t('agents.needsALead') : t('agents.organisationEmpty')}
            </p>
          ) : (
            <ul className="mt-3 space-y-2">
              {(mandates.data ?? []).map((mandate) => (
                <li
                  key={mandate.id}
                  className="flex items-start justify-between gap-3 rounded-lg bg-surface-muted px-3 py-2"
                >
                  <span className="text-sm leading-relaxed">{sentence(mandate, agents, t)}</span>
                  <button
                    type="button"
                    onClick={() => withdraw.mutate(mandate.id)}
                    title={t('agents.withdraw')}
                    aria-label={t('agents.withdraw')}
                    className="rounded p-1 text-muted-foreground transition hover:text-danger focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
                  >
                    <X className="size-3.5" aria-hidden />
                  </button>
                </li>
              ))}
            </ul>
          )}
          <p className="mt-3 max-w-prose text-xs leading-relaxed text-muted-foreground">
            {t('agents.organisationHelp')}
          </p>
        </div>
      </div>
    </section>
  );
}

type Translate = ReturnType<typeof useT>;

export function roleLabel(role: Agent['role'], t: Translate): string {
  if (role === 'lead') return t('agents.roleLead');
  if (role === 'operator') return t('agents.roleOperator');
  return t('agents.roleObserver');
}

/**
 * La phrase.
 *
 * Un agent supprime laisse son identifiant dans un mandat que le serveur n'a
 * pas encore nettoye. On affiche alors l'identifiant plutot que de masquer la
 * ligne : une organisation dont une ligne disparait silencieusement est pire
 * qu'une ligne qu'on ne comprend pas, parce qu'on ne sait pas qu'elle existe.
 */
export function sentence(mandate: AgentMandate, agents: Agent[], t: Translate): string {
  const nameOf = (id: string) => agents.find((agent) => agent.id === id)?.name ?? id;
  return t('agents.mandateSentence')
    .replace('{lead}', nameOf(mandate.leadId))
    .replace('{member}', nameOf(mandate.memberId))
    .replace('{action}', actionLabel(mandate.action, t));
}

export function actionLabel(action: string, t: Translate): string {
  if (action === 'restart_app') return t('agents.askActionRestartApp');
  return action;
}
