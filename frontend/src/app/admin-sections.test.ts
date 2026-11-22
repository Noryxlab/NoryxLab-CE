import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * Tout ce que le menu propose doit exister comme section.
 *
 * `/admin/usage` etait dans la barre laterale et absent de la liste des
 * sections, donc chaque clic rendait la vue d'ensemble : un lien qui menait
 * ailleurs sans rien dire. Ni le typage ni le build ne pouvaient le voir -
 * l'un verifie la clef de traduction, l'autre compile du JSX valide, et la
 * correspondance entre les deux listes n'est ecrite nulle part.
 *
 * Lu comme du texte plutot que monte : ce qu'il faut verifier est que les
 * deux declarations s'accordent, pas que React sache afficher un onglet.
 */
describe('les sections d’administration', () => {
  const src = resolve(new URL('.', import.meta.url).pathname, '..');
  const page = readFileSync(resolve(src, 'routes/admin.tsx'), 'utf8');
  const sidebar = readFileSync(resolve(src, 'components/layout/sidebar.tsx'), 'utf8');

  const declared = new Set(
    [...page.slice(page.indexOf('const SECTIONS'), page.indexOf('] as const')).matchAll(/'([a-z-]+)'/g)].map(
      (match) => match[1],
    ),
  );
  const linked = [...sidebar.matchAll(/to: '\/admin\/([a-z-]+)'/g)].map((match) => match[1] ?? '');

  it('en declare assez pour que ce test verifie quelque chose', () => {
    expect(declared.size).toBeGreaterThan(8);
    expect(linked.length).toBeGreaterThan(8);
  });

  it('declare chaque section que le menu propose', () => {
    for (const section of linked) {
      expect(declared.has(section), `/admin/${section} is linked but not a declared section`).toBe(true);
    }
  });

  it('rend un contenu pour chaque section declaree', () => {
    for (const section of declared) {
      // La vue d'ensemble est la valeur de repli, les autres doivent avoir
      // leur propre bloc.
      if (section === 'overview') continue;
      expect(page, `no <TabsContent value="${section}">`).toContain(`value="${section}"`);
    }
  });
});
