/**
 * Runtime configuration.
 *
 * The EE overlay used to white-label the product by running `sed` over the
 * built HTML inside a Kubernetes manifest:
 *
 *   sed -e 's/>NoryxLab</>Premyom</g' \
 *       -e "s/const productName = 'NoryxLab';/const productName = 'Premyom';/"
 *
 * That coupled the overlay to the exact source text of the application, so any
 * refactor of CE silently broke EE branding. Branding is now configuration:
 * `/config.js` assigns `window.__NORYX_CONFIG__` and is mounted from a small
 * ConfigMap. The built assets are identical between CE and EE.
 */

export type AuthMode = 'oidc' | 'header';

export interface BrandConfig {
  /** Product name shown in the sidebar, document title and empty states. */
  productName: string;
  /** Edition label under the product name. Empty string hides it. */
  editionLabel: string;
  /** Absolute or root-relative URL to the product logo (SVG recommended). */
  logoUrl: string | null;
  /** Overrides for any `--noryx-*` custom property, applied at boot. */
  tokens: Record<string, string>;
}

export interface LinksConfig {
  documentation: string | null;
  apiReference: string | null;
  status: string | null;
  support: string | null;
}

/**
 * One extension module the deployment declares.
 *
 * Declared here rather than in extensions.ts because that module imports this
 * one: the configuration has to know the shape it is carrying, or it drops it.
 * Which is exactly what happened - `extensions` was read by extensions.ts and
 * never copied by merge(), so no extension loaded at all.
 */
export interface ExtensionDescriptor {
  id: string;
  /** Root-relative URL of an ES module whose default export is the extension. */
  url: string;
}

export interface OidcConfig {
  url: string;
  realm: string;
  clientId: string;
}

export interface NoryxConfig {
  apiBaseUrl: string;
  authMode: AuthMode;
  oidc: OidcConfig | null;
  brand: BrandConfig;
  links: LinksConfig;
  /** Feature flags. Unknown keys are ignored; missing keys default to false. */
  features: Record<string, boolean>;
  /** Edition drives which EE-only surfaces are mounted at all. */
  edition: 'ce' | 'ee';
  defaultLocale: 'fr' | 'en';
  /** Extension modules this deployment loads at boot. */
  extensions: ExtensionDescriptor[];
}

declare global {
  interface Window {
    __NORYX_CONFIG__?: DeepPartial<NoryxConfig>;
  }
}

type DeepPartial<T> = {
  [K in keyof T]?: T[K] extends Record<string, unknown> | null ? DeepPartial<NonNullable<T[K]>> | null : T[K];
};

const DEFAULTS: NoryxConfig = {
  apiBaseUrl: '',
  authMode: 'oidc',
  oidc: null,
  brand: {
    productName: 'NoryxLab',
    editionLabel: 'Community Edition',
    logoUrl: '/favicon.svg',
    tokens: {},
  },
  links: {
    documentation: 'https://docs.noryxlab.ai',
    apiReference: '/swagger',
    status: 'https://status.noryxlab.ai',
    support: null,
  },
  features: {},
  edition: 'ce',
  defaultLocale: 'fr',
  extensions: [],
};

function merge(base: NoryxConfig, override: DeepPartial<NoryxConfig> | undefined): NoryxConfig {
  if (!override) return base;
  return {
    ...base,
    ...pick(override, ['apiBaseUrl', 'authMode', 'edition', 'defaultLocale']),
    oidc: override.oidc ? ({ ...base.oidc, ...override.oidc } as OidcConfig) : base.oidc,
    brand: { ...base.brand, ...override.brand, tokens: { ...base.brand.tokens, ...override.brand?.tokens } },
    links: { ...base.links, ...override.links },
    features: mergeFeatures(base.features, override.features),
    extensions: mergeExtensions(base.extensions, override.extensions),
  };
}

/**
 * Keeps only well-formed, same-origin descriptors.
 *
 * The URL check is here rather than at load time so a malformed entry is
 * dropped once, visibly, instead of failing later inside a dynamic import
 * where the only trace is a console error nobody is watching. An extension
 * runs with the user's session, so it must be served by the platform itself.
 */
function mergeExtensions(base: ExtensionDescriptor[], override: unknown): ExtensionDescriptor[] {
  if (!Array.isArray(override)) return base;
  const out: ExtensionDescriptor[] = [];
  for (const entry of override) {
    if (!entry || typeof entry !== 'object') continue;
    const { id, url } = entry as Record<string, unknown>;
    if (typeof id !== 'string' || typeof url !== 'string') continue;
    // Same-origin only, and "//host/x.js" is not same-origin: a
    // protocol-relative URL starts with a slash and resolves to another
    // domain. The extension runs with the user's session, so this is the
    // difference between a plugin and an account takeover.
    if (!url.startsWith('/') || url.startsWith('//')) continue;
    out.push({ id, url });
  }
  return out;
}

function mergeFeatures(
  base: Record<string, boolean>,
  override: Record<string, boolean | undefined> | null | undefined,
): Record<string, boolean> {
  const out: Record<string, boolean> = { ...base };
  for (const [name, value] of Object.entries(override ?? {})) {
    if (typeof value === 'boolean') out[name] = value;
  }
  return out;
}

function pick<T extends object, K extends keyof T>(source: DeepPartial<T>, keys: K[]): Partial<T> {
  const out: Partial<T> = {};
  for (const key of keys) {
    const value = source[key as keyof DeepPartial<T>];
    if (value !== undefined && value !== null) out[key] = value as T[K];
  }
  return out;
}

export const config: NoryxConfig = merge(
  DEFAULTS,
  typeof window === 'undefined' ? undefined : window.__NORYX_CONFIG__,
);

/**
 * Applies brand token overrides to the document root. Only `--noryx-*`
 * properties are accepted, so a malformed config cannot inject arbitrary CSS.
 */
export function applyBrandOverrides(brand: BrandConfig = config.brand): void {
  const root = document.documentElement;
  for (const [name, value] of Object.entries(brand.tokens)) {
    if (!/^--noryx-[a-z0-9-]+$/.test(name)) continue;
    if (!/^[#a-zA-Z0-9\s(),.%/-]+$/.test(value)) continue;
    root.style.setProperty(name, value);
  }
  document.title = brand.productName;
}

export function isFeatureEnabled(name: string): boolean {
  return config.features[name] === true;
}

export function isEnterprise(): boolean {
  return config.edition === 'ee';
}
