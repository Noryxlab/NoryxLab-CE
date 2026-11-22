import { describe, expect, it } from 'vitest';
import { groupByShape, shapeOf } from './environment-shape';
import type { Environment } from '@/lib/api/types';

const environment = (name: string, shape?: string): Environment =>
  ({
    id: name,
    projectId: '',
    name,
    category: 'system',
    ...(shape ? { shape } : {}),
    workspaceIdes: null,
    destinationImage: `harbor.lan/noryx/${name}:1`,
    latestBuildId: '',
    latestStatus: 'succeeded',
    latestGitRepository: '',
    latestGitRef: '',
    latestDockerfilePath: '',
    updatedAt: '2026-11-22T00:00:00Z',
    revisions: null,
  }) as Environment;

describe('environment shapes', () => {
  it('keeps the declared order rather than an alphabetical one', () => {
    const groups = groupByShape([
      environment('noryx-runtime', 'runtime'),
      environment('noryx-slicer', 'desktop'),
      environment('noryx-jupyter', 'notebook'),
    ]);
    expect(groups.map((group) => group.shape)).toEqual(['notebook', 'desktop', 'runtime']);
  });

  it('renders no heading for a shape nothing has', () => {
    const groups = groupByShape([environment('noryx-jupyter', 'notebook')]);
    expect(groups).toHaveLength(1);
  });

  /* Une forme inconnue est rangee, pas perdue.
   *
   *  Le serveur peut en declarer une que cette version de l'interface ne
   *  connait pas - c'est ce qui arrive entre deux deploiements. La ranger
   *  avec les images sans rien a ouvrir la laisse visible et cliquable ; la
   *  filtrer la ferait disparaitre, ce qui est exactement le defaut que ce
   *  groupement repare. */
  it('files an unknown shape with the runtimes instead of dropping it', () => {
    expect(shapeOf(environment('venu-du-futur', 'holodeck'))).toBe('runtime');
    const groups = groupByShape([environment('venu-du-futur', 'holodeck')]);
    expect(groups[0]?.items).toHaveLength(1);
  });

  it('files an environment that declares nothing with the runtimes', () => {
    expect(shapeOf(environment('ancien'))).toBe('runtime');
  });
});
