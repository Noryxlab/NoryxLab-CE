import { describe, expect, it } from 'vitest';
import { filterAccessCells, noAccessFilters } from './access-graph';
import type { RbacCell } from '@/lib/api/types';

/**
 * Les criteres s'appliquaient au graphe et pas au tableau en dessous : on
 * centrait sur un dataset, le dessin obeissait, et les lignes continuaient
 * d'afficher les autres. Deux consommateurs et un seul filtre finissent
 * toujours par se contredire, donc la regle est une fonction, partagee.
 */
const cell = (over: Partial<RbacCell>): RbacCell => ({
  subjectType: 'user',
  subjectId: 'cridelic',
  subjectName: 'cridelic',
  resourceType: 'dataset',
  resourceId: 'mri',
  resourceName: 'hds-essilor.mripreliminarydata',
  role: 'owner',
  source: 'owner',
  inherited: false,
  ...over,
});

describe('filterAccessCells', () => {
  const cells = [
    cell({}),
    cell({ resourceId: 'hds-essilor', resourceName: 'HDS-Essilor' }),
    cell({ subjectId: 'folignoc', subjectName: 'folignoc', resourceId: 'hds-for' }),
    cell({ resourceId: 'projet', resourceType: 'project', inherited: true }),
  ];

  it('ne renvoie que la ressource choisie', () => {
    const kept = filterAccessCells(cells, { ...noAccessFilters, focus: { kind: 'resource', id: 'mri' } });
    expect(kept).toHaveLength(1);
    expect(kept[0]?.resourceId).toBe('mri');
  });

  it('ne renvoie que le sujet choisi', () => {
    const kept = filterAccessCells(cells, {
      ...noAccessFilters,
      focus: { kind: 'subject', id: 'folignoc' },
    });
    expect(kept.every((item) => item.subjectId === 'folignoc')).toBe(true);
  });

  it('ecarte les acces herites quand on les masque', () => {
    const kept = filterAccessCells(cells, { ...noAccessFilters, showInherited: false });
    expect(kept.some((item) => item.inherited)).toBe(false);
  });

  it('filtre par type de ressource', () => {
    const kept = filterAccessCells(cells, { ...noAccessFilters, resourceType: 'project' });
    expect(kept).toHaveLength(1);
    expect(kept[0]?.resourceType).toBe('project');
  });

  it('sans critere, ne retire rien', () => {
    expect(filterAccessCells(cells, noAccessFilters)).toHaveLength(cells.length);
  });
});
