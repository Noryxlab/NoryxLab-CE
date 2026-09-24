import * as React from 'react';
import { Skeleton } from '@/components/ui/skeleton';
import { SectionHeader } from '@/components/common/page-header';
import { useMutation } from '@tanstack/react-query';
import { useParams } from 'react-router';
import { PageHeader } from '@/components/common/page-header';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { AgentRoster } from '@/features/agents/agent-roster';
import { AgentJournal } from '@/features/agents/agent-journal';
import { RecruitDialog } from '@/features/agents/recruit-dialog';
import { TeamBoard } from '@/features/agents/team-board';
import { AllowAskDialog, FormTeamDialog } from '@/features/agents/team-dialogs';
import { AIServicesCard } from '@/features/home/ai-services';
import { platformApi } from '@/lib/api/endpoints';
import { qk, useAgentRuns, useAgentTeams, useAgents, useInvalidate } from '@/lib/api/queries';
import { useToast } from '@/components/ui/toast';
import { useT } from '@/lib/i18n';
import type { Agent, AgentTeam } from '@/lib/api/types';

/**
 * Les agents.
 *
 * Une rangee de collegues en haut, ce que le collegue selectionne a vu en
 * dessous. La rangee donne l'etat de tout le monde d'un regard, le fil donne
 * la substance : sans le premier on ne sait pas qui travaille, sans le second
 * la page ne prouve rien.
 *
 * L'etat des services d'IA est rappele ici et pas seulement sur l'accueil.
 * Un agent qui ne rend plus rien parce que la passerelle est arretee
 * ressemble exactement a un agent casse, et c'est la page ou l'on vient
 * chercher la reponse.
 */
