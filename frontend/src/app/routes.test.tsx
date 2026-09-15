import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';

/**
 * Les routes, lues comme un texte.
 *
 * Un remplacement automatique a interverti deux lignes : la redirection s'est
 * retrouvee dans le bloc du projet et la page est restee au premier niveau,
 * si bien que cliquer sur Agents dans un projet renvoyait vers la liste des
 * projets. Rien ne l'a vu - ni le typage, ni le linter, ni le build, parce
 * qu'une route mal placee est du JSX parfaitement valide.
 */
describe('routes', () => {
  const source = readFileSync(new URL('./routes.tsx', import.meta.url), 'utf8');
  const projectBlock = source.slice(
    source.indexOf('path="projects/:projectId"'),
    source.indexOf('<Route path="account"'),
  );

  it('sert la page des agents a l’interieur du projet', () => {
    expect(projectBlock).toContain('<Route path="agents" element={<AgentsPage />} />');
  });

  it('ne redirige pas les agents depuis l’interieur du projet', () => {
    const insideProject = projectBlock.slice(0, projectBlock.lastIndexOf('</Route>'));
    expect(insideProject).not.toContain('path="agents" element={<Navigate');
  });

  it('renvoie l’ancien lien de premier niveau vers les projets', () => {
    const topLevel = source.slice(source.indexOf('</Route>', source.indexOf('path="settings"')));
    expect(topLevel).toContain('<Route path="agents" element={<Navigate to="/projects" replace />} />');
  });
});
