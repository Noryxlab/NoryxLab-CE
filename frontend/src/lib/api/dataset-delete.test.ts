import { describe, expect, it, vi, beforeEach } from 'vitest';

/**
 * Deleting a folder has to delete the folder.
 *
 * The call went out without `recursive`, so the server removed a single key -
 * and a folder is usually only a prefix inferred from the objects beneath it.
 * S3 answers success for deleting a key that never existed, so the screen
 * reported a deletion, cleared the selection, refreshed the listing, and every
 * file was still there. On `hds-for` that was 3 234 objects and 50 GiB
 * surviving a confirmed delete, on a bucket holding health data.
 */
const calls: string[] = [];
vi.mock('./client', async (importOriginal) => ({
  ...(await importOriginal<typeof import('./client')>()),
  api: {
    get: vi.fn(), post: vi.fn(), put: vi.fn(), patch: vi.fn(), list: vi.fn(),
    delete: vi.fn((path: string) => { calls.push(path); return Promise.resolve(); }),
  },
}));

const { datasetsApi } = await import('./endpoints');

describe('deleteObject', () => {
  beforeEach(() => { calls.length = 0; });

  it('asks for a recursive delete on a folder', async () => {
    await datasetsApi.deleteObject('ds-1', 'old', true);
    expect(calls[0]).toContain('?recursive=true');
  });

  it('leaves a single file alone: one key, no sweep of its neighbours', async () => {
    await datasetsApi.deleteObject('ds-1', 'PREMYOM1000/subject/scan.dcm');
    expect(calls[0]).not.toContain('recursive');
  });

  it('defaults to the safe shape when nobody says', async () => {
    await datasetsApi.deleteObject('ds-1', 'whatever');
    expect(calls[0]).not.toContain('recursive');
  });
});
