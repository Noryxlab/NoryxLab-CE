import { ResourceOwner } from '@/components/common/owner';
import type { Project } from '@/lib/api/types';

/**
 * Who owns a project, and of what kind.
 *
 * The rendering lives in ResourceOwner so a project, a dataset and an ontology
 * state their owner in the same notation; this stays as the project-shaped door
 * onto it.
 */
export function ProjectOwner({ project, className }: { project: Project; className?: string }) {
  return <ResourceOwner owner={project} className={className} />;
}
