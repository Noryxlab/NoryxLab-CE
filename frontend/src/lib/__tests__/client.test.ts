import { describe, expect, it } from 'vitest';
import { encodeObjectPath } from '../api/client';

describe('encodeObjectPath', () => {
  it('keeps path separators and encodes object-key segments', () => {
    expect(encodeObjectPath('reports/2026 annual/report.csv')).toBe('reports/2026%20annual/report.csv');
  });
});
