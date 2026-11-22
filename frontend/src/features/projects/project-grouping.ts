import type { Project } from '@/lib/api/types';

/**
 * Ranger les projets d'un administrateur.
 *
 * « la vue pour les admins de tous les projets est horrible » : un
 * administrateur voit tout, et la page rendait tout dans une grille plate -
 * les trois projets dans lesquels il travaille melanges aux vingt qu'il ne
 * fait qu'administrer, sans rien pour les distinguer. La donnee etait
 * pourtant la : `adminVisible` marque exactement les lignes qui sont a
 * l'ecran parce qu'on administre la plateforme et non parce qu'on y a un
 * droit.
 *
 * Cette coupe-la n'est pas un reglage. Elle ne change pas l'ordre, elle
 * change ce que la liste veut dire : au-dessus, son travail ; en dessous, un
 * inventaire.
 */
export interface ProjectSplit {
  mine: Project[];
  administered: Project[];
}

export function splitProjects(projects: Project[]): ProjectSplit {
  return {
    mine: projects.filter((project) => !project.adminVisible),
    administered: projects.filter((project) => project.adminVisible),
  };
}

export interface OwnerGroup {
  key: string;
  /** Deja resolu par le serveur, qui seul peut le faire : un membre qui n'est
   *  pas administrateur ne peut pas lister les organisations pour traduire un
   *  identifiant. Vide quand le projet n'a pas de proprietaire. */
  label: string;
  projects: Project[];
}

/** Groupe par proprietaire, le plus charge d'abord.
 *
 *  Par taille et non par ordre alphabetique : sur une plateforme partagee
 *  entre un industriel, une ecole et un institut, la question est « qui a
 *  quoi », et celui qui a vingt projets est celui qu'on vient regarder. */
export function groupByOwner(projects: Project[]): OwnerGroup[] {
  const buckets = new Map<string, OwnerGroup>();
  for (const project of projects) {
    const key = project.ownerId || project.ownerName || '';
    const held = buckets.get(key);
    if (held) held.projects.push(project);
    else buckets.set(key, { key, label: project.ownerName || project.ownerId || '', projects: [project] });
  }
  return [...buckets.values()].sort(
    (a, b) => b.projects.length - a.projects.length || a.label.localeCompare(b.label),
  );
}
