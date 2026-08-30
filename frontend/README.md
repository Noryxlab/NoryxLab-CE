# Noryx frontend

React + TypeScript application built with Vite, styled with Tailwind CSS v4 and
Radix UI primitives.

## Why this exists

The previous UI was a single 504 KB `index.html` (9 419 lines of HTML, CSS and
vanilla JavaScript) committed verbatim into a Kubernetes ConfigMap. It had no
build step, no type checking, no tests, and no browser targets. On 2026-05-04 a
syntax error Safari could not parse froze the entire interface, and recovery
meant restoring a previous ConfigMap by hand
(`ops/incidents/FRONTEND_INCIDENT_2026-05-04.md`).

This rewrite addresses that class of failure structurally, and takes the
opportunity to fix the information architecture at the same time.

## Commands

```sh
npm install
npm run dev        # dev server on :5173, proxies /api to NORYX_API_URL
npm run build      # type-check then build to dist/
npm run typecheck
npm run smoke      # asserts the built output (see scripts/smoke.mjs)
```

Point the dev server at a running backend:

```sh
NORYX_API_URL=https://datalab.example.local npm run dev
```

## Layout

```
src/
  app/          providers, routing, auth gate, error boundary
  components/
    ui/         Radix-based primitives (button, field, select, dialog, …)
    common/     composed pieces (data table, empty/error states, log viewer)
    layout/     app shell and two-level sidebar
  features/     per-domain flows (workspace launch, dataset explorer, …)
  routes/       one file per screen
  lib/
    api/        typed client, endpoints, TanStack Query hooks
    i18n/       FR and EN catalogues
    config.ts   runtime configuration
    presenters.ts  backend vocabulary → product vocabulary
    extensions.ts  EE extension points
```

## Navigation model

Two levels, decided against `ADR-001` (project is the unit of ownership) and
`ADR-025` (semantic catalogue):

- **Global** — Home, Projects, Catalog, Production, Administration.
- **Project** — everything under `/projects/:projectId`: overview, workspaces,
  jobs, apps, dashboards, data, environments, members, settings.

The project is in the URL. The previous UI kept it in `localStorage` and showed
an `Aucun projet` chip on pages that needed one, so a user could land on an
empty screen with no explanation. Old tab ids redirect to their new routes.

The catalogue (datasets, data sources, ontologies, repositories, secrets) stays
global because those objects are shared across projects; a project attaches
what it needs.

## Runtime configuration

`public/config.js` assigns `window.__NORYX_CONFIG__`. In Kubernetes it is
mounted from the `noryx-frontend-config` ConfigMap, so the same image serves
every deployment:

```js
window.__NORYX_CONFIG__ = {
  apiBaseUrl: '',
  authMode: 'oidc',              // or 'header' for local development
  oidc: { url, realm, clientId },
  edition: 'ce',                 // 'ee' enables Enterprise surfaces
  defaultLocale: 'fr',
  brand: { productName, editionLabel, logoUrl, tokens },
  links: { documentation, apiReference, status },
  features: { requireOrganization: false },
  extensions: [{ id, url }],
};
```

`brand.tokens` overrides any `--noryx-*` CSS custom property, which is how a
white-labelled deployment changes colours without a rebuild.

## Extension points

Enterprise UI is loaded, not injected. An extension is an ES module served by
the platform whose default export implements the contract in
`src/lib/extensions.ts`:

```js
export default {
  id: 'platform-validation',
  mountPoint: 'admin.section',
  title: { fr: 'Validation', en: 'Validation' },
  mount(element, host) { /* host.api, host.format, host.notify */ },
  unmount(element) {},
};
```

The contract is framework-agnostic (`mount` receives a plain DOM element), so an
extension never has to match this application's React version, and a failing
extension is caught and reported without taking the page down.

This replaces the previous approach, where EE ran `sed` over CE's HTML and
spliced markup at exact source markers.

## Design decisions worth knowing

- **One brand colour**, taken from the marketing site gradient, so the site and
  the product no longer look like different companies. Light and dark palettes
  are defined as tokens in `src/styles/globals.css`.
- **Labels, never placeholders.** `Field` makes the accessible structure the
  default; `ReadOnlyValue` covers attributes the user cannot edit, replacing
  the previous `<select disabled>` pattern.
- **Presenters** (`src/lib/presenters.ts`) keep backend vocabulary off screen:
  hardware tiers are named by size rather than `1x4`, and apps are created by
  picking a framework rather than editing a `python3 -m http.server` command.
- **Polling follows readiness signals**, not fixed timers: list queries refetch
  only while something in them is still converging.
- Every screen renders a skeleton while loading, a scoped error with a retry on
  failure, and an empty state carrying the action that resolves it.

## Deployment

`Dockerfile` builds the app and serves `dist/` from nginx with SPA fallback.
`deploy/k8s/base/noryx-frontend.yaml` carries only the runtime configuration —
the application lives in the image.
