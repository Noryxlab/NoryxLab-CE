import * as React from 'react';
import { useMutation } from '@tanstack/react-query';
import { Building2, ChevronDown, ChevronRight, MoreHorizontal, Plus, ShieldCheck, Trash2, UserPlus, Users } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Field } from '@/components/ui/field';
import { Input } from '@/components/ui/input';
import { Select } from '@/components/ui/select';
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet';
import { Skeleton } from '@/components/ui/skeleton';
import { useToast } from '@/components/ui/toast';
import { SectionHeader } from '@/components/common/page-header';
import { SearchInput } from '@/components/common/search-input';
import { DataTable, type Column } from '@/components/common/data-table';
import { EmptyState } from '@/components/common/states';
import { formatDate, formatDateTime, formatRelative } from '@/lib/format';
import { useConfirm } from '@/components/common/confirm-dialog';
import { adminApi } from '@/lib/api/endpoints';
import {
  qk,
  useAdminOrganizations,
  useAdminTeamMembers,
  useAdminTeams,
  useAdminUsers,
  useInvalidate,
  useSmtp,
} from '@/lib/api/queries';
import { useT } from '@/lib/i18n';
import type { Organization, PlatformUser, Team } from '@/lib/api/types';
import { DeactivateUserSheet } from './deactivate-user';
import { TeamUsageSection } from './team-usage';

/**
 * Les personnes, et ou elles se rangent.
 *
 * Trois objets qui n'en font qu'un pour celui qui administre : une
 * organisation est un fait de l'annuaire, une equipe est ce que l'organisation
 * en fait, une personne appartient aux deux. Ils etaient sur un onglet en trois
 * sections ecrites a des moments differents, sans que rien ne dessine le lien -
 * on lisait l'organisation d'une personne dans une colonne et ses equipes dans
 * une autre, et il fallait ouvrir chaque equipe pour savoir qui y etait.
 *
 * Ici l'arbre est la verite : organisation, puis equipes, puis personnes. Ce
 * qui est a gauche se lit, ce qui est a droite se modifie, et les trois objets
 * obeissent aux memes gestes - creer en haut du parent, appartenances au
 * milieu, supprimer en bas avec l'obstacle nomme avant le clic.
 *
 * Ce que l'ecran ne fait pas, volontairement : supprimer une organisation.
 * Elle est portee par l'annuaire, et c'est la que ca se decide.
 */

type Node =
  | { kind: 'organization'; organization: Organization }
  | { kind: 'team'; team: Team; organization: Organization }
  | { kind: 'person'; user: PlatformUser }
  | { kind: 'unaffiliated' };

/** Ce que la pastille dit : vert a deja travaille, orange n'est jamais venu,
 *  rouge est desactive. Le rouge gagne sur l'orange : un compte desactive qui
 *  n'est jamais venu est d'abord desactive. */
const presenceOf = (user: PlatformUser): 'active' | 'never' | 'disabled' =>
  user.enabled === false ? 'disabled' : user.lastSeenAt ? 'active' : 'never';
const presenceClass: Record<ReturnType<typeof presenceOf>, string> = {
  active: 'bg-emerald-500',
  never: 'bg-amber-400',
  disabled: 'bg-destructive',
};
function PresenceDot({ user }: { user: PlatformUser }) {
  const t = useT();
  const state = presenceOf(user);
  return (
    <span
      className={`inline-block size-2 shrink-0 rounded-full ${presenceClass[state]}`}
      title={t(`people.presence_${state}`)}
      aria-label={t(`people.presence_${state}`)}
    />
  );
}

/** Le marqueur administrateur, le meme qu'avant la refonte : ShieldCheck sur
 *  un badge brand, avec l'explication en infobulle. Il etait dans chaque
 *  ligne de l'ancienne table et n'etait plus que dans le panneau - il fallait
 *  ouvrir chaque personne pour savoir, l'inverse du but. Compact dans l'arbre,
 *  ou un badge entier a la profondeur 2 mangerait le nom. */
function AdminMark({ user, compact }: { user: PlatformUser; compact?: boolean }) {
  const t = useT();
  if (!user.administrator) return null;
  if (compact) {
    return (
      <span className="inline-flex shrink-0" title={t('people.administratorHint')} aria-label={t('admin.administrator')}>
        <ShieldCheck aria-hidden className="size-3.5 text-primary" />
      </span>
    );
  }
  return (
    <Badge tone="brand" title={t('people.administratorHint')}>
      <ShieldCheck aria-hidden className="size-3" />
      {t('admin.administrator')}
    </Badge>
  );
}

const displayName = (user: PlatformUser) =>
  [user.firstName, user.lastName].filter(Boolean).join(' ') || user.username;

