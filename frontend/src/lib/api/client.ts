import { config } from '@/lib/config';
import type { ListResponse } from './types';

/**
 * HTTP client.
 *
 * The previous UI called `fetch` inline at ~150 call sites and surfaced
 * failures as `JSON.stringify(error)` in a toast, so a 403 and a network
 * outage looked identical. `ApiError` carries the status so screens can
 * distinguish "you may not do this" from "the platform is unreachable",
 * which is the difference between a user calling support and a user
 * understanding what happened.
 */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string | undefined;
  readonly details: unknown;

  constructor(message: string, status: number, code?: string, details?: unknown) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.details = details;
  }

  get isUnauthorized(): boolean {
    return this.status === 401;
  }

  get isForbidden(): boolean {
    return this.status === 403;
  }

  get isNotFound(): boolean {
    return this.status === 404;
  }

  get isConflict(): boolean {
    return this.status === 409;
  }

  /** Network failure, timeout, or the API being unreachable. */
  get isOffline(): boolean {
    return this.status === 0;
  }

  get isServerError(): boolean {
    return this.status >= 500;
  }
}

type TokenProvider = () => Promise<string | null> | string | null;

let tokenProvider: TokenProvider = () => null;
let devUser = 'noryx';
let onUnauthorized: (() => void) | null = null;

/** Wired by the auth provider once Keycloak has initialised. */
export function configureAuth(options: {
  getToken?: TokenProvider;
  devUser?: string;
  onUnauthorized?: () => void;
}): void {
  if (options.getToken) tokenProvider = options.getToken;
  if (options.devUser) devUser = options.devUser;
  if (options.onUnauthorized) onUnauthorized = options.onUnauthorized;
}

/** The headers the fetch client would send. Exposed for the XHR upload path,
 *  which needs progress events that fetch cannot provide. */
export async function getAuthHeaders(): Promise<Record<string, string>> {
  return authHeaders();
}

async function authHeaders(): Promise<Record<string, string>> {
  if (config.authMode === 'oidc') {
    const token = await tokenProvider();
    return token ? { Authorization: `Bearer ${token}` } : {};
  }
  return { 'X-Noryx-User': devUser };
}

export interface RequestOptions extends Omit<RequestInit, 'body'> {
  body?: unknown;
  /** Query string parameters; undefined and null values are dropped. */
  params?: Record<string, string | number | boolean | undefined | null>;
  /** Skips JSON parsing and returns the raw Response. */
  raw?: boolean;
}

function buildUrl(path: string, params?: RequestOptions['params']): string {
  const base = config.apiBaseUrl.replace(/\/$/, '');
  const url = `${base}${path}`;
  if (!params) return url;
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === '') continue;
    search.append(key, String(value));
  }
  const query = search.toString();
  return query ? `${url}${url.includes('?') ? '&' : '?'}${query}` : url;
}

async function parseError(response: Response): Promise<ApiError> {
  let message = response.statusText || `Erreur HTTP ${response.status}`;
  let code: string | undefined;
  let details: unknown;
  try {
    const text = await response.text();
    if (text) {
      try {
        const payload = JSON.parse(text) as Record<string, unknown>;
        details = payload;
        for (const key of ['error', 'message', 'detail']) {
          const value = payload[key];
          if (typeof value === 'string' && value.trim()) {
            message = value;
            break;
          }
        }
        if (typeof payload['code'] === 'string') code = payload['code'];
      } catch {
        message = text.slice(0, 500);
      }
    }
  } catch {
    /* body already consumed or unreadable; keep the status line */
  }
  return new ApiError(message, response.status, code, details);
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { body, params, raw, headers, ...rest } = options;
  const auth = await authHeaders();

  const isFormData = body instanceof FormData;
  const isBlob = body instanceof Blob;
  const sendsJson = body !== undefined && !isFormData && !isBlob;

  let response: Response;
  try {
    response = await fetch(buildUrl(path, params), {
      ...rest,
      headers: {
        Accept: 'application/json',
        ...(sendsJson ? { 'Content-Type': 'application/json' } : {}),
        ...auth,
        ...headers,
      },
      body: sendsJson ? JSON.stringify(body) : (body as BodyInit | undefined),
    });
  } catch (cause) {
    throw new ApiError(
      "La plateforme est injoignable. Vérifiez votre connexion réseau puis réessayez.",
      0,
      'network_unreachable',
      cause,
    );
  }

  if (response.status === 401) {
    onUnauthorized?.();
  }

  if (!response.ok) {
    throw await parseError(response);
  }

  if (raw) return response as unknown as T;
  if (response.status === 204) return undefined as T;

  const text = await response.text();
  if (!text) return undefined as T;
  try {
    return JSON.parse(text) as T;
  } catch {
    return text as unknown as T;
  }
}

/** Unwraps the `{ items: [...] }` envelope the API uses for collections,
 *  normalising a null `items` to an empty array. */
export async function requestList<T>(path: string, options: RequestOptions = {}): Promise<T[]> {
  const payload = await request<ListResponse<T> | T[] | null>(path, options);
  if (Array.isArray(payload)) return payload;
  return payload?.items ?? [];
}

export const api = {
  get: <T>(path: string, options?: RequestOptions) => request<T>(path, { ...options, method: 'GET' }),
  list: <T>(path: string, options?: RequestOptions) => requestList<T>(path, { ...options, method: 'GET' }),
  post: <T>(path: string, body?: unknown, options?: RequestOptions) =>
    request<T>(path, { ...options, method: 'POST', body }),
  put: <T>(path: string, body?: unknown, options?: RequestOptions) =>
    request<T>(path, { ...options, method: 'PUT', body }),
  delete: <T>(path: string, options?: RequestOptions) =>
    request<T>(path, { ...options, method: 'DELETE' }),
};

/** Encodes an S3-style object key for use in a path segment while keeping
 *  the slashes that make it a path (`{path...}` wildcard routes). */
export function encodeObjectPath(path: string): string {
  return path
    .split('/')
    .map((segment) => encodeURIComponent(segment))
    .join('/');
}

/** Downloads a URL through the authenticated client and hands the browser a
 *  Blob, so protected exports do not depend on cookie auth. */
export async function downloadFile(path: string, filename: string, options: RequestOptions = {}): Promise<void> {
  const response = await request<Response>(path, { ...options, raw: true });
  const blob = await response.blob();
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
  URL.revokeObjectURL(url);
}
