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
import { Switch } from '@/components/ui/switch';
import { AgentFace } from './agent-face';
import { scheduleKey } from './agent-roster';
import { platformApi } from '@/lib/api/endpoints';
import { qk, useAgentTeams, useInvalidate } from '@/lib/api/queries';
import { useToast } from '@/components/ui/toast';
import { useT } from '@/lib/i18n';
import type { Agent } from '@/lib/api/types';

/**
 * Recruter.
 *
 * Le formulaire est ecrit comme une fiche de poste, pas comme une
 * configuration : un nom, ce qu'on attend, des horaires. Il n'y a ni condition,
 * ni seuil, ni expression a composer - les gens qui savent ce qui merite d'etre
 * surveille ne sont pas ceux qui aiment ecrire des regles, et tous les produits
 * de supervision qui leur ont demande l'inverse finissent inutilises.
 *
 * Le visage se forme pendant qu'on tape le nom. C'est un detail, et c'est le
 * moment ou la chose cesse d'etre un formulaire.
 *
 * Le droit d'agir est un interrupteur separe, ecrit en clair et desactive par
 * defaut. Il ne peut pas etre accorde depuis la consigne : le texte part au
 * modele, et une phrase ne peut pas etre une permission puisque la personne
 * qui l'ecrit est aussi celle que la permission protege.
 */

const EXAMPLES = [
  'agents.exampleWorkspaces',
  'agents.exampleApps',
  'agents.exampleQuiet',
] as const;