export function PeopleSection() {
  const t = useT();
  const organizations = useAdminOrganizations();
  const users = useAdminUsers();
  const [selected, setSelected] = React.useState<Node | null>(null);
  const [search, setSearch] = React.useState('');
  // Deux lectures du meme ensemble. L'arbre repond "qui est ou" ; la liste
  // repond "qui est venu, et quand" - la question d'une revue d'acces, qui se
  // trie par date et ne se lit pas branche par branche.
  const [view, setView] = React.useState<'tree' | 'list'>(() => {
    try { return (sessionStorage.getItem('people-view') as 'tree' | 'list') || 'tree'; } catch { return 'tree'; }
  });
  const switchView = (next: 'tree' | 'list') => {
    setView(next);
    try { sessionStorage.setItem('people-view', next); } catch { /* navigation privee */ }
  };

  const people = users.data ?? [];
  const query = search.trim().toLowerCase();
  const matches = (user: PlatformUser) =>
    !query ||
    [user.username, user.email, user.firstName, user.lastName]
      .filter(Boolean)
      .some((value) => value!.toLowerCase().includes(query));

  // Ranges par organisation, en lisant toutes celles d'une personne : l'annuaire
  // en autorise plusieurs, et n'en montrer qu'une placerait quelqu'un au hasard.
  const byOrganization = React.useMemo(() => {
    const map = new Map<string, PlatformUser[]>();
    for (const user of people) {
      for (const name of user.organizations ?? (user.organization ? [user.organization] : [])) {
        map.set(name, [...(map.get(name) ?? []), user]);
      }
    }
    return map;
  }, [people]);
  const unaffiliated = people.filter((user) => !(user.organizations?.length || user.organization));

  if (organizations.isLoading || users.isLoading) {
    return <Skeleton className="h-64 w-full" />;
  }

  return (
    <div className="space-y-4">
      <SectionHeader
        title={t('people.title')}
        description={t('people.hint')}
        actions={
          <span className="flex items-center gap-2">
            <SearchInput value={search} onValueChange={setSearch} label={t('common.search')} className="w-56" />
            <Button variant={view === 'tree' ? 'primary' : 'secondary'} size="sm" onClick={() => switchView('tree')}>
              {t('people.viewTree')}
            </Button>
            <Button variant={view === 'list' ? 'primary' : 'secondary'} size="sm" onClick={() => switchView('list')}>
              {t('people.viewList')}
            </Button>
          </span>
        }
      />

      {view === 'list' ? (
        <PeopleList people={people} search={search} onOpen={(user) => { setSelected({ kind: 'person', user }); switchView('tree'); }} />
      ) : null}

      <div className={view === 'list' ? 'hidden' : 'grid gap-4 lg:grid-cols-[minmax(280px,1fr)_2fr]'}>
        {/* L'arbre. Il se lit, il ne se modifie pas : chaque noeud ouvre son
            panneau a droite, et c'est la que tout se fait. */}
        <Card>
          <CardContent className="space-y-1 p-2">
            <TreeRoot
              label={t('people.newOrganization')}
              onSelect={() => setSelected(null)}
              active={selected === null}
            />
            {(organizations.data ?? []).map((organization) => (
              <OrganizationBranch
                key={organization.id}
                organization={organization}
                people={(byOrganization.get(organization.name) ?? []).filter(matches)}
                selected={selected}
                onSelect={setSelected}
                filtering={Boolean(query)}
              />
            ))}
            {unaffiliated.filter(matches).length > 0 ? (
              <TreeLeaf
                depth={0}
                icon={<Users className="size-4 text-muted-foreground" />}
                label={t('people.unaffiliated')}
                count={unaffiliated.filter(matches).length}
                active={selected?.kind === 'unaffiliated'}
                onSelect={() => setSelected({ kind: 'unaffiliated' })}
              />
            ) : null}
          </CardContent>
        </Card>

        {/* Le panneau. Un seul composant par type d'objet, et les trois
            partagent la meme grammaire. */}
        <div className="space-y-4">
          {selected === null ? <RootPanel /> : null}
          {selected?.kind === 'organization' ? (
            <OrganizationPanel
              organization={selected.organization}
              people={byOrganization.get(selected.organization.name) ?? []}
              everyone={people}
              onSelect={setSelected}
            />
          ) : null}
          {selected?.kind === 'team' ? (
            <TeamPanel
              team={selected.team}
              organization={selected.organization}
              onDeleted={() => setSelected({ kind: 'organization', organization: selected.organization })}
            />
          ) : null}
          {selected?.kind === 'person' ? <PersonPanel user={selected.user} /> : null}
          {selected?.kind === 'unaffiliated' ? (
            <UnaffiliatedPanel people={unaffiliated} onSelect={setSelected} />
          ) : null}
        </div>
      </div>
    </div>
  );
}

