import { describe, expect, it } from 'vitest';
import { describeStatus } from '@/components/ui/badge';

/**
 * A status the interface does not recognise is worse than an unknown label: it
 * also stops the polling that would have corrected it. "launching" was in none
 * of the lists, so a workspace screen sat on that word until the user reloaded
 * the page, while the workspace had already started.
 */
describe('status descriptors', () => {
  it('treats every in-flight status as pending, so the view keeps polling', () => {
    for (const status of [
      'launching',
      'pending',
      'creating',
      'building',
      'submitted',
      'queued',
      'scheduling',
      'initializing',
      'starting',
      'in-progress',
    ]) {
      expect(describeStatus(status).pending, `${status} must be pending`).toBe(true);
    }
  });

  it('does not keep polling a settled status', () => {
    for (const status of ['running', 'succeeded', 'failed', 'stopped', 'degraded']) {
      expect(describeStatus(status).pending, `${status} must not be pending`).toBe(false);
    }
  });

  // A degraded run completed and did not fulfil its contract. Reading as a
  // success is the failure ADR-034 was written about.
  it('never reads an incomplete result as a success', () => {
    const degraded = describeStatus('degraded');
    expect(degraded.tone).toBe('warning');
    expect(describeStatus('succeeded').tone).toBe('success');
  });
});
