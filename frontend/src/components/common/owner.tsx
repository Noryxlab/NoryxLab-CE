import * as React from 'react';
import { Building2, ShieldCheck, User } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Field } from '@/components/ui/field';
import { Select } from '@/components/ui/select';
import { useAdminUsers, useOrganizations, useMyTeams } from '@/lib/api/queries';
import { useT } from '@/lib/i18n';

/**
 * One notation for an owner, wherever an owner is shown.
 *
 * The distinction is not decoration: something owned by an organization
 * outlives the person who created it and grants access through membership,
 * while something owned by a person has a single point of failure with a name.
 * Projects said this properly; datasets and ontologies showed a bare
 * identifier, so the same organization appeared as "Imt" on one screen and as
 * a UUID on the next.
 */
export interface ResourceOwnership {
  ownerType?: string;
  ownerId?: string;
  ownerName?: string;
  /** On screen only because the viewer administers the platform. */
  adminVisible?: boolean;
}

export function ResourceOwner({
  owner,
  className,
}: {
  owner: ResourceOwnership;
  className?: string;
}) {
  const t = useT();
  const isOrganization = owner.ownerType === 'organization';
  const label = owner.ownerName || owner.ownerId;
  if (!label) return null;

  const Icon = isOrganization ? Building2 : User;
  return (
    <span className="inline-flex min-w-0 items-center gap-1.5">
      <span
        className={`inline-flex min-w-0 items-center gap-1 text-xs text-muted-foreground ${className ?? ''}`}
        title={isOrganization ? t('projects.ownedByOrganization') : t('projects.ownedByUser')}
      >
        <Icon className="size-3 shrink-0" aria-hidden />
        <span className="truncate">{label}</span>
      </span>
      {/* Seeing across every project and organization is real power over
          regulated data. A screen that shows it without saying so invites
          people to forget they are using it. */}
      {owner.adminVisible ? (
        <Badge tone="outline" title={t('admin.visibleAsAdminHint')}>
          <ShieldCheck className="size-3" aria-hidden />
          {t('admin.visibleAsAdmin')}
        </Badge>
      ) : null}
    </span>
  );
}

/** Les trois formes de propriete que le catalogue connait.
 *
 *  Une equipe possede depuis le 2026-10-01 : c'est ainsi qu'un groupe qui
 *  travaille ensemble garde ce qu'il produit quand l'un d'eux s'en va. */
type ProprietairePossible = 'user' | 'team' | 'organization';

function normaliserType(valeur: string | undefined): ProprietairePossible {
  return valeur === 'organization' || valeur === 'team' ? valeur : 'user';
}

/**
 * Handing something over.
 *
 * The API accepted a new owner for datasets and ontologies from the first day;
 * no screen ever asked for one, so in practice a dataset belonged to whoever
 * happened to create it, for good. The form is the same one projects use,
 * because a person transferring a dataset and a person transferring a project
 * are doing the same thing and should not have to learn it twice.
 */
export function OwnerTransfer({
  owner,
  onTransfer,
  pending,
  label,
}: {
  owner: ResourceOwnership;
  onTransfer: (input: { ownerType: string; ownerId: string }) => void;
  pending?: boolean;
  label: string;
}) {
  const t = useT();
  const organizations = useOrganizations();
  const users = useAdminUsers();
  // Celles dont on est membre, et elles seules : le serveur refuse un
  // transfert vers un groupe auquel on n'appartient pas, donc en proposer un
  // serait proposer un refus.
  const teams = useMyTeams();
  const [ownerType, setOwnerType] = React.useState<ProprietairePossible>(
    normaliserType(owner.ownerType),
  );
  const [ownerId, setOwnerId] = React.useState(owner.ownerId ?? '');

  // Follow the selected resource rather than keeping the previous one's owner,
  // which would offer to transfer the wrong thing to the right person.
  React.useEffect(() => {
    setOwnerType(normaliserType(owner.ownerType));
    setOwnerId(owner.ownerId ?? '');
  }, [owner.ownerType, owner.ownerId]);

  const options =
    ownerType === 'organization'
      ? (organizations.data ?? []).map((organization) => ({
          value: organization.alias ?? organization.id,
          label: organization.name,
        }))
      : ownerType === 'team'
        ? (teams.data ?? []).map((team) => ({
            value: team.id,
            // L'organisation en indication : deux equipes peuvent porter le
            // meme nom, et le serveur refuse justement un nom ambigu.
            label: team.name,
            hint: team.organizationName ?? undefined,
          }))
        : (users.data ?? []).map((user) => ({
            value: user.username ?? user.id,
            label: user.username ?? user.id,
            hint: user.email ?? undefined,
          }));

  const unchanged = ownerType === (owner.ownerType ?? 'user') && ownerId === (owner.ownerId ?? '');

  return (
    <div className="grid gap-2 sm:grid-cols-[1fr_1fr_auto] sm:items-end">
      <Field label={t('projects.ownerTypeLabel')}>
        <Select
          value={ownerType}
          onValueChange={(value) => {
            setOwnerType(normaliserType(value));
            setOwnerId('');
          }}
          options={[
            { value: 'user', label: t('common.user') },
            { value: 'team', label: t('common.team') },
            { value: 'organization', label: t('common.organization') },
          ]}
        />
      </Field>
      <Field label={t('projects.ownerLabel')}>
        <Select
          value={ownerId}
          onValueChange={setOwnerId}
          options={options}
          placeholder={t('common.search')}
        />
      </Field>
      <Button
        variant="secondary"
        loading={pending}
        disabled={!ownerId || unchanged}
        onClick={() => onTransfer({ ownerType, ownerId })}
      >
        {label}
      </Button>
    </div>
  );
}