/* -- la liste --------------------------------------------------------------- */

function PeopleList({ people, search, onOpen }: { people: PlatformUser[]; search: string; onOpen: (u: PlatformUser) => void }) {
  const t = useT();
  const orgs = (u: PlatformUser) => (u.organizations ?? (u.organization ? [u.organization] : [])).join(', ');
  // Les colonnes de l'ancienne table, plus la premiere connexion. Le tri par
  // defaut met les plus recemment vus en haut ; sortValue rend null pour les
  // jamais-venus, que la table range ensemble en bas - la liste qu'une revue
  // d'acces relance.
  const columns: Column<PlatformUser>[] = [
    {
      id: 'user',
      header: t('common.user'),
      sortValue: (u) => displayName(u).toLowerCase(),
      searchValue: (u) => [u.username, u.email, u.firstName, u.lastName].filter(Boolean).join(' '),
      cell: (u) => (
        <span className="flex items-center gap-2">
          <PresenceDot user={u} />
          <span>{displayName(u)}</span>
          <AdminMark user={u} />
          <span className="text-xs text-muted-foreground">{u.username}</span>
        </span>
      ),
    },
    { id: 'organization', header: t('common.organization'), sortValue: (u) => orgs(u) || null, searchValue: orgs,
      cell: (u) => <span className="text-xs">{orgs(u) || '—'}</span> },
    { id: 'teams', header: t('admin.teams'), sortValue: (u) => (u.teams ?? []).join(', ') || null, searchValue: (u) => (u.teams ?? []).join(' '),
      cell: (u) => <span className="text-xs">{u.teams?.join(', ') || '—'}</span> },
    { id: 'firstSeen', header: t('people.firstSeen'), sortValue: (u) => (u.firstSeenAt ? new Date(u.firstSeenAt).getTime() : null),
      cell: (u) => <span className="text-xs tabular-nums" title={u.firstSeenAt ? formatDateTime(u.firstSeenAt) : undefined}>{u.firstSeenAt ? formatDate(u.firstSeenAt) : '—'}</span> },
    { id: 'lastSeen', header: t('admin.lastSeen'), sortValue: (u) => (u.lastSeenAt ? new Date(u.lastSeenAt).getTime() : null),
      cell: (u) => u.lastSeenAt
        ? <span className="text-xs text-muted-foreground" title={formatDateTime(u.lastSeenAt)}>{formatRelative(u.lastSeenAt)}</span>
        : <span className="text-xs text-amber-600">{t('people.neverSeen')}</span> },
  ];
  return (
    <DataTable
      data={people}
      columns={columns}
      rowKey={(u) => u.id}
      search={search}
      defaultSort={{ columnId: 'lastSeen', direction: 'desc' }}
      onRowClick={onOpen}
      emptyState={<EmptyState title={t('people.nobody')} />}
    />
  );
}

/* -- l'arbre ---------------------------------------------------------------- */

function TreeRoot({ label, onSelect, active }: { label: string; onSelect: () => void; active: boolean }) {
  return (
    <button
      type="button"
      onClick={onSelect}
      className={`flex w-full items-center gap-2 rounded px-2 py-1.5 text-left text-sm ${active ? 'bg-accent font-medium' : 'hover:bg-accent/50'}`}
    >
      <Plus className="size-4 text-muted-foreground" />
      {label}
    </button>
  );
}

function TreeLeaf({
  depth,
  icon,
  label,
  count,
  active,
  onSelect,
  chevron,
  trailing,
}: {
  depth: number;
  icon: React.ReactNode;
  label: string;
  count?: number;
  active: boolean;
  onSelect: () => void;
  chevron?: React.ReactNode;
  trailing?: React.ReactNode;
}) {
  return (
    <button
      type="button"
      onClick={onSelect}
      style={{ paddingLeft: `${8 + depth * 16}px` }}
      className={`flex w-full items-center gap-2 rounded py-1.5 pr-2 text-left text-sm ${active ? 'bg-accent font-medium' : 'hover:bg-accent/50'}`}
    >
      {chevron ?? <span className="w-4" />}
      {icon}
      <span className="truncate">{label}</span>
      {trailing}
      {count !== undefined ? (
        <span className="ml-auto text-xs tabular-nums text-muted-foreground">{count}</span>
      ) : null}
    </button>
  );
}

