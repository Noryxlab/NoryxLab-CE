import { describe, expect, it, beforeEach, afterEach, vi } from 'vitest';

/**
 * The runtime configuration is built by an allow-list, which is the right
 * shape - a deployment must not be able to inject arbitrary keys - and is also
 * how `extensions` was silently dropped for an entire release. It was read by
 * extensions.ts and never copied by merge(), so no extension ever loaded: the
 * Enterprise assistant and the platform-validation panel were both declared,
 * served, and never mounted.
 *
 * These tests load config.ts fresh each time, because it reads
 * window.__NORYX_CONFIG__ once at module scope.
 */
async function loadConfig(declared: unknown) {
  vi.resetModules();
  (globalThis as unknown as { window: unknown }).window = globalThis;
  (globalThis as unknown as Record<string, unknown>).__NORYX_CONFIG__ = declared;
  return (await import('../config')).config;
}

describe('runtime configuration', () => {
  const originalWindow = (globalThis as unknown as Record<string, unknown>).window;

  beforeEach(() => {
    (globalThis as unknown as Record<string, unknown>).document = {
      documentElement: { style: { setProperty: () => undefined } },
    };
  });

  afterEach(() => {
    (globalThis as unknown as Record<string, unknown>).window = originalWindow;
    delete (globalThis as unknown as Record<string, unknown>).__NORYX_CONFIG__;
  });

  it('carries declared extensions through the merge', async () => {
    const config = await loadConfig({
      edition: 'ee',
      extensions: [
        { id: 'assistant', url: '/extensions/assistant.js' },
        { id: 'platform-validation', url: '/extensions/platform-validation.js' },
      ],
    });
    expect(config.extensions).toHaveLength(2);
    expect(config.extensions[0]).toEqual({ id: 'assistant', url: '/extensions/assistant.js' });
  });

  it('defaults to no extensions rather than undefined', async () => {
    const config = await loadConfig({ edition: 'ce' });
    expect(config.extensions).toEqual([]);
  });

  // An extension runs with the user's session, so it must be served by the
  // platform itself. A cross-origin URL is dropped here, once and visibly,
  // rather than failing later inside a dynamic import.
  it('drops extensions that are not same-origin or well formed', async () => {
    const config = await loadConfig({
      extensions: [
        { id: 'evil', url: 'https://elsewhere.example/x.js' },
        { id: 'protocol-relative', url: '//elsewhere.example/x.js' },
        { id: 'no-url' },
        { url: '/extensions/no-id.js' },
        'not-an-object',
        { id: 'good', url: '/extensions/good.js' },
      ],
    });
    expect(config.extensions).toEqual([{ id: 'good', url: '/extensions/good.js' }]);
  });

  it('still merges the keys it always did', async () => {
    const config = await loadConfig({
      edition: 'ee',
      brand: { productName: 'Premyom' },
      features: { requireOrganization: true },
    });
    expect(config.edition).toBe('ee');
    expect(config.brand.productName).toBe('Premyom');
    expect(config.features.requireOrganization).toBe(true);
    // Untouched defaults survive.
    expect(config.brand.logoUrl).toBe('/favicon.svg');
  });
});
