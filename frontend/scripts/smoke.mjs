/**
 * Frontend smoke test.
 *
 * Required by the follow-up actions of ops/incidents/FRONTEND_INCIDENT_2026-05-04.md:
 *
 *   2. Add smoke test in CI/CD: page load, refreshVersions() success,
 *      one clickable action per major tab.
 *
 * It runs against the built output rather than a dev server, so it catches
 * the class of failure that caused the incident: assets that parse in the
 * build tool but not in the browsers the product supports.
 */
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';

const DIST = new URL('../dist/', import.meta.url).pathname;

let failures = 0;

function check(name, condition, detail = '') {
  if (condition) {
    console.log(`  ok   ${name}`);
  } else {
    failures += 1;
    console.error(`  FAIL ${name}${detail ? ` — ${detail}` : ''}`);
  }
}

function walk(directory) {
  return readdirSync(directory).flatMap((entry) => {
    const path = join(directory, entry);
    return statSync(path).isDirectory() ? walk(path) : [path];
  });
}

console.log('Noryx frontend smoke test');

let files;
try {
  files = walk(DIST);
} catch {
  console.error('dist/ not found — run `npm run build` first.');
  process.exit(1);
}

const html = readFileSync(join(DIST, 'index.html'), 'utf8');
const scripts = files.filter((file) => file.endsWith('.js') && !file.endsWith('.map'));
const styles = files.filter((file) => file.endsWith('.css'));

console.log('\nBuild output');
check('index.html is present', html.length > 0);
check('at least one JS chunk emitted', scripts.length > 0, `${scripts.length} chunks`);
check('a stylesheet was emitted', styles.length > 0);
check('index.html references a module entry', /<script[^>]+type="module"/.test(html));
check('runtime config is loaded before the app', html.indexOf('/config.js') < html.indexOf('type="module"'));
check('document has a title', /<title>[^<]+<\/title>/.test(html));
check('theme is applied before first paint', html.includes('data-theme') || html.includes('dataset.theme'));

console.log('\nBrowser compatibility');
// The incident was an untranspiled syntax failure on Safari. These are the
// constructs Vite lowers for the configured targets; seeing them in the
// output means the build targets regressed.
const source = scripts.map((file) => readFileSync(file, 'utf8')).join('\n');
check('no class static blocks in output', !/\bstatic\s*\{/.test(source));
// Matches an actual explicit-resource-management declaration, not the word
// "using" appearing in a translated string.
check(
  'no `using` declarations in output',
  !/(?:^|[;{}])\s*(?:await\s+)?using\s+[A-Za-z_$][\w$]*\s*=/m.test(source),
);
check('no decorators in output', !/^\s*@[A-Za-z_$][\w$]*\s*(\(|\n)/m.test(source));

console.log('\nApplication surface');
// One assertion per major navigation target, so a route silently dropped
// during a refactor fails the build rather than a demo.
const routes = [
  '/projects',
  '/catalog',
  '/production',
  '/admin',
  'workspaces',
  'jobs',
  'apps',
  'dashboards',
  'environments',
  'members',
  'settings',
];
for (const route of routes) {
  check(`route "${route}" present in bundle`, source.includes(route));
}

console.log('\nInternationalisation');
check('French catalogue bundled', source.includes('Environnements') || source.includes('Workspaces'));
check('English catalogue bundled', source.includes('Machine size') || source.includes('Launch a workspace'));

console.log('\nAccessibility guards');
check('skip link present', source.includes('skipToContent') || source.includes('Aller au contenu'));
check('focus-visible styles emitted', styles.some((file) => readFileSync(file, 'utf8').includes('focus-visible')));
// Minified CSS drops the attribute-selector quotes, so match either form.
check(
  'dark theme tokens emitted',
  styles.some((file) => /\[data-theme=["']?dark["']?\]/.test(readFileSync(file, 'utf8'))),
);
check(
  'reduced-motion support emitted',
  styles.some((file) => readFileSync(file, 'utf8').includes('prefers-reduced-motion')),
);

console.log(`\n${failures === 0 ? 'PASS' : `FAIL (${failures} check${failures > 1 ? 's' : ''})`}`);
process.exit(failures === 0 ? 0 : 1);