export function AgentsPage() {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const agents = useAgents();
  const [selectedId, setSelectedId] = React.useState<string | null>(null);
  const [recruiting, setRecruiting] = React.useState(false);
  const [formingTeam, setFormingTeam] = React.useState(false);
  const [editing, setEditing] = React.useState<Agent | null>(null);
  const [dismissing, setDismissing] = React.useState<Agent | null>(null);
  const [allowAskFor, setAllowAskFor] = React.useState<AgentTeam | null>(null);
  const teams = useAgentTeams();

  // La page vit dans un projet : elle ne montre que ses agents, et recrute
  // dedans. Le projet est au centre de Noryx, et un agent y travaille comme
  // un workspace ou une application.
  const { projectId } = useParams<{ projectId: string }>();
  const items = (agents.data ?? []).filter((item) => item.projectId === projectId);
  // Un agent sans projet n'apparait sur aucune page, puisque chaque page filtre
  // sur le sien - et il continue pourtant de tourner. `test1-rodolphe` sur le
  // DC datait d'avant que le projet devienne obligatoire et s'executait encore
  // toutes les heures des mois plus tard, sans que personne puisse le mettre en
  // pause ni le renvoyer. L'ordonnanceur ne les lance plus ; il fallait aussi
  // un endroit ou quelqu'un puisse les voir et decider.
  const orphans = (agents.data ?? []).filter((item) => !item.projectId);
  // Le premier par defaut, et on suit si celui qui etait choisi disparait.
  const selected = items.find((item) => item.id === selectedId) ?? items[0] ?? null;
  const runs = useAgentRuns(selected?.id);

  const runNow = useMutation({
    mutationFn: (id: string) => platformApi.runAgent(id),
    onSuccess: (run) => {
      invalidate(qk.agents);
      if (selected) invalidate(qk.agentRuns(selected.id));
      if (run.error) toast.error(run.error, t('agents.title'));
      else if (run.quiet) toast.info(t('agents.nothingToReport'), t('agents.title'));
      else toast.success(t('agents.reported'), t('agents.title'));
    },
    onError: (error) => toast.error(error, t('agents.title')),
  });

  const ask = useMutation({
    mutationFn: ({ id, message }: { id: string; message: string }) =>
      platformApi.askAgent(id, message),
    onSuccess: (_run, { id }) => {
      // La reponse est arrivee dans le carnet : c'est le carnet qu'on
      // rafraichit, pas une bulle a cote.
      invalidate(qk.agentRuns(id));
      invalidate(qk.agents);
    },
    onError: (error) => toast.error(error, t('agents.title')),
  });

  const dismiss = useMutation({
    mutationFn: (id: string) => platformApi.deleteAgent(id),
    onSuccess: (_result, id) => {
      invalidate(qk.agents);
      // Si c'etait celui qu'on regardait, la page suit au lieu de rester sur
      // un carnet vide.
      if (selectedId === id) setSelectedId(null);
      toast.success(
        t('agents.dismissed').replace('{name}', dismissing?.name ?? ''),
        t('agents.title'),
      );
      setDismissing(null);
    },
    onError: (error) => toast.error(error, t('agents.title')),
  });

  const toggle = useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      platformApi.updateAgent(id, { enabled }),
    onSuccess: () => invalidate(qk.agents),
    onError: (error) => toast.error(error, t('agents.title')),
  });

  return (
    <div className="space-y-6">
      <PageHeader
        title={t('agents.title')}
        description={t('agents.intro')}
        actions={
          items.length > 0 ? (
            <Button onClick={() => setRecruiting(true)}>{t('agents.recruit')}</Button>
          ) : null
        }
      />

      <AIServicesCard />

      {agents.isLoading ? (
        <Skeleton className="h-10 w-full" />
      ) : items.length === 0 ? (
        <EmptyRoster onRecruit={() => setRecruiting(true)} />
      ) : (
        <>
          <AgentRoster
            agents={items}
            selectedId={selected?.id ?? null}
            onSelect={setSelectedId}
            onRecruit={() => setRecruiting(true)}
          />
          {selected ? (
            <AgentJournal
              agent={selected}
              runs={runs.data ?? []}
              loading={runs.isLoading}
              running={runNow.isPending}
              onRun={() => runNow.mutate(selected.id)}
              onToggle={(enabled) => toggle.mutate({ id: selected.id, enabled })}
              onEdit={() => setEditing(selected)}
              onDismiss={() => setDismissing(selected)}
              onAsk={(message) => ask.mutate({ id: selected.id, message })}
              asking={ask.isPending}
            />
          ) : null}
        </>
      )}

      {orphans.length > 0 ? (
        <OrphanRoster agents={orphans} onDismiss={setDismissing} />
      ) : null}

      {items.length > 0 ? (
        <TeamsSection
          teams={teams.data ?? []}
          agents={items}
          loading={teams.isLoading}
          onForm={() => setFormingTeam(true)}
          onAllowAsk={setAllowAskFor}
        />
      ) : null}

      <RecruitDialog open={recruiting} onOpenChange={setRecruiting} projectId={projectId} />
      <RecruitDialog
        open={editing !== null}
        projectId={projectId}
        editing={editing ?? undefined}
        onOpenChange={(open) => {
          if (!open) setEditing(null);
        }}
      />
      <DismissDialog
        agent={dismissing}
        pending={dismiss.isPending}
        onCancel={() => setDismissing(null)}
        onConfirm={() => dismissing && dismiss.mutate(dismissing.id)}
      />
      <FormTeamDialog open={formingTeam} onOpenChange={setFormingTeam} />
      <AllowAskDialog
        team={allowAskFor}
        agents={items}
        onOpenChange={(open) => {
          if (!open) setAllowAskFor(null);
        }}
      />
    </div>
  );
}

/**
 * Les agents rattaches a aucun projet.
 *
 * Ils ne devraient pas exister - le projet est obligatoire depuis - mais la
 * plateforme en detient, et une ligne que personne ne peut atteindre est
 * exactement ce que la gouvernance des agents sert a empecher. Ils ne
 * s'executent plus, et ce bloc est l'endroit ou quelqu'un decide de leur sort
 * plutot qu'une requete SQL ecrite a la main.
 */
function OrphanRoster({
  agents,
  onDismiss,
}: {
  agents: Agent[];
  onDismiss: (agent: Agent) => void;
}) {
  const t = useT();
  return (
    <section className="rounded-xl border border-amber-500/30 bg-amber-500/5 p-4">
      <SectionHeader title={t('agents.orphansTitle')} />
      <p className="mt-1 text-xs text-muted-foreground">{t('agents.orphansHint')}</p>
      <ul className="mt-3 space-y-2">
        {agents.map((agent) => (
          <li
            key={agent.id}
            className="flex items-center justify-between gap-3 rounded-lg border border-border bg-background px-3 py-2"
          >
            <div className="min-w-0">
              <p className="truncate text-sm font-medium">{agent.name}</p>
              <p className="truncate text-xs text-muted-foreground">{agent.mission}</p>
            </div>
            <Button variant="danger-outline" size="sm" onClick={() => onDismiss(agent)}>
              {t('agents.dismiss')}
            </Button>
          </li>
        ))}
      </ul>
    </section>
  );
}

