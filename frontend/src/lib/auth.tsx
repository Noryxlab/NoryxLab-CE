import * as React from 'react';
import Keycloak from 'keycloak-js';
import { config } from './config';
import { configureAuth } from './api/client';

export interface Identity {
  subject: string;
  username: string;
  displayName: string;
  email: string | null;
  roles: string[];
  organizations: string[];
}

export type AuthStatus = 'initialising' | 'authenticated' | 'anonymous' | 'failed';

interface AuthContextValue {
  status: AuthStatus;
  identity: Identity | null;
  error: unknown;
  isAdmin: boolean;
  login: () => void;
  logout: () => void;
  accountUrl: string | null;
}

const AuthContext = React.createContext<AuthContextValue | null>(null);

export function useAuth(): AuthContextValue {
  const ctx = React.useContext(AuthContext);
  if (!ctx) throw new Error('useAuth must be used inside <AuthProvider>');
  return ctx;
}

interface TokenClaims {
  sub?: string;
  preferred_username?: string;
  name?: string;
  given_name?: string;
  family_name?: string;
  email?: string;
  realm_access?: { roles?: string[] };
  resource_access?: Record<string, { roles?: string[] }>;
  organizations?: unknown;
  organization?: unknown;
  groups?: unknown;
}

function readOrganizations(claims: TokenClaims): string[] {
  // Keycloak organisations surface differently depending on how the realm is
  // configured (an array, an object keyed by alias, or group paths), so all
  // three shapes are accepted rather than assuming one.
  const candidates = [claims.organizations, claims.organization, claims.groups];
  const found = new Set<string>();
  for (const candidate of candidates) {
    if (Array.isArray(candidate)) {
      for (const entry of candidate) {
        if (typeof entry === 'string') found.add(entry.replace(/^\//, ''));
        else if (entry && typeof entry === 'object' && 'alias' in entry) {
          const alias = (entry as { alias?: unknown }).alias;
          if (typeof alias === 'string') found.add(alias);
        }
      }
    } else if (candidate && typeof candidate === 'object') {
      for (const key of Object.keys(candidate as Record<string, unknown>)) found.add(key);
    }
  }
  return [...found];
}

function toIdentity(claims: TokenClaims, clientId: string): Identity {
  const username = claims.preferred_username ?? claims.sub ?? 'inconnu';
  const displayName =
    claims.name ??
    [claims.given_name, claims.family_name].filter(Boolean).join(' ').trim() ??
    username;
  const roles = new Set<string>([
    ...(claims.realm_access?.roles ?? []),
    ...(claims.resource_access?.[clientId]?.roles ?? []),
  ]);
  return {
    subject: claims.sub ?? username,
    username,
    displayName: displayName || username,
    email: claims.email ?? null,
    roles: [...roles],
    organizations: readOrganizations(claims),
  };
}

const ADMIN_ROLES = ['admin', 'noryx-admin', 'platform-admin', 'realm-admin'];
const AUTH_SCOPE = 'openid profile email';

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [status, setStatus] = React.useState<AuthStatus>('initialising');
  const [identity, setIdentity] = React.useState<Identity | null>(null);
  const [error, setError] = React.useState<unknown>(null);
  const keycloakRef = React.useRef<Keycloak | null>(null);

  React.useEffect(() => {
    // Header mode is the local development path: the API trusts X-Noryx-User
    // and no identity provider is involved.
    if (config.authMode === 'header') {
      const devUser = new URLSearchParams(window.location.search).get('user') ?? 'noryx';
      configureAuth({ devUser });
      setIdentity({
        subject: devUser,
        username: devUser,
        displayName: devUser,
        email: null,
        roles: ['admin'],
        organizations: [],
      });
      setStatus('authenticated');
      return;
    }

    if (!config.oidc) {
      setError(new Error("Aucun fournisseur d'identite n'est configure pour cette instance."));
      setStatus('failed');
      return;
    }

    const keycloak = new Keycloak({
      url: config.oidc.url,
      realm: config.oidc.realm,
      clientId: config.oidc.clientId,
    });
    keycloakRef.current = keycloak;
    // Do not let keycloak-js reuse a deep or stateful current URL as the
    // callback URL. Some browsers can grow it past Keycloak's URI limit.
    const redirectUri = `${window.location.origin}/`;
    configureAuth({
      getToken: async () => {
        // A request can happen while React is still mounting providers. Never
        // ask keycloak-js to refresh a token before the initial OIDC callback
        // has been processed: it can restart the authorization flow forever.
        if (!keycloak.authenticated || !keycloak.token) return null;
        try {
          // Refresh when the token has under 30s left, so a long-running page
          // never sends an expired bearer.
          await keycloak.updateToken(30);
        } catch {
          return null;
        }
        return keycloak.token ?? null;
      },
      onUnauthorized: () => {
        // Providers such as theme preferences can make a harmless request
        // before OIDC has finished initialising. A global 401 must never
        // alter the authentication state: callers receive the ApiError and
        // decide how to render it, while the OIDC flow stays authoritative.
      },
    });

    keycloak
      .init({
        pkceMethod: 'S256',
        checkLoginIframe: false,
        redirectUri,
      })
      .then((authenticated) => {
        if (authenticated && keycloak.tokenParsed) {
          setIdentity(toIdentity(keycloak.tokenParsed as TokenClaims, config.oidc?.clientId ?? ''));
          setStatus('authenticated');
        } else {
          setStatus('anonymous');
        }
      })
      .catch((cause: unknown) => {
        setError(cause);
        setStatus('failed');
      });
  }, []);

  const value = React.useMemo<AuthContextValue>(
    () => ({
      status,
      identity,
      error,
      isAdmin: Boolean(identity?.roles.some((role) => ADMIN_ROLES.includes(role.toLowerCase()))),
      login: () => {
        const keycloak = keycloakRef.current;
        if (!keycloak) {
          setError(new Error("Le service d'authentification n'est pas encore initialise."));
          setStatus('failed');
          return;
        }
        try {
          // Navigate explicitly rather than delegating to the adapter. This
          // keeps the interaction reliable in Safari and guarantees that a
          // user click always leaves the anonymous screen.
          void keycloak
            .createLoginUrl({
              redirectUri: `${window.location.origin}/`,
              scope: AUTH_SCOPE,
            })
            .then((url) => window.location.assign(url))
            .catch((cause: unknown) => {
              setError(cause);
              setStatus('failed');
            });
        } catch (cause) {
          setError(cause);
          setStatus('failed');
        }
      },
      logout: () => {
        void keycloakRef.current?.logout({ redirectUri: window.location.origin });
      },
      accountUrl: keycloakRef.current?.createAccountUrl() ?? null,
    }),
    [status, identity, error],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
