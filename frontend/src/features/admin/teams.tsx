import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Plus, Trash2, UserPlus, Users } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { SectionHeader } from '@/components/common/page-header';
import { useConfirm } from '@/components/common/confirm-dialog';
import { useToast } from '@/components/ui/toast';
import { Skeleton } from '@/components/ui/skeleton';
import { adminApi } from '@/lib/api/endpoints';
import { qk, useAdminTeams, useAdminTeamMembers, useInvalidate } from '@/lib/api/queries';
import { useT } from '@/lib/i18n';
import type { Organization, Team } from '@/lib/api/types';

/**
 * Les equipes d'une organisation, et qui les compose.
 *
 * Une equipe est l'unite dans laquelle le travail est reellement organise,
 * entre les deux que la plateforme savait deja nommer : une organisation est
 * un fait d'identite porte par l'annuaire, une personne est un droit accorde
 * projet par projet.
 *
 * Cet ecran ne decide pas ce qu'une equipe atteint. Composer une equipe et lui
 * ouvrir un projet sont deux autorites differentes - c'est ce qui empeche un
 * chef d'equipe de s'octroyer un projet - et l'octroi se fait donc depuis le
 * projet, pas d'ici. La colonne "projets" ci-dessous est une lecture, jamais
 * une commande.
 */

export function TeamsSection({ organization }: { organization: Organization | null }) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();

  const organizationId = organization?.id ?? '';
  const teams = useAdminTeams(organizationId);
  const [draftName, setDraftName] = React.useState('');
  const [openTeamId, setOpenTeamId] = React.useState<string | null>(null);

  const create = useMutation({
    mutationFn: (name: string) => adminApi.createTeam(organizationId, { name }),
    onSuccess: () => {
      invalidate(qk.adminTeams(organizationId));
      setDraftName('');
      toast.success(t('admin.teamCreated'), t('admin.teams'));
    },
    onError: (error) => toast.error(error, t('admin.teams')),
  });

  const remove = useMutation({
    mutationFn: (teamId: string) => adminApi.removeTeam(teamId),
    onSuccess: () => {
      invalidate(qk.adminTeams(organizationId));
      setOpenTeamId(null);
      toast.success(t('admin.teamRemoved'), t('admin.teams'));
    },
    onError: (error) => toast.error(error, t('admin.teams')),
  });

  const items = teams.data ?? [];

  if (!organization) {
    return (
      <section className="space-y-3">
        <SectionHeader title={t('admin.teams')} description={t('admin.teamsHint')} />
        <p className="text-sm text-muted-foreground">{t('admin.teamsPickOrganization')}</p>
      </section>
    );
  }

  return (
    <section className="space-y-3">
      {dialog}
      <SectionHeader title={t('admin.teams')} description={t('admin.teamsHint')} />

      <form
        className="flex flex-wrap items-end gap-2"
        onSubmit={(event) => {
          event.preventDefault();
          const name = draftName.trim();
          if (name) create.mutate(name);
        }}
      >
        <Field label={t('admin.teamName')} className="min-w-64">
          <Input
            value={draftName}
            onChange={(event) => setDraftName(event.target.value)}
            placeholder={t('admin.teamNamePlaceholder')}
            maxLength={120}
          />
        </Field>
        <Button type="submit" variant="secondary" disabled={!draftName.trim() || create.isPending}>
          <Plus className="size-4" />
          {t('admin.teamAdd')}
        </Button>
      </form>

      {teams.isLoading ? (
        <Skeleton className="h-24 w-full" />
      ) : items.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('admin.teamsEmpty')}</p>
      ) : (
        <div className="space-y-2">
          {items.map((team) => (
            <TeamRow
              key={team.id}
              team={team}
              open={openTeamId === team.id}
              onToggle={() => setOpenTeamId(openTeamId === team.id ? null : team.id)}
              onDelete={() => {
                // Supprimer une equipe emporte ses octrois : l'annoncer avant,
                // parce que l'effet visible est ailleurs que sur cet ecran.
                ask({
                  title: t('admin.teamRemove'),
                  description: t('admin.teamRemoveWarning', { name: team.name }),
                  confirmLabel: t('common.delete'),
                  destructive: true,
                  onConfirm: () => remove.mutate(team.id),
                });
              }}
            />
          ))}
        </div>
      )}
    </section>
  );
}

