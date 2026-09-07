import { Building2, User } from 'lucide-react';
import { useT } from '@/lib/i18n';
import type { Project } from '@/lib/api/types';

/**
 * Who owns a project, and of what kind.
 *
 * The distinction is not decoration: an organization-owned project outlives
 * the person who created it and grants access through membership, while a
 * user-owned one has a single point of failure with a name. Somebody scanning
 * a list of projects is entitled to see which is which without opening each
 * one.
 */
export function ProjectOwner({ project, className }: { project: Project; className?: string }) {
  const t = useT();
  const isOrganization = project.ownerType === 'organization';
  const label = project.ownerName || project.ownerId;
  if (!label) return null;

  const Icon = isOrganization ? Building2 : User;
  return (
    <span
      className={`inline-flex min-w-0 items-center gap-1 text-xs text-muted-foreground ${className ?? ''}`}
      title={isOrganization ? t('projects.ownedByOrganization') : t('projects.ownedByUser')}
    >
      <Icon className="size-3 shrink-0" aria-hidden />
      <span className="truncate">{label}</span>
    </span>
  );
}
