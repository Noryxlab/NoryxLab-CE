import { describe, expect, it, vi } from 'vitest';
import { encodeObjectPath } from '../api/client';

describe('encodeObjectPath', () => {
  it('keeps path separators and encodes object-key segments', () => {
    expect(encodeObjectPath('reports/2026 annual/report.csv')).toBe('reports/2026%20annual/report.csv');
  });
});

describe('a request without a session', () => {
  // The screen said "missing bearer token" on the evening the identity
  // provider restarted: the token resolver returned null, the client sent the
  // request anyway with no Authorization header, and the API's own words went
  // to the screen. The client now refuses, and says the one thing the person
  // can act on.
  it('is refused before it leaves, with the session named and not the header', async () => {
    const { configureAuth, api, ApiError } = await import('../api/client');
    configureAuth({ getToken: () => null });

    const fetchSpy = vi.spyOn(globalThis, 'fetch');
    try {
      await expect(api.get('/api/v1/projects')).rejects.toThrowError(ApiError);
      await expect(api.get('/api/v1/projects')).rejects.toMatchObject({
        status: 401,
        code: 'session_expired',
      });
      expect(fetchSpy).not.toHaveBeenCalled();
    } finally {
      fetchSpy.mockRestore();
      configureAuth({ getToken: () => 'test-token' });
    }
  });
});