function OrganizationBranch({
  organization,
  people,
  selected,
  onSelect,
  filtering,
}: {
  organization: Organization;
  people: PlatformUser[];
  selected: Node | null;
  onSelect: (node: Node) => void;
  filtering: boolean;
}) {
  const teams = useAdminTeams(organization.id);
  // Deplie a la main, ou par la recherche : un filtre qui trouve quelqu'un
  // dans une branche fermee ne sert a rien.
  const [open, setOpen] = React.useState(false);
  const expanded = open || filtering;
  const items = teams.data ?? [];
  const inTeam = new Set(items.flatMap((team) => people.filter((p) => p.teams?.includes(team.name)).map((p) => p.username)));
  const direct = people.filter((p) => !inTeam.has(p.username));

  return (
    <div>
      <TreeLeaf
        depth={0}
        icon={<Building2 className="size-4 text-muted-foreground" />}
        label={organization.name}
        count={people.length}
        active={selected?.kind === 'organization' && selected.organization.id === organization.id}
        onSelect={() => {
          setOpen(true);
          onSelect({ kind: 'organization', organization });
        }}
        chevron={
          <span
            role="button"
            tabIndex={-1}
            onClick={(event) => {
              event.stopPropagation();
              setOpen((value) => !value);
            }}
            className="inline-flex"
          >
            {expanded ? <ChevronDown className="size-4" /> : <ChevronRight className="size-4" />}
          </span>
        }
      />
      {expanded ? (
        <>
          {items.map((team) => {
            const members = people.filter((p) => p.teams?.includes(team.name));
            return (
              <div key={team.id}>
                <TreeLeaf
                  depth={1}
                  icon={<Users className="size-4 text-muted-foreground" />}
                  label={team.name}
                  count={members.length}
                  active={selected?.kind === 'team' && selected.team.id === team.id}
                  onSelect={() => onSelect({ kind: 'team', team, organization })}
                />
                {members.map((user) => (
                  <PersonLeaf key={user.id} depth={2} user={user} selected={selected} onSelect={onSelect} />
                ))}
              </div>
            );
          })}
          {direct.map((user) => (
            <PersonLeaf key={user.id} depth={1} user={user} selected={selected} onSelect={onSelect} />
          ))}
        </>
      ) : null}
    </div>
  );
}

function PersonLeaf({
  depth,
  user,
  selected,
  onSelect,
}: {
  depth: number;
  user: PlatformUser;
  selected: Node | null;
  onSelect: (node: Node) => void;
}) {
  return (
    <TreeLeaf
      depth={depth}
      icon={<PresenceDot user={user} />}
      label={displayName(user)}
      trailing={<AdminMark user={user} compact />}
      active={selected?.kind === 'person' && selected.user.id === user.id}
      onSelect={() => onSelect({ kind: 'person', user })}
    />
  );
}

/* -- les panneaux ------------------------------------------------------------ */

/** La racine : creer une organisation. Le seul geste possible a ce niveau,
 *  parce que les organisations sont portees par l'annuaire - on en cree ici,
 *  on ne les supprime pas ici. */
function RootPanel() {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const [name, setName] = React.useState('');
  const [alias, setAlias] = React.useState('');
  const create = useMutation({
    mutationFn: () => adminApi.createOrganization({ name: name.trim(), alias: alias.trim() || undefined }),
    onSuccess: () => {
      invalidate(qk.adminOrganizations, qk.organizations);
      setName('');
      setAlias('');
      toast.success(t('people.organizationCreated'), t('common.organization'));
    },
    onError: (error) => toast.error(error, t('people.newOrganization')),
  });
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('people.newOrganization')}</CardTitle>
        <CardDescription>{t('people.newOrganizationHint')}</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-wrap items-end gap-2">
        <Field label={t('common.name')} required className="min-w-56">
          <Input value={name} onChange={(e) => setName(e.target.value)} maxLength={120} />
        </Field>
        <Field label={t('people.alias')} className="min-w-40">
          <Input value={alias} onChange={(e) => setAlias(e.target.value)} maxLength={60} />
        </Field>
        <Button variant="primary" disabled={!name.trim()} loading={create.isPending} onClick={() => create.mutate()}>
          <Plus aria-hidden />
          {t('common.create')}
        </Button>
      </CardContent>
    </Card>
  );
}

