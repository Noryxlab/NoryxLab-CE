import { describe, expect, it, vi, beforeEach, afterEach } from 'vitest';
import { appsApi, jobsApi, datasourcesApi } from '../api/endpoints';

/**
 * Every log endpoint answers with an envelope - `{appId, podName, logs}` - and
 * the client asked for it as a bare string. The viewer then received the
 * object and threw on `content.split('\n')`, so the panel rendered nothing:
 * the logs looked missing while the request had succeeded.
 */
describe('log endpoints', () => {
  beforeEach(() => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        new Response(JSON.stringify({ appId: 'a', podName: 'p', logs: 'line one\nline two' }), {
          status: 200,
          headers: { 'content-type': 'application/json' },
        }),
      ),
    );
  });
  afterEach(() => vi.unstubAllGlobals());

  it('hands the viewer the text and not the envelope', async () => {
    for (const call of [
      () => appsApi.logs('app-1'),
      () => jobsApi.logs('job-1'),
      () => datasourcesApi.logs('ds-1'),
    ]) {
      const logs = await call();
      expect(typeof logs).toBe('string');
      expect(logs.split('\n')).toHaveLength(2);
    }
  });
});
