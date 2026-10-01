import { useMutation } from '@tanstack/react-query';
import { Check, FolderOpen } from 'lucide-react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Sheet, SheetBody, SheetContent } from '@/components/ui/sheet';
import { useToast } from '@/components/ui/toast';
import { projectsApi } from '@/lib/api/endpoints';
import { qk, useInvalidate, useProjects, useResourceMounts } from '@/lib/api/queries';
import { useT } from '@/lib/i18n';

/**
 * Ou un objet du catalogue est monte, et comment l'y mettre.
 *
 * Le rattachement n'avait qu'un sens a l'ecran : un projet listait ce qu'il
 * montait, et l'objet ne disait rien de la ou il etait monte ni comment l'y
 * mettre. Quelqu'un qui regardait un extrait et voulait travailler avec devait
 * savoir qu'il faut passer par le projet - et pour un extrait c'est
 * exactement a l'envers, puisqu'il existe pour etre repris par plusieurs
 * projets. C'est tout l'interet du lien plutot que de l'appartenance.
 *
 * Un seul composant pour les trois objets qui se rattachent de la meme facon :
 * un dataset, une ontologie, un extrait. Trois copies d'un ecran de
 * rattachement seraient trois occasions d'en ecrire une legerement de travers.
 */
export type MountKind = 'dataset' | 'ontology' | 'extract';

const attach = (kind: MountKind, projectId: string, resourceId: string) => {
  if (kind === 'dataset') return projectsApi.attachDataset(projectId, resourceId);
  if (kind === 'ontology') return projectsApi.attachOntology(projectId, resourceId);
  return projectsApi.attachExtract(projectId, resourceId);
};

const detach = (kind: MountKind, projectId: string, resourceId: string) => {
  if (kind === 'dataset') return projectsApi.detachDataset(projectId, resourceId);
  if (kind === 'ontology') return projectsApi.detachOntology(projectId, resourceId);
  return projectsApi.detachExtract(projectId, resourceId);
};

/** Les projets, pour la colonne d'une table. Les noms, pas les identifiants,
 *  et le compte de ceux que la personne n'a pas a connaitre. */
export function MountedProjects({ kind, resourceId }: { kind: MountKind; resourceId: string }) {
  const t = useT();
  const mounts = useResourceMounts(kind, resourceId);
  const items = mounts.data?.items ?? [];
  const hidden = mounts.data?.hidden ?? 0;

  if (mounts.isLoading) {
    return <span className="text-xs text-muted-foreground">…</span>;
  }
  if (items.length === 0 && hidden === 0) {
    return <span className="text-xs text-muted-foreground">{t('mounts.none')}</span>;
  }
  return (
    <div className="flex flex-wrap items-center gap-1">
      {items.map((item) => (
        <Badge key={item.id} tone="outline">
          {item.name}
        </Badge>
      ))}
      {hidden > 0 ? (
        <span className="text-xs text-muted-foreground">{t('mounts.hidden', { count: hidden })}</span>
      ) : null}
    </div>
  );
}

/** Le rattachement lui-meme : les projets de la personne, et un bouton par
 *  projet qui l'y attache ou l'en detache. */
export function ProjectMountsSheet({
  kind,
  resourceId,
  resourceName,
  open,
  onOpenChange,
}: {
  kind: MountKind;
  resourceId: string | null;
  resourceName?: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  const t = useT();
  const toast = useToast();
  const invalidate = useInvalidate();
  const projects = useProjects({ enabled: open });
  const mounts = useResourceMounts(kind, open && resourceId ? resourceId : undefined);
  const attached = new Set((mounts.data?.items ?? []).map((item) => item.id));

  const change = useMutation({
    mutationFn: async (input: { projectId: string; mount: boolean }) => {
      if (!resourceId) return;
      if (input.mount) {
        await attach(kind, input.projectId, resourceId);
      } else {
        await detach(kind, input.projectId, resourceId);
      }
    },
    onSuccess: (_result, input) => {
      if (resourceId) invalidate(qk.resourceMounts(kind, resourceId));
      // L'ecran du projet lit sa propre liste : sans ca, rattacher ici laisse
      // le projet afficher l'etat d'avant jusqu'a un rechargement.
      invalidate(qk.projectDatasets(input.projectId));
      invalidate(qk.projectOntologies(input.projectId));
      invalidate(qk.projectExtracts(input.projectId));
      toast.success(t(input.mount ? 'mounts.attached' : 'mounts.detached'));
    },
    onError: (error) => toast.error(error, t('mounts.title')),
  });

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent title={resourceName ?? t('mounts.title')}>
        <SheetBody>
          <p className="mb-4 text-xs text-muted-foreground">{t('mounts.hint')}</p>
          {(projects.data ?? []).length === 0 ? (
            <p className="text-sm text-muted-foreground">{t('mounts.noProject')}</p>
          ) : (
            <div className="flex flex-col gap-1.5">
              {(projects.data ?? []).map((project) => {
                const mounted = attached.has(project.id);
                return (
                  <div
                    key={project.id}
                    className="flex items-center justify-between gap-3 rounded-md border border-border px-3 py-2"
                  >
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{project.name}</p>
                      {project.description ? (
                        <p className="truncate text-xs text-muted-foreground">
                          {project.description}
                        </p>
                      ) : null}
                    </div>
                    <Button
                      size="sm"
                      variant={mounted ? 'secondary' : 'primary'}
                      loading={change.isPending && change.variables?.projectId === project.id}
                      onClick={() => change.mutate({ projectId: project.id, mount: !mounted })}
                    >
                      {mounted ? <Check aria-hidden /> : <FolderOpen aria-hidden />}
                      {mounted ? t('mounts.detach') : t('mounts.attach')}
                    </Button>
                  </div>
                );
              })}
            </div>
          )}
          {(mounts.data?.hidden ?? 0) > 0 ? (
            <p className="mt-4 text-xs text-muted-foreground">
              {t('mounts.hiddenHint', { count: mounts.data?.hidden ?? 0 })}
            </p>
          ) : null}
        </SheetBody>
      </SheetContent>
    </Sheet>
  );
}