export function RecruitDialog({
  open,
  onOpenChange,
  projectId,
  editing,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Le projet dans lequel l'agent travaille. Obligatoire : la portee d'un
   *  agent est une donnee, pas une intention, et c'est ce qui rend simples
   *  toutes les regles qui viennent ensuite - donnees, cout, perimetre. */
  projectId: string | undefined;
  editing?: Agent | null;
}) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();

  const [name, setName] = React.useState('');
  const [mission, setMission] = React.useState('');
  const [schedule, setSchedule] = React.useState<Agent['schedule']>('hourly');
  const [mayRestart, setMayRestart] = React.useState(false);
  const [teamId, setTeamId] = React.useState('');
  const [role, setRole] = React.useState<Agent['role']>('observer');
  const teams = useAgentTeams();

  // Remis a l'etat du sujet chaque fois que la fenetre s'ouvre, sans quoi on
  // rouvre sur le brouillon de la fois precedente.
  React.useEffect(() => {
    if (!open) return;
    setName(editing?.name ?? '');
    setMission(editing?.mission ?? '');
    setSchedule(editing?.schedule ?? 'hourly');
    setMayRestart(Boolean(editing?.actions?.includes('restart_app')));
    setTeamId(editing?.teamId ?? '');
    setRole(editing?.role ?? 'observer');
  }, [open, editing]);

  // Le role est un plafond, pas une etiquette : un observateur ne garde aucune
  // action. Accorder le droit d'agir le fait donc passer operateur, au lieu de
  // laisser une fiche qui se contredit elle-meme.
  React.useEffect(() => {
    if (mayRestart && role === 'observer') setRole('operator');
    if (!mayRestart && role === 'operator') setRole('observer');
  }, [mayRestart, role]);

  const save = useMutation({
    mutationFn: () => {
      const body = {
        name: name.trim(),
        mission: mission.trim(),
        projectId,
        schedule,
        actions: mayRestart ? ['restart_app'] : [],
        teamId,
        role,
      };
      return editing ? platformApi.updateAgent(editing.id, body) : platformApi.createAgent(body);
    },
    onSuccess: () => {
      invalidate(qk.agents);
      onOpenChange(false);
      toast.success(
        editing ? t('agents.updated') : t('agents.hired', { name: name.trim() }),
        t('agents.title'),
      );
    },
    onError: (error) => toast.error(error, t('agents.title')),
  });

  const ready =
    name.trim().length > 0 && mission.trim().length > 0 && Boolean(projectId);

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent size="lg">
        <DialogHeader>
          <DialogTitle>{editing ? t('agents.editTitle') : t('agents.recruitTitle')}</DialogTitle>
          <DialogDescription>{t('agents.recruitIntro')}</DialogDescription>
        </DialogHeader>
        <DialogBody className="space-y-5">
          <div className="flex items-center gap-4">
            <AgentFace name={name || '?'} mood="calm" size={56} />
            <div className="flex-1">
              <Field label={t('agents.nameLabel')} required>
                <Input
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  placeholder={t('agents.namePlaceholder')}
                  maxLength={60}
                  autoFocus
                />
              </Field>
            </div>
          </div>

          <Field
            label={t('agents.missionLabel')}
            description={t('agents.missionHelp')}
            required
          >
            <Textarea
              value={mission}
              onChange={(event) => setMission(event.target.value)}
              rows={5}
              maxLength={4000}
              placeholder={t('agents.missionPlaceholder')}
            />
          </Field>

          {mission.trim() === '' ? (
            <div className="space-y-2">
              <p className="text-xs text-muted-foreground">{t('agents.examplesTitle')}</p>
              <div className="flex flex-wrap gap-2">
                {EXAMPLES.map((key) => (
                  <button
                    key={key}
                    type="button"
                    onClick={() => setMission(t(key))}
                    className="rounded-full border border-border px-3 py-1 text-left text-xs text-muted-foreground transition hover:border-brand hover:text-brand"
                  >
                    {t(key)}
                  </button>
                ))}
              </div>
            </div>
          ) : null}

          <Field label={t('agents.scheduleLabel')}>
            <div className="flex flex-wrap gap-2">
                {(['manual', 'hourly', 'daily'] as const).map((option) => (
                  <button
                    key={option}
                    type="button"
                    aria-pressed={schedule === option}
                    onClick={() => setSchedule(option)}
                    className={
                      schedule === option
                        ? 'rounded-lg border border-brand bg-brand-subtle px-3 py-1.5 text-sm text-brand-subtle-foreground'
                        : 'rounded-lg border border-border px-3 py-1.5 text-sm text-muted-foreground transition hover:border-border-strong'
                    }
                  >
                    {t(scheduleKey(option))}
                  </button>
              ))}
            </div>
          </Field>

          {(teams.data ?? []).length > 0 ? (
            <div className="grid gap-4 sm:grid-cols-2">
              <Field label={t('agents.teamLabel')}>
                <select
                  value={teamId}
                  onChange={(event) => setTeamId(event.target.value)}
                  className="h-9 w-full rounded-md border border-border bg-surface px-3 text-sm"
                >
                  <option value="">{t('agents.noTeam')}</option>
                  {(teams.data ?? []).map((team) => (
                    <option key={team.id} value={team.id}>
                      {team.name}
                    </option>
                  ))}
                </select>
              </Field>
              {teamId ? (
                <Field label={t('agents.roleLabel')}>
                  <select
                    value={role}
                    onChange={(event) => setRole(event.target.value as Agent['role'])}
                    className="h-9 w-full rounded-md border border-border bg-surface px-3 text-sm"
                  >
                    {/* Observateur seulement quand rien ne lui est accorde :
                        le plafond du role est applique a l'ecriture, et offrir
                        un choix que le serveur annule est pire que ne pas
                        l'offrir. */}
                    {!mayRestart ? <option value="observer">{t('agents.roleObserver')}</option> : null}
                    {mayRestart ? <option value="operator">{t('agents.roleOperator')}</option> : null}
                    <option value="lead">{t('agents.roleLead')}</option>
                  </select>
                </Field>
              ) : null}
            </div>
          ) : null}

          <div className="rounded-lg border border-border bg-surface-muted px-4 py-3">
            <label className="flex items-start gap-3">
              <Switch checked={mayRestart} onCheckedChange={setMayRestart} />
              <span className="space-y-1">
                <span className="block text-sm font-medium">{t('agents.mayRestartLabel')}</span>
                <span className="block text-xs leading-relaxed text-muted-foreground">
                  {t('agents.mayRestartHelp')}
                </span>
              </span>
            </label>
          </div>
        </DialogBody>
        <DialogFooter>
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            {t('common.cancel')}
          </Button>
          <Button onClick={() => save.mutate()} disabled={!ready || save.isPending}>
            {editing ? t('common.save') : t('agents.hire')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
