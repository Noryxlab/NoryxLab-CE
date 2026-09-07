import { describe, expect, it, vi, afterEach } from 'vitest';
import { environmentsApi } from './endpoints';

// The API answers with the file and where it came from. The screen shows the
// file, and typing this call as a string put the whole envelope in the editor:
// the Dockerfile of every system environment read "[object Object]".
describe('environmentsApi.dockerfile', () => {
  afterEach(() => vi.unstubAllGlobals());

  const answer = (payload: unknown) =>
    vi.fn().mockResolvedValue(
      new Response(JSON.stringify(payload), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      }),
    );

  it('returns the file, not the envelope around it', async () => {
    vi.stubGlobal('fetch', answer({ buildId: 'system-vscode', content: 'FROM debian:12\nRUN true\n' }));
    await expect(environmentsApi.dockerfile('system-vscode')).resolves.toBe('FROM debian:12\nRUN true\n');
  });

  it('answers with an empty file rather than undefined when there is no content', async () => {
    vi.stubGlobal('fetch', answer({ buildId: 'system-vscode' }));
    await expect(environmentsApi.dockerfile('system-vscode')).resolves.toBe('');
  });
});
