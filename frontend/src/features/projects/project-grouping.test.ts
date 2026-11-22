import { describe, expect, it } from 'vitest';
import { groupByOwner, splitProjects } from './project-grouping';
import type { Project } from '@/lib/api/types';

const project = (name: string, extra: Partial<Project> = {}): Project =>
  ({
    id: name,
    name,
    description: '',
    ownerType: 'organization',
    ownerId: 'org-1',
    createdAt: '2026-11-01T00:00:00Z',
    updatedAt: '2026-11-01T00:00:00Z',
    lastActivityAt: '2026-11-01T00:00:00Z',
    runningApps: 0,
    runningJobs: 0,
    runningWorkspaces: 0,
    ...extra,
  }) as Project;

describe('splitProjects', () => {
  /* La coupe qui compte.
   *
   *  Un administrateur voit tout. Melanger les projets ou il travaille avec
   *  ceux qu'il ne fait qu'administrer est ce qui rendait la page illisible :
   *  les deux listes repondent a deux questions differentes. */
  it('separates the projects held from the ones only administered', () => {
    const split = splitProjects([
      project('le mien'),
      project('celui des autres', { adminVisible: true }),
    ]);
    expect(split.mine.map((item) => item.name)).toEqual(['le mien']);
    expect(split.administered.map((item) => item.name)).toEqual(['celui des autres']);
  });

  // Pour qui n'administre rien, la deuxieme liste est vide et la page doit
  // alors ressembler exactement a ce qu'elle etait.
  it('leaves the administered list empty for an ordinary member', () => {
    expect(splitProjects([project('a'), project('b')]).administered).toEqual([]);
  });
});

describe('groupByOwner', () => {
  it('puts the busiest owner first', () => {
    const groups = groupByOwner([
      project('a', { ownerId: 'petit', ownerName: 'Petit' }),
      project('b', { ownerId: 'gros', ownerName: 'Gros' }),
      project('c', { ownerId: 'gros', ownerName: 'Gros' }),
    ]);
    expect(groups.map((group) => group.label)).toEqual(['Gros', 'Petit']);
  });

  // Le nom resolu par le serveur, jamais l'identifiant, quand les deux sont
  // la : une colonne d'UUID ne dit pas a qui appartient quoi.
  it('labels a group with the resolved name', () => {
    const groups = groupByOwner([project('a', { ownerId: 'uuid-1', ownerName: 'EMSE' })]);
    expect(groups[0]?.label).toBe('EMSE');
  });

  it('keeps a project with no owner in a group of its own', () => {
    const groups = groupByOwner([project('orphelin', { ownerId: '', ownerName: '' })]);
    expect(groups).toHaveLength(1);
    expect(groups[0]?.label).toBe('');
  });
});