function OrganizationPanel({
  organization,
  people,
  everyone,
  onSelect,
}: {
  organization: Organization;
  people: PlatformUser[];
  everyone: PlatformUser[];
  onSelect: (node: Node) => void;
}) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const teams = useAdminTeams(organization.id);
  const [teamName, setTeamName] = React.useState('');
  const [memberToAdd, setMemberToAdd] = React.useState('');
  const [creatingUser, setCreatingUser] = React.useState(false);

  const createTeam = useMutation({
    mutationFn: () => adminApi.createTeam(organization.id, { name: teamName.trim() }),
    onSuccess: () => {
      invalidate(qk.adminTeams(organization.id));
      setTeamName('');
      toast.success(t('admin.teamCreated'), organization.name);
    },
    onError: (error) => toast.error(error, t('admin.teams')),
  });
  const addMember = useMutation({
    mutationFn: () => adminApi.addOrganizationMember(organization.id, memberToAdd),
    onSuccess: () => {
      invalidate(qk.adminUsers, qk.adminOrganizationMembers(organization.id));
      setMemberToAdd('');
    },
    onError: (error) => toast.error(error, t('people.addToOrganization')),
  });
  const removeMember = useMutation({
    mutationFn: (userId: string) => adminApi.removeOrganizationMember(organization.id, userId),
    onSuccess: () => invalidate(qk.adminUsers, qk.adminOrganizationMembers(organization.id)),
    onError: (error) => toast.error(error, t('common.delete')),
  });

  const inside = new Set(people.map((p) => p.username.toLowerCase()));
  const candidates = everyone
    .filter((p) => p.enabled !== false && !inside.has(p.username.toLowerCase()))
    .map((p) => ({ value: p.username, label: displayName(p), hint: p.username }))
    .sort((a, b) => a.label.localeCompare(b.label));

  return (
    <>
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Building2 className="size-5 text-muted-foreground" />
            {organization.name}
            {organization.alias ? <Badge tone="outline">{organization.alias}</Badge> : null}
          </CardTitle>
          <CardDescription>
            {t('people.organizationSummary', { people: people.length, teams: (teams.data ?? []).length })}
          </CardDescription>
        </CardHeader>
        <CardContent className="space-y-5">
          {/* Creer, en haut. Une equipe ou une personne, depuis leur parent. */}
          <div className="flex flex-wrap items-end gap-2">
            <Field label={t('people.newTeam')} className="min-w-56">
              <Input value={teamName} onChange={(e) => setTeamName(e.target.value)} maxLength={120} />
            </Field>
            <Button variant="secondary" disabled={!teamName.trim()} loading={createTeam.isPending} onClick={() => createTeam.mutate()}>
              <Plus aria-hidden />
              {t('common.create')}
            </Button>
            <Button variant="secondary" onClick={() => setCreatingUser(true)}>
              <UserPlus aria-hidden />
              {t('identity.createUser')}
            </Button>
          </div>

          {/* Appartenances, au milieu : qui est dans l'organisation. */}
          <div className="space-y-2">
            <div className="flex flex-wrap items-end gap-2">
              <Field label={t('people.addToOrganization')} className="min-w-72">
                <Select value={memberToAdd} onValueChange={setMemberToAdd} options={candidates} placeholder={t('admin.teamMemberPlaceholder')} />
              </Field>
              <Button variant="secondary" disabled={!memberToAdd} loading={addMember.isPending} onClick={() => addMember.mutate()}>
                <UserPlus aria-hidden />
                {t('common.add')}
              </Button>
            </div>
            <MemberList people={people} onOpen={(user) => onSelect({ kind: 'person', user })} onRemove={(user) => removeMember.mutate(user.username)} removing={removeMember.isPending} />
          </div>
        </CardContent>
      </Card>

      <TeamUsageSection />

      <CreateUserSheet organization={organization} open={creatingUser} onOpenChange={setCreatingUser} />
    </>
  );
}

