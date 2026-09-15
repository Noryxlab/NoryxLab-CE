import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import {
  Dialog,
  DialogBody,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Input, Textarea } from '@/components/ui/input';
import { actionLabel, roleLabel } from './team-board';
import { platformApi } from '@/lib/api/endpoints';
import { qk, useInvalidate } from '@/lib/api/queries';
import { useToast } from '@/components/ui/toast';
import { useT } from '@/lib/i18n';
import type { Agent, AgentTeam } from '@/lib/api/types';

/**
 * Former une equipe, et autoriser une demande.
 *
 * Le second est celui qui compte. Il est ecrit comme une phrase qu'on compose
 * - qui demande, a qui, de faire quoi - et la phrase complete s'affiche sous
 * les champs avant qu'on valide. On lit ce qu'on va autoriser, dans les mots
 * ou on le relira ensuite.
 *
 * Le serveur refuse une ligne qui ne pourrait pas aboutir, et son refus dit
 * laquelle des deux parties manque du droit. Il est affiche tel quel : c'est
 * une phrase ecrite pour etre lue par la personne qui vient d'essayer, et la
 * reformuler ici ferait deux versions de la meme regle.
 */

export function FormTeamDialog({
  open,
  onOpenChange,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const [name, setName] = React.useState('');
  const [purpose, setPurpose] = React.useState('');

  React.useEffect(() => {
    if (open) {
      setName('');
      setPurpose('');
    }
  }, [open]);

  const form = useMutation({
    mutationFn: () => platformApi.createAgentTeam({ name: name.trim(), purpose: purpose.trim() }),
    onSuccess: (team) => {
      invalidate(qk.agentTeams);
      toast.success(t('agents.teamFormed').replace('{name}', team.name), t('agents.teamsTitle'));
      onOpenChange(false);
    },
    onError: (error) => toast.error(error, t('agents.teamsTitle')),
  });

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('agents.formTeamTitle')}</DialogTitle>
          <DialogDescription>{t('agents.formTeamIntro')}</DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <Field label={t('agents.teamNameLabel')}>
            <Input
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder={t('agents.teamNamePlaceholder')}
              autoFocus
            />
          </Field>
          <Field label={t('agents.teamPurposeLabel')}>
            <Textarea
              rows={3}
              value={purpose}
              onChange={(event) => setPurpose(event.target.value)}
              placeholder={t('agents.teamPurposePlaceholder')}
            />
          </Field>
        </DialogBody>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
          <Button
            onClick={() => form.mutate()}
            disabled={name.trim().length === 0 || form.isPending}
          >
            {t('agents.formTeam')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

const ACTIONS = ['restart_app'] as const;

export function AllowAskDialog({
  team,
  agents,
  onOpenChange,
}: {
  team: AgentTeam | null;
  agents: Agent[];
  onOpenChange: (open: boolean) => void;
}) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();

  const members = agents.filter((agent) => agent.teamId === team?.id);
  const leads = members.filter((agent) => agent.role === 'lead');

  const [leadId, setLeadId] = React.useState('');
  const [memberId, setMemberId] = React.useState('');
  const [action, setAction] = React.useState<string>(ACTIONS[0]);

  // Remis a zero quand le dialogue s'ouvre sur une equipe. Le premier
  // responsable est pre-choisi parce que c'est presque toujours le bon, et
  // qu'un champ vide de plus est un champ de plus a comprendre.
  const teamId = team?.id ?? null;
  React.useEffect(() => {
    if (teamId === null) return;
    const firstLead = agents.find((agent) => agent.teamId === teamId && agent.role === 'lead');
    setLeadId(firstLead?.id ?? '');
    setMemberId('');
    setAction(ACTIONS[0]);
  }, [teamId, agents]);

  const write = useMutation({
    mutationFn: () =>
      platformApi.createAgentMandate(team!.id, { leadId, memberId, action }),
    onSuccess: () => {
      invalidate(qk.agentMandates(team!.id));
      toast.success(t('agents.askWritten'), t('agents.teamsTitle'));
      onOpenChange(false);
    },
    // Le refus du serveur nomme la partie qui manque du droit. Affiche tel
    // quel plutot que reformule : deux versions de la meme regle finissent par
    // se contredire.
    onError: (error) => toast.error(error, t('agents.teamsTitle')),
  });

  const nameOf = (id: string) => agents.find((agent) => agent.id === id)?.name ?? '…';
  const preview = t('agents.mandateSentence')
    .replace('{lead}', leadId ? nameOf(leadId) : '…')
    .replace('{member}', memberId ? nameOf(memberId) : '…')
    .replace('{action}', actionLabel(action, t));

  return (
    <Dialog open={team !== null} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t('agents.allowAskTitle')}</DialogTitle>
          <DialogDescription>{t('agents.allowAskIntro')}</DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-4">
          <Field label={t('agents.askLeadLabel')}>
            <select
              value={leadId}
              onChange={(event) => setLeadId(event.target.value)}
              className="h-9 w-full rounded-md border border-border bg-surface px-3 text-sm"
            >
              {leads.map((agent) => (
                <option key={agent.id} value={agent.id}>
                  {agent.name} — {roleLabel(agent.role, t)}
                </option>
              ))}
            </select>
          </Field>
          <Field label={t('agents.askMemberLabel')}>
            <select
              value={memberId}
              onChange={(event) => setMemberId(event.target.value)}
              className="h-9 w-full rounded-md border border-border bg-surface px-3 text-sm"
            >
              <option value="">…</option>
              {members
                .filter((agent) => agent.id !== leadId)
                .map((agent) => (
                  <option key={agent.id} value={agent.id}>
                    {agent.name} — {roleLabel(agent.role, t)}
                  </option>
                ))}
            </select>
          </Field>
          <Field label={t('agents.askActionLabel')}>
            <select
              value={action}
              onChange={(event) => setAction(event.target.value)}
              className="h-9 w-full rounded-md border border-border bg-surface px-3 text-sm"
            >
              {ACTIONS.map((name) => (
                <option key={name} value={name}>
                  {actionLabel(name, t)}
                </option>
              ))}
            </select>
          </Field>
          {/* La phrase complete, avant de valider : on lit ce qu'on autorise,
              dans les mots ou on le relira ensuite. */}
          <p className="rounded-lg bg-surface-muted px-4 py-3 text-sm leading-relaxed">{preview}</p>
        </DialogBody>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
          <Button
            onClick={() => write.mutate()}
            disabled={!leadId || !memberId || write.isPending}
          >
            {t('agents.allowAsk')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