/**
 * La page vide, qui est la premiere chose que tout le monde voit.
 *
 * Elle dit ce qu'on peut demander avec un exemple ecrit dans la langue qu'on
 * utilisera pour le faire - pas une liste de fonctionnalites. Quelqu'un qui
 * lit "Tu surveilles mes workspaces..." sait immediatement qu'il peut ecrire
 * la sienne, ce qu'une phrase sur les agents autonomes ne transmet jamais.
 */
function EmptyRoster({ onRecruit }: { onRecruit: () => void }) {
  const t = useT();
  return (
    <div className="rounded-xl border border-dashed border-border px-6 py-10 text-center">
      <p className="mx-auto max-w-lg text-sm leading-relaxed text-muted-foreground">
        {t('agents.emptyLead')}
      </p>
      <p className="mx-auto mt-4 max-w-lg rounded-lg bg-surface-muted px-4 py-3 text-left text-sm leading-relaxed">
        {t('agents.exampleWorkspaces')}
      </p>
      <Button className="mt-6" onClick={onRecruit}>
        {t('agents.recruitFirst')}
      </Button>
    </div>
  );
}

/**
 * Les equipes, sous la rangee de collegues.
 *
 * En dessous et non au-dessus : on recrute avant de s'organiser, et une page
 * qui ouvre sur une structure vide avant d'avoir un seul agent decrit un
 * produit plutot qu'un travail.
 */
function TeamsSection({
  teams,
  agents,
  loading,
  onForm,
  onAllowAsk,
}: {
  teams: AgentTeam[];
  agents: Agent[];
  loading: boolean;
  onForm: () => void;
  onAllowAsk: (team: AgentTeam) => void;
}) {
  const t = useT();
  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <SectionHeader title={t('agents.teamsTitle')} />
          <p className="mt-1 max-w-prose text-sm text-muted-foreground">{t('agents.teamsIntro')}</p>
        </div>
        {teams.length > 0 ? (
          <Button variant="secondary" size="sm" onClick={onForm}>
            {t('agents.formTeam')}
          </Button>
        ) : null}
      </div>

      {loading ? (
        <Skeleton className="h-10 w-full" />
      ) : teams.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border px-6 py-8 text-center">
          <p className="mx-auto max-w-lg text-sm leading-relaxed text-muted-foreground">
            {t('agents.teamsEmpty')}
          </p>
          <Button className="mt-5" variant="secondary" onClick={onForm}>
            {t('agents.formFirstTeam')}
          </Button>
        </div>
      ) : (
        <div className="space-y-4">
          {teams.map((team) => (
            <TeamBoard key={team.id} team={team} agents={agents} onAllowAsk={onAllowAsk} />
          ))}
        </div>
      )}
    </section>
  );
}

/**
 * Renvoyer un agent.
 *
 * La confirmation dit ce qui disparait et ce qui reste. Quelqu'un qui hesite
 * a renvoyer un collegue hesite sur la trace : ses relances et ses signalements
 * restent dans l'audit de la plateforme, et le dire ici evite de garder un
 * agent inutile par precaution.
 */
function DismissDialog({
  agent,
  pending,
  onCancel,
  onConfirm,
}: {
  agent: Agent | null;
  pending: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const t = useT();
  return (
    <Dialog open={agent !== null} onOpenChange={(open) => !open && onCancel()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('agents.dismissTitle').replace('{name}', agent?.name ?? '')}</DialogTitle>
          <DialogDescription>{t('agents.dismissBody')}</DialogDescription>
        </DialogHeader>
        <DialogFooter>
          <Button variant="ghost" onClick={onCancel}>
            {t('common.cancel')}
          </Button>
          <Button variant="danger" onClick={onConfirm} disabled={pending}>
            {t('agents.dismiss')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