function TeamPanel({ team, organization, onDeleted }: { team: Team; organization: Organization; onDeleted: () => void }) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const { dialog, ask } = useConfirm();
  const users = useAdminUsers();
  const members = useAdminTeamMembers(team.id);
  const [name, setName] = React.useState(team.name);
  const [memberToAdd, setMemberToAdd] = React.useState('');
  React.useEffect(() => setName(team.name), [team.id, team.name]);

  const refresh = () => invalidate(qk.adminTeamMembers(team.id), qk.adminTeams(organization.id), qk.adminUsers);
  const rename = useMutation({
    mutationFn: () => adminApi.updateTeam(team.id, { name: name.trim() }),
    onSuccess: () => {
      invalidate(qk.adminTeams(organization.id), qk.adminUsers);
      toast.success(t('people.teamRenamed'), team.name);
    },
    onError: (error) => toast.error(error, t('admin.teams')),
  });
  const add = useMutation({
    mutationFn: () => adminApi.addTeamMember(team.id, memberToAdd),
    onSuccess: () => {
      refresh();
      setMemberToAdd('');
    },
    onError: (error) => toast.error(error, team.name),
  });
  const remove = useMutation({
    mutationFn: (userId: string) => adminApi.removeTeamMember(team.id, userId),
    onSuccess: refresh,
    onError: (error) => toast.error(error, team.name),
  });
  const destroy = useMutation({
    mutationFn: () => adminApi.removeTeam(team.id),
    onSuccess: () => {
      refresh();
      toast.success(t('admin.teamRemoved'), team.name);
      onDeleted();
    },
    onError: (error) => toast.error(error, t('common.delete')),
  });

  const items = members.data ?? [];
  const inside = new Set(items.map((m) => m.userId.toLowerCase()));
  const everyone = users.data ?? [];
  const byUsername = new Map(everyone.map((u) => [u.username.toLowerCase(), u]));
  // Les candidats sont les membres de l'organisation, pas toute la plateforme :
  // une equipe est une subdivision de son organisation, et proposer quelqu'un
  // d'ailleurs offrirait une erreur a decouvrir apres coup.
  const candidates = everyone
    .filter((u) => u.enabled !== false && !inside.has(u.username.toLowerCase()))
    .filter((u) => (u.organizations ?? [u.organization ?? '']).includes(organization.name))
    .map((u) => ({ value: u.username, label: displayName(u), hint: u.username }))
    .sort((a, b) => a.label.localeCompare(b.label));

  return (
    <Card>
      {dialog}
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          <Users className="size-5 text-muted-foreground" />
          {team.name}
          <span className="text-sm font-normal text-muted-foreground">· {organization.name}</span>
        </CardTitle>
        <CardDescription>{t('people.teamSummary', { count: items.length })}</CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        <div className="flex flex-wrap items-end gap-2">
          <Field label={t('common.name')} className="min-w-56">
            <Input value={name} onChange={(e) => setName(e.target.value)} maxLength={120} />
          </Field>
          <Button variant="secondary" disabled={!name.trim() || name.trim() === team.name} loading={rename.isPending} onClick={() => rename.mutate()}>
            {t('common.save')}
          </Button>
        </div>

        <div className="space-y-2">
          <div className="flex flex-wrap items-end gap-2">
            <Field label={t('admin.teamMemberAdd')} className="min-w-72">
              <Select value={memberToAdd} onValueChange={setMemberToAdd} options={candidates} placeholder={t('admin.teamMemberPlaceholder')} />
            </Field>
            <Button variant="secondary" disabled={!memberToAdd} loading={add.isPending} onClick={() => add.mutate()}>
              <UserPlus aria-hidden />
              {t('common.add')}
            </Button>
          </div>
          {members.isLoading ? (
            <Skeleton className="h-12 w-full" />
          ) : items.length === 0 ? (
            <EmptyState title={t('admin.teamNoMembers')} className="py-4" />
          ) : (
            <ul className="divide-y text-sm">
              {items.map((member) => {
                const user = byUsername.get(member.userId.toLowerCase());
                return (
                  <li key={member.userId} className="flex items-center gap-2 py-1.5">
                    <span>{user ? displayName(user) : member.userId}</span>
                    <span className="text-xs text-muted-foreground">{formatDate(member.joinedAt)}</span>
                    <Button variant="ghost" size="sm" className="ml-auto" onClick={() => remove.mutate(member.userId)} aria-label={t('admin.teamMemberRemove')}>
                      <Trash2 className="size-4" />
                    </Button>
                  </li>
                );
              })}
            </ul>
          )}
        </div>

        {/* Supprimer, en bas, avec ce que ca emporte dit avant le clic : les
            membres perdent leur acces par cette equipe, et les octrois sur les
            projets partent avec elle - c'est ce que fait le serveur, et c'est
            ecrit ici plutot que decouvert apres. */}
        <div className="border-t pt-4">
          <Button
            variant="danger-outline"
            size="sm"
            loading={destroy.isPending}
            onClick={() =>
              ask({
                title: t('admin.teamRemove'),
                description: t('people.teamRemoveWarning', { name: team.name, count: items.length }),
                confirmLabel: t('common.delete'),
                destructive: true,
                onConfirm: () => destroy.mutateAsync(),
              })
            }
          >
            <Trash2 aria-hidden />
            {t('admin.teamRemove')}
          </Button>
          <p className="mt-1 text-xs text-muted-foreground">{t('people.teamRemoveHint', { count: items.length })}</p>
        </div>
      </CardContent>
    </Card>
  );
}

