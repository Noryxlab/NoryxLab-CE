import { describe, expect, it } from 'vitest';
import { accountState } from './service-accounts';

/**
 * What an administrator needs to tell apart at a glance.
 *
 * "Never used" is a state of its own, not an empty cell: it is the one that
 * says a credential was created and forgotten. Seven backup-runner tokens sat
 * on the DC in exactly that state - one per redeploy, none revoked - and no
 * screen existed to show it.
 */
const now = new Date('2026-09-27T12:00:00Z');
const base = { id: 't', userId: '', name: 'backup', createdAt: '2026-09-01T00:00:00Z' };

describe('accountState', () => {
  it('names a credential nobody ever presented', () => {
    expect(accountState({ ...base }, now)).toBe('idle');
  });

  it('calls a used credential active', () => {
    expect(accountState({ ...base, lastUsedAt: '2026-09-26T09:00:00Z' }, now)).toBe('active');
  });

  it('prefers expired over used, because an expired token no longer works', () => {
    expect(
      accountState({ ...base, lastUsedAt: '2026-09-26T09:00:00Z', expiresAt: '2026-09-26T10:00:00Z' }, now),
    ).toBe('expired');
  });

  it('prefers revoked over everything, because revocation is a decision', () => {
    expect(
      accountState(
        { ...base, lastUsedAt: '2026-09-26T09:00:00Z', expiresAt: '2027-01-01T00:00:00Z', revokedAt: '2026-09-26T11:00:00Z' },
        now,
      ),
    ).toBe('revoked');
  });
});