function TeamRow({
  team,
  open,
  onToggle,
  onDelete,
}: {
  team: Team;
  open: boolean;
  onToggle: () => void;
  onDelete: () => void;
}) {
  const t = useT();
  return (
    <Card>
      <CardContent className="space-y-3 p-3">
        <div className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            className="flex items-center gap-2 text-left font-medium"
            onClick={onToggle}
          >
            <Users className="size-4 text-muted-foreground" />
            {team.name}
          </button>
          <Badge tone="outline">
            {t('admin.teamMemberCount', { count: team.memberCount ?? 0 })}
          </Badge>
          <div className="ml-auto">
            <Button variant="ghost" size="sm" onClick={onDelete} aria-label={t('admin.teamRemove')}>
              <Trash2 className="size-4" />
            </Button>
          </div>
        </div>
        {open ? <TeamMembers teamId={team.id} teamName={team.name} /> : null}
      </CardContent>
    </Card>
  );
}

function TeamMembers({ teamId, teamName }: { teamId: string; teamName: string }) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const members = useAdminTeamMembers(teamId);
  const [draftUser, setDraftUser] = React.useState('');

  // Les deux listes : celle de cet encart et celle du tableau au-dessus, qui
  // porte le compte. N'en rafraichir qu'une laisse un compte qui ment jusqu'au
  // prochain rechargement.
  const refresh = (organizationless = false) => {
    invalidate(qk.adminTeamMembers(teamId));
    if (!organizationless) invalidate(['admin', 'teams']);
  };

  const add = useMutation({
    mutationFn: (userId: string) => adminApi.addTeamMember(teamId, userId),
    onSuccess: () => {
      refresh();
      setDraftUser('');
      toast.success(t('admin.teamMemberAdded'), teamName);
    },
    onError: (error) => toast.error(error, teamName),
  });

  const remove = useMutation({
    mutationFn: (userId: string) => adminApi.removeTeamMember(teamId, userId),
    onSuccess: () => {
      refresh();
      toast.success(t('admin.teamMemberRemoved'), teamName);
    },
    onError: (error) => toast.error(error, teamName),
  });

  const items = members.data ?? [];

  return (
    <div className="space-y-2 border-t pt-3">
      <form
        className="flex flex-wrap items-end gap-2"
        onSubmit={(event) => {
          event.preventDefault();
          const userId = draftUser.trim();
          if (userId) add.mutate(userId);
        }}
      >
        <Field label={t('admin.teamMemberAdd')} className="min-w-64">
          <Input
            value={draftUser}
            onChange={(event) => setDraftUser(event.target.value)}
            placeholder={t('admin.teamMemberPlaceholder')}
          />
        </Field>
        <Button type="submit" variant="secondary" disabled={!draftUser.trim() || add.isPending}>
          <UserPlus className="size-4" />
          {t('common.add')}
        </Button>
      </form>

      {members.isLoading ? (
        <Skeleton className="h-12 w-full" />
      ) : items.length === 0 ? (
        <p className="text-sm text-muted-foreground">{t('admin.teamNoMembers')}</p>
      ) : (
        <ul className="divide-y text-sm">
          {items.map((member) => (
            <li key={member.userId} className="flex items-center gap-2 py-1.5">
              <span className="font-mono">{member.userId}</span>
              {/* La date d'entree est ce qu'un auditeur lit pour expliquer
                  depuis quand cette personne atteint ce que l'equipe ouvre. */}
              <span className="text-xs text-muted-foreground">
                {new Date(member.joinedAt).toLocaleDateString()}
              </span>
              <Button
                variant="ghost"
                size="sm"
                className="ml-auto"
                onClick={() => remove.mutate(member.userId)}
                aria-label={t('admin.teamMemberRemove')}
              >
                <Trash2 className="size-4" />
              </Button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}