function PersonPanel({ user }: { user: PlatformUser }) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const smtp = useSmtp();
  const { dialog, ask } = useConfirm();
  const [deactivating, setDeactivating] = React.useState<PlatformUser | null>(null);

  const reactivate = useMutation({
    mutationFn: () => adminApi.reactivateUser(user.id),
    onSuccess: () => {
      invalidate(qk.adminUsers);
      toast.success(t('admin.reactivate'), displayName(user));
    },
    onError: (error) => toast.error(error, t('admin.reactivate')),
  });
  const resetByEmail = useMutation({
    mutationFn: () => adminApi.sendPasswordResetEmail(user.id),
    onSuccess: () => toast.success(t('admin.resetByEmailSent'), displayName(user)),
    onError: (error) => toast.error(error, t('admin.resetByEmail')),
  });
  const resetPassword = useMutation({
    mutationFn: () => adminApi.resetUserPassword(user.id),
    onSuccess: (result) => {
      if (result.temporaryPassword) {
        void navigator.clipboard?.writeText(result.temporaryPassword);
        toast.success(t('identity.passwordCopied'), displayName(user));
      }
    },
    onError: (error) => toast.error(error, t('identity.resetPassword')),
  });

  const disabled = user.enabled === false;
  return (
    <Card>
      {dialog}
      <CardHeader>
        <CardTitle className="flex items-center gap-2">
          {displayName(user)}
          <AdminMark user={user} />
          {disabled ? <Badge tone="danger">{t('people.deactivated')}</Badge> : null}
        </CardTitle>
        <CardDescription className="font-mono text-xs">
          {user.username} · {user.email || '—'}
        </CardDescription>
      </CardHeader>
      <CardContent className="space-y-5">
        {/* Ou cette personne se range. Lu depuis la liste, jamais recalcule ici. */}
        <dl className="grid gap-2 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-xs text-muted-foreground">{t('common.organization')}</dt>
            <dd>{(user.organizations ?? (user.organization ? [user.organization] : [])).join(', ') || '—'}</dd>
          </div>
          <div>
            <dt className="text-xs text-muted-foreground">{t('admin.teams')}</dt>
            <dd className="flex flex-wrap gap-1">
              {user.teams?.length ? user.teams.map((n) => <Badge key={n} tone="outline">{n}</Badge>) : '—'}
            </dd>
          </div>
          <div>
            <dt className="text-xs text-muted-foreground">{t('people.firstSeen')}</dt>
            <dd>{user.firstSeenAt ? formatDateTime(user.firstSeenAt) : '—'}</dd>
          </div>
          <div>
            <dt className="text-xs text-muted-foreground">{t('admin.lastSeen')}</dt>
            <dd>{user.lastSeenAt ? formatDateTime(user.lastSeenAt) : t('people.neverSeen')}</dd>
          </div>
        </dl>

        {/* Les gestes sur le compte, dans l'ordre du risque. */}
        <div className="flex flex-wrap gap-2 border-t pt-4">
          {!disabled && smtp.data?.configured ? (
            <Button variant="secondary" size="sm" loading={resetByEmail.isPending} onClick={() => ask({ title: t('admin.resetByEmail'), description: t('admin.resetByEmailHint', { user: user.email }), confirmLabel: t('admin.resetByEmail'), onConfirm: () => resetByEmail.mutateAsync() })}>
              {t('admin.resetByEmail')}
            </Button>
          ) : null}
          {!disabled ? (
            <Button variant="secondary" size="sm" loading={resetPassword.isPending} onClick={() => ask({ title: t('identity.resetPassword'), description: t('identity.resetPasswordHint', { user: user.username }), confirmLabel: t('identity.resetPassword'), onConfirm: () => resetPassword.mutateAsync() })}>
              {t('identity.resetPassword')}
            </Button>
          ) : (
            <Button variant="secondary" size="sm" loading={reactivate.isPending} onClick={() => reactivate.mutate()}>
              {t('admin.reactivate')}
            </Button>
          )}
          {/* Desactiver puis supprimer : deux etapes, jamais une. Le composant
              qui les porte demande un successeur pour ce que le compte possede,
              avant de s'engager - il est repris tel quel. */}
          <Button variant="danger-outline" size="sm" className="ml-auto" onClick={() => setDeactivating(user)}>
            <Trash2 aria-hidden />
            {disabled ? t('admin.deleteAccount') : t('admin.deactivate')}
          </Button>
        </div>
      </CardContent>
      <DeactivateUserSheet user={deactivating} open={deactivating !== null} onOpenChange={(open) => !open && setDeactivating(null)} />
    </Card>
  );
}

function UnaffiliatedPanel({ people, onSelect }: { people: PlatformUser[]; onSelect: (node: Node) => void }) {
  const t = useT();
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('people.unaffiliated')}</CardTitle>
        <CardDescription>{t('people.unaffiliatedHint')}</CardDescription>
      </CardHeader>
      <CardContent>
        <MemberList people={people} onOpen={(user) => onSelect({ kind: 'person', user })} />
      </CardContent>
    </Card>
  );
}

