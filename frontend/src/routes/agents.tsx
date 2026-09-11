import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { PageHeader } from '@/components/common/page-header';
import { Button } from '@/components/ui/button';
import { AgentRoster } from '@/features/agents/agent-roster';
import { AgentJournal } from '@/features/agents/agent-journal';
import { RecruitDialog } from '@/features/agents/recruit-dialog';
import { AIServicesCard } from '@/features/home/ai-services';
import { platformApi } from '@/lib/api/endpoints';
import { qk, useAgentRuns, useAgents, useInvalidate } from '@/lib/api/queries';
import { useToast } from '@/components/ui/toast';
import { useT } from '@/lib/i18n';

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

  const items = agents.data ?? [];
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
        <p className="text-sm text-muted-foreground">{t('common.loading')}</p>
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
            />
          ) : null}
        </>
      )}

      <RecruitDialog open={recruiting} onOpenChange={setRecruiting} />
    </div>
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
