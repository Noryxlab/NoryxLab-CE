import { describe, expect, it } from 'vitest';
import { formatDateTime, formatRelative, parseDate } from './format';

// Go writes an unset timestamp as year 1, which is a perfectly valid date and
// formatted as one: the environment list told somebody their system
// environment was built "il y a 2 055 ans".
describe('an unset timestamp', () => {
  it('is absent, not ancient', () => {
    expect(parseDate('0001-01-01T00:00:00Z')).toBeNull();
    expect(formatRelative('0001-01-01T00:00:00Z')).toBe('—');
    expect(formatDateTime('0001-01-01T00:00:00Z')).toBe('—');
  });

  it('does not swallow a real date', () => {
    expect(parseDate('2026-09-07T18:00:00Z')).not.toBeNull();
    expect(formatRelative('2026-09-07T18:00:00Z')).not.toBe('—');
  });
});