function MemberList({ people, onOpen, onRemove, removing }: { people: PlatformUser[]; onOpen: (u: PlatformUser) => void; onRemove?: (u: PlatformUser) => void; removing?: boolean }) {
  const t = useT();
  if (people.length === 0) return <EmptyState title={t('people.nobody')} className="py-4" />;
  return (
    <ul className="divide-y text-sm">
      {[...people].sort((a, b) => displayName(a).localeCompare(displayName(b))).map((user) => (
        <li key={user.id} className="flex items-center gap-2 py-1.5">
          <PresenceDot user={user} />
          <button type="button" className="text-left hover:underline" onClick={() => onOpen(user)}>{displayName(user)}</button>
          <AdminMark user={user} compact />
          <span className="text-xs text-muted-foreground">{user.username}</span>
          {user.teams?.length ? <span className="text-xs text-muted-foreground">· {user.teams.join(', ')}</span> : null}
          {onRemove ? (
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <Button variant="ghost" size="sm" className="ml-auto" aria-label={t('common.actions')} disabled={removing}><MoreHorizontal className="size-4" /></Button>
              </DropdownMenuTrigger>
              <DropdownMenuContent align="end">
                <DropdownMenuItem destructive onSelect={() => onRemove(user)}>{t('people.removeFromOrganization')}</DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          ) : null}
        </li>
      ))}
    </ul>
  );
}

/** La fiche de creation, reprise de l'ancien onglet : le mot de passe emis
 *  n'existe que dans la reponse et s'affiche une seule fois. L'organisation est
 *  celle du panneau d'ou l'on vient - on cree quelqu'un *dans* une
 *  organisation, pas dans le vide. */
function CreateUserSheet({ organization, open, onOpenChange }: { organization: Organization; open: boolean; onOpenChange: (o: boolean) => void }) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const blank = { username: '', email: '', firstName: '', lastName: '', organizationId: organization.id };
  const [form, setForm] = React.useState(blank);
  const [issued, setIssued] = React.useState<{ who: string; password: string } | null>(null);
  // Remise a zero a l'ouverture, et rattachee a l'organisation du panneau
  // courant : les deux seules choses dont la fiche depend vraiment.
  React.useEffect(() => {
    if (!open) return;
    setForm({ username: '', email: '', firstName: '', lastName: '', organizationId: organization.id });
    setIssued(null);
  }, [open, organization.id]);

  const create = useMutation({
    mutationFn: () => adminApi.createUser(form),
    onSuccess: (result) => {
      invalidate(qk.adminUsers, qk.adminOrganizationMembers(organization.id));
      if (result.temporaryPassword) setIssued({ who: result.username ?? form.username, password: result.temporaryPassword });
      else onOpenChange(false);
    },
    onError: (error) => toast.error(error, t('identity.createUser')),
  });
  const set = (key: keyof typeof blank) => (e: React.ChangeEvent<HTMLInputElement>) => setForm((f) => ({ ...f, [key]: e.target.value }));

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent>
        <SheetHeader>
          <SheetTitle>{t('identity.createUser')}</SheetTitle>
          <SheetDescription>{t('people.createUserIn', { organization: organization.name })}</SheetDescription>
        </SheetHeader>
        {issued ? (
          <Card className="mt-4">
            <CardHeader>
              <CardTitle>{issued.who}</CardTitle>
              <CardDescription>{t('identity.passwordHint')}</CardDescription>
            </CardHeader>
            <CardContent className="space-y-2">
              <code className="block rounded bg-muted p-2 font-mono text-sm">{issued.password}</code>
              <Button variant="secondary" size="sm" onClick={() => { void navigator.clipboard?.writeText(issued.password); toast.success(t('identity.passwordCopied'), issued.who); }}>{t('common.copy')}</Button>
            </CardContent>
          </Card>
        ) : (
          <div className="mt-4 space-y-3">
            <Field label={t('identity.username')} required><Input value={form.username} onChange={set('username')} autoFocus /></Field>
            <Field label="Email" required><Input type="email" value={form.email} onChange={set('email')} /></Field>
            <div className="flex gap-2">
              <Field label={t('identity.firstName')} className="flex-1"><Input value={form.firstName} onChange={set('firstName')} /></Field>
              <Field label={t('identity.lastName')} className="flex-1"><Input value={form.lastName} onChange={set('lastName')} /></Field>
            </div>
            <Button variant="primary" disabled={!form.username.trim() || !form.email.trim()} loading={create.isPending} onClick={() => create.mutate()}>
              <UserPlus aria-hidden />
              {t('identity.createUser')}
            </Button>
          </div>
        )}
      </SheetContent>
    </Sheet>
  );
}
