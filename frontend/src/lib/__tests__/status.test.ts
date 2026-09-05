import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
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

/**
 * The two halves, compared.
 *
 * The backend declares its vocabulary in Go; this interface classifies with
 * regular expressions. Nothing connected them, which is how "launching" came to
 * be emitted by one and unknown to the other. This reads the declaration and
 * checks every status in it against the classification here - in the language
 * the declaration is written in, because a copy of it on this side would be a
 * third source of truth.
 */
describe('the vocabulary the backend declares', () => {
  const declaration = readFileSync(
    resolve(__dirname, '../../../../backend/internal/domain/status/status.go'),
    'utf8',
  );
  const vocabulary = [...declaration.matchAll(/"([a-z][a-z-]*)":\s*Kind([A-Za-z]+),/g)].map(
    (match) => ({ name: match[1] ?? '', kind: (match[2] ?? '').toLowerCase() }),
  );

  it('is not empty, or this test checks nothing', () => {
    expect(vocabulary.length).toBeGreaterThan(8);
  });

  it('classifies every declared status the way the backend says to read it', () => {
    const expectedPending: Record<string, boolean> = {
      pending: true,
      success: false,
      failed: false,
      degraded: false,
      stopped: false,
      unknown: false,
    };
    const expectedTone: Record<string, string> = {
      pending: 'warning',
      success: 'success',
      failed: 'danger',
      // Degraded is a warning on purpose: it finished, and not as asked.
      degraded: 'warning',
      stopped: 'neutral',
      unknown: 'neutral',
    };

    for (const { name, kind } of vocabulary) {
      const described = describeStatus(name);
      expect(described.pending, `${name} (${kind}) polling`).toBe(expectedPending[kind]);
      expect(described.tone, `${name} (${kind}) tone`).toBe(expectedTone[kind]);
      // A status that falls through renders its raw backend word to a user.
      expect(described.label, `${name} must have a human label`).not.toBe(name);
    }
  });

  it('keeps polling a status it has never heard of, rather than freezing on it', () => {
    const described = describeStatus('teleporting');
    expect(described.pending).toBe(true);
  });
});
